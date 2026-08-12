package gemini

import (
	"mime"
	"net/url"
	"path"
	"strings"

	"github.com/looplj/axonhub/llm"
)

func imageMIMEType(image *llm.ImageURL) string {
	if image.MIMEType != "" {
		return image.MIMEType
	}

	parsed, err := url.Parse(image.URL)
	if err != nil {
		return ""
	}

	mediaType := mime.TypeByExtension(path.Ext(parsed.Path))
	if value, _, ok := strings.Cut(mediaType, ";"); ok {
		return value
	}

	return mediaType
}
