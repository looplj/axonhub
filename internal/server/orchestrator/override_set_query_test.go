package orchestrator

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"

	"github.com/looplj/axonhub/internal/objects"
)

const placeholder = `{"type":"text","text":"[removed]"}`

func setOp(path string) objects.OverrideOperation {
	return objects.OverrideOperation{Op: objects.OverrideOpSet, Path: path, Value: placeholder}
}

func TestApplyBodySet_QueryPathReplacesEveryMatchInArray(t *testing.T) {
	body := []byte(`{"messages":[{"role":"user","content":[` +
		`{"type":"text","text":"q"},` +
		`{"type":"image_url","image_url":{"url":"d1"}},` +
		`{"type":"image_url","image_url":{"url":"d2"}},` +
		`{"type":"image_url","image_url":{"url":"d3"}},` +
		`{"type":"text","text":"r"}]}]}`)

	out, err := applyBodySet(context.Background(), body, setOp(`messages.#.content.#(type=="image_url")`), RenderContext{})
	require.NoError(t, err)
	require.True(t, gjson.ValidBytes(out))

	content := gjson.GetBytes(out, "messages.0.content").Array()
	require.Len(t, content, 5)
	require.Equal(t, "q", content[0].Get("text").String())
	for _, i := range []int{1, 2, 3} {
		require.Equal(t, "[removed]", content[i].Get("text").String(), "element %d", i)
		require.False(t, content[i].Get("image_url").Exists(), "element %d", i)
	}
	require.Equal(t, "r", content[4].Get("text").String())
}

func TestApplyBodySet_QueryPathReplacesMatchesAcrossMessages(t *testing.T) {
	body := []byte(`{"messages":[` +
		`{"content":[{"type":"image_url","u":1},{"type":"image_url","u":2}]},` +
		`{"content":"plain"},` +
		`{"content":[{"type":"text","text":"x"},{"type":"image_url","u":3}]}]}`)

	out, err := applyBodySet(context.Background(), body, setOp(`messages.#.content.#(type=="image_url")`), RenderContext{})
	require.NoError(t, err)
	require.True(t, gjson.ValidBytes(out))
	require.NotContains(t, string(out), "image_url")
	require.Equal(t, "plain", gjson.GetBytes(out, "messages.1.content").String())
	require.Equal(t, "[removed]", gjson.GetBytes(out, "messages.0.content.1.text").String())
	require.Equal(t, "[removed]", gjson.GetBytes(out, "messages.2.content.1.text").String())
}

func TestApplyBodySet_QueryPathWithNoMatchLeavesBodyUntouched(t *testing.T) {
	body := []byte(`{"messages":[{"content":[{"type":"text","text":"q"}]}]}`)

	out, err := applyBodySet(context.Background(), body, setOp(`messages.#.content.#(type=="image_url")`), RenderContext{})
	require.NoError(t, err)
	require.JSONEq(t, string(body), string(out))
}

func TestApplyBodySet_NeverReturnsInvalidJSON(t *testing.T) {
	body := []byte(`{"messages":[{"content":[{"type":"text"},{"type":"image_url"},{"type":"image_url"}]}]}`)

	// sjson turns this "all matches" form into broken JSON without reporting an error.
	out, err := applyBodySet(context.Background(), body, setOp(`messages.#.content.#(type=="image_url")#`), RenderContext{})
	require.Error(t, err)
	require.JSONEq(t, string(body), string(out))
}

func TestApplyBodySet_QueryPathWithSuffixSetsFieldOnEveryMatch(t *testing.T) {
	body := []byte(`{"messages":[{"content":[` +
		`{"type":"image_url","detail":"high"},{"type":"text"},{"type":"image_url","detail":"high"}]}]}`)

	op := objects.OverrideOperation{Op: objects.OverrideOpSet, Value: "low", Path: `messages.#.content.#(type=="image_url").detail`}

	out, err := applyBodySet(context.Background(), body, op, RenderContext{})
	require.NoError(t, err)
	require.JSONEq(t,
		`{"messages":[{"content":[{"type":"image_url","detail":"low"},{"type":"text"},{"type":"image_url","detail":"low"}]}]}`,
		string(out))
}

func TestApplyBodySet_QueryPathCreatesMissingFieldOnEveryMatch(t *testing.T) {
	body := []byte(`{"messages":[{"content":[{"type":"image_url"},{"type":"text"},{"type":"image_url"}]}]}`)

	op := objects.OverrideOperation{Op: objects.OverrideOpSet, Value: "low", Path: `messages.#.content.#(type=="image_url").detail`}

	out, err := applyBodySet(context.Background(), body, op, RenderContext{})
	require.NoError(t, err)
	require.JSONEq(t,
		`{"messages":[{"content":[{"type":"image_url","detail":"low"},{"type":"text"},{"type":"image_url","detail":"low"}]}]}`,
		string(out))
}

func TestSpliceTargets_RewritesAllMatchesWithoutFallingBack(t *testing.T) {
	body := []byte(`{"messages":[{"content":[{"type":"image_url"},{"type":"text"},{"type":"image_url"}]},{"content":[{"type":"image_url"}]}]}`)

	targets := expandQueryPath(body, `messages.#.content.#(type=="image_url")`)
	require.Len(t, targets, 3)

	out, ok, err := spliceTargets(body, targets, map[string]any{"type": "text", "text": "[removed]"})
	require.NoError(t, err)
	require.True(t, ok, "targets should have verifiable offsets, so no sjson fallback is needed")
	require.NotContains(t, string(out), "image_url")
	require.Equal(t, 3, strings.Count(string(out), "[removed]"))
	require.True(t, gjson.ValidBytes(out))
}
