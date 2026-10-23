package safeguards

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

const requestBody = `{"safeguards":[{"type":"dangerous_tool_use","classifier_context":{"live_cwd":"/project","trusted_directories":["/project"],"secret_field":"omit"}}],"messages":[{"role":"user","content":"Run tests"}]}`

func evaluator(t *testing.T, call ClassifierFunc) *Evaluator {
	t.Helper()
	e, err := New([]byte(requestBody), Config{ClassifierModel: "cheap", Timeout: time.Second}, call)
	if err != nil {
		t.Fatal(err)
	}
	return e
}

func tools() []ToolUse {
	return []ToolUse{{ID: "safe", Name: "Bash", Input: json.RawMessage(`{"command":"go test"}`)}, {ID: "danger", Name: "Bash", Input: json.RawMessage(`{"command":"rm -rf /"}`)}}
}

func TestVerdicts(t *testing.T) {
	cases := []struct{ name, text, safe, danger string }{
		{"valid", `{"verdicts":{"safe":{"flagged":false},"danger":{"flagged":true,"explanation":"destruction"}}}`, "not_flagged", "flagged"},
		{"missing", `{"verdicts":{"safe":{"flagged":false}}}`, "not_flagged", ""},
		{"string boolean", `{"verdicts":{"safe":{"flagged":"false"}}}`, "", ""},
		{"null", `{"verdicts":{"safe":{"flagged":null}}}`, "", ""},
		{"duplicate id", `{"verdicts":{"safe":{"flagged":true},"safe":{"flagged":false}}}`, "", ""},
		{"duplicate flag", `{"verdicts":{"safe":{"flagged":true,"flagged":false}}}`, "", ""},
		{"trailing", `{"verdicts":{"safe":{"flagged":false}}} garbage`, "", ""},
		{"bad explanation", `{"verdicts":{"safe":{"flagged":false,"explanation":42}}}`, "", ""},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			e := evaluator(t, func(ctx context.Context, r ClassifierRequest) (string, error) {
				if r.Model != "cheap" || r.MaxTokens != 4096 || strings.Contains(r.User, "secret_field") {
					t.Fatal("incorrect classifier request")
				}
				if _, ok := ctx.Deadline(); !ok {
					t.Fatal("missing deadline")
				}
				return tt.text, nil
			})
			result := e.Evaluate(context.Background(), tools())[0].Status.ToolUses
			if result["safe"].Outcome != tt.safe || result["danger"].Outcome != tt.danger {
				t.Fatalf("verdicts: %+v", result)
			}
			for _, v := range result {
				if v.Outcome == "" && (v.Type != "unavailable" || v.Reason != "error") {
					t.Fatalf("must fail closed: %+v", v)
				}
			}
		})
	}
}

func TestTimeoutAndCancel(t *testing.T) {
	e := evaluator(t, func(ctx context.Context, _ ClassifierRequest) (string, error) { <-ctx.Done(); return "", ctx.Err() })
	e.timeout = time.Millisecond
	if v := e.Evaluate(context.Background(), tools())[0].Status.ToolUses["safe"]; v.Reason != "timeout" {
		t.Fatal(v)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if v := e.Evaluate(ctx, tools())[0].Status.ToolUses["safe"]; v.Reason != "error" {
		t.Fatal(v)
	}
	e.call = func(context.Context, ClassifierRequest) (string, error) { return "", errors.New("upstream failed") }
	if v := e.Evaluate(context.Background(), tools())[0].Status.ToolUses["safe"]; v.Type != "unavailable" {
		t.Fatal(v)
	}
}

func TestTruncationAndLimits(t *testing.T) {
	calls := 0
	e := evaluator(t, func(context.Context, ClassifierRequest) (string, error) { calls++; return `{"verdicts":{}}`, nil })
	uses := tools()
	uses[0].Truncated = true
	uses[1].Input = json.RawMessage(`{"s":"` + strings.Repeat("x", maxToolBytes) + `"}`)
	result := e.Evaluate(context.Background(), uses)[0].Status.ToolUses
	if calls != 0 || result["safe"].Reason != "truncated" || result["danger"].Reason != "input_too_long" {
		t.Fatal(result, calls)
	}
	uses = tools()
	uses[1].ID = "safe"
	e.Evaluate(context.Background(), uses)
	if calls != 0 {
		t.Fatal("duplicate tool IDs reviewed")
	}
}

func TestTranscriptBound(t *testing.T) {
	var messages []json.RawMessage
	for range 100 {
		m, _ := json.Marshal(map[string]string{"role": "user", "content": strings.Repeat("a", 1800)})
		messages = append(messages, m)
	}
	text := renderTranscript(messages, 24000)
	if len(text) > 24000 || !strings.Contains(text, "earlier turns omitted") {
		t.Fatal("unbounded or missing omission marker")
	}
	if renderTranscript(nil, 24000) != "" {
		t.Fatal("empty transcript")
	}
}

func TestOptIn(t *testing.T) {
	e, err := New([]byte(`{"messages":[]}`), Config{ClassifierModel: "cheap"}, nil)
	if err != nil || e != nil {
		t.Fatal(e, err)
	}
	e, err = New([]byte(requestBody), Config{}, nil)
	if err != nil || e != nil {
		t.Fatal(e, err)
	}
}

func TestAnnotateResponse(t *testing.T) {
	e := evaluator(t, func(context.Context, ClassifierRequest) (string, error) {
		return `{"verdicts":{"safe":{"flagged":true,"explanation":"reviewed"}}}`, nil
	})
	body := []byte(`{"content":[{"type":"tool_use","id":"safe","name":"Bash","input":{}}],"stop_reason":"tool_use","safeguard_results":[{"type":"other","status":{"type":"available"}},{"type":"dangerous_tool_use","status":{"type":"unsupported"}}]}`)
	result, err := e.AnnotateResponse(context.Background(), body)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(result), `"outcome":"flagged"`) || !strings.Contains(string(result), `"type":"other"`) || strings.Count(string(result), `"type":"dangerous_tool_use"`) != 1 {
		t.Fatal(string(result))
	}
	result, err = e.AnnotateResponse(context.Background(), []byte(strings.ReplaceAll(string(body), "tool_use\",\"safeguard_results", "max_tokens\",\"safeguard_results")))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(result), `"reason":"truncated"`) {
		t.Fatal(string(result))
	}
}

