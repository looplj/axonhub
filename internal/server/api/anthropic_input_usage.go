package api

import (
	"encoding/json"
	"strings"
	"sync"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
	"github.com/tiktoken-go/tokenizer"

	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/streams"
)

var inputUsageTokenizer struct {
	sync.Mutex

	codec tokenizer.Codec
}

// estimateAnthropicInput counts textual evidence, not transport/control fields or
// encoded media. This is a display estimate, not provider usage or billing data.
func estimateAnthropicInput(body []byte) (int64, error) {
	var request struct {
		System   json.RawMessage `json:"system"`
		Messages []struct {
			Role    string          `json:"role"`
			Content json.RawMessage `json:"content"`
		} `json:"messages"`
		Tools []struct {
			Name        string          `json:"name"`
			Description string          `json:"description"`
			Schema      json.RawMessage `json:"input_schema"`
		} `json:"tools"`
		ToolChoice json.RawMessage `json:"tool_choice"`
	}
	if err := json.Unmarshal(body, &request); err != nil {
		return 0, err
	}
	var parts []string
	appendText := func(s string) {
		if s != "" {
			parts = append(parts, s)
		}
	}
	var content func(gjson.Result)
	content = func(v gjson.Result) {
		if v.Type == gjson.String {
			appendText(v.String())
			return
		}
		if v.IsArray() {
			for _, child := range v.Array() {
				content(child)
			}
			return
		}
		switch v.Get("type").String() {
		case "text":
			appendText(v.Get("text").String())
		case "thinking":
			appendText(v.Get("thinking").String())
		case "tool_use", "server_tool_use", "mcp_tool_use":
			appendText(v.Get("name").String())
			appendText(v.Get("input").Raw)
		case "tool_result", "mcp_tool_result", "web_search_tool_result", "web_fetch_tool_result", "code_execution_tool_result", "bash_code_execution_tool_result", "text_editor_code_execution_tool_result":
			content(v.Get("content"))
		case "document":
			appendText(v.Get("title").String())
			appendText(v.Get("context").String())
			source := v.Get("source")
			if source.Get("type").String() == "text" {
				appendText(source.Get("data").String())
			}
			if source.Get("type").String() == "content" {
				content(source.Get("content"))
			}
		case "search_result", "web_search_result":
			appendText(v.Get("title").String())
			appendText(v.Get("url").String())
			content(v.Get("content"))
		case "web_fetch_result":
			content(v.Get("content"))
		}
	}
	content(gjson.ParseBytes(request.System))
	for _, m := range request.Messages {
		appendText(m.Role)
		content(gjson.ParseBytes(m.Content))
	}
	for _, tool := range request.Tools {
		appendText(tool.Name)
		appendText(tool.Description)
		appendText(string(tool.Schema))
	}
	if len(request.ToolChoice) > 0 {
		appendText(string(request.ToolChoice))
	}
	if len(parts) == 0 {
		return 0, nil
	}
	// Keep tokenizer access serialized: requests never share mutable stream state.
	inputUsageTokenizer.Lock()
	defer inputUsageTokenizer.Unlock()
	if inputUsageTokenizer.codec == nil {
		codec, err := tokenizer.Get(tokenizer.O200kBase)
		if err != nil {
			return 0, err
		}
		inputUsageTokenizer.codec = codec
	}
	count, err := inputUsageTokenizer.codec.Count(strings.Join(parts, "\n"))
	return int64(count), err
}

type anthropicInputUsageStream struct {
	streams.Stream[*httpclient.StreamEvent]
	body    []byte
	handled bool
	current *httpclient.StreamEvent
}

// Wrap after persistence: only the client sees the estimate. Final usage, stored
// responses and costs remain authoritative, even when the stream is interrupted.
func wrapAnthropicInputUsage(source streams.Stream[*httpclient.StreamEvent], request *httpclient.Request) streams.Stream[*httpclient.StreamEvent] {
	if request == nil || !strings.HasSuffix(request.Path, "/messages") {
		return source
	}
	return preserveStreamDelivery(&anthropicInputUsageStream{Stream: source, body: request.Body}, source)
}

func (s *anthropicInputUsageStream) Next() bool {
	if !s.Stream.Next() {
		return false
	}
	s.current = s.Stream.Current()
	if s.handled || s.current == nil || !json.Valid(s.current.Data) || gjson.GetBytes(s.current.Data, "type").String() != "message_start" {
		return true
	}
	s.handled = true
	defer func() { s.body = nil }()
	usage := gjson.GetBytes(s.current.Data, "message.usage")
	if usage.Get("input_tokens").Int() != 0 || usage.Get("cache_read_input_tokens").Int() != 0 || usage.Get("cache_creation_input_tokens").Int() != 0 {
		return true
	}
	estimate, err := estimateAnthropicInput(s.body)
	if err != nil || estimate <= 0 {
		return true
	}
	data, err := sjson.SetBytes(s.current.Data, "message.usage.input_tokens", estimate)
	if err != nil {
		return true
	}
	event := *s.current
	event.Data = data
	s.current = &event
	return true
}

func (s *anthropicInputUsageStream) Current() *httpclient.StreamEvent { return s.current }
