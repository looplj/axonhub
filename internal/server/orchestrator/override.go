package orchestrator

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"text/template"

	"github.com/tidwall/sjson"

	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/pipeline"
)

// RenderContext is the context used for rendering override templates.
type RenderContext struct {
	// RequestModel is the model used in the original request.
	RequestModel string `json:"request_model"`
	// Model is the model sent to the LLM service.
	Model string `json:"model"`
	// Metadata is the metadata used in the current request.
	Metadata map[string]string `json:"metadata"`
	// ReasoningEffort is the reasoning effort used in the current request.
	ReasoningEffort string `json:"reasoning_effort"`
}

// renderOverrideValue renders a template string using RenderContext derived from llm.Request.
// It also attempts to parse the result as JSON if it looks like a structured value (object, array) or a number/boolean/null.
func renderOverrideValue(ctx context.Context, value string, llmReq *llm.Request, requestModel string) any {
	if !strings.Contains(value, "{{") || !strings.Contains(value, "}}") {
		return value
	}

	rendered := value
	renderCtx := RenderContext{
		RequestModel:    requestModel,
		Model:           llmReq.Model,
		Metadata:        llmReq.Metadata,
		ReasoningEffort: llmReq.ReasoningEffort,
	}

	funcMap := template.FuncMap{}

	tmpl, err := template.New("override").Funcs(funcMap).Parse(value)
	if err != nil {
		log.Warn(ctx, "failed to parse override template",
			log.String("template", value),
			log.Cause(err),
		)
	} else {
		var buf bytes.Buffer
		if err := tmpl.Execute(&buf, renderCtx); err != nil {
			log.Warn(ctx, "failed to execute override template",
				log.String("template", value),
				log.Cause(err),
			)
		} else {
			rendered = buf.String()
		}
	}

	trimmed := strings.TrimSpace(rendered)
	if trimmed == "" {
		return rendered
	}

	// If the rendered value is a valid JSON (like an object, array, number, bool, null),
	// we should try to return it as a raw value instead of a string.
	// This allows overriding complex structures or types via templates or direct values.
	firstChar := trimmed[0]
	if firstChar == '{' || firstChar == '[' || (firstChar >= '0' && firstChar <= '9') || firstChar == '-' ||
		trimmed == "true" || trimmed == "false" || trimmed == "null" {
		var jsonVal any
		if json.Unmarshal([]byte(trimmed), &jsonVal) == nil {
			return jsonVal
		}
	}

	return rendered
}

// applyOverrideRequestBody creates a middleware that applies channel override parameters.
func applyOverrideRequestBody(outbound *PersistentOutboundTransformer) pipeline.Middleware {
	return pipeline.OnRawRequest("override-request-body", func(ctx context.Context, request *httpclient.Request) (*httpclient.Request, error) {
		channel := outbound.GetCurrentChannel()

		overrideParams := channel.GetOverrideParameters()
		if len(overrideParams) == 0 {
			return request, nil
		}

		// Apply each override parameter using sjson
		body := request.Body

		for key, value := range overrideParams {
			if strings.EqualFold(key, "stream") {
				log.Warn(ctx, "stream override parameter ignored",
					log.String("channel", channel.Name),
					log.Int("channel_id", channel.ID),
				)

				continue
			}

			var (
				overridedBody []byte
				err           error
			)

			// Render template if value is a string and contains template syntax
			renderedValue := value
			if strValue, ok := value.(string); ok {
				renderedValue = renderOverrideValue(ctx, strValue, outbound.state.LlmRequest, outbound.state.OriginalModel)
			}

			if renderedValue == "__AXONHUB_CLEAR__" {
				overridedBody, err = sjson.DeleteBytes(body, key)
			} else {
				overridedBody, err = sjson.SetBytes(body, key, renderedValue)
			}

			if err != nil {
				log.Warn(ctx, "failed to apply override parameter",
					log.String("channel", channel.Name),
					log.String("key", key),
					log.Cause(err),
				)

				continue
			}

			body = overridedBody
		}

		if log.DebugEnabled(ctx) {
			log.Debug(ctx, "applied override parameters",
				log.String("channel", channel.Name),
				log.Int("channel_id", channel.ID),
				log.Any("override_params", overrideParams),
				log.String("old_body", string(request.Body)),
				log.String("new_body", string(body)),
			)
		}

		request.Body = body

		return request, nil
	})
}

// applyOverrideRequestHeaders creates a middleware that applies channel override headers.
func applyOverrideRequestHeaders(outbound *PersistentOutboundTransformer) pipeline.Middleware {
	return pipeline.OnRawRequest("override-request-headers", func(ctx context.Context, request *httpclient.Request) (*httpclient.Request, error) {
		channel := outbound.GetCurrentChannel()
		if channel == nil {
			return request, nil
		}

		overrideHeaders := channel.GetOverrideHeaders()
		if len(overrideHeaders) == 0 {
			return request, nil
		}

		// Apply each override header
		if request.Headers == nil {
			request.Headers = make(http.Header)
		}

		// Prepare render context
		llmReq := outbound.state.LlmRequest

		for _, entry := range overrideHeaders {
			if entry.Key == "" {
				log.Warn(ctx, "empty header key ignored",
					log.String("channel", channel.Name),
					log.Int("channel_id", channel.ID),
				)

				continue
			}

			renderedValue := renderOverrideValue(ctx, entry.Value, llmReq, outbound.state.OriginalModel)

			// If rendered value is __AXONHUB_CLEAR__, remove header.
			if renderedValue == "__AXONHUB_CLEAR__" {
				log.Debug(ctx, "cleared header",
					log.String("channel", channel.Name),
					log.Int("channel_id", channel.ID),
					log.String("key", entry.Key),
				)

				request.Headers.Del(entry.Key)

				continue
			}

			strValue := fmt.Sprintf("%v", renderedValue)
			request.Headers.Set(entry.Key, strValue)

			if log.DebugEnabled(ctx) {
				log.Debug(ctx, "overrided header",
					log.String("channel", channel.Name),
					log.String("key", entry.Key),
					log.String("value", strValue),
				)
			}
		}

		return request, nil
	})
}
