package biz

import (
	"context"
	"crypto/sha256"
	"encoding/hex"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channelclientid"
)

type ChannelClientIDService struct {
	client *ent.Client
}

func NewChannelClientIDService(client *ent.Client) *ChannelClientIDService {
	return &ChannelClientIDService{client: client}
}

// GetOrCreateClientID retrieves or creates a client ID for the given channel and principal.
func (s *ChannelClientIDService) GetOrCreateClientID(ctx context.Context, channelID int, principalKind, principalHash string) (string, error) {
	// Try to find existing
	existing, err := s.client.ChannelClientID.Query().
		Where(
			channelclientid.ChannelIDEQ(channelID),
			channelclientid.PrincipalHashEQ(principalHash),
		).
		Only(ctx)

	if err == nil {
		return existing.ClientIDHex, nil
	}

	if !ent.IsNotFound(err) {
		return "", err
	}

	// Generate new client ID
	clientIDHex := generateClientIDHex(channelID, principalHash)

	// Create new record
	created, err := s.client.ChannelClientID.Create().
		SetChannelID(channelID).
		SetPrincipalKind(principalKind).
		SetPrincipalHash(principalHash).
		SetClientIDHex(clientIDHex).
		Save(ctx)

	if err != nil {
		// Handle race condition: another goroutine created it
		if ent.IsConstraintError(err) {
			existing, err := s.client.ChannelClientID.Query().
				Where(
					channelclientid.ChannelIDEQ(channelID),
					channelclientid.PrincipalHashEQ(principalHash),
				).
				Only(ctx)
			if err != nil {
				return "", err
			}
			return existing.ClientIDHex, nil
		}
		return "", err
	}

	return created.ClientIDHex, nil
}

// generateClientIDHex creates a deterministic hex string based on channel and principal.
func generateClientIDHex(channelID int, principalHash string) string {
	h := sha256.New()
	h.Write([]byte("channel_client_id"))
	h.Write([]byte{byte(channelID), byte(channelID >> 8), byte(channelID >> 16), byte(channelID >> 24)})
	h.Write([]byte(principalHash))
	return hex.EncodeToString(h.Sum(nil))
}

// ComputePrincipalHash computes the principal hash for OAuth or API key.
func ComputePrincipalHash(principalKind string, apiKey string) string {
	if principalKind == "oauth" {
		return "__oauth__"
	}

	// API key
	h := sha256.New()
	h.Write([]byte(apiKey))
	return "api_key:" + hex.EncodeToString(h.Sum(nil))
}
