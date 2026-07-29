package api

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/fx"

	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/server/biz"
)

// InvitationHandlersParams contains dependencies for invitation HTTP handlers.
type InvitationHandlersParams struct {
	fx.In

	InvitationService *biz.InvitationService
	AuthService       *biz.AuthService
}

// InvitationHandlers serves invitation creation, lookup, and registration endpoints.
type InvitationHandlers struct {
	InvitationService *biz.InvitationService
	AuthService       *biz.AuthService
}

// NewInvitationHandlers creates invitation HTTP handlers.
func NewInvitationHandlers(params InvitationHandlersParams) *InvitationHandlers {
	return &InvitationHandlers{
		InvitationService: params.InvitationService,
		AuthService:       params.AuthService,
	}
}

// CreateInvitationRequest contains the parameters for creating an invitation.
type CreateInvitationRequest struct {
	ExpiresInHours *int `json:"expiresInHours"`
	MaxUses        int  `json:"maxUses"`
	RoleID         int  `json:"roleID" binding:"required"`
}

// InvitationResponse contains invitation metadata returned by the API.
type InvitationResponse struct {
	Token         string     `json:"token,omitempty"`
	ProjectName   string     `json:"projectName"`
	ExpiresAt     *time.Time `json:"expiresAt"`
	MaxUses       int        `json:"maxUses"`
	UsedCount     int        `json:"usedCount"`
	RemainingUses int        `json:"remainingUses"`
}

// RegisterInvitationRequest contains the credentials and profile data for registration.
type RegisterInvitationRequest struct {
	Email     string `json:"email" binding:"required,email"`
	Password  string `json:"password" binding:"required,min=7"`
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
}

// RegisterInvitationResponse contains the registered user and session token.
type RegisterInvitationResponse struct {
	User  *objects.UserInfo `json:"user"`
	Token string            `json:"token"`
}

// Create handles invitation creation for a project.
func (h *InvitationHandlers) Create(c *gin.Context) {
	projectID, ok := contexts.GetProjectID(c.Request.Context())
	if !ok {
		JSONError(c, http.StatusBadRequest, errors.New("X-Project-ID header is required"))
		return
	}

	var req CreateInvitationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		JSONError(c, http.StatusBadRequest, errors.New("invalid invitation request"))
		return
	}

	created, err := h.InvitationService.CreateInvitation(c.Request.Context(), projectID, req.RoleID, req.ExpiresInHours, req.MaxUses)
	if err != nil {
		JSONError(c, http.StatusForbidden, err)
		return
	}

	c.JSON(http.StatusOK, invitationResponse(created.Token, created.Info))
}

// Get handles public invitation metadata lookup.
func (h *InvitationHandlers) Get(c *gin.Context) {
	info, err := h.InvitationService.GetInvitation(c.Request.Context(), c.Param("token"))
	if err != nil {
		JSONError(c, http.StatusNotFound, err)
		return
	}

	c.JSON(http.StatusOK, invitationResponse("", *info))
}

// Register handles account registration through an invitation.
func (h *InvitationHandlers) Register(c *gin.Context) {
	var req RegisterInvitationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		JSONError(c, http.StatusBadRequest, errors.New("invalid registration request"))
		return
	}

	registeredUser, err := h.InvitationService.RegisterInvitation(c.Request.Context(), c.Param("token"), req.Email, req.Password, req.FirstName, req.LastName)
	if err != nil {
		JSONError(c, http.StatusBadRequest, err)
		return
	}

	token, err := h.AuthService.GenerateJWTToken(c.Request.Context(), registeredUser)
	if err != nil {
		JSONError(c, http.StatusInternalServerError, errors.New("failed to create session"))
		return
	}

	c.JSON(http.StatusOK, RegisterInvitationResponse{
		User:  biz.ConvertUserToUserInfo(c.Request.Context(), registeredUser),
		Token: token,
	})
}

func invitationResponse(token string, info biz.InvitationInfo) InvitationResponse {
	return InvitationResponse{
		Token:         token,
		ProjectName:   info.ProjectName,
		ExpiresAt:     info.ExpiresAt,
		MaxUses:       info.MaxUses,
		UsedCount:     info.UsedCount,
		RemainingUses: info.RemainingUses,
	}
}
