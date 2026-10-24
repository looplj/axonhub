package api

import (
	"encoding/json"
	"math"
	"strconv"
	"strings"
	"sync"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
	"github.com/tiktoken-go/tokenizer"

	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/streams"
)

// Count only reads the codec vocabulary and uses regexp2's concurrent-safe
// matching API. Never call Decode on this shared codec: it lazily mutates the
// reverse vocabulary. Initialization is the only operation we serialize.
var inputUsageTokenizer = sync.OnceValues(func() (tokenizer.Codec, error) {
	return tokenizer.Get(tokenizer.O200kBase)
})

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
	codec, err := inputUsageTokenizer()
	if err != nil {
		return 0, err
	}
	count, err := codec.Count(strings.Join(parts, "\n"))
	return int64(count), err
}

type anthropicInputUsageStream struct {
	streams.Stream[*httpclient.StreamEvent]

	calibration                          *inputUsageCalibration
	actual                               int64
	terminal, stopped, failed, completed bool
	body                                 []byte
	handled                              bool
	current                              *httpclient.StreamEvent
}

// Wrap after persistence: only the client sees the estimate. Final usage, stored
// responses and costs remain authoritative, even when the stream is interrupted.
func wrapAnthropicInputUsage(source streams.Stream[*httpclient.StreamEvent], request *httpclient.Request, calibration *inputUsageCalibration) streams.Stream[*httpclient.StreamEvent] {
	if request == nil || !strings.HasSuffix(request.Path, "/messages") {
		return source
	}
	return preserveStreamDelivery(&anthropicInputUsageStream{Stream: source, body: request.Body, calibration: calibration}, source)
}

func (s *anthropicInputUsageStream) Next() bool {
	if !s.Stream.Next() {
		if !s.completed && !s.failed && s.terminal && s.stopped && s.Stream.Err() == nil && s.calibration != nil {
			s.calibration.complete(s.actual)
		}
		s.completed = true
		return false
	}
	s.current = s.Stream.Current()
	if s.current == nil || !json.Valid(s.current.Data) {
		if s.current != nil && len(s.current.Data) > 0 {
			s.failed = true
		}
		return true
	}
	eventType := gjson.GetBytes(s.current.Data, "type").String()
	switch eventType {
	case "message_delta":
		if usage := gjson.GetBytes(s.current.Data, "usage"); usage.Get("input_tokens").Exists() {
			s.actual = finalAnthropicInputTokens(usage)
		}
		switch gjson.GetBytes(s.current.Data, "delta.stop_reason").String() {
		case "end_turn", "tool_use", "stop_sequence":
			s.terminal = true
		}
	case "message_stop":
		s.stopped = true
	case "error":
		s.failed = true
	}
	if s.handled || eventType != "message_start" {
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
	if s.calibration != nil {
		estimate = s.calibration.estimate(s.body, gjson.GetBytes(s.current.Data, "message.model").String(), estimate)
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

// finalAnthropicInputTokens requires explicit provider input usage and includes
// cache reads/writes once. Missing, fractional, negative or overflowing values
// cannot become a calibration sample.
func finalAnthropicInputTokens(usage gjson.Result) int64 {
	if !usage.Get("input_tokens").Exists() {
		return 0
	}
	var total int64
	for _, key := range []string{"input_tokens", "cache_read_input_tokens", "cache_creation_input_tokens"} {
		value := usage.Get(key)
		if !value.Exists() {
			continue
		}
		if value.Type != gjson.Number {
			return 0
		}
		n, err := strconv.ParseInt(value.Raw, 10, 64)
		if err != nil || n < 0 || total > math.MaxInt64-n {
			return 0
		}
		total += n
	}
	return total
}
