package api

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/fx"

	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/server/biz"
)

type APIKeyHandlersParams struct {
	fx.In

	APIKeyService *biz.APIKeyService
}

type APIKeyHandlers struct {
	APIKeyService *biz.APIKeyService
}

func NewAPIKeyHandlers(params APIKeyHandlersParams) *APIKeyHandlers {
	return &APIKeyHandlers{
		APIKeyService: params.APIKeyService,
	}
}

type CreateLLMAPIKeyRequest struct {
	Name string `json:"name" binding:"required"`
}

type LLMAPIKeyResponse struct {
	ID        objects.GUID `json:"id"`
	ProjectID objects.GUID `json:"projectID"`
	Name      string       `json:"name"`
	Key       string       `json:"key"`
	Type      string       `json:"type"`
	Status    string       `json:"status"`
	Scopes    []string     `json:"scopes"`
}

func newLLMAPIKeyResponse(apiKey *ent.APIKey) LLMAPIKeyResponse {
	return LLMAPIKeyResponse{
		ID:        objects.GUID{Type: ent.TypeAPIKey, ID: apiKey.ID},
		ProjectID: objects.GUID{Type: ent.TypeProject, ID: apiKey.ProjectID},
		Name:      apiKey.Name,
		Key:       apiKey.Key,
		Type:      string(apiKey.Type),
		Status:    string(apiKey.Status),
		Scopes:    apiKey.Scopes,
	}
}

// CreateLLMAPIKey creates a LLM-only API key for the same project as the service account.
func (h *APIKeyHandlers) CreateLLMAPIKey(c *gin.Context) {
	ctx := c.Request.Context()

	var req CreateLLMAPIKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
JSONError(c, http.StatusBadRequest, err)
		return
	}

	ownerKey, ok := contexts.GetAPIKey(ctx)
	if !ok || ownerKey == nil {
JSONError(c, http.StatusUnauthorized, errors.New("API key not found or invalid"))
		return
	}

	apiKey, err := h.APIKeyService.CreateLLMAPIKey(ctx, ownerKey, req.Name)
	if err != nil {
		switch {
		case errors.Is(err, biz.ErrAPIKeyNameRequired):
			JSONError(c, http.StatusBadRequest, err)
		case errors.Is(err, biz.ErrServiceAccountRequired),
			errors.Is(err, biz.ErrAPIKeyScopeRequired):
			JSONError(c, http.StatusForbidden, err)
		case errors.Is(err, biz.ErrAPIKeyOwnerRequired):
			JSONError(c, http.StatusUnauthorized, err)
		default:
			JSONError(c, http.StatusInternalServerError, err)
		}

		return
	}

	c.JSON(http.StatusOK, newLLMAPIKeyResponse(apiKey))
}
