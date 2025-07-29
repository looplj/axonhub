package server

import (
	"context"
	"fmt"
	"strings"

	"github.com/golang-jwt/jwt/v5"

	"github.com/looplj/axonhub/ent"
	"github.com/looplj/axonhub/server/biz"
)

// AuthMiddleware provides authentication middleware functionality
type AuthMiddleware struct {
	authService *biz.AuthService
}

// NewAuthMiddleware creates a new auth middleware
func NewAuthMiddleware(authService *biz.AuthService) *AuthMiddleware {
	return &AuthMiddleware{
		authService: authService,
	}
}

// ValidateJWTToken validates a JWT token from Authorization header
func (am *AuthMiddleware) ValidateJWTToken(ctx context.Context, authHeader string) (jwt.MapClaims, error) {
	if authHeader == "" {
		return nil, fmt.Errorf("authorization header is required")
	}

	// Extract token from "Bearer <token>" format
	parts := strings.Split(authHeader, " ")
	if len(parts) != 2 || parts[0] != "Bearer" {
		return nil, fmt.Errorf("invalid authorization header format")
	}

	return am.authService.ValidateJWTToken(ctx, parts[1])
}

// GetUserFromToken extracts user information from JWT token
func (am *AuthMiddleware) GetUserFromToken(ctx context.Context, authHeader string) (*ent.User, error) {
	claims, err := am.ValidateJWTToken(ctx, authHeader)
	if err != nil {
		return nil, err
	}

	userID, ok := claims["user_id"].(float64)
	if !ok {
		return nil, fmt.Errorf("invalid user_id in token")
	}

	// You might want to fetch the user from database here
	// For now, we'll create a minimal user object
	user := &ent.User{
		ID: int(userID),
	}

	if email, ok := claims["email"].(string); ok {
		user.Email = email
	}

	return user, nil
}
