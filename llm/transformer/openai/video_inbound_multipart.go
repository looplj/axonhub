package openai

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"strings"

	"github.com/samber/lo"

	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/internal/pkg/xurl"
	"github.com/looplj/axonhub/llm/transformer"
)

const (
	maxVideoBodySize        = 20 * 1024 * 1024
	maxVideoReferenceSize   = 4 * 1024 * 1024
	maxVideoFieldValueSize  = 4 * 1024 * 1024
	videoReferenceFieldName = "input_reference"
)

func parseVideoMultipartRequest(httpReq *httpclient.Request) (*VideoCreateRequest, error) {
	if len(httpReq.Body) > maxVideoBodySize {
		return nil, fmt.Errorf("%w: request body too large", transformer.ErrInvalidRequest)
	}

	contentType := httpReq.Headers.Get("Content-Type")

	mediaType, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid content-type", transformer.ErrInvalidRequest)
	}

	if !strings.HasPrefix(strings.ToLower(mediaType), "multipart/") {
		return nil, fmt.Errorf("%w: expected multipart/form-data", transformer.ErrInvalidRequest)
	}

	boundary := params["boundary"]
	if boundary == "" {
		return nil, fmt.Errorf("%w: missing boundary in content-type", transformer.ErrInvalidRequest)
	}

	reader := multipart.NewReader(bytes.NewReader(httpReq.Body), boundary)
	fields := map[string]string{}

	var referenceFile *multipartFile

	for {
		part, err := reader.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}

		if err != nil {
			return nil, fmt.Errorf("%w: failed to read multipart", transformer.ErrInvalidRequest)
		}

		fieldName := part.FormName()
		filename := part.FileName()

		if filename == "" {
			value, err := io.ReadAll(io.LimitReader(part, maxVideoFieldValueSize+1))
			if err != nil {
				return nil, fmt.Errorf("%w: failed to read multipart field", transformer.ErrInvalidRequest)
			}

			if len(value) > maxVideoFieldValueSize {
				return nil, fmt.Errorf("%w: multipart field too large", transformer.ErrInvalidRequest)
			}

			fields[fieldName] = string(value)
			continue
		}

		if fieldName != videoReferenceFieldName {
			continue
		}

		if referenceFile != nil {
			return nil, fmt.Errorf("%w: multiple input_reference files are not supported", transformer.ErrInvalidRequest)
		}

		contentType := strings.TrimSpace(part.Header.Get("Content-Type"))
		data, err := io.ReadAll(io.LimitReader(part, maxVideoReferenceSize+1))
		if err != nil {
			return nil, fmt.Errorf("%w: failed to read multipart file", transformer.ErrInvalidRequest)
		}

		if len(data) > maxVideoReferenceSize {
			return nil, fmt.Errorf("%w: file too large", transformer.ErrInvalidRequest)
		}

		if contentType == "" {
			contentType = http.DetectContentType(lo.Ternary(len(data) > 512, data[:512], data))
		}

		if !isAllowedImageType(contentType) {
			return nil, fmt.Errorf("%w: unsupported image type", transformer.ErrInvalidRequest)
		}

		referenceFile = &multipartFile{ContentType: contentType, Data: data}
	}

	model := strings.TrimSpace(fields["model"])
	prompt := strings.TrimSpace(fields["prompt"])
	size := strings.TrimSpace(fields["size"])

	var seconds *string
	if s := strings.TrimSpace(fields["seconds"]); s != "" {
		seconds = &s
	}

	inputReference := strings.TrimSpace(fields[videoReferenceFieldName])
	if referenceFile != nil {
		inputReference = buildImageDataURL(referenceFile.ContentType, referenceFile.Data)
	}

	request := &VideoCreateRequest{
		Model:          model,
		Prompt:         prompt,
		InputReference: inputReference,
		Seconds:        seconds,
		Size:           size,
	}
	if err := applyVideoMultipartExtensions(request, fields); err != nil {
		return nil, err
	}
	return request, nil
}

func buildImageDataURL(contentType string, data []byte) string {
	// Use xurl.BuildDataURL (single exact-size concat) instead of fmt.Sprintf to
	// avoid the printer's doubling-growth buffer churn on large base64 data.
	return xurl.BuildDataURL(contentType, base64.StdEncoding.EncodeToString(data), true)
}
