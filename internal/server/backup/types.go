package backup

import (
	"encoding/json"
	"time"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/request"
	"github.com/looplj/axonhub/internal/ent/usagelog"
	"github.com/looplj/axonhub/internal/objects"
)

type BackupData struct {
	Version            string                     `json:"version"`
	Timestamp          time.Time                  `json:"timestamp"`
	Projects           []*BackupProject           `json:"projects,omitempty"`
	Channels           []*BackupChannel           `json:"channels"`
	Models             []*BackupModel             `json:"models"`
	ChannelModelPrices []*BackupChannelModelPrice `json:"channel_model_prices,omitempty"`
	APIKeys            []*BackupAPIKey            `json:"api_keys,omitempty"`
	UsageRequests      []*BackupUsageRequest      `json:"usage_requests,omitempty"`
	UsageLogs          []*BackupUsageLog          `json:"usage_logs,omitempty"`
}

type BackupProject struct {
	ent.Project
}

type BackupChannel struct {
	ent.Channel

	Credentials objects.ChannelCredentials `json:"credentials"`
}

type BackupModel struct {
	ent.Model
}

type BackupAPIKey struct {
	ent.APIKey

	ProjectName string `json:"project_name"`
}

type BackupChannelModelPrice struct {
	ChannelName string             `json:"channel_name"`
	ModelID     string             `json:"model_id"`
	Price       objects.ModelPrice `json:"price"`
	ReferenceID string             `json:"reference_id"`
}

type BackupUsageRequest struct {
	ent.Request

	ProjectName string `json:"project_name,omitempty"`
	ChannelName string `json:"channel_name,omitempty"`
	APIKeyKey   string `json:"api_key_key,omitempty"`
}

func (r BackupUsageRequest) MarshalJSON() ([]byte, error) {
	type requestData struct {
		ID                         int                      `json:"id,omitempty"`
		CreatedAt                  time.Time                `json:"created_at,omitzero"`
		UpdatedAt                  time.Time                `json:"updated_at,omitzero"`
		ProjectID                  int                      `json:"project_id,omitempty"`
		Source                     request.Source           `json:"source,omitempty"`
		ModelID                    string                   `json:"model_id,omitempty"`
		ReasoningEffort            string                   `json:"reasoning_effort,omitempty"`
		Format                     string                   `json:"format,omitempty"`
		RequestHeaders             objects.JSONRawMessage   `json:"request_headers,omitempty"`
		RequestBody                objects.JSONRawMessage   `json:"request_body,omitempty"`
		ResponseBody               objects.JSONRawMessage   `json:"response_body,omitempty"`
		ResponseChunks             []objects.JSONRawMessage `json:"response_chunks,omitempty"`
		ChannelID                  int                      `json:"channel_id,omitempty"`
		ExternalID                 string                   `json:"external_id,omitempty"`
		Status                     request.Status           `json:"status,omitempty"`
		Stream                     bool                     `json:"stream,omitempty"`
		ClientIP                   string                   `json:"client_ip,omitempty"`
		MetricsLatencyMs           *int64                   `json:"metrics_latency_ms,omitempty"`
		MetricsFirstTokenLatencyMs *int64                   `json:"metrics_first_token_latency_ms,omitempty"`
		MetricsReasoningDurationMs *int64                   `json:"metrics_reasoning_duration_ms,omitempty"`
		ContentSaved               bool                     `json:"content_saved,omitempty"`
		ContentStorageID           *int                     `json:"content_storage_id,omitempty"`
		ContentStorageKey          *string                  `json:"content_storage_key,omitempty"`
		ContentSavedAt             *time.Time               `json:"content_saved_at,omitempty"`
		ProjectName                string                   `json:"project_name,omitempty"`
		ChannelName                string                   `json:"channel_name,omitempty"`
		APIKeyKey                  string                   `json:"api_key_key,omitempty"`
	}

	return json.Marshal(requestData{
		ID:                         r.ID,
		CreatedAt:                  r.CreatedAt,
		UpdatedAt:                  r.UpdatedAt,
		ProjectID:                  r.ProjectID,
		Source:                     r.Source,
		ModelID:                    r.ModelID,
		ReasoningEffort:            r.ReasoningEffort,
		Format:                     r.Format,
		RequestHeaders:             r.RequestHeaders,
		RequestBody:                r.RequestBody,
		ResponseBody:               r.ResponseBody,
		ResponseChunks:             r.ResponseChunks,
		ChannelID:                  r.ChannelID,
		ExternalID:                 r.ExternalID,
		Status:                     r.Status,
		Stream:                     r.Stream,
		ClientIP:                   r.ClientIP,
		MetricsLatencyMs:           r.MetricsLatencyMs,
		MetricsFirstTokenLatencyMs: r.MetricsFirstTokenLatencyMs,
		MetricsReasoningDurationMs: r.MetricsReasoningDurationMs,
		ContentSaved:               r.ContentSaved,
		ContentStorageID:           r.ContentStorageID,
		ContentStorageKey:          r.ContentStorageKey,
		ContentSavedAt:             r.ContentSavedAt,
		ProjectName:                r.ProjectName,
		ChannelName:                r.ChannelName,
		APIKeyKey:                  r.APIKeyKey,
	})
}

