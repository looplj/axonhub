// 职责: 管理 OpenAI 翻译会话的请求预约、音频时长计量和审计。
// 关联文件:
//   internal/server/orchestrator/realtime.go
//   internal/server/biz/realtime_translation_billing.go
//   llm/transformer/openai/realtime/translation.go
// 注意事项: 只统计成功写到上游的音频，不保存媒体；上游不回传账单时长。

package orchestrator

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/tidwall/gjson"

	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/request"
	"github.com/looplj/axonhub/internal/ent/usagelog"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/server/biz"
	rt "github.com/looplj/axonhub/internal/server/realtime"
	"github.com/looplj/axonhub/llm"
	openairt "github.com/looplj/axonhub/llm/transformer/openai/realtime"
)

type translationAccounting struct {
	audit       *realtimeResponse
	sentSamples atomic.Int64
	closed      bool
	finalized   bool
}

func (c *RealtimeConversation) translationClient(data []byte) ([]byte, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	wire, err := c.adapter.Client(data)
	if err != nil || gjson.GetBytes(wire, "type").String() != "session.input_audio_buffer.append" {
		return wire, err
	}
	if c.translation.audit == nil {
		c.checkMu.Lock()
		key := c.latestKey
		c.checkMu.Unlock()
		if key == nil {
			key = c.target.APIKey
		}
		ctx, cancel := context.WithTimeout(c.ctx, 5*time.Second)
		defer cancel()
		if err := c.service.checkQuota(ctx, key); err != nil {
			return nil, err
		}
		audit, err := c.createResponse("translation")
		if err != nil {
			return nil, rt.ErrAccountingUnavailable
		}
		c.translation.audit = audit
	}
	return wire, nil
}

// UpstreamWritten is called after a successful WebSocket write.
func (c *RealtimeConversation) UpstreamWritten(data []byte) {
	kind := gjson.GetBytes(data, "type").String()
	if kind == "input_audio_buffer.append" || kind == "session.input_audio_buffer.append" {
		if count := realtimePCMBytes(gjson.GetBytes(data, "audio").String()); count > 0 {
			c.audioInBytes.Add(count)
		}
	}
	if c.translation != nil && kind == "session.input_audio_buffer.append" {
		if samples, err := openairt.TranslationAudioSamples(data); err == nil {
			c.translation.sentSamples.Add(samples)
		}
	}
}

func (c *RealtimeConversation) ServerDone(data []byte) bool {
	return c.translation != nil && gjson.GetBytes(data, "type").String() == "session.closed"
}

func (c *RealtimeConversation) checkTranslationQuota(ctx context.Context, key *ent.APIKey) error {
	profile := key.GetActiveProfile()
	if c.target.Price == nil || profile == nil || profile.Quota == nil || profile.Quota.Cost == nil {
		return c.service.checkQuota(ctx, key, false)
	}
	cost, err := biz.RealtimeTranslationCost(c.target.Price, c.translation.sentSamples.Load())
	if err != nil {
		return rt.ErrAccountingUnavailable
	}
	quota := *profile.Quota
	quota.Requests = nil
	remaining := quota.Cost.Sub(cost)
	quota.Cost = &remaining
	result, err := c.service.Quota.CheckAPIKeyQuota(ctx, key.ID, &quota)
	if err != nil {
		return rt.ErrAccountingUnavailable
	}
	if !result.Allowed {
		return rt.ErrQuotaExceeded
	}
	return nil
}

func (c *RealtimeConversation) closeTranslation(ctx context.Context) {
	t := c.translation
	if t.audit == nil || t.finalized {
		return
	}
	t.finalized = true
	samples := t.sentSamples.Load()
	projectID, _ := contexts.GetProjectID(ctx)
	summary := c.withPriceAudit(map[string]any{
		"realtime": true, "translation": true, "audio_samples_sent": samples,
		"audio_sample_rate_hz": 24000, "usage_confirmed": false,
		"metering_source": "gateway_sent_pcm", "provider_session_closed": t.closed,
		"upstream_model": c.target.Model,
	})
	status := request.StatusFailed
	if samples > 0 {
		_, err := c.service.Usage.CreateRealtimeTranslationUsage(ctx, biz.CreateUsageLogParams{
			RequestID: t.audit.request.ID, ProjectID: projectID, ChannelID: c.target.Channel.ID,
			ActualModelID: c.target.Model, Source: usagelog.SourceAPI, Format: llm.APIFormatOpenAIRealtime.String(),
			APIKeyID: &c.target.APIKey.ID,
		}, c.target.Price, samples)
		if err != nil {
			log.Warn(ctx, "Realtime translation usage persistence failed", log.Int("request_id", t.audit.request.ID))
		} else if t.closed {
			status = request.StatusCompleted
			if c.credential != "" {
				c.service.Channels.RecordRealtimeCredentialSuccess(c.target.Channel.ID, c.credential)
			}
		}
	}
	if status == request.StatusFailed {
		summary["error_code"] = c.closeAuditErrorCode()
	}
	latency := time.Since(t.audit.started).Milliseconds()
	if err := c.service.Requests.UpdateRealtimeAudit(ctx, t.audit.request, status, "", summary, &biz.LatencyMetrics{LatencyMs: &latency}, nil); err != nil {
		log.Warn(ctx, "Realtime translation audit update failed", log.Int("request_id", t.audit.request.ID))
	}
}
