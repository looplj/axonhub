package gemini

import (
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
)

func TestConvertDocumentURLToGeminiPart(t *testing.T) {
	tests := []struct {
		name     string
		doc      *llm.DocumentURL
		validate func(t *testing.T, result *Part)
	}{
		{
			name: "PDF data URL",
			doc: &llm.DocumentURL{
				URL:      "data:application/pdf;base64,JVBERi0xLjQK",
				MIMEType: "application/pdf",
			},
			validate: func(t *testing.T, result *Part) {
				require.NotNil(t, result)
				require.NotNil(t, result.InlineData)
				assert.Equal(t, "application/pdf", result.InlineData.MIMEType)
				assert.Equal(t, "JVBERi0xLjQK", result.InlineData.Data)
				assert.Nil(t, result.FileData)
			},
		},
		{
			name: "PDF file URL",
			doc: &llm.DocumentURL{
				URL:      "https://example.com/document.pdf",
				MIMEType: "application/pdf",
			},
			validate: func(t *testing.T, result *Part) {
				require.NotNil(t, result)
				require.NotNil(t, result.FileData)
				assert.Equal(t, "https://example.com/document.pdf", result.FileData.FileURI)
				assert.Equal(t, "application/pdf", result.FileData.MIMEType)
				assert.Nil(t, result.InlineData)
			},
		},
		{
			name: "Word document data URL",
			doc: &llm.DocumentURL{
				URL:      "data:application/msword;base64,0M8R4KGx",
				MIMEType: "application/msword",
			},
			validate: func(t *testing.T, result *Part) {
				require.NotNil(t, result)
				require.NotNil(t, result.InlineData)
				assert.Equal(t, "application/msword", result.InlineData.MIMEType)
				assert.Equal(t, "0M8R4KGx", result.InlineData.Data)
			},
		},
		{
			name: "PDF URL without MIME type",
			doc: &llm.DocumentURL{
				URL: "https://example.com/report.pdf",
			},
			validate: func(t *testing.T, result *Part) {
				require.NotNil(t, result)
				require.NotNil(t, result.FileData)
				assert.Equal(t, "https://example.com/report.pdf", result.FileData.FileURI)
				assert.Equal(t, "", result.FileData.MIMEType)
			},
		},
		{
			name: "nil document",
			doc:  nil,
			validate: func(t *testing.T, result *Part) {
				assert.Nil(t, result)
			},
		},
		{
			name: "empty URL",
			doc: &llm.DocumentURL{
				URL: "",
			},
			validate: func(t *testing.T, result *Part) {
				assert.Nil(t, result)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertDocumentURLToGeminiPart(tt.doc)
			tt.validate(t, result)
		})
	}
}

func TestIsDocumentMIMEType(t *testing.T) {
	tests := []struct {
		name     string
		mimeType string
		expected bool
	}{
		{
			name:     "PDF",
			mimeType: "application/pdf",
			expected: true,
		},
		{
			name:     "Word document",
			mimeType: "application/msword",
			expected: true,
		},
		{
			name:     "Word docx",
			mimeType: "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
			expected: true,
		},
		{
			name:     "Excel",
			mimeType: "application/vnd.ms-excel",
			expected: true,
		},
		{
			name:     "Plain text",
			mimeType: "text/plain",
			expected: true,
		},
		{
			name:     "HTML",
			mimeType: "text/html",
			expected: true,
		},
		{
			name:     "PNG image",
			mimeType: "image/png",
			expected: false,
		},
		{
			name:     "JPEG image",
			mimeType: "image/jpeg",
			expected: false,
		},
		{
			name:     "GIF image",
			mimeType: "image/gif",
			expected: false,
		},
		{
			name:     "Empty MIME type",
			mimeType: "",
			expected: false,
		},
		{
			name:     "Case insensitive - PDF uppercase",
			mimeType: "APPLICATION/PDF",
			expected: true,
		},
		{
			name:     "Case insensitive - image uppercase",
			mimeType: "IMAGE/PNG",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isDocumentMIMEType(tt.mimeType)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestConvertLLMToGeminiRequest_WithDocuments(t *testing.T) {
	tests := []struct {
		name     string
		input    *llm.Request
		validate func(t *testing.T, result *GenerateContentRequest)
	}{
		{
			name: "PDF document in request",
			input: &llm.Request{
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							MultipleContent: []llm.MessageContentPart{
								{
									Type: "document",
									Document: &llm.DocumentURL{
										URL:      "data:application/pdf;base64,JVBERi0xLjQK",
										MIMEType: "application/pdf",
									},
								},
								{
									Type: "text",
									Text: lo.ToPtr("Summarize this PDF"),
								},
							},
						},
					},
				},
			},
			validate: func(t *testing.T, result *GenerateContentRequest) {
				require.Len(t, result.Contents, 1)
				require.Len(t, result.Contents[0].Parts, 2)

				// Check PDF part
				pdfPart := result.Contents[0].Parts[0]
				require.NotNil(t, pdfPart.InlineData)
				assert.Equal(t, "application/pdf", pdfPart.InlineData.MIMEType)
				assert.Equal(t, "JVBERi0xLjQK", pdfPart.InlineData.Data)

				// Check text part
				textPart := result.Contents[0].Parts[1]
				assert.Equal(t, "Summarize this PDF", textPart.Text)
			},
		},
		{
			name: "Mixed image and document",
			input: &llm.Request{
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							MultipleContent: []llm.MessageContentPart{
								{
									Type: "image_url",
									ImageURL: &llm.ImageURL{
										URL: "data:image/png;base64,iVBORw0KGgo",
									},
								},
								{
									Type: "document",
									Document: &llm.DocumentURL{
										URL:      "https://example.com/doc.pdf",
										MIMEType: "application/pdf",
									},
								},
								{
									Type: "text",
									Text: lo.ToPtr("Compare these"),
								},
							},
						},
					},
				},
			},
			validate: func(t *testing.T, result *GenerateContentRequest) {
				require.Len(t, result.Contents, 1)
				require.Len(t, result.Contents[0].Parts, 3)

				// Check image part
				imagePart := result.Contents[0].Parts[0]
				require.NotNil(t, imagePart.InlineData)
				assert.Equal(t, "image/png", imagePart.InlineData.MIMEType)

				// Check document part
				docPart := result.Contents[0].Parts[1]
				require.NotNil(t, docPart.FileData)
				assert.Equal(t, "https://example.com/doc.pdf", docPart.FileData.FileURI)
				assert.Equal(t, "application/pdf", docPart.FileData.MIMEType)

				// Check text part
				textPart := result.Contents[0].Parts[2]
				assert.Equal(t, "Compare these", textPart.Text)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertLLMToGeminiRequest(tt.input)
			tt.validate(t, result)
		})
	}
}
