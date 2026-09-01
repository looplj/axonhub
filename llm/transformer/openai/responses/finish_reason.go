package responses

// IsAbnormalChatFinishReason reports whether a Chat finish reason marks a
// truncated or failed run when that run is converted back to Responses.
func IsAbnormalChatFinishReason(reason string) bool {
	switch reason {
	case "length", "content_filter", "error", "cancelled", "canceled":
		return true
	default:
		return false
	}
}
