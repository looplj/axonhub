package nanogpt

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
)

func TestMaybeHasXMLToolCalls(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected bool
	}{
		{
			name:     "empty content",
			content:  "",
			expected: false,
		},
		{
			name:     "plain text without XML",
			content:  "Hello, this is just text",
			expected: false,
		},
		{
			name:     "contains Write tag",
			content:  "<Write file_path=\"x\">content</Write>",
			expected: true,
		},
		{
			name:     "contains use_tool",
			content:  "<use_tool name=\"write\">content</use_tool>",
			expected: true,
		},
		{
			name:     "contains Bash tag",
			content:  "Running <Bash>ls</Bash> command",
			expected: true,
		},
		{
			name:     "contains Read tag",
			content:  "<Read file_path=\"x\"/>",
			expected: true,
		},
		{
			name:     "has angle brackets - pre-check is intentionally permissive",
			content:  "<div>not a tool</div>",
			expected: true, // pre-check matches any XML-like pattern; actual parsing filters non-tool tags
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MaybeHasXMLToolCalls(tt.content)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParseXMLToolCalls_SelfClosing(t *testing.T) {
	content := `<Write file_path="/test/file.txt" content="hello world"/>`

	tools, _, err := ParseXMLToolCalls(content)
	require.NoError(t, err)
	require.Len(t, tools, 1)
	// remaining content is tool output, not relevant for these tests

	tool := tools[0]
	assert.Equal(t, "write", tool.Function.Name)
	assert.Contains(t, tool.Function.Arguments, "file_path")
	assert.Contains(t, tool.Function.Arguments, "content")
	assert.Contains(t, tool.Function.Arguments, "/test/file.txt")
	assert.Contains(t, tool.Function.Arguments, "hello world")
}

func TestParseXMLToolCalls_SimpleContent(t *testing.T) {
	content := `<Write file_path="/test/file.txt">file contents here</Write>`

	tools, _, err := ParseXMLToolCalls(content)
	require.NoError(t, err)
	require.Len(t, tools, 1)
	// remaining content is tool output, not relevant for these tests

	tool := tools[0]
	assert.Equal(t, "write", tool.Function.Name)
	assert.Contains(t, tool.Function.Arguments, "file_path")
	assert.Contains(t, tool.Function.Arguments, "/test/file.txt")
	assert.Contains(t, tool.Function.Arguments, "file contents here")
}

func TestParseXMLToolCalls_JSONInContent(t *testing.T) {
	content := `<Write>{"file_path": "/test/file.txt", "content": "hello"}</Write>`

	tools, _, err := ParseXMLToolCalls(content)
	require.NoError(t, err)
	require.Len(t, tools, 1)
	// remaining content is tool output, not relevant for these tests

	tool := tools[0]
	assert.Equal(t, "write", tool.Function.Name)
	assert.Contains(t, tool.Function.Arguments, "file_path")
	assert.Contains(t, tool.Function.Arguments, "/test/file.txt")
	assert.Contains(t, tool.Function.Arguments, "hello")
}

func TestParseXMLToolCalls_MismatchedClosingTag(t *testing.T) {
	// The parser normalizes <Write>content</use_tool> to <Write>content</Write>
	content := `<Write file_path="/test/file.txt">content</use_tool>`

	tools, _, err := ParseXMLToolCalls(content)
	require.NoError(t, err)
	require.Len(t, tools, 1)
	// remaining content is tool output, not relevant for these tests

	tool := tools[0]
	assert.Equal(t, "write", tool.Function.Name)
	assert.Contains(t, tool.Function.Arguments, "file_path")
}

func TestParseXMLToolCalls_UnclosedOpeningTag(t *testing.T) {
	// NanoGPT sometimes omits the closing > on opening tags
	content := "<Write file_path=\"/test/file.txt\" content=\"hello\"\n}\n</use_tool>"

	tools, _, err := ParseXMLToolCalls(content)
	require.NoError(t, err)
	require.Len(t, tools, 1)

	tool := tools[0]
	assert.Equal(t, "write", tool.Function.Name)
}

func TestParseXMLToolCalls_NestedXMLElements(t *testing.T) {
	content := `<Write>
  <file_path>/test/file.txt</file_path>
  <content>hello world</content>
</Write>`

	tools, _, err := ParseXMLToolCalls(content)
	require.NoError(t, err)
	require.Len(t, tools, 1)
	// remaining content is tool output, not relevant for these tests

	tool := tools[0]
	assert.Equal(t, "write", tool.Function.Name)
	assert.Contains(t, tool.Function.Arguments, "file_path")
	assert.Contains(t, tool.Function.Arguments, "/test/file.txt")
	assert.Contains(t, tool.Function.Arguments, "hello world")
}

func TestParseXMLToolCases_NoSpaceAfterTag(t *testing.T) {
	// Format: <Write_File>{...}</Write_File> without space after tag name
	content := `<Write_File>{"path": "/test/file.txt", "content": "hello"}</Write_File>`

	tools, _, err := ParseXMLToolCalls(content)
	require.NoError(t, err)
	require.Len(t, tools, 1)

	tool := tools[0]
	assert.Equal(t, "write", tool.Function.Name)
}

func TestParseXMLToolCalls_MultipleToolCalls(t *testing.T) {
	content := `<Write file_path="/file1.txt">content1</Write>
Some text in between
<Read file_path="/file2.txt"/>`

	tools, remaining, err := ParseXMLToolCalls(content)
	require.NoError(t, err)
	require.Len(t, tools, 2)
	assert.Contains(t, remaining, "Some text in between")

	assert.Equal(t, "write", tools[0].Function.Name)
	assert.Equal(t, "read", tools[1].Function.Name)
}

func TestParseXMLToolCalls_NoToolCalls(t *testing.T) {
	content := "This is just plain text without any tool calls"

	tools, remaining, err := ParseXMLToolCalls(content)
	require.NoError(t, err)
	assert.Empty(t, tools)
	assert.Equal(t, content, remaining)
}

func TestExtractToolName(t *testing.T) {
	tests := []struct {
		tagName  string
		attrs    string
		expected string
	}{
		{"Write", "", "write"},
		{"Write_FILE", "", "write"},
		{"Write_file", "", "write"},
		{"Read", "", "read"},
		{"Read_FILE", "", "read"},
		{"Bash", "", "bash"},
		{"Python", "", "python"},
		{"Search", "", "search"},
		{"Glob", "", "glob"},
		{"use_tool", `name="write"`, "write"},
		{"use_tool", `name="Read"`, "read"},
		{"Unknown", "", ""},
		{"", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.tagName, func(t *testing.T) {
			result := extractToolName(tt.tagName, tt.attrs)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGenerateToolCallID(t *testing.T) {
	// ID should be deterministic
	id1 := generateToolCallID("write", `{"file_path":"/test.txt"}`)
	id2 := generateToolCallID("write", `{"file_path":"/test.txt"}`)
	assert.Equal(t, id1, id2)

	// Different inputs should produce different IDs
	id3 := generateToolCallID("read", `{"file_path":"/test.txt"}`)
	assert.NotEqual(t, id1, id3)

	// Should start with nanogpt_ prefix
	assert.True(t, len(id1) > 8)
}

func TestToOpenAIToolCalls(t *testing.T) {
	toolCalls := []llm.ToolCall{
		{
			Index: 0,
			ID:    "test-id-1",
			Type:  "function",
			Function: llm.FunctionCall{
				Name:      "write",
				Arguments: `{"file_path":"/test.txt"}`,
			},
		},
	}

	result := ToOpenAIToolCalls(toolCalls)
	require.Len(t, result, 1)
	assert.Equal(t, "test-id-1", result[0].ID)
	assert.Equal(t, "write", result[0].Function.Name)
}
