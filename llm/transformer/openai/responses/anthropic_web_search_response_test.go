package responses

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"

	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/streams"
	"github.com/looplj/axonhub/llm/transformer/anthropic"
)

func TestAnthropicWebSearchResponseBlocks(t *testing.T) {
	for _, tc := range []struct {
		name, item string
		sources    int
	}{
		{"with sources", `{"id":"ws_1","type":"web_search_call","status":"completed","action":{"type":"search","query":"Debian","sources":[{"type":"url","url":"https://debian.org","title":"Debian"}]}}`, 1},
		{"without action", `{"id":"ws_1","type":"web_search_call","status":"completed"}`, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			out, err := NewOutboundTransformer("https://api.openai.com", "key")
			require.NoError(t, err)
			body := []byte(`{"id":"resp_1","model":"gpt-6-luna","status":"completed","output":[` + tc.item + `,{"type":"message","role":"assistant","content":[{"type":"output_text","text":"Answer"}]}]}`)
			unified, err := out.TransformResponse(ctx, &httpclient.Response{StatusCode: 200, Body: body})
			require.NoError(t, err)
			converted, err := anthropic.NewInboundTransformer().TransformResponse(ctx, unified)
			require.NoError(t, err)
			blocks := gjson.GetBytes(converted.Body, "content").Array()
			require.Equal(t, "server_tool_use", blocks[0].Get("type").String())
			require.Equal(t, "web_search_tool_result", blocks[1].Get("type").String())
			require.Equal(t, blocks[0].Get("id").String(), blocks[1].Get("tool_use_id").String())
			require.Len(t, blocks[1].Get("content").Array(), tc.sources)
			require.Equal(t, "end_turn", gjson.GetBytes(converted.Body, "stop_reason").String())
		})
	}
}

func TestAnthropicWebSearchStreamProgressAndDeduplication(t *testing.T) {
	for _, terminalOnly := range []bool{false, true} {
		t.Run(fmt.Sprint("terminalOnly=", terminalOnly), func(t *testing.T) {
			ctx := context.Background()
			out, err := NewOutboundTransformer("https://api.openai.com", "key")
			require.NoError(t, err)
			item := `{"id":"ws_1","type":"web_search_call","status":"completed","action":{"type":"search","query":"Debian","sources":[{"type":"url","url":"https://debian.org","title":"Debian"}]}}`
			events := []*httpclient.StreamEvent{{Data: []byte(`{"type":"response.created","response":{"id":"resp_1","model":"gpt-6-luna","output":[]}}`)}}
			if !terminalOnly {
				events = append(events, &httpclient.StreamEvent{Data: []byte(`{"type":"response.output_item.added","output_index":0,"item":{"id":"ws_1","type":"web_search_call","status":"in_progress","action":{"type":"search","query":"Debian"}}}`)})
				// Duplicate item.done events and the terminal snapshot must count only once.
				for range 2 {
					events = append(events, &httpclient.StreamEvent{Data: []byte(`{"type":"response.output_item.done","output_index":0,"item":` + item + `}`)})
				}
			}
			events = append(events, &httpclient.StreamEvent{Data: []byte(`{"type":"response.output_text.delta","delta":"Answer"}`)}, &httpclient.StreamEvent{Data: []byte(`{"type":"response.completed","response":{"id":"resp_1","model":"gpt-6-luna","status":"completed","output":[` + item + `]}}`)})
			unified, err := out.TransformStream(ctx, nil, streams.SliceStream(events))
			require.NoError(t, err)
			converted, err := anthropic.NewInboundTransformer().TransformStream(ctx, unified)
			require.NoError(t, err)
			result, err := streams.All(converted)
			require.NoError(t, err)
			starts, stops, uses, results := 0, 0, 0, 0
			active := false
			for _, event := range result {
				kind := gjson.GetBytes(event.Data, "type").String()
				switch kind {
				case "content_block_start":
					require.False(t, active, "blocks must not overlap")
					require.Equal(t, int64(starts), gjson.GetBytes(event.Data, "index").Int())
					active = true
					starts++
					switch gjson.GetBytes(event.Data, "content_block.type").String() {
					case "server_tool_use":
						uses++
					case "web_search_tool_result":
						results++
						require.Len(t, gjson.GetBytes(event.Data, "content_block.content").Array(), 1)
					}
				case "content_block_stop":
					require.True(t, active)
					active = false
					stops++
				case "message_delta":
					require.Equal(t, "end_turn", gjson.GetBytes(event.Data, "delta.stop_reason").String())
				}
			}
			require.Equal(t, 1, uses)
			require.Equal(t, 1, results)
			require.Equal(t, starts, stops)
			require.False(t, active)

			// The same progress snapshots must not duplicate calls for a
			// Responses client consuming this unified stream.
			replay, err := out.TransformStream(ctx, nil, streams.SliceStream(events))
			require.NoError(t, err)
			wire, err := NewInboundTransformer().TransformStream(ctx, replay)
			require.NoError(t, err)
			responseEvents, err := streams.All(wire)
			require.NoError(t, err)
			completed := false
			for _, event := range responseEvents {
				if gjson.GetBytes(event.Data, "type").String() != "response.completed" {
					continue
				}
				completed = true
				count := 0
				for _, item := range gjson.GetBytes(event.Data, "response.output").Array() {
					if item.Get("type").String() == "web_search_call" {
						count++
						require.Equal(t, "completed", item.Get("status").String())
					}
				}
				require.Equal(t, 1, count)
			}
			require.True(t, completed)
		})
	}
}

