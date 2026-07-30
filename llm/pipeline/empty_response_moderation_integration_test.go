package pipeline_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/pipeline"
	"github.com/looplj/axonhub/llm/transformer/openai"
)

func TestPipeline_ModerationEmptyResponseDetection(t *testing.T) {
	inbound := openai.NewModerationInboundTransformer()
	outbound, err := openai.NewOutboundTransformer("https://api.openai.com", "test-api-key")
	require.NoError(t, err)

	executorCalls := 0
	executor := &mockExecutor{
		doFunc: func(ctx context.Context, request *httpclient.Request) (*httpclient.Response, error) {
			executorCalls++
			require.Equal(t, http.MethodPost, request.Method)
			require.Equal(t, "https://api.openai.com/v1/moderations", request.URL)
			require.Equal(t, string(llm.RequestTypeModeration), request.RequestType)
			require.Equal(t, string(llm.APIFormatOpenAIModeration), request.APIFormat)
			require.Nil(t, request.Auth)
			require.Equal(t, "Bearer test-api-key", request.Headers.Get("Authorization"))

			return &httpclient.Response{
				StatusCode: http.StatusOK,
				Headers: http.Header{
					"Content-Type": []string{"application/json"},
				},
				Request: request,
				Body: []byte(`{
					"id":"modr-integration-test",
					"model":"omni-moderation-latest",
					"results":[{
						"flagged":false,
						"categories":{"hate":false,"sexual/minors":false},
						"category_scores":{"hate":0.01,"sexual/minors":0.001},
						"category_applied_input_types":{"hate":["text"],"sexual/minors":["text"]}
					}]
				}`),
			}, nil
		},
	}

	p := pipeline.NewFactory(executor).Pipeline(
		inbound,
		outbound,
		pipeline.WithEmptyResponseDetection(),
	)

	result, err := p.Process(context.Background(), &httpclient.Request{
		Method: http.MethodPost,
		URL:    "/v1/moderations",
		Headers: http.Header{
			"Content-Type": []string{"application/json"},
		},
		Body: []byte(`{"input":"这里是需要进行内容安全检测的文本","model":"omni-moderation-latest"}`),
	})
	require.NoError(t, err)
	require.Equal(t, 1, executorCalls)
	require.NotNil(t, result)
	require.False(t, result.Stream)
	require.NotNil(t, result.Response)
	require.Equal(t, http.StatusOK, result.Response.StatusCode)
	require.Equal(t, "application/json", result.Response.Headers.Get("Content-Type"))

	var response struct {
		ID      string `json:"id"`
		Model   string `json:"model"`
		Results []struct {
			Flagged                   *bool               `json:"flagged"`
			Categories                map[string]bool     `json:"categories"`
			CategoryScores            map[string]float64  `json:"category_scores"`
			CategoryAppliedInputTypes map[string][]string `json:"category_applied_input_types"`
		} `json:"results"`
	}
	require.NoError(t, json.Unmarshal(result.Response.Body, &response))
	require.Equal(t, "modr-integration-test", response.ID)
	require.Equal(t, "omni-moderation-latest", response.Model)
	require.Len(t, response.Results, 1)
	require.NotNil(t, response.Results[0].Flagged)
	require.False(t, *response.Results[0].Flagged)
	require.Contains(t, response.Results[0].Categories, "sexual/minors")
	require.False(t, response.Results[0].Categories["sexual/minors"])
	require.Equal(t, 0.001, response.Results[0].CategoryScores["sexual/minors"])
	require.Equal(t, []string{"text"}, response.Results[0].CategoryAppliedInputTypes["sexual/minors"])
}
