package orchestrator

import (
	"context"
	"errors"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/tidwall/gjson"

	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/internal/server/events"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
)

const eventErrorMessageMaxBytes = 512

func publishEvent(ctx context.Context, bus *events.Bus, eventType, severity string, build func() (events.Subject, map[string]any)) {
	if bus == nil || !bus.HasSubscribers(eventType) {
		return
	}

	subject, data := build()
	bus.Publish(ctx, events.Event{
		ID:         uuid.NewString(),
		Type:       eventType,
		Severity:   severity,
		OccurredAt: time.Now().UTC(),
		Subject:    subject,
		Data:       data,
		Origin:     "",
		Depth:      0,
	})
}

func (p *PersistentOutboundTransformer) publishRequestFailover(
	ctx context.Context,
	left *biz.Channel,
	model string,
	attempt int,
	to *biz.Channel,
) {
	state := (*PersistenceState)(nil)
	if p != nil {
		state = p.state
	}
	publishEvent(ctx, eventBus(state), events.TypeRequestFailover, events.SeverityWarning, func() (events.Subject, map[string]any) {
		toID := 0
		toName := ""
		if to != nil && to.Channel != nil {
			toID = to.ID
			toName = to.Name
		}
		if model == "" && state != nil {
			model = state.OriginalModel
			if model == "" && state.Request != nil {
				model = state.Request.ModelID
			}
		}

		return requestSubjectRefs(state, model, bizChannelRef(left), nil), map[string]any{
			"toChannelId":   toID,
			"toChannelName": toName,
			"attempt":       attempt,
		}
	})
}

func publishRequestAttemptFailed(ctx context.Context, state *PersistenceState, err error) {
	publishEvent(ctx, eventBus(state), events.TypeRequestAttemptFailed, events.SeverityWarning, func() (events.Subject, map[string]any) {
		ch := currentBizChannel(state)
		oauth, key := upstreamCredentialKey(ctx, ch, statePerf(state))
		statusCode, errorType, errorMessage := eventErrorFields(err)

		return requestSubjectRefs(state, eventModel(state), bizChannelRef(ch), credentialEventRef(oauth, key)), map[string]any{
			"attempt":      outboundAttempts(state),
			"statusCode":   statusCode,
			"errorType":    errorType,
			"errorMessage": errorMessage,
		}
	})
}

func publishRequestFailed(ctx context.Context, state *PersistenceState, err error) {
	if state == nil {
		return
	}
	state.requestFailedEvent.Do(func() {
		publishEvent(ctx, eventBus(state), events.TypeRequestFailed, events.SeverityError, func() (events.Subject, map[string]any) {
			statusCode, errorType, errorMessage := eventErrorFields(err)

			return requestSubjectRefs(state, eventModel(state), bizChannelRef(currentBizChannel(state)), nil), map[string]any{
				"statusCode":   statusCode,
				"errorType":    errorType,
				"errorMessage": errorMessage,
				"attempts":     outboundAttempts(state),
			}
		})
	})
}

func publishRequestCompleted(ctx context.Context, state *PersistenceState, usage *llm.Usage, stream bool, metrics *biz.LatencyMetrics) {
	publishEvent(ctx, eventBus(state), events.TypeRequestCompleted, events.SeverityInfo, func() (events.Subject, map[string]any) {
		promptTokens := int64(0)
		completionTokens := int64(0)
		if usage != nil {
			promptTokens = usage.PromptTokens
			completionTokens = usage.CompletionTokens
		}
		latency := requestLatencyMs(state)
		if metrics != nil && metrics.LatencyMs != nil {
			latency = *metrics.LatencyMs
		}

		return requestSubjectRefs(state, eventModel(state), bizChannelRef(currentBizChannel(state)), nil), map[string]any{
			"latencyMs":        latency,
			"promptTokens":     promptTokens,
			"completionTokens": completionTokens,
			"attempts":         outboundAttempts(state),
			"stream":           stream,
		}
	})
}

