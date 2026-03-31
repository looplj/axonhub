package nanogpt

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/transformer/openai"
)

// Maximum content length to prevent ReDoS attacks
const maxXMLParseLength = 100000 // 100KB

// toolCallPattern matches XML-like tool calls with content: <Tag>content</Tag>
// Uses [^<] to match content safely without ReDoS backtracking
// Allows optional whitespace after tag name for formats like <Write_File>{...}</Write_File>
var toolCallPattern = regexp.MustCompile(`<([a-zA-Z_][a-zA-Z0-9_-]*)[\s]*([^>]*)>([^<]*)</([a-zA-Z_][a-zA-Z0-9_-]*)>`)

// selfClosingPattern matches self-closing XML tags: <Tag attr="val" />
// Allows optional space between tag name and attributes
var selfClosingPattern = regexp.MustCompile(`<([a-zA-Z_][a-zA-Z0-9_-]*)[\s]*([^>]*)/>`)

// attrPattern matches attributes like name="value" or name='value'
// Handles both single and double quotes, allows empty values
var attrPattern = regexp.MustCompile(`([a-zA-Z_][a-zA-Z0-9_-]*)[\s]*=[\s]*["']([^"']*)["']`)

// normalizeTagPattern matches tags without space before />
var normalizeTagPattern = regexp.MustCompile(`([^\s])/>`)
// nestedXMLPattern matches nested XML like <Write><file_path>X</file_path><content>Y</content></Write>
var nestedXMLPattern = regexp.MustCompile(`<(Write|Read)[^>]*>\s*<file_path>([^<]*)</file_path>\s*<content>([\s\S]*?)</content>\s*</(Write|Read)>`)
// mismatchTagPattern matches <Write>content</use_tool> type patterns
// Uses [^<] to match content safely without ReDoS backtracking
var mismatchTagPattern = regexp.MustCompile(`<(Write|Read|Write_FILE|Write_file|Read_FILE|Read_file)([^>]*)>([^<]*)</use_tool>`)

// unclosedPattern matches unclosed opening tags like <Write attr="..."\n</use_tool>
var unclosedPattern = regexp.MustCompile(`<(Write|Read|Write_FILE|Write_file|Read_FILE|Read_file)([^>]*)\n([\s\S]*?)</use_tool>`)
// MaybeHasXMLToolCalls is a fast pre-check to determine if content likely contains XML tool calls.
func MaybeHasXMLToolCalls(content string) bool {
	// Limit content length to prevent ReDoS
	if len(content) > maxXMLParseLength {
		content = content[:maxXMLParseLength]
	}

	// Check for common tool-related patterns
	return strings.Contains(content, "<") && strings.Contains(content, ">") &&
		(toolCallPattern.MatchString(content) ||
			selfClosingPattern.MatchString(content) ||
			strings.Contains(content, "use_tool") ||
			strings.Contains(content, "Write") ||
			strings.Contains(content, "Bash") ||
			strings.Contains(content, "Read"))
}