type BackupUsageLog struct {
	ent.UsageLog

	ProjectName string `json:"project_name,omitempty"`
	ChannelName string `json:"channel_name,omitempty"`
	APIKeyKey   string `json:"api_key_key,omitempty"`
}

func (l BackupUsageLog) MarshalJSON() ([]byte, error) {
	type usageLogData struct {
		ID                                 int                `json:"id,omitempty"`
		CreatedAt                          time.Time          `json:"created_at,omitzero"`
		UpdatedAt                          time.Time          `json:"updated_at,omitzero"`
		RequestID                          int                `json:"request_id,omitempty"`
		ProjectID                          int                `json:"project_id,omitempty"`
		ChannelID                          int                `json:"channel_id,omitempty"`
		ModelID                            string             `json:"model_id,omitempty"`
		PromptTokens                       int64              `json:"prompt_tokens,omitempty"`
		CompletionTokens                   int64              `json:"completion_tokens,omitempty"`
		TotalTokens                        int64              `json:"total_tokens,omitempty"`
		PromptAudioTokens                  int64              `json:"prompt_audio_tokens,omitempty"`
		PromptCachedTokens                 int64              `json:"prompt_cached_tokens,omitempty"`
		PromptWriteCachedTokens            int64              `json:"prompt_write_cached_tokens,omitempty"`
		PromptWriteCachedTokens5m          int64              `json:"prompt_write_cached_tokens_5m,omitempty"`
		PromptWriteCachedTokens1h          int64              `json:"prompt_write_cached_tokens_1h,omitempty"`
		CompletionAudioTokens              int64              `json:"completion_audio_tokens,omitempty"`
		CompletionReasoningTokens          int64              `json:"completion_reasoning_tokens,omitempty"`
		CompletionAcceptedPredictionTokens int64              `json:"completion_accepted_prediction_tokens,omitempty"`
		CompletionRejectedPredictionTokens int64              `json:"completion_rejected_prediction_tokens,omitempty"`
		Source                             usagelog.Source    `json:"source,omitempty"`
		Format                             string             `json:"format,omitempty"`
		TotalCost                          *float64           `json:"total_cost,omitempty"`
		CostItems                          []objects.CostItem `json:"cost_items,omitempty"`
		CostPriceReferenceID               string             `json:"cost_price_reference_id,omitempty"`
		ProjectName                        string             `json:"project_name,omitempty"`
		ChannelName                        string             `json:"channel_name,omitempty"`
		APIKeyKey                          string             `json:"api_key_key,omitempty"`
	}

	return json.Marshal(usageLogData{
		ID:                                 l.ID,
		CreatedAt:                          l.CreatedAt,
		UpdatedAt:                          l.UpdatedAt,
		RequestID:                          l.RequestID,
		ProjectID:                          l.ProjectID,
		ChannelID:                          l.ChannelID,
		ModelID:                            l.ModelID,
		PromptTokens:                       l.PromptTokens,
		CompletionTokens:                   l.CompletionTokens,
		TotalTokens:                        l.TotalTokens,
		PromptAudioTokens:                  l.PromptAudioTokens,
		PromptCachedTokens:                 l.PromptCachedTokens,
		PromptWriteCachedTokens:            l.PromptWriteCachedTokens,
		PromptWriteCachedTokens5m:          l.PromptWriteCachedTokens5m,
		PromptWriteCachedTokens1h:          l.PromptWriteCachedTokens1h,
		CompletionAudioTokens:              l.CompletionAudioTokens,
		CompletionReasoningTokens:          l.CompletionReasoningTokens,
		CompletionAcceptedPredictionTokens: l.CompletionAcceptedPredictionTokens,
		CompletionRejectedPredictionTokens: l.CompletionRejectedPredictionTokens,
		Source:                             l.Source,
		Format:                             l.Format,
		TotalCost:                          l.TotalCost,
		CostItems:                          l.CostItems,
		CostPriceReferenceID:               l.CostPriceReferenceID,
		ProjectName:                        l.ProjectName,
		ChannelName:                        l.ChannelName,
		APIKeyKey:                          l.APIKeyKey,
	})
}

const (
	BackupVersion   = "1.3"
	BackupVersionV1 = "1.0"
	BackupVersionV2 = "1.1"
	BackupVersionV3 = "1.2"
)

type BackupOptions struct {
	IncludeProjects    bool
	IncludeChannels    bool
	IncludeModels      bool
	IncludeAPIKeys     bool
	IncludeModelPrices bool
	IncludeUsageStats  bool
	IncludeRequestLogs bool
}

type ConflictStrategy string

const (
	ConflictStrategySkip      ConflictStrategy = "skip"
	ConflictStrategyOverwrite ConflictStrategy = "overwrite"
	ConflictStrategyError     ConflictStrategy = "error"
)

type RestoreOptions struct {
	IncludeProjects            bool
	IncludeChannels            bool
	IncludeModels              bool
	IncludeAPIKeys             bool
	IncludeModelPrices         bool
	IncludeUsageStats          bool
	IncludeRequestLogs         bool
	ProjectConflictStrategy    ConflictStrategy
	ChannelConflictStrategy    ConflictStrategy
	ModelConflictStrategy      ConflictStrategy
	ModelPriceConflictStrategy ConflictStrategy
	APIKeyConflictStrategy     ConflictStrategy
}
