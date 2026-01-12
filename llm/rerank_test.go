package llm

import (
	"encoding/json"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRerankRequestJSON(t *testing.T) {
	req := RerankRequest{
		Query:     "test query",
		Documents: []string{"doc1", "doc2", "doc3"},
		TopN:      lo.ToPtr(2),
	}

	data, err := json.Marshal(req)
	require.NoError(t, err)

	var decoded RerankRequest

	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, req.Query, decoded.Query)
	assert.Equal(t, req.Documents, decoded.Documents)
	assert.Equal(t, *req.TopN, *decoded.TopN)
}

func TestRerankResponseJSON(t *testing.T) {
	resp := RerankResponse{
		Results: []RerankResult{
			{
				Index:          0,
				RelevanceScore: 0.95,
				Document:       &RerankDocument{Text: "doc1"},
			},
			{
				Index:          1,
				RelevanceScore: 0.85,
				Document:       &RerankDocument{Text: "doc2"},
			},
		},
		Usage: &RerankUsage{
			PromptTokens: 10,
			TotalTokens:  15,
		},
	}

	data, err := json.Marshal(resp)
	require.NoError(t, err)

	var decoded RerankResponse

	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, len(resp.Results), len(decoded.Results))
	assert.Equal(t, resp.Results[0].Index, decoded.Results[0].Index)
	assert.Equal(t, resp.Results[0].RelevanceScore, decoded.Results[0].RelevanceScore)
	assert.NotNil(t, decoded.Usage)
	assert.Equal(t, resp.Usage.TotalTokens, decoded.Usage.TotalTokens)
}
