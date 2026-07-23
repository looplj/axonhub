package shared

import "strings"

// SanitizeUserID ensures the user ID matches common platform constraints:
// ^[a-zA-Z0-9_-]{6,128}$
func SanitizeUserID(userID string) string {
	if userID == "" {
		return ""
	}

	var sb strings.Builder
	for _, c := range userID {
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' || c == '-' {
			sb.WriteRune(c)
		} else {
			sb.WriteRune('_')
		}
	}

	sanitized := sb.String()

	if len(sanitized) < 6 {
		sanitized = "user_" + sanitized
		if len(sanitized) < 6 {
			sanitized = sanitized + strings.Repeat("_", 6-len(sanitized))
		}
	}

	if len(sanitized) > 128 {
		sanitized = sanitized[:128]
	}

	return sanitized
}
