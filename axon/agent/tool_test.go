package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockTool is a test implementation of the Tool interface
type mockTool struct {
	name        string
	description string
	params      jsonschema.Schema
}

func (m *mockTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:        m.name,
		Description: m.description,
		Parameters:  m.params,
	}
}

func (m *mockTool) Execute(ctx context.Context, arguments json.RawMessage) ToolResult {
	text := "executed"
	return ToolResult{Content: Content{Text: &text}}
}

func newMockTool(name string) *mockTool {
	return &mockTool{
		name:        name,
		description: "Test tool " + name,
		params: jsonschema.Schema{
			Type: "object",
		},
	}
}

func TestNewToolRegistry(t *testing.T) {
	registry := NewToolRegistry()
	require.NotNil(t, registry)
	assert.NotNil(t, registry.tools)
	assert.NotNil(t, registry.order)
	assert.NotNil(t, registry.validator)
	assert.Empty(t, registry.order)
	assert.Empty(t, registry.tools)
}

func TestToolRegistry_Register(t *testing.T) {
	t.Run("register single tool", func(t *testing.T) {
		registry := NewToolRegistry()
		tool := newMockTool("tool1")

		registry.Register(tool)

		assert.Len(t, registry.tools, 1)
		assert.Len(t, registry.order, 1)
		assert.Equal(t, "tool1", registry.order[0])
	})

	t.Run("register multiple tools maintains order", func(t *testing.T) {
		registry := NewToolRegistry()
		tools := []string{"tool1", "tool2", "tool3", "tool4", "tool5"}

		for _, name := range tools {
			registry.Register(newMockTool(name))
		}

		assert.Len(t, registry.tools, 5)
		assert.Len(t, registry.order, 5)
		assert.Equal(t, tools, registry.order)
	})

	t.Run("re-register tool does not change order", func(t *testing.T) {
		registry := NewToolRegistry()
		registry.Register(newMockTool("tool1"))
		registry.Register(newMockTool("tool2"))
		registry.Register(newMockTool("tool3"))

		// Re-register tool2
		registry.Register(newMockTool("tool2"))

		assert.Len(t, registry.tools, 3)
		assert.Len(t, registry.order, 3)
		// Order should remain: tool1, tool2, tool3
		assert.Equal(t, []string{"tool1", "tool2", "tool3"}, registry.order)
	})

	t.Run("register tool with same name updates tool but keeps order", func(t *testing.T) {
		registry := NewToolRegistry()
		originalTool := newMockTool("tool1")
		registry.Register(originalTool)

		newTool := &mockTool{
			name:        "tool1",
			description: "Updated description",
			params:      jsonschema.Schema{Type: "object"},
		}
		registry.Register(newTool)

		// Order should still be 1
		assert.Len(t, registry.order, 1)
		// But tool should be updated
		retrieved, ok := registry.Get("tool1")
		require.True(t, ok)
		assert.Equal(t, "Updated description", retrieved.Definition().Description)
	})
}

func TestToolRegistry_Get(t *testing.T) {
	t.Run("get existing tool", func(t *testing.T) {
		registry := NewToolRegistry()
		tool := newMockTool("tool1")
		registry.Register(tool)

		retrieved, ok := registry.Get("tool1")

		assert.True(t, ok)
		assert.Equal(t, tool, retrieved)
	})

	t.Run("get non-existent tool", func(t *testing.T) {
		registry := NewToolRegistry()

		retrieved, ok := registry.Get("nonexistent")

		assert.False(t, ok)
		assert.Nil(t, retrieved)
	})

	t.Run("get returns correct tool from multiple", func(t *testing.T) {
		registry := NewToolRegistry()
		tool1 := newMockTool("tool1")
		tool2 := newMockTool("tool2")
		tool3 := newMockTool("tool3")

		registry.Register(tool1)
		registry.Register(tool2)
		registry.Register(tool3)

		retrieved, ok := registry.Get("tool2")
		assert.True(t, ok)
		assert.Equal(t, tool2, retrieved)
	})
}

func TestToolRegistry_List(t *testing.T) {
	t.Run("list empty registry", func(t *testing.T) {
		registry := NewToolRegistry()

		list := registry.List()

		assert.Empty(t, list)
		assert.NotNil(t, list)
	})

	t.Run("list returns tools in registration order", func(t *testing.T) {
		registry := NewToolRegistry()
		tools := []string{"alpha", "beta", "gamma", "delta"}

		for _, name := range tools {
			registry.Register(newMockTool(name))
		}

		list := registry.List()

		assert.Equal(t, tools, list)
	})

	t.Run("list returns copy of order slice", func(t *testing.T) {
		registry := NewToolRegistry()
		registry.Register(newMockTool("tool1"))
		registry.Register(newMockTool("tool2"))

		list1 := registry.List()
		list2 := registry.List()

		// Modify list1
		list1[0] = "modified"

		// list2 should not be affected
		assert.Equal(t, "tool1", list2[0])
		// registry order should not be affected
		assert.Equal(t, "tool1", registry.order[0])
	})

	t.Run("list after re-registration maintains original order", func(t *testing.T) {
		registry := NewToolRegistry()
		registry.Register(newMockTool("first"))
		registry.Register(newMockTool("second"))
		registry.Register(newMockTool("third"))
		registry.Register(newMockTool("second")) // Re-register

		list := registry.List()

		assert.Equal(t, []string{"first", "second", "third"}, list)
	})
}

