package biz

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"strings"
	"text/template"
	"time"

	"github.com/samber/lo"

	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/pkg/xerrors"
	"github.com/looplj/axonhub/internal/server/biz/provider_quota"
	"github.com/looplj/axonhub/internal/server/events"
	"github.com/looplj/axonhub/llm/httpclient"
)

const EventChannelAutoDisabled = "channel.auto_disabled"

const (
	EventProviderQuotaWarning   = "provider.quota.warning"
	EventProviderQuotaExhausted = "provider.quota.exhausted"
	EventProviderQuotaRecovered = "provider.quota.recovered"
)

// QuotaEventSeverity maps quota status-change events to their webhook severity.
var QuotaEventSeverity = map[string]string{
	EventProviderQuotaWarning:   "warning",
	EventProviderQuotaExhausted: "critical",
	EventProviderQuotaRecovered: "info",
}

const (
	defaultWebhookMethod    = http.MethodPost
	defaultWebhookTimeoutMs = 3000
)

type ChannelAutoDisabledEvent struct {
	ChannelID       int
	ChannelName     string
	ChannelProvider string
	ChannelBaseURL  string
	ChannelStatus   string
	StatusCode      int
	Threshold       int
	ActualCount     int
	Reason          string
	OccurredAt      time.Time
}

type QuotaStatusChangedEvent struct {
	ChannelID       int
	ChannelName     string
	ChannelProvider string // ch.Type.String()
	ChannelBaseURL  string
	ChannelStatus   string  // ch.Status.String()
	PreviousStatus  string  // raw previous state before the save ("available"|"warning"|"exhausted"|"unknown"); transition logic used the last known non-unknown status
	NewStatus       string  // "warning"|"exhausted"|"available"
	UsageRatio      float64 // overall usage ratio; -1 when not derivable
	NextResetAt     string  // RFC3339, "" when nil
	Limits          []string
	OccurredAt      time.Time
}

type WebhookRenderContext struct {
	Event    string `json:"event"`
	Severity string `json:"severity"`

	// OccurredAt is RFC3339 in the configured system timezone, so it carries that
	// zone's offset rather than a trailing Z. It is set by notify.
	OccurredAt string `json:"occurred_at"`

	Channel struct {
		ID       int    `json:"id"`
		Name     string `json:"name"`
		Provider string `json:"provider"`
		BaseURL  string `json:"base_url"`
		Status   string `json:"status"`
	} `json:"channel"`

	Trigger struct {
		Type        string `json:"type"`
		StatusCode  int    `json:"status_code"`
		Threshold   int    `json:"threshold"`
		ActualCount int    `json:"actual_count"`
		Reason      string `json:"reason"`
	} `json:"trigger"`

	Quota struct {
		PreviousStatus        string   `json:"previous_status"`
		NewStatus             string   `json:"new_status"`
		UsageRatio            float64  `json:"usage_ratio"`
		UsagePercent          int      `json:"usage_percent"`
		NextResetAt           string   `json:"next_reset_at"`
		Limits                []string `json:"limits"`
		WarningThresholdRatio float64  `json:"warning_threshold_ratio"`
	} `json:"quota"`

	// Data is the plugin webhook payload. NotifyXxx leaves it nil.
	Data any `json:"data,omitempty"`
	// Message is payload.message when that value is a string.
	Message string `json:"message,omitempty"`
}

var (
	// ErrWebhookTargetNotFound means the configured target list has no such name.
	ErrWebhookTargetNotFound = errors.New("webhook target not found")
	// ErrWebhookTargetDisabled means the target is turned off or has no URL.
	ErrWebhookTargetDisabled = errors.New("webhook target is disabled")
	// ErrWebhookUnavailable means the single delivery attempt failed.
	ErrWebhookUnavailable = errors.New("webhook delivery failed")
)

type WebhookNotifier struct {
	SystemService *SystemService
	httpClient    *httpclient.HttpClient
}

func NewWebhookNotifier(systemService *SystemService, httpClient *httpclient.HttpClient) *WebhookNotifier {
	return &WebhookNotifier{
		SystemService: systemService,
		httpClient:    httpClient,
	}
}

