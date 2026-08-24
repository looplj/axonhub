package codex

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/tidwall/sjson"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/transformer/shared"
)

var ErrAlphaSearchRequest = errors.New("invalid alpha search request")

func (t *OutboundTransformer) transformAlphaSearchRequest(ctx context.Context, llmReq *llm.Request) (*httpclient.Request, error) {
	if llmReq == nil || llmReq.AlphaSearch == nil || len(llmReq.AlphaSearch.Body) == 0 {
		return nil, ErrAlphaSearchRequest
	}

	creds, err := t.tokens.Get(ctx)
	if err != nil {
		return nil, err
	}
	body, err := sjson.SetBytes(llmReq.AlphaSearch.Body, "model", llmReq.Model)
	if err != nil {
		return nil, err
	}

	accountID := ExtractChatGPTAccountIDFromJWT(creds.AccessToken)
	var rawHeaders http.Header
	if llmReq.RawRequest != nil && llmReq.RawRequest.Headers != nil {
		rawHeaders = llmReq.RawRequest.Headers
	}
	sessionID := GetSessionIDFromHeaders(rawHeaders)
	if sessionID == "" {
		var envelope struct {
			ID string `json:"id"`
		}
		if json.Unmarshal(llmReq.AlphaSearch.Body, &envelope) == nil {
			sessionID = strings.TrimSpace(envelope.ID)
		}
	}
	if sessionID == "" {
		if value, ok := shared.GetSessionID(ctx); ok {
			sessionID = value
		} else {
			sessionID = uuid.NewString()
		}
	}

	headers := make(http.Header)
	headers.Set("Content-Type", "application/json")
	headers.Set("Accept", "application/json")
	for _, name := range PassthroughHeaders {
		for _, value := range rawHeaders.Values(name) {
			headers.Add(name, value)
		}
	}
	if originator := rawHeaders.Get("Originator"); originator != "" {
		headers.Set("Originator", originator)
	} else {
		headers.Set("Originator", AxonHubOriginator)
	}
	if userAgent := rawHeaders.Get("User-Agent"); userAgent != "" {
		headers.Set("User-Agent", userAgent)
	}
	if headers.Get(SessionHeaderHyphen) == "" {
		headers.Set(SessionHeaderHyphen, sessionID)
	}
	if headers.Get(ThreadIDHeader) == "" {
		headers.Set(ThreadIDHeader, sessionID)
	}
	if headers.Get(WindowIDHeader) == "" {
		headers.Set(WindowIDHeader, sessionID+":0")
	}
	if headers.Get(ClientRequestIDHeader) == "" {
		headers.Set(ClientRequestIDHeader, uuid.NewString())
	}
	if headers.Get(BetaFeaturesHeader) == "" {
		headers.Set(BetaFeaturesHeader, fabricatedBetaFeatures)
	}
	if headers.Get(TurnMetadataHeader) == "" {
		installationID := ""
		if accountID != "" {
			installationID = uuid.NewSHA1(uuid.NameSpaceOID, []byte(accountID)).String()
		}
		metadata := TurnMetadata{
			InstallationID: installationID,
			SessionID:      sessionID,
			ThreadID:       sessionID,
			TurnID:         uuid.NewString(),
			WindowID:       headers.Get(WindowIDHeader),
			RequestKind:    "turn",
			ThreadSource:   "user",
			Sandbox:        "none",
		}
		if encoded, marshalErr := json.Marshal(metadata); marshalErr == nil {
			headers.Set(TurnMetadataHeader, string(encoded))
		}
	}
	if accountID != "" {
		headers.Set("Chatgpt-Account-Id", accountID)
	}
	if headers.Get("Version") == "" {
		headers.Set("Version", codexDefaultVersion)
	}

	baseURL := strings.TrimRight(t.baseURL, "#/")
	return &httpclient.Request{
		Method:      http.MethodPost,
		URL:         baseURL + t.alphaSearchPath,
		Headers:     headers,
		Body:        body,
		Auth:        &httpclient.AuthConfig{Type: httpclient.AuthTypeBearer, APIKey: creds.AccessToken},
		RequestType: llm.RequestTypeAlphaSearch.String(),
		APIFormat:   llm.APIFormatOpenAIAlphaSearch.String(),
	}, nil
}

func (t *OutboundTransformer) transformAlphaSearchResponse(ctx context.Context, resp *httpclient.Response) (*llm.Response, error) {
	if resp == nil {
		return nil, ErrAlphaSearchRequest
	}
	if resp.StatusCode >= http.StatusBadRequest {
		return nil, t.responsesOutbound.TransformError(ctx, &httpclient.Error{StatusCode: resp.StatusCode, Body: resp.Body})
	}
	return &llm.Response{
		RequestType: llm.RequestTypeAlphaSearch,
		APIFormat:   llm.APIFormatOpenAIAlphaSearch,
		AlphaSearch: &llm.AlphaSearchResponse{Body: append([]byte(nil), resp.Body...)},
	}, nil
}