func TestStreamParallel(t *testing.T) {
	calls := 0
	e := evaluator(t, func(_ context.Context, r ClassifierRequest) (string, error) {
		calls++
		if !strings.Contains(r.User, `"command":"go test"`) {
			t.Fatal(r.User)
		}
		return `{"verdicts":{"safe":{"flagged":false},"danger":{"flagged":true}}}`, nil
	})
	stream := NewStream(e)
	events := []string{
		`{"type":"content_block_start","index":0,"content_block":{"type":"tool_use","id":"safe","name":"Bash","input":{}}}`,
		`{"type":"content_block_start","index":1,"content_block":{"type":"tool_use","id":"danger","name":"Bash","input":{}}}`,
		`{"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"{\"command\":"}}`,
		`{"type":"content_block_delta","index":1,"delta":{"type":"input_json_delta","partial_json":"{\"command\":\"rm /\"}"}}`,
		`{"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"\"go test\"}"}}`,
		`{"type":"content_block_stop","index":0}`, `{"type":"content_block_stop","index":1}`,
	}
	for _, event := range events {
		out, err := stream.Observe(context.Background(), []byte(event))
		if err != nil || string(out) != event {
			t.Fatal(string(out), err)
		}
	}
	final := []byte(`{"type":"message_delta","delta":{"stop_reason":"tool_use"},"usage":{"output_tokens":10}}`)
	for range 2 {
		out, err := stream.Observe(context.Background(), final)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(out), `"outcome":"flagged"`) || !strings.Contains(string(out), `"outcome":"not_flagged"`) || !strings.Contains(string(out), `"output_tokens":10`) {
			t.Fatal(string(out))
		}
	}
	if calls != 1 {
		t.Fatal(calls)
	}
}

func TestStreamIncompleteFailsClosed(t *testing.T) {
	for _, partial := range []string{`{`, `{"a":1}`} {
		calls := 0
		e := evaluator(t, func(context.Context, ClassifierRequest) (string, error) { calls++; return "", nil })
		stream := NewStream(e)
		_, _ = stream.Observe(context.Background(), []byte(`{"type":"content_block_start","index":0,"content_block":{"type":"tool_use","id":"safe","name":"Bash","input":{}}}`))
		event, _ := json.Marshal(map[string]any{"type": "content_block_delta", "index": 0, "delta": map[string]string{"type": "input_json_delta", "partial_json": partial}})
		_, _ = stream.Observe(context.Background(), event)
		out, err := stream.Observe(context.Background(), []byte(`{"type":"message_delta","delta":{"stop_reason":"max_tokens"}}`))
		if err != nil || calls != 0 || !strings.Contains(string(out), `"reason":"truncated"`) {
			t.Fatal(string(out), calls, err)
		}
	}
}