// ParseXMLToolCalls extracts tool calls from XML content using regex-based parsing.
// Handles various XML formats including:
// - <use_tool name="X"><arg>value</arg></use_tool>
// - <Write file_path="X" content="Y"/>
// - <Write file_path="X">content</Write>
// - <Write> {"file_path": "X", "content": "Y"}</use_tool>
// Returns the parsed tool calls, any remaining content after tool calls, and any error encountered.
func ParseXMLToolCalls(content string) ([]llm.ToolCall, string, error) {
	// Fast check - if no XML tool tags, return as-is
	if !MaybeHasXMLToolCalls(content) {
		return nil, content, nil
	}

	// Normalize common malformed variations
	content = normalizeXML(content)

	var toolCalls []llm.ToolCall
	var remainingContent strings.Builder
	lastEnd := 0

	// Find all matches from both patterns
	type matchInfo struct {
		start        int
		end          int
		tagName      string
		attrs        string
		innerContent string
		closingTag   string // for validation
	}

	var matches []matchInfo

	// Handle nested XML format: <Write><file_path>X</file_path><content>Y</content></Write>
	// This must be processed before other patterns to avoid matching inner elements
	for _, m := range nestedXMLPattern.FindAllStringSubmatchIndex(content, -1) {
		if len(m) >= 10 {
			// m[2]:m[3] = opening tag (Write/Read)
			// m[4]:m[5] = file_path value
			// m[6]:m[7] = content value
			// m[8]:m[9] = closing tag
			tagName := content[m[2]:m[3]]
			filePath := content[m[4]:m[5]]
			innerContent := content[m[6]:m[7]]
			closingTag := content[m[8]:m[9]]

			// Only process if opening and closing tags match
			if strings.EqualFold(tagName, closingTag) {
				// Create synthetic attributes - escape quotes to prevent XML injection
				filePathEscaped := strings.ReplaceAll(filePath, `"`, `\"`)
				innerContentEscaped := strings.ReplaceAll(innerContent, `"`, `\"`)
				attrs := fmt.Sprintf(`file_path="%s" content="%s"`, filePathEscaped, innerContentEscaped)
				matches = append(matches, matchInfo{
					start:        m[0],
					end:          m[1],
					tagName:      tagName,
					attrs:        attrs,
					innerContent: "", // Content is already in attrs
					closingTag:   closingTag,
				})
			}
		}
	}
	// Find opening/closing tag patterns
	for _, m := range toolCallPattern.FindAllStringSubmatchIndex(content, -1) {
		if len(m) >= 10 {
			openingTag := content[m[2]:m[3]]
			closingTag := content[m[8]:m[9]]
			// Only accept if opening and closing tags match
			if strings.EqualFold(openingTag, closingTag) {
				matches = append(matches, matchInfo{
					start:        m[0],
					end:          m[1],
					tagName:      openingTag,
					attrs:        content[m[4]:m[5]],
					innerContent: content[m[6]:m[7]],
					closingTag:   closingTag,
				})
			}
		}
	}

	// Find self-closing patterns
	for _, m := range selfClosingPattern.FindAllStringSubmatchIndex(content, -1) {
		if len(m) >= 6 {
			matches = append(matches, matchInfo{
				start:   m[0],
				end:     m[1],
				tagName: content[m[2]:m[3]],
				attrs:   content[m[4]:m[5]],
			})
		}
	}

	// Sort matches by start position using efficient O(n log n) sort
	sort.Slice(matches, func(i, j int) bool {
		return matches[i].start < matches[j].start
	})

	for _, match := range matches {
		// Skip if not a valid tool tag
		toolName := extractToolName(match.tagName, match.attrs)
		if toolName == "" {
			// Not a recognized tool pattern, keep in remaining content
			if lastEnd < match.start {
				remainingContent.WriteString(content[lastEnd:match.start])
			}
			lastEnd = match.end
			continue
		}

		// Extract tool arguments
		args := extractToolArguments(toolName, match.attrs, match.innerContent)

		// Generate deterministic ID
		id := generateToolCallID(toolName, args)

		toolCalls = append(toolCalls, llm.ToolCall{
			Index: len(toolCalls),
			ID:    id,
			Type:  "function",
			Function: llm.FunctionCall{
				Name:      toolName,
				Arguments: args,
			},
		})

		// Add any text before this tool call to remaining content
		if lastEnd < match.start {
			remainingContent.WriteString(content[lastEnd:match.start])
		}
		lastEnd = match.end
	}

	// Add any remaining content after the last tool call
	if lastEnd < len(content) {
		remainingContent.WriteString(content[lastEnd:])
	}

	if len(toolCalls) == 0 {
		return nil, content, nil
	}

	remaining := strings.TrimSpace(remainingContent.String())
	return toolCalls, remaining, nil
}