func (p *PersistentInboundTransformer) publishAPIKeyQuotaExceeded(
	ctx context.Context,
	llmRequest *llm.Request,
	profile string,
	window biz.QuotaWindow,
) {
	if p == nil || p.state == nil {
		return
	}
	state := p.state
	publishEvent(ctx, eventBus(state), events.TypeAPIKeyQuotaExceeded, events.SeverityWarning, func() (events.Subject, map[string]any) {
		model := ""
		if llmRequest != nil {
			model = llmRequest.Model
		}
		return quotaSubject(state.APIKey, state.Request, model), map[string]any{
			"profile": profile,
			"window":  quotaWindowData(window),
		}
	})
}

func eventBus(state *PersistenceState) *events.Bus {
	if state == nil {
		return nil
	}

	return state.EventBus
}

func (p *PersistentOutboundTransformer) channelBeingLeft() (channel *biz.Channel, model string, attempt int) {
	if p == nil || p.state == nil {
		return nil, "", 0
	}

	state := p.state
	attempt = state.OutboundAttempt
	if attempt <= 0 && state.CurrentCandidateIndex >= 0 {
		attempt = state.CurrentCandidateIndex + 1
	}
	if state.CurrentCandidateIndex >= 0 && state.CurrentCandidateIndex < len(state.ChannelModelsCandidates) {
		candidate := state.ChannelModelsCandidates[state.CurrentCandidateIndex]
		if candidate != nil {
			channel = candidate.Channel
			if state.CurrentModelIndex >= 0 && state.CurrentModelIndex < len(candidate.Models) {
				model = candidate.Models[state.CurrentModelIndex].ActualModel
			}
		}
	}
	if channel == nil {
		channel = p.GetCurrentChannel()
	}
	if model == "" && state.CurrentCandidate != nil &&
		state.CurrentModelIndex >= 0 &&
		state.CurrentModelIndex < len(state.CurrentCandidate.Models) {
		model = state.CurrentCandidate.Models[state.CurrentModelIndex].ActualModel
	}
	if model == "" {
		model = state.OriginalModel
	}

	return channel, model, attempt
}

func currentBizChannel(state *PersistenceState) *biz.Channel {
	if state == nil || state.CurrentCandidate == nil {
		return nil
	}

	return state.CurrentCandidate.Channel
}

func statePerf(state *PersistenceState) *biz.PerformanceRecord {
	if state == nil {
		return nil
	}

	return state.Perf
}

func eventModel(state *PersistenceState) string {
	if state == nil {
		return ""
	}
	if state.CurrentCandidate != nil && state.CurrentModelIndex >= 0 && state.CurrentModelIndex < len(state.CurrentCandidate.Models) {
		if model := state.CurrentCandidate.Models[state.CurrentModelIndex].ActualModel; model != "" {
			return model
		}
	}
	if state.OriginalModel != "" {
		return state.OriginalModel
	}
	if state.LlmRequest != nil && state.LlmRequest.Model != "" {
		return state.LlmRequest.Model
	}
	if state.Request != nil {
		return state.Request.ModelID
	}

	return ""
}

func requestSubjectRefs(state *PersistenceState, model string, channel *events.ChannelRef, credential *events.CredentialRef) events.Subject {
	subject := events.Subject{
		Channel:    channel,
		Credential: credential,
		Model:      model,
	}
	if state == nil {
		return subject
	}
	if state.APIKey != nil {
		subject.APIKey = &events.APIKeyRef{ID: state.APIKey.ID, Name: state.APIKey.Name}
		if state.APIKey.ProjectID != 0 {
			subject.Project = &events.ProjectRef{ID: state.APIKey.ProjectID}
		}
	}
	if state.Request != nil {
		subject.Request = &events.RequestRef{ID: state.Request.ID}
		if subject.Project == nil && state.Request.ProjectID != 0 {
			subject.Project = &events.ProjectRef{ID: state.Request.ProjectID}
		}
	}

	return subject
}

