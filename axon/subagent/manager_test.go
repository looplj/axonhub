package subagent

import (
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewManager(t *testing.T) {
	t.Run("with valid fs", func(t *testing.T) {
		fsys := fstest.MapFS{}
		m := NewManager(fsys)
		assert.NotNil(t, m)
		assert.NotNil(t, m.agents)
	})
}

func TestNewManagerFromPath(t *testing.T) {
	t.Run("with empty path", func(t *testing.T) {
		m := NewManagerFromPath("")
		assert.NotNil(t, m)
		assert.Nil(t, m.fsys)
	})
}

func TestManager_Load(t *testing.T) {
	t.Run("nil fs", func(t *testing.T) {
		m := NewManager(nil)
		err := m.Load()
		assert.NoError(t, err)
	})

	t.Run("empty directory", func(t *testing.T) {
		fsys := fstest.MapFS{}
		m := NewManager(fsys)
		err := m.Load()
		require.NoError(t, err)
		assert.Empty(t, m.agents)
	})

	t.Run("skips non-markdown files", func(t *testing.T) {
		fsys := fstest.MapFS{
			"test.txt": &fstest.MapFile{
				Data: []byte("some content"),
			},
			"readme.md": &fstest.MapFile{
				Data: []byte("no frontmatter"),
			},
		}
		m := NewManager(fsys)
		err := m.Load()
		require.NoError(t, err)
		assert.Empty(t, m.agents)
	})

	t.Run("skips directories", func(t *testing.T) {
		fsys := fstest.MapFS{
			"subdir/agent.md": &fstest.MapFile{
				Data: []byte("---\nmodel: test\n---\ncontent"),
			},
		}
		m := NewManager(fsys)
		err := m.Load()
		require.NoError(t, err)
		assert.Empty(t, m.agents)
	})

	t.Run("loads single agent", func(t *testing.T) {
		fsys := fstest.MapFS{
			"pr-reviewer.md": &fstest.MapFile{
				Data: []byte(`---
hidden: true
model: deepseek-chat
color: "#E67E22"
tools:
  "*": false
  "github-pr-search": true
---

You are a duplicate PR detection agent. When a PR is opened, your job is to search for potentially duplicate or related open PRs.`),
			},
		}
		m := NewManager(fsys)
		err := m.Load()
		require.NoError(t, err)

		def, ok := m.Get("pr-reviewer")
		require.True(t, ok)
		assert.Equal(t, "pr-reviewer", def.Name)
		assert.True(t, def.Hidden)
		assert.Equal(t, "deepseek-chat", def.Model)
		assert.Equal(t, "#E67E22", def.Color)
		assert.Equal(t, map[string]bool{
			"*":                false,
			"github-pr-search": true,
		}, def.Tools)
		assert.Contains(t, def.Description, "duplicate PR detection agent")
	})

	t.Run("loads multiple agents", func(t *testing.T) {
		fsys := fstest.MapFS{
			"agent1.md": &fstest.MapFile{
				Data: []byte("---\nmodel: model-a\n---\nAgent 1 description"),
			},
			"agent2.md": &fstest.MapFile{
				Data: []byte("---\nmodel: model-b\nhidden: true\n---\nAgent 2 description"),
			},
			"other.txt": &fstest.MapFile{
				Data: []byte("ignored"),
			},
		}
		m := NewManager(fsys)
		err := m.Load()
		require.NoError(t, err)

		list := m.List()
		assert.Len(t, list, 2)

		visible := m.ListVisible()
		assert.Len(t, visible, 1)
		assert.Equal(t, "agent1", visible[0].Name)
	})

	t.Run("handles string tool values", func(t *testing.T) {
		fsys := fstest.MapFS{
			"agent.md": &fstest.MapFile{
				Data: []byte(`---
tools:
  "read": "true"
  "write": "false"
  "search": true
---
content`),
			},
		}
		m := NewManager(fsys)
		err := m.Load()
		require.NoError(t, err)

		def, ok := m.Get("agent")
		require.True(t, ok)
		assert.True(t, def.Tools["read"])
		assert.False(t, def.Tools["write"])
		assert.True(t, def.Tools["search"])
	})

	t.Run("handles ... end marker", func(t *testing.T) {
		fsys := fstest.MapFS{
			"agent.md": &fstest.MapFile{
				Data: []byte("---\nmodel: test\n...\ncontent here"),
			},
		}
		m := NewManager(fsys)
		err := m.Load()
		require.NoError(t, err)

		def, ok := m.Get("agent")
		require.True(t, ok)
		assert.Equal(t, "test", def.Model)
		assert.Equal(t, "content here", def.Description)
	})

	t.Run("handles BOM prefix", func(t *testing.T) {
		fsys := fstest.MapFS{
			"agent.md": &fstest.MapFile{
				Data: []byte("\ufeff---\nmodel: test\n---\ncontent"),
			},
		}
		m := NewManager(fsys)
		err := m.Load()
		require.NoError(t, err)

		def, ok := m.Get("agent")
		require.True(t, ok)
		assert.Equal(t, "test", def.Model)
	})

	t.Run("returns error on invalid YAML", func(t *testing.T) {
		fsys := fstest.MapFS{
			"agent.md": &fstest.MapFile{
				Data: []byte("---\nmodel: [invalid yaml\n---\ncontent"),
			},
		}
		m := NewManager(fsys)
		err := m.Load()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to parse front matter")
	})
}

func TestParseAgentContent(t *testing.T) {
	t.Run("no frontmatter", func(t *testing.T) {
		def, err := parseAgentContent("just plain text")
		assert.NoError(t, err)
		assert.Nil(t, def)
	})

	t.Run("empty content", func(t *testing.T) {
		def, err := parseAgentContent("")
		assert.NoError(t, err)
		assert.Nil(t, def)
	})

	t.Run("unclosed frontmatter", func(t *testing.T) {
		def, err := parseAgentContent("---\nmodel: test\nno end marker")
		assert.NoError(t, err)
		assert.Nil(t, def)
	})

	t.Run("minimal frontmatter", func(t *testing.T) {
		def, err := parseAgentContent("---\n---\nsome description")
		require.NoError(t, err)
		require.NotNil(t, def)
		assert.False(t, def.Hidden)
		assert.Empty(t, def.Model)
		assert.Empty(t, def.Color)
		assert.Empty(t, def.Tools)
		assert.Equal(t, "some description", def.Description)
	})

	t.Run("full frontmatter", func(t *testing.T) {
		content := `---
hidden: true
model: gpt-4
color: blue
tools:
  read: true
  write: false
---

This is the agent description.
It can span multiple lines.`

		def, err := parseAgentContent(content)
		require.NoError(t, err)
		require.NotNil(t, def)
		assert.True(t, def.Hidden)
		assert.Equal(t, "gpt-4", def.Model)
		assert.Equal(t, "blue", def.Color)
		assert.Equal(t, map[string]bool{"read": true, "write": false}, def.Tools)
		assert.Contains(t, def.Description, "multiple lines")
	})

	t.Run("handles CRLF", func(t *testing.T) {
		content := "---\r\nmodel: test\r\n---\r\n\r\ndescription"
		def, err := parseAgentContent(content)
		require.NoError(t, err)
		require.NotNil(t, def)
		assert.Equal(t, "test", def.Model)
		assert.Equal(t, "description", def.Description)
	})
}

func TestManager_Get(t *testing.T) {
	t.Run("existing agent", func(t *testing.T) {
		fsys := fstest.MapFS{
			"my-agent.md": &fstest.MapFile{
				Data: []byte("---\nmodel: test\n---\ndesc"),
			},
		}
		m := NewManager(fsys)
		_ = m.Load()

		def, ok := m.Get("my-agent")
		assert.True(t, ok)
		assert.NotNil(t, def)
	})

	t.Run("non-existing agent", func(t *testing.T) {
		m := NewManager(nil)
		def, ok := m.Get("nonexistent")
		assert.False(t, ok)
		assert.Nil(t, def)
	})
}

func TestManager_List(t *testing.T) {
	t.Run("empty manager", func(t *testing.T) {
		m := NewManager(nil)
		list := m.List()
		assert.Empty(t, list)
	})

	t.Run("with agents", func(t *testing.T) {
		fsys := fstest.MapFS{
			"a.md": &fstest.MapFile{Data: []byte("---\n---\na")},
			"b.md": &fstest.MapFile{Data: []byte("---\n---\nb")},
		}
		m := NewManager(fsys)
		_ = m.Load()

		list := m.List()
		assert.Len(t, list, 2)
		names := []string{list[0].Name, list[1].Name}
		assert.ElementsMatch(t, []string{"a", "b"}, names)
	})
}

func TestManager_ListVisible(t *testing.T) {
	t.Run("filters hidden agents", func(t *testing.T) {
		fsys := fstest.MapFS{
			"visible.md": &fstest.MapFile{Data: []byte("---\n---\nvisible")},
			"hidden.md":  &fstest.MapFile{Data: []byte("---\nhidden: true\n---\nhidden")},
		}
		m := NewManager(fsys)
		_ = m.Load()

		visible := m.ListVisible()
		assert.Len(t, visible, 1)
		assert.Equal(t, "visible", visible[0].Name)
	})

	t.Run("all visible", func(t *testing.T) {
		fsys := fstest.MapFS{
			"a.md": &fstest.MapFile{Data: []byte("---\n---\na")},
			"b.md": &fstest.MapFile{Data: []byte("---\nhidden: false\n---\nb")},
		}
		m := NewManager(fsys)
		_ = m.Load()

		visible := m.ListVisible()
		assert.Len(t, visible, 2)
	})

	t.Run("all hidden", func(t *testing.T) {
		fsys := fstest.MapFS{
			"a.md": &fstest.MapFile{Data: []byte("---\nhidden: true\n---\na")},
			"b.md": &fstest.MapFile{Data: []byte("---\nhidden: true\n---\nb")},
		}
		m := NewManager(fsys)
		_ = m.Load()

		visible := m.ListVisible()
		assert.Empty(t, visible)
	})
}
