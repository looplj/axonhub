package anthropic

import (
	"encoding/json"
	"net/url"
	"slices"
	"strings"

	"github.com/samber/lo"

	"github.com/looplj/axonhub/llm"
)

// Responses search calls are metadata rather than client function calls. Keep
// this adapter on the Anthropic side so other clients retain their own protocol.
type responsesSearchSource struct {
	URL   string `json:"url"`
	Title string `json:"title"`
}

type responsesSearchAction struct {
	Query   string                  `json:"query"`
	Queries []string                `json:"queries"`
	Sources []responsesSearchSource `json:"sources"`
}

type responsesSearchCall struct {
	ID     string                 `json:"id"`
	Status string                 `json:"status"`
	Action *responsesSearchAction `json:"action"`
}

type responsesSearchState struct {
	started, finished bool
	order             int
	call              responsesSearchCall
}

func responsesSearchBlocks(metadata map[string]any, seen map[string]responsesSearchState, final bool, citedSources []responsesSearchSource) []MessageContentBlock {
	// Accumulate snapshots even after a call completes: its final source list
	// may be supplied only with response.completed. Missing fields are not deletions.
	raw, hasSnapshot := metadata["openai_responses_web_search_calls"]
	if !hasSnapshot && !final {
		return nil
	}
	if hasSnapshot {
		data, err := json.Marshal(raw)
		var calls []responsesSearchCall
		if err == nil && json.Unmarshal(data, &calls) == nil {
			for _, call := range calls {
				if call.ID == "" {
					continue
				}
				state, exists := seen[call.ID]
				if !exists {
					state.order = len(seen)
				}
				if call.Status == "" || state.call.Status == "completed" || state.call.Status == "failed" {
					call.Status = state.call.Status
				}
				if call.Action == nil {
					call.Action = state.call.Action
				} else if state.call.Action != nil {
					if call.Action.Query == "" {
						call.Action.Query = state.call.Action.Query
					}
					if len(call.Action.Queries) == 0 {
						call.Action.Queries = state.call.Action.Queries
					}
					sources := slices.Clone(state.call.Action.Sources)
					for _, source := range call.Action.Sources {
						index := slices.IndexFunc(sources, func(existing responsesSearchSource) bool { return existing.URL == source.URL })
						if index < 0 {
							sources = append(sources, source)
						} else if source.Title != "" {
							sources[index] = source
						}
					}
					call.Action.Sources = sources
				}
				state.call = call
				seen[call.ID] = state
			}
		}
	}
	ids := make([]string, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}
	slices.SortFunc(ids, func(a, b string) int { return seen[a].order - seen[b].order })
	var blocks []MessageContentBlock
	for _, callID := range ids {
		state := seen[callID]
		call := state.call
		id := "srvtoolu_" + call.ID
		hasQuery := call.Action != nil && (call.Action.Query != "" || len(call.Action.Queries) > 0)
		if !state.started && (hasQuery || final) {
			input := map[string]any{}
			if call.Action != nil {
				if call.Action.Query != "" {
					input["query"] = call.Action.Query
				} else if len(call.Action.Queries) == 1 {
					input["query"] = call.Action.Queries[0]
				}
				if len(call.Action.Queries) > 1 {
					input["queries"] = call.Action.Queries
				}
			}
			encoded, _ := json.Marshal(input)
			blocks = append(blocks, MessageContentBlock{Type: "server_tool_use", ID: id, Name: lo.ToPtr("web_search"), Input: encoded})
			state.started = true
		}
		if final && !state.finished && (call.Status == "completed" || call.Status == "failed") {
			content := &MessageContent{}
			if call.Status == "failed" {
				content.SetRaw(json.RawMessage(`{"type":"web_search_tool_result_error","error_code":"unavailable"}`))
			} else {
				// OpenAI does not provide Anthropic's encrypted_content. Do not fabricate
				// it or infer sources from generated prose. Empty source lists are valid
				// here: the cited answer is delivered separately in text blocks.
				sources := make([]map[string]string, 0)
				urls := make(map[string]bool)
				if call.Action != nil {
					for _, source := range call.Action.Sources {
						if source.URL == "" || urls[source.URL] {
							continue
						}
						urls[source.URL] = true
						title := source.Title
						if strings.TrimSpace(title) == "" {
							// A matching citation can supply a title without changing
							// which sources belong to this search.
							for _, citation := range citedSources {
								if responsesSearchTitleURL(citation.URL) == responsesSearchTitleURL(source.URL) && strings.TrimSpace(citation.Title) != "" {
									title = citation.Title
									break
								}
							}
						}
						if strings.TrimSpace(title) == "" {
							title = source.URL
						}
						sources = append(sources, map[string]string{"type": "web_search_result", "url": source.URL, "title": title})
					}
				}
				// Citations describe the answer, not a particular call. They
				// are attributable only when exactly one search was observed.
				// Never replace an explicit source list or parse links from prose.
				if len(sources) == 0 && len(seen) == 1 {
					for _, source := range citedSources {
						sources = append(sources, map[string]string{"type": "web_search_result", "url": source.URL, "title": source.Title})
					}
				}
				encoded, _ := json.Marshal(sources)
				content.SetRaw(encoded)
			}
			blocks = append(blocks, MessageContentBlock{Type: "web_search_tool_result", ToolUseID: &id, Content: content})
			state.finished = true
		}
		seen[call.ID] = state
	}
	return blocks
}