// normalizeXML fixes common XML malformations from NanoGPT
func normalizeXML(content string) string {
	// Fix unclosed opening tags: <Write attr="..."\ncontent</use_tool> -> <Write attr="...">\ncontent</use_tool>
	// NanoGPT sometimes omits the closing > on the opening tag
	content = unclosedPattern.ReplaceAllStringFunc(content, func(match string) string {
		parts := unclosedPattern.FindStringSubmatch(match)
		if len(parts) >= 4 {
			return "<" + parts[1] + parts[2] + ">\n" + parts[3] + "</use_tool>"
		}
		return match
	})
	// Fix mismatched closing tags - handle variations like </use_tool>, </use_use>, etc.
	content = strings.ReplaceAll(content, "</use_use>", "</use_tool>")
	content = strings.ReplaceAll(content, "</Write_file>", "</Write>")
	content = strings.ReplaceAll(content, "</Write_FILE>", "</Write>")
	content = strings.ReplaceAll(content, "</Read_file>", "</Read>")
	content = strings.ReplaceAll(content, "</Read_FILE>", "</Read>")

	// Fix weird patterns like <Write>content</use_tool> -> <Write>content</Write>
	// Use ReplaceAllFunc to preserve the opening tag name
	content = mismatchTagPattern.ReplaceAllStringFunc(content, func(match string) string {
		// Extract parts from the match
		parts := mismatchTagPattern.FindStringSubmatch(match)
		if len(parts) >= 4 {
			tagName := parts[1]
			attrs := parts[2]
			innerContent := parts[3]
			return "<" + tagName + attrs + ">" + innerContent + "</" + tagName + ">"
		}
		return match
	})

	// Normalize self-closing tags without space before />
	content = normalizeTagPattern.ReplaceAllString(content, "$1 />")

	return content
}

// extractToolName determines the tool name from tag name and attributes
func extractToolName(tagName, attrs string) string {
	tagName = strings.TrimSpace(strings.ToLower(tagName))

	// Direct tool name tags (handle variations like Write_FILE, Write_file, etc.)
	switch {
	case strings.HasPrefix(tagName, "write"):
		return "write"
	case strings.HasPrefix(tagName, "read"):
		return "read"
	case tagName == "bash", tagName == "python", tagName == "search", tagName == "glob":
		return tagName
	case tagName == "use_tool":
		// Extract from name attribute
		if matches := attrPattern.FindAllStringSubmatch(attrs, -1); matches != nil {
			for _, match := range matches {
				if len(match) >= 3 && strings.ToLower(match[1]) == "name" {
					return strings.ToLower(match[2])
				}
			}
		}
	}

	return ""
}

// extractToolArguments extracts arguments from attributes and/or inner content
func extractToolArguments(toolName, attrs, innerContent string) string {
	args := make(map[string]interface{})

	// Extract attributes
	attrMatches := attrPattern.FindAllStringSubmatch(attrs, -1)
	for _, match := range attrMatches {
		if len(match) >= 3 {
			key := match[1]
			value := match[2]
			// Skip the "name" attribute for use_tool tags
			if strings.ToLower(key) == "name" && toolName != "" {
				continue
			}
			args[key] = value
		}
	}

	// Handle inner content
	innerContent = strings.TrimSpace(innerContent)
	if innerContent != "" {
		// Try to parse as JSON first
		var jsonContent interface{}
		if err := json.Unmarshal([]byte(innerContent), &jsonContent); err == nil {
			// If valid JSON, merge with args
			if jsonMap, ok := jsonContent.(map[string]interface{}); ok {
				for k, v := range jsonMap {
					if _, exists := args[k]; !exists {
						args[k] = v
					}
				}
			} else {
				args["content"] = jsonContent
			}
		} else {
			// Not valid JSON, add as content or arg
			if _, hasContent := args["content"]; !hasContent {
				args["content"] = innerContent
			} else {
				args["arg"] = innerContent
			}
		}
	}

	// Serialize to JSON
	result, err := json.Marshal(args)
	if err != nil || len(args) == 0 {
		return "{}"
	}

	return string(result)
}

// generateToolCallID generates a deterministic ID from the tool name and arguments.
func generateToolCallID(name, args string) string {
	hasher := sha256.New()
	hasher.Write([]byte(name))
	hasher.Write([]byte(args))
	hash := hasher.Sum(nil)
	return "nanogpt_" + hex.EncodeToString(hash)[:16]
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

