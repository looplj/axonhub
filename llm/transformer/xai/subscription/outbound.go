package subscription

import (
	"context"
	"errors"
	"fmt"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/auth"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/oauth"
	"github.com/looplj/axonhub/llm/streams"
	"github.com/looplj/axonhub/llm/transformer"
	"github.com/looplj/axonhub/llm/transformer/openai/responses"
)

type OutboundTransformer struct {
	tokens    oauth.TokenGetter
	responses *responses.OutboundTransformer
}

func NewOutboundTransformer(tokenProvider oauth.TokenGetter) (*OutboundTransformer, error) {
	if tokenProvider == nil {
		return nil, errors.New("token provider is required")
	}

	outbound, err := responses.NewOutboundTransformerWithConfig(&responses.Config{
		BaseURL:        DefaultBaseURL,
		APIKeyProvider: auth.NewStaticKeyProvider("unused"),
	})
	if err != nil {
		return nil, err
	}

	return &OutboundTransformer{tokens: tokenProvider, responses: outbound}, nil
}

func (t *OutboundTransformer) APIFormat() llm.APIFormat {
	return llm.APIFormatOpenAIResponse
}

func (t *OutboundTransformer) TransformRequest(ctx context.Context, request *llm.Request) (*httpclient.Request, error) {
	if request != nil {
		//nolint:exhaustive // The subscription proxy only exposes text Responses.
		switch request.RequestType {
		case llm.RequestTypeChat, "":
		default:
			return nil, fmt.Errorf("%w: xAI subscription does not support %s requests", transformer.ErrInvalidRequest, request.RequestType)
		}
	}

	credentials, err := t.tokens.Get(ctx)
	if err != nil {
		return nil, err
	}

	httpRequest, err := t.responses.TransformRequest(ctx, request)
	if err != nil {
		return nil, err
	}

	httpRequest.Auth = &httpclient.AuthConfig{Type: httpclient.AuthTypeBearer, APIKey: credentials.AccessToken}
	httpRequest.Headers.Set(CLITokenAuthHeader, CLITokenAuth)
	httpRequest.Headers.Set(CLIClientVersionHeader, CLIClientVersion)
	httpRequest.Headers.Set(CLIClientIdentifierHeader, CLIClientIdentifier)
	httpRequest.Headers.Set("User-Agent", CLIUserAgent)

	return httpRequest, nil
}

func (t *OutboundTransformer) TransformResponse(ctx context.Context, response *httpclient.Response) (*llm.Response, error) {
	return t.responses.TransformResponse(ctx, response)
}

func (t *OutboundTransformer) TransformStream(ctx context.Context, request *httpclient.Request, stream streams.Stream[*httpclient.StreamEvent]) (streams.Stream[*llm.Response], error) {
	return t.responses.TransformStream(ctx, request, stream)
}

func (t *OutboundTransformer) TransformError(ctx context.Context, err *httpclient.Error) *llm.ResponseError {
	return t.responses.TransformError(ctx, err)
}

func (t *OutboundTransformer) AggregateStreamChunks(ctx context.Context, request *httpclient.Request, chunks []*httpclient.StreamEvent) ([]byte, llm.ResponseMeta, error) {
	return t.responses.AggregateStreamChunks(ctx, request, chunks)
}
