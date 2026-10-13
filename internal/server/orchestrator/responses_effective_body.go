package orchestrator

import (
	"context"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/pipeline"
	"github.com/looplj/axonhub/llm/transformer/openai/responses"
)

func applyResponsesContinuationBody(p *PersistentOutboundTransformer) pipeline.Middleware {
	return pipeline.OnRawRequest("apply_responses_continuation_body", func(ctx context.Context, request *httpclient.Request) (*httpclient.Request, error) {
		c := p.state.ResponsesContinuation
		if c == nil || c.previous == "" || !responses.IsResponsesFormat(llm.APIFormat(request.APIFormat)) {
			return request, nil
		}
		effective, err := mergePassThroughRequestBodyWithPromptProtection(c.effective, p.state.LlmRequest.APIFormat, p.GetCurrentModelID(), p.state.PromptProtectionMaskRules)
		if err != nil {
			return request, nil //nolint:nilerr // Keep the already-transformed protected body.
		}
		if p.state.PromptProtectionBodyCheck != nil {
			if err := p.state.PromptProtectionBodyCheck.validateExpanded(ctx, effective); err != nil {
				return request, nil //nolint:nilerr // Keep the already-transformed protected body.
			}
		}
		payload := gjson.ParseBytes(effective)
		body := request.Body
		if input := payload.Get("input"); input.Exists() {
			body, err = sjson.SetRawBytes(body, "input", []byte(input.Raw))
			if err != nil {
				return nil, err
			}
		}
		if previous := payload.Get("previous_response_id"); previous.Exists() {
			body, err = sjson.SetRawBytes(body, "previous_response_id", []byte(previous.Raw))
		} else {
			body, err = sjson.DeleteBytes(body, "previous_response_id")
		}
		if err != nil {
			return nil, err
		}
		request.Body = body
		return request, nil
	})
}
