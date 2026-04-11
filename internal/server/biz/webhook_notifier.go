package biz

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"strings"
	"text/template"
	"time"

	"github.com/samber/lo"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/llm/httpclient"
)

const EventChannelAutoDisabled = "channel.auto_disabled"

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

type WebhookRenderContext struct {
	Event      string `json:"event"`
	Severity   string `json:"severity"`
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
}

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
		Event:      EventChannelAutoDisabled,
		Severity:   "warning",
		OccurredAt: event.OccurredAt.UTC().Format(time.RFC3339),
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

	n.notify(ctx, EventChannelAutoDisabled, renderCtx)
}

func (n *WebhookNotifier) notify(ctx context.Context, eventName string, renderCtx WebhookRenderContext) {
	ctx = authz.WithSystemBypass(context.WithoutCancel(ctx), "webhook-notifier")
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

func renderWebhookTemplate(value string, renderCtx WebhookRenderContext) (string, error) {
	if !strings.Contains(value, "{{") || !strings.Contains(value, "}}") {
		return value, nil
	}

	tmpl, err := template.New("webhook").Funcs(template.FuncMap{}).Parse(value)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, renderCtx); err != nil {
		return "", err
	}

	return buf.String(), nil
}
