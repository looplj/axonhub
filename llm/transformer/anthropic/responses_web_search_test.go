package anthropic

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"

	"github.com/looplj/axonhub/llm"
)

func TestResponsesSearchBlocksTrackOnlyObservedCalls(t *testing.T) {
	seen := make(map[string]responsesSearchState)
	blocks := func(raw string) []MessageContentBlock {
		return responsesSearchBlocks(map[string]any{"openai_responses_web_search_calls": json.RawMessage(raw)}, seen, true, nil)
	}
	require.Empty(t, blocks(`[{"status":"completed"}]`), "missing IDs must not fabricate searches")
	started := blocks(`[{"id":"ws_a","status":"in_progress"}]`)
	require.Len(t, started, 1)
	require.Equal(t, "server_tool_use", started[0].Type)
	require.JSONEq(t, `{}`, string(started[0].Input), "unknown queries must not be invented")
	require.Empty(t, blocks(`[{"id":"ws_a","status":"in_progress"}]`))
	completed := blocks(`[{"id":"ws_a","status":"completed"}]`)
	require.Len(t, completed, 1)
	require.Equal(t, "web_search_tool_result", completed[0].Type)
	raw, err := json.Marshal(completed[0].Content)
	require.NoError(t, err)
	require.JSONEq(t, `[]`, string(raw))
	require.Empty(t, blocks(`[{"id":"ws_a","status":"completed"}]`), "repeated snapshots must not count twice")
	failed := blocks(`[{"id":"ws_b","status":"failed"}]`)
	require.Len(t, failed, 2)
	raw, err = json.Marshal(failed[1].Content)
	require.NoError(t, err)
	require.Equal(t, "web_search_tool_result_error", gjson.GetBytes(raw, "type").String())
	require.Len(t, seen, 2)
}

func TestResponsesSearchBlocksPreserveSourcesWithoutFabricatingEncryption(t *testing.T) {
	blocks := responsesSearchBlocks(map[string]any{"openai_responses_web_search_calls": json.RawMessage(`[{"id":"ws_a","status":"completed","action":{"queries":["Debian"],"sources":[{"url":"https://debian.org","title":"Debian"},{"url":"https://debian.org","title":"Duplicate"},{"title":"No URL"}]}}]`)}, make(map[string]responsesSearchState), true, nil)
	require.Len(t, blocks, 2)
	require.JSONEq(t, `{"query":"Debian"}`, string(blocks[0].Input))
	raw, err := json.Marshal(blocks[1].Content)
	require.NoError(t, err)
	require.JSONEq(t, `[{"type":"web_search_result","url":"https://debian.org","title":"Debian"}]`, string(raw))
}

func TestResponsesSearchBlocksFillMissingSourceTitles(t *testing.T) {
	seen := make(map[string]responsesSearchState)
	metadata := map[string]any{"openai_responses_web_search_calls": json.RawMessage(`[
		{"id":"a","status":"completed","action":{"query":"Debian","sources":[
			{"url":"https://debian.org/","title":"Original"},
			{"url":"https://debian.org/releases/"},
			{"url":"https://debian.org/distrib/","title":" \t"}
		]}},
		{"id":"b","status":"completed","action":{"query":"Other","sources":[{"url":"https://example.org/"}]}}
	]`)}
	// Titles may arrive in answer citations after the search completes.
	responsesSearchBlocks(metadata, seen, false, nil)
	cited := []responsesSearchSource{
		{URL: "https://debian.org/", Title: "Replacement"},
		{URL: "https://debian.org/releases/?utm_source=openai", Title: "Debian Releases"},
		{URL: "https://debian.org/distrib", Title: "Not an exact match"},
		{URL: "https://example.org/", Title: "Example"},
		{URL: "https://unrelated.org/", Title: "Unrelated"},
	}
	blocks := responsesSearchBlocks(nil, seen, true, cited)
	require.Len(t, blocks, 2)
	raw, err := json.Marshal(blocks[0].Content)
	require.NoError(t, err)
	require.JSONEq(t, `[
		{"type":"web_search_result","url":"https://debian.org/","title":"Original"},
		{"type":"web_search_result","url":"https://debian.org/releases/","title":"Debian Releases"},
		{"type":"web_search_result","url":"https://debian.org/distrib/","title":"https://debian.org/distrib/"}
	]`, string(raw))
	raw, err = json.Marshal(blocks[1].Content)
	require.NoError(t, err)
	require.JSONEq(t, `[{"type":"web_search_result","url":"https://example.org/","title":"Example"}]`, string(raw))
}

func TestResponsesSearchTitleURL(t *testing.T) {
	for _, tc := range []struct {
		name, source, citation string
		match                  bool
	}{
		{"tracking", "https://example.org/?page=2", "https://example.org/?utm_source=openai&page=2&utm_medium=search", true},
		{"query order", "https://example.org/?a=1&b=2", "https://example.org/?b=2&a=1", true},
		{"different page", "https://example.org/?page=2", "https://example.org/?page=3&utm_source=openai", false},
		{"unknown parameter", "https://example.org/", "https://example.org/?source=other", false},
		{"fragment", "https://example.org/#one", "https://example.org/#two", false},
		{"path", "https://example.org/one", "https://example.org/two", false},
		{"host", "https://example.org/", "https://other.org/", false},
		{"scheme", "https://example.org/", "http://example.org/", false},
		{"malformed query", "https://example.org/", "https://example.org/?utm_source=%zz", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.match, responsesSearchTitleURL(tc.source) == responsesSearchTitleURL(tc.citation))
		})
	}
}