func TestToolRegistry_Definitions(t *testing.T) {
	t.Run("definitions from empty registry", func(t *testing.T) {
		registry := NewToolRegistry()

		defs := registry.Definitions()

		assert.Empty(t, defs)
		assert.NotNil(t, defs)
	})

	t.Run("definitions in registration order", func(t *testing.T) {
		registry := NewToolRegistry()
		tools := []string{"tool-c", "tool-a", "tool-b"}

		for _, name := range tools {
			registry.Register(newMockTool(name))
		}

		defs := registry.Definitions()

		require.Len(t, defs, 3)
		assert.Equal(t, "tool-c", defs[0].Name)
		assert.Equal(t, "tool-a", defs[1].Name)
		assert.Equal(t, "tool-b", defs[2].Name)
	})

	t.Run("definitions after re-registration", func(t *testing.T) {
		registry := NewToolRegistry()
		registry.Register(&mockTool{
			name:        "tool1",
			description: "Original",
			params:      jsonschema.Schema{Type: "object"},
		})
		registry.Register(newMockTool("tool2"))

		// Re-register tool1 with new description
		registry.Register(&mockTool{
			name:        "tool1",
			description: "Updated",
			params:      jsonschema.Schema{Type: "object"},
		})

		defs := registry.Definitions()

		require.Len(t, defs, 2)
		assert.Equal(t, "tool1", defs[0].Name)
		assert.Equal(t, "Updated", defs[0].Description)
		assert.Equal(t, "tool2", defs[1].Name)
	})
}

func TestToolRegistry_ValidateArguments(t *testing.T) {
	t.Run("validate with non-existent tool", func(t *testing.T) {
		registry := NewToolRegistry()

		err := registry.ValidateArguments("nonexistent", json.RawMessage(`{}`))

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("validate with invalid json", func(t *testing.T) {
		registry := NewToolRegistry()
		registry.Register(newMockTool("tool1"))

		err := registry.ValidateArguments("tool1", json.RawMessage(`{invalid`))

		assert.Error(t, err)
	})

	t.Run("validate with valid json", func(t *testing.T) {
		registry := NewToolRegistry()
		registry.Register(newMockTool("tool1"))

		err := registry.ValidateArguments("tool1", json.RawMessage(`{"key": "value"}`))

		// Should not error for object schema
		assert.NoError(t, err)
	})
}

func TestToolRegistry_ConcurrentAccess(t *testing.T) {
	t.Run("concurrent register operations", func(t *testing.T) {
		registry := NewToolRegistry()
		var wg sync.WaitGroup

		// Register 100 tools concurrently
		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				name := fmt.Sprintf("tool-%d", idx)
				registry.Register(newMockTool(name))
			}(i)
		}

		wg.Wait()

		assert.Len(t, registry.tools, 100)
		assert.Len(t, registry.order, 100)
	})

	t.Run("concurrent read and write operations", func(t *testing.T) {
		registry := NewToolRegistry()
		var wg sync.WaitGroup

		// Pre-register some tools
		for i := 0; i < 10; i++ {
			registry.Register(newMockTool(fmt.Sprintf("tool-%d", i)))
		}

		// Concurrent reads
		for i := 0; i < 50; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_ = registry.List()
				_ = registry.Definitions()
				_, _ = registry.Get("tool-1")
			}()
		}

		// Concurrent writes
		for i := 10; i < 20; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				registry.Register(newMockTool(fmt.Sprintf("tool-%d", idx)))
			}(i)
		}

		wg.Wait()

		assert.Len(t, registry.tools, 20)
	})
}

func TestToolResult_MarshalJSON(t *testing.T) {
	t.Run("marshal result with error", func(t *testing.T) {
		text := "ignored"
		result := ToolResult{
			Content: Content{Text: &text},
			Error:   errors.New("something went wrong"),
		}

		data, err := json.Marshal(result)
		require.NoError(t, err)

		var unmarshaled toolResultJSON
		err = json.Unmarshal(data, &unmarshaled)
		require.NoError(t, err)

		assert.Equal(t, "something went wrong", unmarshaled.Error)
	})

	t.Run("marshal result without error", func(t *testing.T) {
		text := "success"
		result := ToolResult{
			Content: Content{Text: &text},
			Error:   nil,
		}

		data, err := json.Marshal(result)
		require.NoError(t, err)

		assert.Contains(t, string(data), "success")
		assert.NotContains(t, string(data), "error")
	})
}

func TestToolResult_UnmarshalJSON(t *testing.T) {
	t.Run("unmarshal result with error", func(t *testing.T) {
		jsonData := `{"content":"test error content","error":"test error"}`

		var result ToolResult
		err := json.Unmarshal([]byte(jsonData), &result)
		require.NoError(t, err)

		assert.Error(t, result.Error)
		assert.Equal(t, "test error", result.Error.Error())
		assert.Equal(t, "test error content", result.Content.String())
	})

	t.Run("unmarshal result without error", func(t *testing.T) {
		jsonData := `{"content":"success content"}`

		var result ToolResult
		err := json.Unmarshal([]byte(jsonData), &result)
		require.NoError(t, err)

		assert.NoError(t, result.Error)
		assert.Equal(t, "success content", result.Content.String())
	})
}