func TestStreamMalformedEventsReturnErrors(t *testing.T) {
	for _, event := range []string{
		`{"type":"content_block_start","index":"bad"}`,
		`{"type":"content_block_delta","index":"bad"}`,
		`{"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":42}}`,
		`{"type":"message_delta","delta":[]}`,
	} {
		t.Run(event, func(t *testing.T) {
			calls := 0
			e := evaluator(t, func(context.Context, ClassifierRequest) (string, error) {
				calls++
				return `{"verdicts":{"safe":{"flagged":false}}}`, nil
			})
			stream := NewStream(e)
			_, err := stream.Observe(t.Context(), []byte(`{"type":"content_block_start","index":0,"content_block":{"type":"tool_use","id":"safe","name":"Bash","input":{}}}`))
			if err != nil {
				t.Fatal(err)
			}
			out, err := stream.Observe(t.Context(), []byte(event))
			if err == nil || out != nil || calls != 0 {
				t.Fatalf("malformed event must fail before review: output %s, error %v, calls %d", out, err, calls)
			}
		})
	}
}

func TestInvalidToolInputsFailClosed(t *testing.T) {
	for _, raw := range []string{`null`, `[]`, `"command"`, `{"command":"rm /","command":"ls"}`} {
		calls := 0
		e := evaluator(t, func(context.Context, ClassifierRequest) (string, error) {
			calls++
			return `{"verdicts":{"safe":{"flagged":false}}}`, nil
		})
		uses := tools()[:1]
		uses[0].Input = json.RawMessage(raw)
		result := e.Evaluate(context.Background(), uses)[0].Status.ToolUses["safe"]
		if calls != 0 || result.Type != "unavailable" {
			t.Fatalf("input %s: calls %d verdict %+v", raw, calls, result)
		}
	}
}

func TestStreamReplacesUpstreamApprovals(t *testing.T) {
	calls := 0
	e := evaluator(t, func(context.Context, ClassifierRequest) (string, error) {
		calls++
		return `{"verdicts":{"safe":{"flagged":true,"explanation":"local denial"}}}`, nil
	})
	stream := NewStream(e)
	for _, event := range []string{
		`{"type":"message_start","message":{"safeguard_results":[{"type":"dangerous_tool_use","status":{"type":"available","tool_uses":{"safe":{"type":"evaluated","outcome":"not_flagged"}}}},{"type":"other","status":{"type":"available"}}]}}`,
		`{"type":"message_delta","delta":{"safeguard_results":[{"type":"dangerous_tool_use","status":{"type":"available","tool_uses":{"safe":{"type":"evaluated","outcome":"not_flagged"}}}}]}}`,
	} {
		out, err := stream.Observe(context.Background(), []byte(event))
		if err != nil || strings.Contains(string(out), "not_flagged") || strings.Contains(string(out), "dangerous_tool_use") {
			t.Fatal(string(out), err)
		}
		if strings.Contains(event, `"other"`) && !strings.Contains(string(out), `"other"`) {
			t.Fatal("lost unknown safeguard")
		}
	}
	if calls != 0 {
		t.Fatal("premature classification")
	}
	_, _ = stream.Observe(context.Background(), []byte(`{"type":"content_block_start","index":0,"content_block":{"type":"tool_use","id":"safe","name":"Bash","input":{}}}`))
	_, _ = stream.Observe(context.Background(), []byte(`{"type":"content_block_stop","index":0}`))
	out, err := stream.Observe(context.Background(), []byte(`{"type":"message_delta","delta":{"stop_reason":"tool_use","safeguard_results":[{"type":"dangerous_tool_use","status":{"type":"available","tool_uses":{"safe":{"type":"evaluated","outcome":"not_flagged"}}}}]}}`))
	if err != nil || calls != 1 || strings.Contains(string(out), "not_flagged") || !strings.Contains(string(out), `"outcome":"flagged"`) {
		t.Fatal(string(out), calls, err)
	}
}

func TestTranscriptTinyBudgets(t *testing.T) {
	messages := []json.RawMessage{json.RawMessage(`{"role":"user","content":"hello"}`), json.RawMessage(`{"role":"assistant","content":"a longer conversation"}`)}
	for budget := 0; budget <= 100; budget++ {
		text := renderTranscript(messages, budget)
		if len(text) > budget {
			t.Fatalf("budget %d produced %d bytes: %q", budget, len(text), text)
		}
	}
}

func TestTranscriptMalformedTextDoesNotRepeat(t *testing.T) {
	messages := []json.RawMessage{json.RawMessage(`{"role":"user","content":[{"type":"text","text":"Explicit authorization"},{"type":"text","text":42},{"type":"text"}]}`)}
	text := renderTranscript(messages, 24000)
	if strings.Count(text, "Explicit authorization") != 1 {
		t.Fatalf("malformed text reused preceding authorization: %s", text)
	}
}
