package subscription

import "strings"

const maxSSOTokenLength = 16 << 10

func NormalizeSSOToken(value string) string {
	value = strings.TrimSpace(value)
	if strings.HasPrefix(strings.ToLower(value), "cookie:") {
		value = strings.TrimSpace(value[len("cookie:"):])
	}

	for _, part := range strings.Split(value, ";") {
		name, token, found := strings.Cut(strings.TrimSpace(part), "=")
		if !found {
			continue
		}
		switch strings.ToLower(strings.TrimSpace(name)) {
		case "sso", "sso-rw":
			return sanitizeSSOToken(token)
		}
	}

	if token, _, found := strings.Cut(value, ";"); found {
		value = strings.TrimSpace(token)
	}

	return sanitizeSSOToken(value)
}

func sanitizeSSOToken(value string) string {
	value = strings.NewReplacer("\r", "", "\n", "", "\x00", "").Replace(strings.TrimSpace(value))
	if len(value) > maxSSOTokenLength {
		return ""
	}

	return value
}
