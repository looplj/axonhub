package bailian

import (
	"encoding/json"
	"testing"

	"github.com/looplj/axonhub/llm"
)

func TestSanitizeForBailian_StripResponseFormat(t *testing.T) {
	req := &llm.Request{
		ResponseFormat: &llm.ResponseFormat{
			Type: "json_object",
		},
		Messages: []llm.Message{},
	}

	result := sanitizeForBailian(req)

	if result.ResponseFormat != nil {
		t.Errorf("expected ResponseFormat to be nil, got %+v", result.ResponseFormat)
	}
}

func TestSanitizeForBailian_EmptyArguments(t *testing.T) {
	args := ""
	req := &llm.Request{
		Messages: []llm.Message{
			{
				Role: "assistant",
				ToolCalls: []llm.ToolCall{
					{
						Function: llm.FunctionCall{
							Name:      "test_func",
							Arguments: args,
						},
					},
				},
			},
		},
	}

	result := sanitizeForBailian(req)

	expected := "{}"
	if result.Messages[0].ToolCalls[0].Function.Arguments != expected {
		t.Errorf("expected empty arguments to become %q, got %q", expected, result.Messages[0].ToolCalls[0].Function.Arguments)
	}
}

func TestSanitizeForBailian_NonJSONArguments(t *testing.T) {
	req := &llm.Request{
		Messages: []llm.Message{
			{
				Role: "assistant",
				ToolCalls: []llm.ToolCall{
					{
						Function: llm.FunctionCall{
							Name:      "test_func",
							Arguments: "not json at all",
						},
					},
				},
			},
		},
	}

	result := sanitizeForBailian(req)

	args := result.Messages[0].ToolCalls[0].Function.Arguments
	if !json.Valid([]byte(args)) {
		t.Errorf("expected arguments to be valid JSON, got %q", args)
	}
}

func TestSanitizeForBailian_ValidJSONArgumentsUnchanged(t *testing.T) {
	original := `{"key": "value"}`
	req := &llm.Request{
		Messages: []llm.Message{
			{
				Role: "assistant",
				ToolCalls: []llm.ToolCall{
					{
						Function: llm.FunctionCall{
							Name:      "test_func",
							Arguments: original,
						},
					},
				},
			},
		},
	}

	result := sanitizeForBailian(req)

	if result.Messages[0].ToolCalls[0].Function.Arguments != original {
		t.Errorf("expected valid JSON arguments to be unchanged, got %q", result.Messages[0].ToolCalls[0].Function.Arguments)
	}
}

func TestSanitizeForBailian_NilRequest(t *testing.T) {
	result := sanitizeForBailian(nil)
	if result != nil {
		t.Errorf("expected nil result for nil request")
	}
}
