package oauth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
)

const (
	minCodeVerifierBytes = 32
	maxCodeVerifierBytes = 96
	minStateBytes        = 16
	maxStateBytes        = 96
)

func GenerateCodeVerifier(byteLength int) (string, error) {
	if byteLength < minCodeVerifierBytes || byteLength > maxCodeVerifierBytes {
		return "", fmt.Errorf("code verifier byte length must be between %d and %d", minCodeVerifierBytes, maxCodeVerifierBytes)
	}
	return randomURLSafeString(byteLength)
}

func GenerateCodeChallenge(verifier string) string {
	hash := sha256.Sum256([]byte(verifier))

	return base64.RawURLEncoding.EncodeToString(hash[:])
}

func GenerateState(byteLength int) (string, error) {
	if byteLength < minStateBytes || byteLength > maxStateBytes {
		return "", fmt.Errorf("state byte length must be between %d and %d", minStateBytes, maxStateBytes)
	}
	return randomURLSafeString(byteLength)
}

func randomURLSafeString(byteLength int) (string, error) {
	value := make([]byte, byteLength)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}
