package nanogpt

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"strings"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/transformer/openai"
)

// UseToolCall represents a <use_tool> XML element.
type UseToolCall struct {
	XMLName xml.Name `xml:"use_tool"`
	Name    string   `xml:"name,attr"`
	Arg     string   `xml:"arg"`
	Content string   `xml:",innerxml"`
}

// MaybeHasXMLToolCalls is a fast pre-check to determine if content likely contains XML tool calls.
// This avoids the overhead of full XML parsing for content that doesn't contain tool calls.
func MaybeHasXMLToolCalls(content string) bool {
	return strings.Contains(content, "<use_tool") ||
		strings.Contains(content, "</use_tool>") ||
		strings.Contains(content, "</use_use>")
}

// ParseXMLToolCalls extracts tool calls from XML content.
// It parses the <use_tool name="X"><arg>value</arg></use_tool> format.
// Returns the parsed tool calls, any remaining content after tool calls, and any error encountered.
func ParseXMLToolCalls(content string) ([]llm.ToolCall, string, error) {
	// Fast check - if no XML tool tags, return as-is
	if !MaybeHasXMLToolCalls(content) {
		return nil, content, nil
	}

	// Try to parse as XML containing use_tool elements
	decoder := xml.NewDecoder(strings.NewReader(content))

	var toolCalls []llm.ToolCall
	var remainingContent strings.Builder
	var hasToolCalls bool

	for {
		token, err := decoder.Token()
		if err != nil {
			break
		}

		switch t := token.(type) {
		case xml.StartElement:
			if t.Name.Local == "use_tool" {
				hasToolCalls = true
				var tool UseToolCall
				if err := decoder.DecodeElement(&tool, &t); err != nil {
					// If we can't parse, treat the original content as remaining
					if !hasToolCalls {
						return nil, content, nil
					}
					continue
				}

				if tool.Name != "" {
					// Generate deterministic ID from tool name and arguments
					id := generateToolCallID(tool.Name, tool.Arg)

					// Parse arguments - try JSON first, then treat as plain string
					var args string
					trimmedArg := strings.TrimSpace(tool.Arg)
					if trimmedArg != "" {
						// Check if it looks like JSON
						if (strings.HasPrefix(trimmedArg, "{") && strings.HasSuffix(trimmedArg, "}")) ||
							(strings.HasPrefix(trimmedArg, "[") && strings.HasSuffix(trimmedArg, "]")) {
							args = trimmedArg
						} else {
							// Wrap single value in JSON object with "value" key
							args = `{"value":` + jsonStringify(tool.Arg) + `}`
						}
					} else {
						args = "{}"
					}

					toolCalls = append(toolCalls, llm.ToolCall{
						Index: len(toolCalls),
						ID:    id,
						Type:  "function",
						Function: llm.FunctionCall{
							Name:      tool.Name,
							Arguments: args,
						},
					})
				}
			} else {
				// For other elements, include them in remaining content
				remainingContent.WriteString(token.(xml.StartElement).Name.Local)
			}

		case xml.CharData:
			text := strings.TrimSpace(string(t))
			if text != "" && !hasToolCalls {
				remainingContent.WriteString(text)
			}

		case xml.EndElement:
			if t.Name.Local != "use_tool" {
				remainingContent.WriteString("</")
				remainingContent.WriteString(t.Name.Local)
				remainingContent.WriteString(">")
			}
		}
	}

	if len(toolCalls) == 0 {
		// No valid tool calls found, return original content
		return nil, content, nil
	}

	// Return tool calls and remaining content
	remaining := remainingContent.String()
	if remaining == "" {
		return toolCalls, "", nil
	}

	return toolCalls, remaining, nil
}

// generateToolCallID generates a deterministic ID from the tool name and arguments.
func generateToolCallID(name, args string) string {
	hasher := sha256.New()
	hasher.Write([]byte(name))
	hasher.Write([]byte(args))
	hash := hasher.Sum(nil)
	return "nanogpt_" + hex.EncodeToString(hash)[:16]
}

// jsonStringify properly escapes a string for JSON.
func jsonStringify(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

// ToOpenAIToolCalls converts llm.ToolCall slice to openai.ToolCall slice.
func ToOpenAIToolCalls(toolCalls []llm.ToolCall) []openai.ToolCall {
	if len(toolCalls) == 0 {
		return nil
	}

	result := make([]openai.ToolCall, len(toolCalls))
	for i, tc := range toolCalls {
		result[i] = openai.ToolCall{
			ID:    tc.ID,
			Type:  tc.Type,
			Index: tc.Index,
			Function: openai.FunctionCall{
				Name:      tc.Function.Name,
				Arguments: tc.Function.Arguments,
			},
		}
	}
	return result
}

// ToOpenAIMessageContent converts a string to openai.MessageContent.
func ToOpenAIMessageContent(content string) openai.MessageContent {
	return openai.MessageContent{
		Content: &content,
	}
}