func TestResponsesSearchBlocksWaitForLateDetails(t *testing.T) {
	seen := make(map[string]responsesSearchState)
	snapshot := func(raw string, final bool) []MessageContentBlock {
		return responsesSearchBlocks(map[string]any{"openai_responses_web_search_calls": json.RawMessage(raw)}, seen, final, nil)
	}
	require.Empty(t, snapshot(`[{"id":"late","status":"in_progress"}]`, false), "do not close an empty query before the real query arrives")
	started := snapshot(`[{"id":"late","status":"completed","action":{"query":"actual query","sources":[{"url":"https://example.org/one"}]}}]`, false)
	require.Len(t, started, 1, "query progress can stream, but results must wait for the terminal snapshot")
	require.Equal(t, "server_tool_use", started[0].Type)
	require.JSONEq(t, `{"query":"actual query"}`, string(started[0].Input))
	require.Empty(t, snapshot(`[{"id":"late","status":"in_progress","action":{"sources":[{"url":"https://example.org/two","title":"Two"},{"url":"https://example.org/one","title":"One"}]}}]`, false))
	results := responsesSearchBlocks(nil, seen, true, nil)
	require.Len(t, results, 1)
	require.Equal(t, "web_search_tool_result", results[0].Type)
	raw, err := json.Marshal(results[0].Content)
	require.NoError(t, err)
	require.JSONEq(t, `[{"type":"web_search_result","url":"https://example.org/one","title":"One"},{"type":"web_search_result","url":"https://example.org/two","title":"Two"}]`, string(raw))
	require.Empty(t, responsesSearchBlocks(nil, seen, true, nil), "flush only once")
}

func TestResponsesSearchBlocksFlushUnknownQueryInCallOrder(t *testing.T) {
	seen := make(map[string]responsesSearchState)
	require.Empty(t, responsesSearchBlocks(map[string]any{"openai_responses_web_search_calls": json.RawMessage(`[{"id":"second_alphabetically","status":"completed"},{"id":"first_alphabetically","status":"failed"}]`)}, seen, false, nil))
	blocks := responsesSearchBlocks(nil, seen, true, nil)
	require.Len(t, blocks, 4)
	require.Equal(t, "srvtoolu_second_alphabetically", blocks[0].ID)
	require.Equal(t, "srvtoolu_first_alphabetically", blocks[2].ID)
	require.JSONEq(t, `{}`, string(blocks[0].Input))
	require.Empty(t, responsesSearchBlocks(nil, seen, true, nil))
}

func TestResponsesSearchCitationFallbackBoundaries(t *testing.T) {
	cited := []responsesSearchSource{{URL: "https://example.org/cited", Title: "Cited"}}
	for _, tc := range []struct {
		name, raw string
		wantURL   string
		results   int
	}{
		{"single search", `[{"id":"a","status":"completed"}]`, "https://example.org/cited", 1},
		{"explicit sources win", `[{"id":"a","status":"completed","action":{"sources":[{"url":"https://example.org/explicit","title":"Explicit"}]}}]`, "https://example.org/explicit", 1},
		{"multiple searches are ambiguous", `[{"id":"a","status":"completed"},{"id":"b","status":"completed"}]`, "", 2},
		{"no search must not invent one", `[]`, "", 0},
		{"failed search stays failed", `[{"id":"a","status":"failed"}]`, "", 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			blocks := responsesSearchBlocks(map[string]any{"openai_responses_web_search_calls": json.RawMessage(tc.raw)}, make(map[string]responsesSearchState), true, cited)
			count := 0
			for _, block := range blocks {
				if block.Type != "web_search_tool_result" {
					continue
				}
				count++
				raw, err := json.Marshal(block.Content)
				require.NoError(t, err)
				require.Equal(t, tc.wantURL, gjson.GetBytes(raw, "0.url").String())
				if tc.name == "failed search stays failed" {
					require.Equal(t, "web_search_tool_result_error", gjson.GetBytes(raw, "type").String())
				}
			}
			require.Equal(t, tc.results, count)
		})
	}
}

func TestResponsesSearchCitationsOnlyUseStructuredWebURLs(t *testing.T) {
	annotations := []llm.Annotation{
		{Type: "url_citation", URLCitation: &llm.URLCitation{URL: "https://example.org"}},
		{Type: "url_citation", URLCitation: &llm.URLCitation{URL: "https://example.org", Title: "Example"}},
		{Type: "url_citation", URLCitation: &llm.URLCitation{URL: "javascript:alert(1)"}},
		{Type: "url_citation", URLCitation: &llm.URLCitation{URL: "file:///tmp/private"}},
		{Type: "url_citation", URLCitation: &llm.URLCitation{URL: "/relative"}},
		{Type: "file_citation", URLCitation: &llm.URLCitation{URL: "https://example.org/wrong-kind"}},
	}
	sources := appendResponsesSearchCitations(nil, annotations)
	require.Equal(t, []responsesSearchSource{{URL: "https://example.org", Title: "Example"}}, sources)
	require.Equal(t, sources, appendResponsesSearchCitations(sources, annotations))
}