func (n *WebhookNotifier) NotifyChannelAutoDisabled(ctx context.Context, event ChannelAutoDisabledEvent) {
	log.Info(ctx, "notify channel auto disabled", log.Any("event", event))

	renderCtx := WebhookRenderContext{
		Event:    EventChannelAutoDisabled,
		Severity: "warning",
	}
	renderCtx.Channel.ID = event.ChannelID
	renderCtx.Channel.Name = event.ChannelName
	renderCtx.Channel.Provider = event.ChannelProvider
	renderCtx.Channel.BaseURL = event.ChannelBaseURL
	renderCtx.Channel.Status = event.ChannelStatus
	renderCtx.Trigger.Type = "error_status_rule"
	renderCtx.Trigger.StatusCode = event.StatusCode
	renderCtx.Trigger.Threshold = event.Threshold
	renderCtx.Trigger.ActualCount = event.ActualCount
	renderCtx.Trigger.Reason = event.Reason

	n.notify(ctx, EventChannelAutoDisabled, renderCtx, event.OccurredAt)
}

func (n *WebhookNotifier) NotifyProviderQuotaWarning(ctx context.Context, event QuotaStatusChangedEvent) {
	n.notifyProviderQuotaEvent(ctx, EventProviderQuotaWarning, event)
}

func (n *WebhookNotifier) NotifyProviderQuotaExhausted(ctx context.Context, event QuotaStatusChangedEvent) {
	n.notifyProviderQuotaEvent(ctx, EventProviderQuotaExhausted, event)
}

func (n *WebhookNotifier) NotifyProviderQuotaRecovered(ctx context.Context, event QuotaStatusChangedEvent) {
	n.notifyProviderQuotaEvent(ctx, EventProviderQuotaRecovered, event)
}

func (n *WebhookNotifier) notifyProviderQuotaEvent(ctx context.Context, eventName string, event QuotaStatusChangedEvent) {
	log.Info(ctx, "notify provider quota status change",
		log.Int("channel_id", event.ChannelID),
		log.String("event", eventName),
		log.String("previous_status", event.PreviousStatus),
		log.String("new_status", event.NewStatus),
	)

	usagePercent := 0
	if event.UsageRatio >= 0 {
		usagePercent = int(math.Round(event.UsageRatio * 100))
	}

	renderCtx := WebhookRenderContext{
		Event:    eventName,
		Severity: QuotaEventSeverity[eventName],
	}
	renderCtx.Channel.ID = event.ChannelID
	renderCtx.Channel.Name = event.ChannelName
	renderCtx.Channel.Provider = event.ChannelProvider
	renderCtx.Channel.BaseURL = event.ChannelBaseURL
	renderCtx.Channel.Status = event.ChannelStatus
	renderCtx.Quota.PreviousStatus = event.PreviousStatus
	renderCtx.Quota.NewStatus = event.NewStatus
	renderCtx.Quota.UsageRatio = event.UsageRatio
	renderCtx.Quota.UsagePercent = usagePercent
	renderCtx.Quota.NextResetAt = event.NextResetAt
	renderCtx.Quota.Limits = event.Limits
	renderCtx.Quota.WarningThresholdRatio = provider_quota.WarningThresholdRatio

	n.notify(ctx, eventName, renderCtx, event.OccurredAt)
}

