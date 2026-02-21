package fetch

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseMarkdownFrontMatter_NoFrontMatter(t *testing.T) {
	title, description, content := parseMarkdownFrontMatter("# Hello\n\nWorld\n")
	require.Equal(t, "", title)
	require.Equal(t, "", description)
	require.Equal(t, "# Hello\n\nWorld\n", content)
}

func TestParseMarkdownFrontMatter_Basic(t *testing.T) {
	md := `---
title: Markdown for Agents
description: Markdown has quickly become the lingua franca for agents and AI systems as a whole.
---

# Markdown for Agents

Body.
`

	title, description, content := parseMarkdownFrontMatter(md)
	require.Equal(t, "Markdown for Agents", title)
	require.Equal(t, "Markdown has quickly become the lingua franca for agents and AI systems as a whole.", description)
	require.Contains(t, content, "# Markdown for Agents")
	require.NotContains(t, content, "description:")
}

func TestParseMarkdownFrontMatter_MultilineDescription(t *testing.T) {
	md := `---
title: T
description: |
  Line one
  Line two
---

# Heading
`

	title, description, content := parseMarkdownFrontMatter(md)
	require.Equal(t, "T", title)
	require.Equal(t, "Line one Line two", description)
	require.Equal(t, "# Heading\n", content)
}

func TestParseMarkdownFrontMatter_EndMarkerDots(t *testing.T) {
	md := `---
title: A
description: B
...

X
`

	title, description, content := parseMarkdownFrontMatter(md)
	require.Equal(t, "A", title)
	require.Equal(t, "B", description)
	require.Equal(t, "X\n", content)
}
