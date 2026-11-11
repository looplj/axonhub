package xsecret

// MaskAPIKey hides keys of at most eight characters and otherwise shows only
// the first and last four characters, without splitting UTF-8 sequences.
func MaskAPIKey(key string) string {
	runes := []rune(key)
	if len(runes) <= 8 {
		return "****"
	}
	return string(runes[:4]) + "****" + string(runes[len(runes)-4:])
}
