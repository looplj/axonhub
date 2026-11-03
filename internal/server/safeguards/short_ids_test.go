package safeguards

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestPrimaryShortIDsMapToOriginalTools(t *testing.T) {
	uses := []ToolUse{
		{ID: "call_1485e0bfabd34851ad2bd5c0718d4a88", Name: "Bash", Input: json.RawMessage(`{"command":"rm /project/data"}`)},
		{ID: "t0", Name: "Read", Input: json.RawMessage(`{"path":"/project/README.md"}`)},
		{ID: "t1", Name: "Write", Input: json.RawMessage(`{"path":"/project/config"}`)},
	}
	original := append([]ToolUse(nil), uses...)
	calls := 0
	e := evaluator(t, func(_ context.Context, r ClassifierRequest) (string, error) {
		calls++
		if r.System != ClassifierSystem {
			t.Fatal("short IDs changed the classifier policy or output contract")
		}
		_, encoded, found := strings.Cut(r.User, "\n\nProposed tool calls:\n")
		if !found {
			t.Fatal("missing proposed tools")
		}
		var proposed []struct {
			ID    string          `json:"id"`
			Name  string          `json:"name"`
			Input json.RawMessage `json:"input"`
		}
		if err := json.Unmarshal([]byte(encoded), &proposed); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(proposedIDs(t, r), []string{"t0", "t1", "t2"}) {
			t.Fatalf("unexpected primary IDs: %v", proposedIDs(t, r))
		}
		for i, tool := range proposed {
			if tool.Name != uses[i].Name || string(tool.Input) != string(uses[i].Input) {
				t.Fatalf("short IDs changed tool %d: %+v", i, tool)
			}
		}
		// Response object order must not determine which original tool is denied.
		return `{"verdicts":{"t2":{"flagged":true,"explanation":"third"},"t0":{"flagged":true,"explanation":"first"},"t1":{"flagged":false}}}`, nil
	})
	got := e.Evaluate(t.Context(), uses)[0].Status.ToolUses
	want := map[string]Verdict{
		uses[0].ID: {Type: "evaluated", Outcome: "flagged", Explanation: "first"},
		uses[1].ID: {Type: "evaluated", Outcome: "not_flagged"},
		uses[2].ID: {Type: "evaluated", Outcome: "flagged", Explanation: "third"},
	}
	if calls != 1 || !reflect.DeepEqual(got, want) {
		t.Fatalf("calls %d, verdicts %+v, want %+v", calls, got, want)
	}
	if !reflect.DeepEqual(uses, original) {
		t.Fatal("review mutated original tool calls")
	}
}

func TestPrimaryShortIDsPreserveOriginalIDsAcrossFallbacks(t *testing.T) {
	uses := []ToolUse{
		{ID: "t1", Name: "Bash", Input: json.RawMessage(`{"command":"first"}`)},
		{ID: "t0", Name: "Bash", Input: json.RawMessage(`{"command":"second"}`)},
		{ID: "call_final", Name: "Bash", Input: json.RawMessage(`{"command":"third"}`)},
	}
	var called []string
	e := fallbackEvaluator(t, Config{ClassifierModel: "primary", FallbackModels: []string{"backup", "last"}}, func(_ context.Context, r ClassifierRequest) (string, error) {
		called = append(called, r.Model)
		switch r.Model {
		case "primary":
			if !reflect.DeepEqual(proposedIDs(t, r), []string{"t0", "t1", "t2"}) {
				t.Fatal("unexpected primary IDs")
			}
			// Alias t1 approves original t0; missing first and third verdicts retry.
			return `{"verdicts":{"t1":{"flagged":false}}}`, nil
		case "backup":
			if !reflect.DeepEqual(proposedIDs(t, r), []string{"t1", "call_final"}) {
				t.Fatalf("backup IDs: %v", proposedIDs(t, r))
			}
			// Here t1 is an original ID. The stale primary alias t2 must be ignored.
			return `{"verdicts":{"t1":{"flagged":true,"explanation":"first"},"t2":{"flagged":false}}}`, nil
		case "last":
			if !reflect.DeepEqual(proposedIDs(t, r), []string{"call_final"}) {
				t.Fatalf("last reviewer IDs: %v", proposedIDs(t, r))
			}
			return `{"verdicts":{"call_final":{"flagged":true,"explanation":"third"}}}`, nil
		default:
			t.Fatalf("unexpected reviewer %q", r.Model)
			return "", nil
		}
	})
	got := e.Evaluate(t.Context(), uses)[0].Status.ToolUses
	want := map[string]Verdict{
		"t1":         {Type: "evaluated", Outcome: "flagged", Explanation: "first"},
		"t0":         {Type: "evaluated", Outcome: "not_flagged"},
		"call_final": {Type: "evaluated", Outcome: "flagged", Explanation: "third"},
	}
	if !reflect.DeepEqual(called, []string{"primary", "backup", "last"}) || !reflect.DeepEqual(got, want) {
		t.Fatalf("calls %v, verdicts %+v, want %+v", called, got, want)
	}
}
