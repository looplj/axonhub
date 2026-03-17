package memory

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/looplj/axonhub/cmd/axonclaw/conf"
)

type Embedder interface {
	Model() string
	Embed(ctx context.Context, texts []string) ([][]float64, error)
}

type OpenAIEmbedder struct {
	baseURL string
	apiKey  string
	model   string
	client  *http.Client
}

type openAIEmbeddingRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

type openAIEmbeddingResponse struct {
	Data []struct {
		Embedding []float64 `json:"embedding"`
		Index     int       `json:"index"`
	} `json:"data"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func NewEmbedderFromConfig() Embedder {
	cfg, err := conf.LoadConfig()
	if err != nil {
		return nil
	}

	if strings.TrimSpace(cfg.BaseURL) == "" || strings.TrimSpace(cfg.APIKey) == "" {
		return nil
	}

	model := strings.TrimSpace(cfg.MemoryEmbeddingModel)
	if model == "" {
		model = "text-embedding-3-small"
	}

	return &OpenAIEmbedder{
		baseURL: strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/"),
		apiKey:  strings.TrimSpace(cfg.APIKey),
		model:   model,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (e *OpenAIEmbedder) Model() string {
	if e == nil {
		return ""
	}

	return e.model
}

func (e *OpenAIEmbedder) Embed(ctx context.Context, texts []string) ([][]float64, error) {
	if e == nil {
		return nil, fmt.Errorf("embedder is nil")
	}

	if len(texts) == 0 {
		return nil, nil
	}

	payload, err := json.Marshal(openAIEmbeddingRequest{
		Model: e.model,
		Input: texts,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal embedding request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.baseURL+"/v1/embeddings", bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("build embedding request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+e.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request embeddings: %w", err)
	}
	defer resp.Body.Close()

	var body openAIEmbeddingResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("decode embedding response: %w", err)
	}

	if resp.StatusCode >= http.StatusBadRequest {
		if body.Error != nil && strings.TrimSpace(body.Error.Message) != "" {
			return nil, fmt.Errorf("embedding API returned %s: %s", resp.Status, body.Error.Message)
		}

		return nil, fmt.Errorf("embedding API returned %s", resp.Status)
	}

	vectors := make([][]float64, len(texts))
	for _, item := range body.Data {
		if item.Index < 0 || item.Index >= len(vectors) {
			continue
		}
		vectors[item.Index] = item.Embedding
	}

	for i, vector := range vectors {
		if len(vector) == 0 {
			return nil, fmt.Errorf("embedding response missing vector for item %d", i)
		}
	}

	return vectors, nil
}
