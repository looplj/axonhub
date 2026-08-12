package gemini

import (
	"mime"
	"net/url"
	"path"
	"strings"

	"github.com/looplj/axonhub/llm"
)

func mediaMIMEType(explicitMIMEType, rawURL string, accepts func(string) bool) string {
	if explicitMIMEType != "" && accepts(explicitMIMEType) {
		return explicitMIMEType
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}

	extension := strings.ToLower(path.Ext(parsed.Path))
	mediaType := mime.TypeByExtension(extension)
	if value, _, ok := strings.Cut(mediaType, ";"); ok {
		mediaType = value
	}

	if accepts(mediaType) {
		return mediaType
	}

	return ""
}

func imageMIMEType(image *llm.ImageURL) string {
	return mediaMIMEType(image.MIMEType, image.URL, func(mediaType string) bool {
		return strings.HasPrefix(mediaType, "image/")
	})
}