// notify renders and delivers one event to every subscribed target.
//
// occurredAt is formatted here rather than by the caller because resolving the
// configured timezone reads system settings, which the ent privacy policy denies
// on a context without a user. The system bypass below is what makes that read
// succeed, so the formatting has to happen after it.
func (n *WebhookNotifier) notify(
	ctx context.Context,
	eventName string,
	renderCtx WebhookRenderContext,
	occurredAt time.Time,
) {
	ctx = context.WithoutCancel(ctx)
	renderCtx.OccurredAt = occurredAt.In(n.SystemService.TimeLocation(ctx)).Format(time.RFC3339)
	cfg := *n.SystemService.WebhookNotifierConfigOrDefault(ctx)
	targets := n.selectTargets(cfg, eventName)

	log.Debug(ctx, "notify webhook",
		log.Any("cfg", cfg),
		log.String("event_name", eventName),
		log.Any("targets", targets),
	)

	if len(targets) == 0 {
		return
	}

	for _, target := range targets {
		body, err := renderWebhookTemplate(target.Body, renderCtx)
		if err != nil {
			log.Warn(ctx, "failed to render webhook body template",
				log.String("event", eventName),
				log.String("target", target.Name),
				log.Cause(err),
			)

			continue
		}

		headers, err := renderWebhookHeaders(target.Headers, renderCtx)
		if err != nil {
			log.Warn(ctx, "failed to render webhook headers",
				log.String("event", eventName),
				log.String("target", target.Name),
				log.Cause(err),
			)

			continue
		}

		if err := n.send(ctx, target, body, headers); err != nil {
			log.Warn(ctx, "failed to send webhook notification",
				log.String("event", eventName),
				log.String("target", target.Name),
				log.Cause(err),
			)
		}
	}
}

func (n *WebhookNotifier) selectTargets(cfg WebhookNotifierConfig, eventName string) []WebhookTarget {
	subscription, ok := lo.Find(cfg.Subscriptions, func(item WebhookSubscription) bool {
		return item.Event == eventName
	})
	if !ok {
		return nil
	}

	names := subscription.TargetNames
	if len(names) == 0 || len(cfg.Targets) == 0 {
		return nil
	}

	targets := make([]WebhookTarget, 0, len(names))
	for _, name := range names {
		target, ok := lo.Find(cfg.Targets, func(item WebhookTarget) bool {
			return item.Name == name
		})
		if !ok || !target.Enabled || strings.TrimSpace(target.URL) == "" {
			continue
		}

		targets = append(targets, target)
	}

	return targets
}

func (n *WebhookNotifier) send(ctx context.Context, target WebhookTarget, body string, headers http.Header) error {
	timeout := time.Duration(target.TimeoutMs) * time.Millisecond
	if timeout <= 0 {
		timeout = defaultWebhookTimeoutMs * time.Millisecond
	}

	reqCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	if headers.Get("Content-Type") == "" {
		headers.Set("Content-Type", "application/json")
	}

	client := n.httpClient
	if target.Proxy != nil {
		client = client.WithProxy(target.Proxy)
	}

	_, err := client.Do(reqCtx, &httpclient.Request{
		Method:      defaultWebhookMethod,
		URL:         target.URL,
		Headers:     headers,
		ContentType: headers.Get("Content-Type"),
		Body:        []byte(body),
	})
	if err != nil {
		return fmt.Errorf("do request: %w", err)
	}

	return nil
}

