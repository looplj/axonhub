package shared

import "encoding/base64"

func EnsureBase64Encoding(str string) string {
	if _, err := base64.StdEncoding.DecodeString(str); err == nil {
		return str
	}

	return base64.StdEncoding.EncodeToString([]byte(str))
}
