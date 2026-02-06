package orchestrator

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"text/template"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"

	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/objects"
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

func buildRenderContext(llmReq *llm.Request, requestModel string) RenderContext {
	return RenderContext{
		RequestModel:    requestModel,
		Model:           llmReq.Model,
		Metadata:        llmReq.Metadata,
		ReasoningEffort: llmReq.ReasoningEffort,
	}
}

// renderTemplate renders a Go template string against RenderContext. Returns the original value on error.
func renderTemplate(ctx context.Context, value string, renderCtx RenderContext) string {
	if !strings.Contains(value, "{{") || !strings.Contains(value, "}}") {
		return value
	}

	tmpl, err := template.New("override").Funcs(template.FuncMap{}).Parse(value)
	if err != nil {
		log.Warn(ctx, "failed to parse override template",
			log.String("template", value),
			log.Cause(err),
		)

		return value
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, renderCtx); err != nil {
		log.Warn(ctx, "failed to execute override template", log.String("template", value), log.Cause(err))
		return value
	}

	return buf.String()
}

// renderOverrideValue renders a template string using RenderContext derived from llm.Request.
// It also attempts to parse the result as JSON if it looks like a structured value (object, array) or a number/boolean/null.
func renderOverrideValue(ctx context.Context, value string, renderCtx RenderContext) any {
	rendered := renderTemplate(ctx, value, renderCtx)

	trimmed := strings.TrimSpace(rendered)
	if trimmed == "" {
		return rendered
	}

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

// evaluateCondition renders the condition template and returns true
// if the result (trimmed) equals "true". Empty condition means always execute.
func evaluateCondition(ctx context.Context, condition string, renderCtx RenderContext) bool {
	if condition == "" {
		return true
	}

	rendered := renderTemplate(ctx, condition, renderCtx)

	return strings.TrimSpace(rendered) == "true"
}

// applyOverrideRequestBody creates a middleware that applies channel override operations.
func applyOverrideRequestBody(outbound *PersistentOutboundTransformer) pipeline.Middleware {
	return pipeline.OnRawRequest("override-request-body", func(ctx context.Context, request *httpclient.Request) (*httpclient.Request, error) {
		channel := outbound.GetCurrentChannel()

		ops := channel.GetBodyOverrideOperations()
		if len(ops) == 0 {
			return request, nil
		}

		llmReq := outbound.state.LlmRequest
		renderCtx := buildRenderContext(llmReq, outbound.state.OriginalModel)
		body := request.Body

		for _, op := range ops {
			if strings.EqualFold(op.Path, "stream") {
				log.Warn(ctx, "stream override parameter ignored",
					log.String("channel", channel.Name),
					log.Int("channel_id", channel.ID),
				)

				continue
			}

			var err error

			body, err = applyBodyOperation(ctx, body, op, renderCtx)
			if err != nil {
				log.Warn(ctx, "failed to apply override operation",
					log.String("channel", channel.Name),
					log.String("op", op.Op),
					log.String("path", op.Path),
					log.Cause(err),
				)
			}
		}

		if log.DebugEnabled(ctx) {
			log.Debug(ctx, "applied body override operations",
				log.String("channel", channel.Name),
				log.Int("channel_id", channel.ID),
				log.Any("operations", ops),
				log.String("old_body", string(request.Body)),
				log.String("new_body", string(body)),
			)
		}

		request.Body = body

		return request, nil
	})
}

func applyBodyOperation(
	ctx context.Context,
	body []byte,
	op objects.OverrideOperation,
	renderCtx RenderContext,
) ([]byte, error) {
	if !evaluateCondition(ctx, op.Condition, renderCtx) {
		return body, nil
	}

	switch op.Op {
	case objects.OverrideOpSet:
		return applyBodySet(ctx, body, op, renderCtx)
	case objects.OverrideOpDelete:
		return applyBodyDelete(body, op)
	case objects.OverrideOpRename:
		return applyBodyRename(body, op)
	case objects.OverrideOpCopy:
		return applyBodyCopy(body, op)
	default:
		log.Warn(ctx, "unknown override operation",
			log.String("op", op.Op),
		)

		return body, nil
	}
}

func applyBodySet(
	ctx context.Context,
	body []byte,
	op objects.OverrideOperation,
	renderCtx RenderContext,
) ([]byte, error) {
	renderedValue := renderOverrideValue(ctx, op.Value, renderCtx)

	if renderedValue == "__AXONHUB_CLEAR__" {
		return sjson.DeleteBytes(body, op.Path)
	}

	return sjson.SetBytes(body, op.Path, renderedValue)
}

func applyBodyDelete(body []byte, op objects.OverrideOperation) ([]byte, error) {
	return sjson.DeleteBytes(body, op.Path)
}

func applyBodyRename(body []byte, op objects.OverrideOperation) ([]byte, error) {
	result := gjson.GetBytes(body, op.From)
	if !result.Exists() {
		return body, nil
	}

	body, err := sjson.DeleteBytes(body, op.From)
	if err != nil {
		return body, err
	}

	return sjson.SetBytes(body, op.To, result.Value())
}

func applyBodyCopy(body []byte, op objects.OverrideOperation) ([]byte, error) {
	result := gjson.GetBytes(body, op.From)
	if !result.Exists() {
		return body, nil
	}

	return sjson.SetBytes(body, op.To, result.Value())
}

func applyOverrideOperationToHeaders(
	ctx context.Context,
	headers http.Header,
	op objects.OverrideOperation,
	renderCtx RenderContext,
) {
	if !evaluateCondition(ctx, op.Condition, renderCtx) {
		return
	}

	switch op.Op {
	case objects.OverrideOpSet:
		renderedValue := renderOverrideValue(ctx, op.Value, renderCtx)
		// For backward compatibility, we still support "__AXONHUB_CLEAR__" to clear the header.
		if renderedValue == "__AXONHUB_CLEAR__" {
			headers.Del(op.Path)
			return
		}

		headers.Set(op.Path, fmt.Sprintf("%v", renderedValue))
	case objects.OverrideOpDelete:
		headers.Del(op.Path)
	case objects.OverrideOpRename:
		values := headers.Values(op.From)
		if len(values) == 0 {
			return
		}

		headers.Del(op.From)

		for _, v := range values {
			headers.Add(op.To, v)
		}
	case objects.OverrideOpCopy:
		values := headers.Values(op.From)
		for _, v := range values {
			headers.Add(op.To, v)
		}
	default:
		log.Warn(ctx, "unknown header override operation",
			log.String("op", op.Op),
		)
	}
}

// applyOverrideRequestHeaders creates a middleware that applies channel override headers.
func applyOverrideRequestHeaders(outbound *PersistentOutboundTransformer) pipeline.Middleware {
	return pipeline.OnRawRequest("override-request-headers", func(ctx context.Context, request *httpclient.Request) (*httpclient.Request, error) {
		channel := outbound.GetCurrentChannel()
		if channel == nil {
			return request, nil
		}

		overrideHeaders := channel.GetHeaderOverrideOperations()
		if len(overrideHeaders) == 0 {
			return request, nil
		}

		if request.Headers == nil {
			request.Headers = make(http.Header)
		}

		llmReq := outbound.state.LlmRequest
		renderCtx := buildRenderContext(llmReq, outbound.state.OriginalModel)

		for _, op := range overrideHeaders {
			applyOverrideOperationToHeaders(ctx, request.Headers, op, renderCtx)
		}

		return request, nil
	})
}