// SendToTarget renders one configured target and delivers it once.
// payload is the template Data value. A string payload.message also sets Message.
// The current event supplies Event, Severity, OccurredAt, and Channel.
// A missing target is ErrWebhookTargetNotFound. A disabled target, or one with a
// blank URL, is ErrWebhookTargetDisabled. Transport errors and non-2xx responses
// are ErrWebhookUnavailable. This method does not retry.
func (n *WebhookNotifier) SendToTarget(ctx context.Context, name string, payload map[string]any, event events.Event) error {
	if n == nil || n.SystemService == nil || n.httpClient == nil {
		return fmt.Errorf("%w: notifier is not configured", ErrWebhookUnavailable)
	}
	if ctx == nil {
		ctx = context.Background()
	}
	target, ok := webhookTargetByName(n.SystemService.WebhookNotifierConfigOrDefault(ctx), name)
	if !ok {
		return ErrWebhookTargetNotFound
	}
	if !target.Enabled || strings.TrimSpace(target.URL) == "" {
		return ErrWebhookTargetDisabled
	}

	renderCtx := WebhookRenderContext{
		Event:    event.Type,
		Severity: event.Severity,
		Data:     payload,
	}
	if message, ok := payload["message"].(string); ok {
		renderCtx.Message = message
	}
	occurredAt := event.OccurredAt
	if occurredAt.IsZero() {
		occurredAt = time.Now()
	}
	renderCtx.OccurredAt = occurredAt.In(n.SystemService.TimeLocation(ctx)).Format(time.RFC3339)
	if event.Subject.Channel != nil {
		renderCtx.Channel.ID = event.Subject.Channel.ID
		renderCtx.Channel.Name = event.Subject.Channel.Name
		renderCtx.Channel.Provider = event.Subject.Channel.Type
		renderCtx.Channel.BaseURL = event.Subject.Channel.BaseURL
		renderCtx.Channel.Status = event.Subject.Channel.Status
	}

	body, err := renderWebhookTemplate(target.Body, renderCtx)
	if err != nil {
		return fmt.Errorf("%w: render body: %w", ErrWebhookUnavailable, err)
	}
	headers, err := renderWebhookHeaders(target.Headers, renderCtx)
	if err != nil {
		return fmt.Errorf("%w: render headers: %w", ErrWebhookUnavailable, err)
	}
	if err := n.sendOnce(ctx, target, body, headers); err != nil {
		if xerrors.Is(err, context.Canceled) || xerrors.Is(err, context.DeadlineExceeded) {
			return err
		}
		return fmt.Errorf("%w: %w", ErrWebhookUnavailable, err)
	}
	return nil
}

func webhookTargetByName(cfg *WebhookNotifierConfig, name string) (WebhookTarget, bool) {
	if cfg == nil {
		return WebhookTarget{}, false
	}
	for _, target := range cfg.Targets {
		if target.Name == name {
			return target, true
		}
	}
	return WebhookTarget{}, false
}

// sendOnce delivers one request. A non-2xx status is an error. It does not retry.
func (n *WebhookNotifier) sendOnce(ctx context.Context, target WebhookTarget, body string, headers http.Header) error {
	timeout := time.Duration(target.TimeoutMs) * time.Millisecond
	if timeout <= 0 {
		timeout = defaultWebhookTimeoutMs * time.Millisecond
	}

	reqCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	if headers.Get("Content-Type") == "" {
		headers.Set("Content-Type", "application/json")
	}

	client := n.httpClient
	if target.Proxy != nil {
		client = client.WithProxy(target.Proxy)
	}

	resp, err := client.Do(reqCtx, &httpclient.Request{
		Method:      defaultWebhookMethod,
		URL:         target.URL,
		Headers:     headers,
		ContentType: headers.Get("Content-Type"),
		Body:        []byte(body),
	})
	if err != nil {
		return err
	}
	if resp == nil || resp.StatusCode < http.StatusOK || resp.StatusCode >= 300 {
		status := 0
		if resp != nil {
			status = resp.StatusCode
		}
		return fmt.Errorf("webhook status %d", status)
	}
	return nil
}

func renderWebhookHeaders(headers []objects.HeaderEntry, renderCtx WebhookRenderContext) (http.Header, error) {
	result := make(http.Header, len(headers))
	for _, header := range headers {
		key := strings.TrimSpace(header.Key)
		if key == "" {
			continue
		}

		value, err := renderWebhookTemplate(header.Value, renderCtx)
		if err != nil {
			return nil, fmt.Errorf("header %q: %w", key, err)
		}

		result.Set(key, value)
	}

	return result, nil
}

// webhookTemplateFuncs makes values render as compact JSON so the default
// body template stays valid JSON (e.g. "limits": {{.Quota.Limits | json}}).
var webhookTemplateFuncs = template.FuncMap{
	"json": func(v any) string {
		b, err := json.Marshal(v)
		if err != nil {
			return "null"
		}
		return string(b)
	},
}

func renderWebhookTemplate(value string, renderCtx WebhookRenderContext) (string, error) {
	if !strings.Contains(value, "{{") || !strings.Contains(value, "}}") {
		return value, nil
	}

	tmpl, err := template.New("webhook").Funcs(webhookTemplateFuncs).Parse(value)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, renderCtx); err != nil {
		return "", err
	}

	return buf.String(), nil
}