// responsesSearchTitleURL ignores known tracking parameters only for title
// matching. Preserve the original result URLs and all page-selecting parameters.
func responsesSearchTitleURL(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return raw
	}
	query, err := url.ParseQuery(parsed.RawQuery)
	if err != nil {
		return raw
	}
	for _, key := range []string{"utm_source", "utm_medium", "utm_campaign", "utm_term", "utm_content"} {
		query.Del(key)
	}
	parsed.RawQuery = query.Encode()
	return parsed.String()
}

func (s *anthropicInboundStream) emitResponsesSearchBlocks(metadata map[string]any, final bool) error {
	if s.responsesSearches == nil {
		s.responsesSearches = make(map[string]responsesSearchState)
	}
	blocks := responsesSearchBlocks(metadata, s.responsesSearches, final, s.responsesSearchCitations)
	if len(blocks) == 0 {
		return nil
	}
	if err := s.closeOpenContentBlocks(); err != nil {
		return err
	}
	for _, block := range blocks {
		input := block.Input
		if block.Type == "server_tool_use" {
			block.Input = json.RawMessage(`{}`)
		}
		if err := s.enqueEvent(&StreamEvent{Type: "content_block_start", Index: &s.contentIndex, ContentBlock: &block}); err != nil {
			return err
		}
		if block.Type == "server_tool_use" {
			args := string(input)
			if err := s.enqueEvent(&StreamEvent{Type: "content_block_delta", Index: &s.contentIndex, Delta: &StreamDelta{Type: lo.ToPtr("input_json_delta"), PartialJSON: &args}}); err != nil {
				return err
			}
		}
		if err := s.enqueEvent(&StreamEvent{Type: "content_block_stop", Index: &s.contentIndex}); err != nil {
			return err
		}
		s.contentIndex++
	}
	return nil
}

// appendResponsesSearchCitations accepts only structured URL annotations. These
// are cited sources, not a reconstruction of every result the search retrieved.
func appendResponsesSearchCitations(sources []responsesSearchSource, annotations []llm.Annotation) []responsesSearchSource {
	for _, annotation := range annotations {
		citation := annotation.URLCitation
		if citation == nil || (annotation.Type != "url_citation" && annotation.Type != "web_search_result_location") {
			continue
		}
		parsed, err := url.Parse(citation.URL)
		if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
			continue
		}
		index := slices.IndexFunc(sources, func(source responsesSearchSource) bool { return source.URL == citation.URL })
		if index >= 0 {
			if citation.Title != "" {
				sources[index].Title = citation.Title
			}
			continue
		}
		title := citation.Title
		if title == "" {
			title = citation.URL
		}
		sources = append(sources, responsesSearchSource{URL: citation.URL, Title: title})
	}
	return sources
}