func TestAnthropicWebSearchStreamLateMetadata(t *testing.T) {
	ctx := context.Background()
	out, err := NewOutboundTransformer("https://api.openai.com", "key")
	require.NoError(t, err)
	events := []*httpclient.StreamEvent{
		{Data: []byte(`{"type":"response.created","response":{"id":"resp_late","model":"test","output":[]}}`)},
		{Data: []byte(`{"type":"response.output_item.added","output_index":0,"item":{"id":"ws_late","type":"web_search_call","status":"in_progress"}}`)},
		{Data: []byte(`{"type":"response.output_item.done","output_index":0,"item":{"id":"ws_late","type":"web_search_call","status":"completed","action":{"type":"search","query":"late query"}}}`)},
		{Data: []byte(`{"type":"response.output_text.delta","delta":"Answer"}`)},
		// Sources arrive only in the terminal snapshot, which omits the earlier query.
		{Data: []byte(`{"type":"response.completed","response":{"id":"resp_late","model":"test","status":"completed","output":[{"id":"ws_late","type":"web_search_call","status":"completed","action":{"type":"search","sources":[{"type":"url","url":"https://example.org/late","title":"Late source"}]}}]}}`)},
	}
	unified, err := out.TransformStream(ctx, nil, streams.SliceStream(events))
	require.NoError(t, err)
	converted, err := anthropic.NewInboundTransformer().TransformStream(ctx, unified)
	require.NoError(t, err)
	result, err := streams.All(converted)
	require.NoError(t, err)
	uses, results, starts, stops := 0, 0, 0, 0
	var query string
	for _, event := range result {
		switch gjson.GetBytes(event.Data, "type").String() {
		case "content_block_start":
			starts++
			switch gjson.GetBytes(event.Data, "content_block.type").String() {
			case "server_tool_use":
				uses++
			case "web_search_tool_result":
				results++
				require.Equal(t, "https://example.org/late", gjson.GetBytes(event.Data, "content_block.content.0.url").String())
			}
		case "content_block_delta":
			if gjson.GetBytes(event.Data, "delta.type").String() == "input_json_delta" {
				query += gjson.GetBytes(event.Data, "delta.partial_json").String()
			}
		case "content_block_stop":
			stops++
		}
	}
	require.Equal(t, 1, uses)
	require.Equal(t, 1, results)
	require.Equal(t, "late query", gjson.Get(query, "query").String())
	require.Equal(t, starts, stops, "no unmatched stop when results are emitted at the terminal event")
}

