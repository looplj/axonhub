package oauth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
)

func GenerateCodeVerifier(byteLength int) (string, error) {
	value := make([]byte, byteLength)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}

	return base64.RawURLEncoding.EncodeToString(value), nil
}

func GenerateCodeChallenge(verifier string) string {
	hash := sha256.Sum256([]byte(verifier))

	return base64.RawURLEncoding.EncodeToString(hash[:])
}

func GenerateState(byteLength int) (string, error) {
	return GenerateCodeVerifier(byteLength)
}