func quotaSubject(apiKey *ent.APIKey, request *ent.Request, model string) events.Subject {
	subject := events.Subject{Model: model}
	if apiKey != nil {
		subject.APIKey = &events.APIKeyRef{ID: apiKey.ID, Name: apiKey.Name}
		if apiKey.ProjectID != 0 {
			subject.Project = &events.ProjectRef{ID: apiKey.ProjectID}
		}
	}
	if request != nil {
		subject.Request = &events.RequestRef{ID: request.ID}
		if subject.Project == nil && request.ProjectID != 0 {
			subject.Project = &events.ProjectRef{ID: request.ProjectID}
		}
	}

	return subject
}

func bizChannelRef(ch *biz.Channel) *events.ChannelRef {
	if ch == nil || ch.Channel == nil {
		return nil
	}

	return &events.ChannelRef{
		ID:      ch.ID,
		Name:    ch.Name,
		Type:    ch.Type.String(),
		BaseURL: ch.BaseURL,
		Status:  ch.Status.String(),
	}
}

// credentialEventRef identifies a credential without its plaintext.
// OAuth has no key string: both fields are events.OAuthFingerprint, and the
// token is not hashed.
func credentialEventRef(oauth bool, key string) *events.CredentialRef {
	if oauth || key == objects.OAuthCredentialRef {
		return &events.CredentialRef{
			Fingerprint: events.OAuthFingerprint,
			Masked:      events.OAuthFingerprint,
		}
	}
	if key == "" {
		return nil
	}

	return &events.CredentialRef{
		Fingerprint: events.Fingerprint(key),
		Masked:      events.Mask(key),
	}
}

func upstreamCredentialKey(ctx context.Context, ch *biz.Channel, perf *biz.PerformanceRecord) (oauth bool, key string) {
	if ch != nil && ch.Channel != nil && ch.Credentials.IsOAuth() {
		return true, objects.OAuthCredentialRef
	}
	if ctx != nil {
		if apiKey, ok := contexts.GetChannelAPIKey(ctx); ok && apiKey != "" {
			if apiKey == objects.OAuthCredentialRef {
				return true, apiKey
			}
			return false, apiKey
		}
	}
	if perf != nil && perf.APIKey != "" {
		if perf.APIKey == objects.OAuthCredentialRef {
			return true, perf.APIKey
		}
		return false, perf.APIKey
	}

	return false, ""
}

func outboundAttempts(state *PersistenceState) int {
	if state == nil || state.OutboundAttempt < 0 {
		return 0
	}

	return state.OutboundAttempt
}

func requestLatencyMs(state *PersistenceState) int64 {
	if state == nil || state.Perf == nil || state.Perf.StartTime.IsZero() {
		return 0
	}
	_, latency, _ := state.Perf.Calculate()

	return latency
}

func quotaWindowData(window biz.QuotaWindow) map[string]any {
	return map[string]any{
		"start":        rfc3339OrNil(window.Start),
		"end":          rfc3339OrNil(window.End),
		"endInclusive": window.EndInclusive,
	}
}

func rfc3339OrNil(value *time.Time) any {
	if value == nil {
		return nil
	}

	return value.UTC().Format(time.RFC3339)
}

func eventErrorFields(err error) (statusCode int, errorType, errorMessage string) {
	if err == nil {
		return 0, "", ""
	}

	return ExtractStatusCodeFromError(err), eventErrorType(err), truncateEventMessage(ExtractErrorMessage(err))
}

func eventErrorType(err error) string {
	if err == nil {
		return ""
	}

	var respErr *llm.ResponseError
	if errors.As(err, &respErr) && respErr != nil && respErr.Detail.Type != "" {
		return respErr.Detail.Type
	}

	var httpErr *httpclient.Error
	if errors.As(err, &httpErr) && httpErr != nil {
		if errorType := gjson.GetBytes(httpErr.Body, "error.type").String(); errorType != "" {
			return errorType
		}
	}

	return ""
}

func truncateEventMessage(message string) string {
	if len(message) <= eventErrorMessageMaxBytes {
		return message
	}

	cut := message[:eventErrorMessageMaxBytes]
	for len(cut) > 0 && !utf8.ValidString(cut) {
		cut = cut[:len(cut)-1]
	}

	return cut
}