func TestWebSearchMetadataMergesSparseSnapshots(t *testing.T) {
	metadata := make(map[string]any)
	completed, inProgress := "completed", "in_progress"
	appendResponseWebSearchCallMetadata(metadata, Item{ID: "ws_1", Type: "web_search_call", Status: &completed, Action: NewWebSearchAction(&WebSearchAction{Type: "search", Query: "actual query", Sources: []WebSearchSource{{URL: "https://example.org/one", Title: "One"}}})})
	before := metadata[responsesWebSearchCallsTransformerMetadataKey].([]Item)
	appendResponseWebSearchCallMetadata(metadata, Item{ID: "ws_1", Type: "web_search_call", Status: &inProgress, Action: NewWebSearchAction(&WebSearchAction{Sources: []WebSearchSource{{URL: "https://example.org/two", Title: "Two"}}})})
	calls := metadata[responsesWebSearchCallsTransformerMetadataKey].([]Item)
	require.Len(t, calls, 1)
	require.Equal(t, "completed", *calls[0].Status)
	require.Equal(t, "actual query", calls[0].Action.WebSearch.Query)
	require.Equal(t, "search", calls[0].Action.WebSearch.Type)
	require.Len(t, calls[0].Action.WebSearch.Sources, 2)
	require.Len(t, before[0].Action.WebSearch.Sources, 1, "published metadata snapshots must remain immutable")
}

func TestAnthropicWebSearchUsesCitationsWhenSourcesOmitted(t *testing.T) {
	ctx := context.Background()
	out, err := NewOutboundTransformer("https://api.openai.com", "key")
	require.NoError(t, err)
	search := `{"id":"ws_cited","type":"web_search_call","status":"completed"}`
	message := `{"id":"msg_cited","type":"message","role":"assistant","content":[{"type":"output_text","text":"Cited answer","annotations":[{"type":"url_citation","start_index":0,"end_index":5,"url":"https://example.org/cited","title":"Cited source"}]}]}`
	body := []byte(`{"id":"resp_cited","model":"test","status":"completed","output":[` + search + `,` + message + `]}`)
	unified, err := out.TransformResponse(ctx, &httpclient.Response{StatusCode: 200, Body: body})
	require.NoError(t, err)
	converted, err := anthropic.NewInboundTransformer().TransformResponse(ctx, unified)
	require.NoError(t, err)
	require.Equal(t, "https://example.org/cited", gjson.GetBytes(converted.Body, `content.#(type=="web_search_tool_result").content.0.url`).String())
	events := []*httpclient.StreamEvent{
		{Data: []byte(`{"type":"response.created","response":{"id":"resp_cited","model":"test","output":[]}}`)},
		{Data: []byte(`{"type":"response.output_item.done","item":` + search + `}`)},
		{Data: []byte(`{"type":"response.output_text.delta","delta":"Cited answer"}`)},
		{Data: []byte(`{"type":"response.output_item.done","item":` + message + `}`)},
		{Data: []byte(`{"type":"response.completed","response":{"id":"resp_cited","model":"test","status":"completed","output":[` + search + `]}}`)},
	}
	source, err := out.TransformStream(ctx, nil, streams.SliceStream(events))
	require.NoError(t, err)
	wire, err := anthropic.NewInboundTransformer().TransformStream(ctx, source)
	require.NoError(t, err)
	result, err := streams.All(wire)
	require.NoError(t, err)
	urls := []string{}
	for _, event := range result {
		if gjson.GetBytes(event.Data, "content_block.type").String() == "web_search_tool_result" {
			urls = append(urls, gjson.GetBytes(event.Data, "content_block.content.0.url").String())
		}
	}
	require.Equal(t, []string{"https://example.org/cited"}, urls)
}
