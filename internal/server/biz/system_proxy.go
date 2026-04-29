package biz

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/looplj/axonhub/internal/ent"
)

const (
	// SystemKeyProxyPresets is the key used to store proxy preset configurations.
	// The value is JSON-encoded []ProxyPreset.
	SystemKeyProxyPresets = "system_proxy_presets"

	// envSecretKey is the environment variable used to derive the encryption key.
	envSecretKey = "AXONHUB_SECRET"
)

// ProxyPreset represents a proxy configuration preset.
type ProxyPreset struct {
	Name     string `json:"name,omitempty"`
	URL      string `json:"url"`
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
}

// encryptPassword encrypts a password using AES-GCM with a key derived from AXONHUB_SECRET.
// Returns a REDACTED marker if no secret is configured (TODO: proper encryption).
func encryptPassword(plaintext string) (string, error) {
	secret := os.Getenv(envSecretKey)
	if secret == "" {
		// No secret configured — cannot encrypt. Flag with a marker.
		return "**REDACTED**", nil
	}

	key := sha256.Sum256([]byte(secret))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}

	nonce := make([]byte, aesGCM.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("failed to generate nonce: %w", err)
	}

	ciphertext := aesGCM.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// decryptPassword decrypts an AES-GCM encrypted password.
// Returns the input unchanged if it doesn't look encrypted (legacy or redacted values).
func decryptPassword(encoded string) string {
	if encoded == "" || strings.HasPrefix(encoded, "**REDACTED**") {
		return encoded
	}

	secret := os.Getenv(envSecretKey)
	if secret == "" {
		return encoded
	}

	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		// Not base64 — assume plaintext or redacted.
		return encoded
	}

	key := sha256.Sum256([]byte(secret))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return encoded
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return encoded
	}

	nonceSize := aesGCM.NonceSize()
	if len(data) < nonceSize {
		return encoded
	}

	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	plaintext, err := aesGCM.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		// Decryption failed — likely wrong key or not encrypted data.
		return encoded
	}

	return string(plaintext)
}

// ProxyPresets retrieves all proxy presets, decrypting passwords on read.
func (s *SystemService) ProxyPresets(ctx context.Context) ([]ProxyPreset, error) {
	value, err := s.getSystemValue(ctx, SystemKeyProxyPresets)
	if err != nil {
		if ent.IsNotFound(err) {
			return []ProxyPreset{}, nil
		}

		return nil, fmt.Errorf("failed to get proxy presets: %w", err)
	}

	var presets []ProxyPreset
	if err := json.Unmarshal([]byte(value), &presets); err != nil {
		return nil, fmt.Errorf("failed to unmarshal proxy presets: %w", err)
	}

	// F-D92: Decrypt passwords on read.
	for i := range presets {
		presets[i].Password = decryptPassword(presets[i].Password)
	}

	return presets, nil
}

// SaveProxyPreset adds or updates a proxy preset, deduplicating by URL.
// F-D92: Encrypts passwords before storage.
func (s *SystemService) SaveProxyPreset(ctx context.Context, preset ProxyPreset) error {
	presets, err := s.ProxyPresets(ctx)
	if err != nil {
		return err
	}

	found := false

	for i, p := range presets {
		if p.URL == preset.URL {
			// Encrypt the new password before storing.
			encryptedPw, err := encryptPassword(preset.Password)
			if err != nil {
				return fmt.Errorf("failed to encrypt proxy password: %w", err)
			}
			preset.Password = encryptedPw
			presets[i] = preset
			found = true

			break
		}
	}

	if !found {
		// Encrypt the new password before storing.
		encryptedPw, err := encryptPassword(preset.Password)
		if err != nil {
			return fmt.Errorf("failed to encrypt proxy password: %w", err)
		}
		preset.Password = encryptedPw
		presets = append(presets, preset)
	}

	jsonBytes, err := json.Marshal(presets)
	if err != nil {
		return fmt.Errorf("failed to marshal proxy presets: %w", err)
	}

	return s.setSystemValue(ctx, SystemKeyProxyPresets, string(jsonBytes))
}

// DeleteProxyPreset removes a proxy preset by URL.
func (s *SystemService) DeleteProxyPreset(ctx context.Context, url string) error {
	presets, err := s.ProxyPresets(ctx)
	if err != nil {
		return err
	}

	filtered := make([]ProxyPreset, 0, len(presets))
	for _, p := range presets {
		if p.URL != url {
			filtered = append(filtered, p)
		}
	}

	jsonBytes, err := json.Marshal(filtered)
	if err != nil {
		return fmt.Errorf("failed to marshal proxy presets: %w", err)
	}

	return s.setSystemValue(ctx, SystemKeyProxyPresets, string(jsonBytes))
}
