package codex

import (
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

func ExtractChatGPTAccountIDFromJWT(tokenStr string) string {
	parser := jwt.NewParser(jwt.WithoutClaimsValidation())

	token, _, err := parser.ParseUnverified(tokenStr, jwt.MapClaims{})
	if err != nil {
		return ""
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return ""
	}

	authClaims, ok := claims["https://api.openai.com/auth"].(map[string]any)
	if !ok {
		return ""
	}

	accountID, ok := authClaims["chatgpt_account_id"].(string)
	if !ok {
		return ""
	}

	return accountID
}

func isCodexCLIVersion(value string) bool {
	v := strings.TrimSpace(value)
	if v == "" {
		return false
	}

	dot := false

	for i := range len(v) {
		c := v[i]
		if c == '.' {
			dot = true
			continue
		}

		if c >= '0' && c <= '9' {
			continue
		}

		if c >= 'a' && c <= 'z' {
			continue
		}

		if c >= 'A' && c <= 'Z' {
			continue
		}

		if c == '-' || c == '+' || c == '_' {
			continue
		}

		return false
	}

	return dot
}
