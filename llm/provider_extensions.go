package llm

import (
	"encoding/json"
	"log/slog"
)

// ProviderExtensions carries provider/API-format private data that must not be
// serialized through the common llm request/response JSON model.
type ProviderExtensions struct {
	OpenAIResponses *OpenAIResponsesProviderExtensions `json:"openai_responses,omitempty"`
}

func (e *ProviderExtensions) LogValue() slog.Value {
	if e == nil || e.OpenAIResponses == nil {
		return slog.GroupValue()
	}

	return slog.GroupValue(
		slog.Bool("openai_responses", true),
		slog.Int("request_input_items", len(e.OpenAIResponsesRequest().InputItems)),
		slog.Int("request_tools", len(e.OpenAIResponsesRequest().Tools)),
		slog.Int("request_protectable_fragments", len(e.OpenAIResponsesRequest().ProtectableFragments)),
	)
}

func (e *ProviderExtensions) OpenAIResponsesRequest() *OpenAIResponsesRequestExtensions {
	if e == nil || e.OpenAIResponses == nil || e.OpenAIResponses.Request == nil {
		return &OpenAIResponsesRequestExtensions{}
	}

	return e.OpenAIResponses.Request
}

type OpenAIResponsesProviderExtensions struct {
	Request  *OpenAIResponsesRequestExtensions  `json:"request,omitempty"`
	Response *OpenAIResponsesResponseExtensions `json:"response,omitempty"`
	Stream   *OpenAIResponsesStreamExtensions   `json:"stream,omitempty"`
	Dirty    OpenAIResponsesDirtySet            `json:"dirty,omitempty"`
}

type OpenAIResponsesRequestExtensions struct {
	RawBody               json.RawMessage                      `json:"-"`
	TopLevelExtra         map[string]json.RawMessage           `json:"-"`
	TopLevelSemanticExtra map[string]json.RawMessage           `json:"-"`
	MetadataRaw           json.RawMessage                      `json:"-"`
	MetadataExtra         map[string]json.RawMessage           `json:"-"`
	InputKind             string                               `json:"input_kind,omitempty"`
	InputRaw              json.RawMessage                      `json:"-"`
	InstructionsRaw       json.RawMessage                      `json:"-"`
	InputItems            []OpenAIResponsesRawItem             `json:"input_items,omitempty"`
	Tools                 []OpenAIResponsesRawItem             `json:"tools,omitempty"`
	ToolChoiceRaw         json.RawMessage                      `json:"-"`
	ProtectableFragments  []OpenAIResponsesProtectableFragment `json:"protectable_fragments,omitempty"`
}

type OpenAIResponsesResponseExtensions struct {
	Raw           json.RawMessage            `json:"-"`
	TopLevelExtra map[string]json.RawMessage `json:"-"`
	MetadataRaw   json.RawMessage            `json:"-"`
	MetadataExtra map[string]json.RawMessage `json:"-"`
	OutputItems   []OpenAIResponsesRawItem   `json:"output_items,omitempty"`
}

type OpenAIResponsesStreamExtensions struct {
	RawEvent *OpenAIResponsesRawEvent `json:"raw_event,omitempty"`
}

type OpenAIResponsesRawItem struct {
	Type          string                       `json:"type,omitempty"`
	ID            string                       `json:"id,omitempty"`
	OriginalIndex *int                         `json:"original_index,omitempty"`
	Path          string                       `json:"path,omitempty"`
	SemanticKey   string                       `json:"semantic_key,omitempty"`
	CallID        string                       `json:"call_id,omitempty"`
	ContentIndex  *int                         `json:"content_index,omitempty"`
	ConsumedSpan  *OpenAIResponsesConsumedSpan `json:"consumed_span,omitempty"`
	Raw           json.RawMessage              `json:"-"`
	Extra         map[string]json.RawMessage   `json:"-"`
	Protection    OpenAIResponsesRawProtection `json:"protection,omitempty"`
}

type OpenAIResponsesConsumedSpan struct {
	Start int `json:"start"`
	End   int `json:"end"`
}

type OpenAIResponsesProtectionStatus string

const (
	OpenAIResponsesProtectionNotSupported      OpenAIResponsesProtectionStatus = "not_supported"
	OpenAIResponsesProtectionEvaluatedNoRules  OpenAIResponsesProtectionStatus = "evaluated_no_rules"
	OpenAIResponsesProtectionEvaluatedNoMatch  OpenAIResponsesProtectionStatus = "evaluated_no_match"
	OpenAIResponsesProtectionMatchedNoChange   OpenAIResponsesProtectionStatus = "matched_no_change"
	OpenAIResponsesProtectionChangedRewritable OpenAIResponsesProtectionStatus = "changed_rewritable"
	OpenAIResponsesProtectionChangedDrop       OpenAIResponsesProtectionStatus = "changed_drop"
	OpenAIResponsesProtectionChangedReject     OpenAIResponsesProtectionStatus = "changed_reject"
)

type OpenAIResponsesRawProtection struct {
	Status        OpenAIResponsesProtectionStatus `json:"status,omitempty"`
	Scanned       bool                            `json:"scanned,omitempty"`
	TextExtracted bool                            `json:"text_extracted,omitempty"`
	Changed       bool                            `json:"changed,omitempty"`
	ReplayAllowed bool                            `json:"replay_allowed,omitempty"`
	Scope         string                          `json:"scope,omitempty"`
	TextPaths     []string                        `json:"text_paths,omitempty"`
}

type OpenAIResponsesProtectableFragment struct {
	Path        string `json:"path,omitempty"`
	Scope       string `json:"scope,omitempty"`
	Text        string `json:"-"`
	RewriteMode string `json:"rewrite_mode,omitempty"`
}

type OpenAIResponsesRawEvent struct {
	Type           string                     `json:"type,omitempty"`
	SSEType        string                     `json:"sse_type,omitempty"`
	LastEventID    string                     `json:"last_event_id,omitempty"`
	SequenceNumber *int                       `json:"sequence_number,omitempty"`
	DataRaw        json.RawMessage            `json:"-"`
	Raw            json.RawMessage            `json:"-"`
	Extra          map[string]json.RawMessage `json:"-"`
	SemanticPath   string                     `json:"semantic_path,omitempty"`
	ReplayMode     string                     `json:"replay_mode,omitempty"`
}

type OpenAIResponsesDirtyScope string

const (
	OpenAIResponsesDirtyMessages              OpenAIResponsesDirtyScope = "messages"
	OpenAIResponsesDirtyInstructions          OpenAIResponsesDirtyScope = "instructions"
	OpenAIResponsesDirtyInputItems            OpenAIResponsesDirtyScope = "input_items"
	OpenAIResponsesDirtyTools                 OpenAIResponsesDirtyScope = "tools"
	OpenAIResponsesDirtyToolChoice            OpenAIResponsesDirtyScope = "tool_choice"
	OpenAIResponsesDirtyTopLevelSemanticExtra OpenAIResponsesDirtyScope = "top_level_semantic_extra"
	OpenAIResponsesDirtyResponseOutput        OpenAIResponsesDirtyScope = "response_output"
	OpenAIResponsesDirtyResponseEnvelope      OpenAIResponsesDirtyScope = "response_envelope"
	OpenAIResponsesDirtyStream                OpenAIResponsesDirtyScope = "stream"
)

type OpenAIResponsesDirtySet struct {
	Scopes map[OpenAIResponsesDirtyScope]bool `json:"scopes,omitempty"`
}

func (d *OpenAIResponsesDirtySet) Mark(scopes ...OpenAIResponsesDirtyScope) {
	if d == nil {
		return
	}

	if d.Scopes == nil {
		d.Scopes = map[OpenAIResponsesDirtyScope]bool{}
	}

	for _, scope := range scopes {
		if scope != "" {
			d.Scopes[scope] = true
		}
	}
}

func (d OpenAIResponsesDirtySet) Has(scope OpenAIResponsesDirtyScope) bool {
	if d.Scopes == nil {
		return false
	}

	return d.Scopes[scope]
}

func (d OpenAIResponsesDirtySet) HasAny(scopes ...OpenAIResponsesDirtyScope) bool {
	for _, scope := range scopes {
		if d.Has(scope) {
			return true
		}
	}

	return false
}

func EnsureOpenAIResponsesProviderExtensions(req *Request) *OpenAIResponsesProviderExtensions {
	if req == nil {
		return nil
	}

	if req.ProviderExtensions == nil {
		req.ProviderExtensions = &ProviderExtensions{}
	}

	if req.ProviderExtensions.OpenAIResponses == nil {
		req.ProviderExtensions.OpenAIResponses = &OpenAIResponsesProviderExtensions{}
	}

	return req.ProviderExtensions.OpenAIResponses
}

func MarkOpenAIResponsesDirty(req *Request, scopes ...OpenAIResponsesDirtyScope) {
	ext := EnsureOpenAIResponsesProviderExtensions(req)
	if ext == nil {
		return
	}

	ext.Dirty.Mark(scopes...)
}

func CloneProviderExtensions(src *ProviderExtensions) *ProviderExtensions {
	if src == nil {
		return nil
	}

	cloned := &ProviderExtensions{}
	if src.OpenAIResponses != nil {
		cloned.OpenAIResponses = cloneOpenAIResponsesProviderExtensions(src.OpenAIResponses)
	}

	return cloned
}

func cloneOpenAIResponsesProviderExtensions(src *OpenAIResponsesProviderExtensions) *OpenAIResponsesProviderExtensions {
	if src == nil {
		return nil
	}

	return &OpenAIResponsesProviderExtensions{
		Request:  cloneOpenAIResponsesRequestExtensions(src.Request),
		Response: cloneOpenAIResponsesResponseExtensions(src.Response),
		Stream:   cloneOpenAIResponsesStreamExtensions(src.Stream),
		Dirty:    cloneOpenAIResponsesDirtySet(src.Dirty),
	}
}

func cloneOpenAIResponsesRequestExtensions(src *OpenAIResponsesRequestExtensions) *OpenAIResponsesRequestExtensions {
	if src == nil {
		return nil
	}

	return &OpenAIResponsesRequestExtensions{
		RawBody:               cloneJSONRaw(src.RawBody),
		TopLevelExtra:         cloneJSONRawMap(src.TopLevelExtra),
		TopLevelSemanticExtra: cloneJSONRawMap(src.TopLevelSemanticExtra),
		MetadataRaw:           cloneJSONRaw(src.MetadataRaw),
		MetadataExtra:         cloneJSONRawMap(src.MetadataExtra),
		InputKind:             src.InputKind,
		InputRaw:              cloneJSONRaw(src.InputRaw),
		InstructionsRaw:       cloneJSONRaw(src.InstructionsRaw),
		InputItems:            cloneOpenAIResponsesRawItems(src.InputItems),
		Tools:                 cloneOpenAIResponsesRawItems(src.Tools),
		ToolChoiceRaw:         cloneJSONRaw(src.ToolChoiceRaw),
		ProtectableFragments:  cloneOpenAIResponsesProtectableFragments(src.ProtectableFragments),
	}
}

func cloneOpenAIResponsesResponseExtensions(src *OpenAIResponsesResponseExtensions) *OpenAIResponsesResponseExtensions {
	if src == nil {
		return nil
	}

	return &OpenAIResponsesResponseExtensions{
		Raw:           cloneJSONRaw(src.Raw),
		TopLevelExtra: cloneJSONRawMap(src.TopLevelExtra),
		MetadataRaw:   cloneJSONRaw(src.MetadataRaw),
		MetadataExtra: cloneJSONRawMap(src.MetadataExtra),
		OutputItems:   cloneOpenAIResponsesRawItems(src.OutputItems),
	}
}

func cloneOpenAIResponsesStreamExtensions(src *OpenAIResponsesStreamExtensions) *OpenAIResponsesStreamExtensions {
	if src == nil {
		return nil
	}

	return &OpenAIResponsesStreamExtensions{
		RawEvent: cloneOpenAIResponsesRawEvent(src.RawEvent),
	}
}

func cloneOpenAIResponsesRawItems(src []OpenAIResponsesRawItem) []OpenAIResponsesRawItem {
	if len(src) == 0 {
		return nil
	}

	cloned := make([]OpenAIResponsesRawItem, len(src))
	for i, item := range src {
		cloned[i] = item
		cloned[i].OriginalIndex = cloneValuePtr(item.OriginalIndex)
		cloned[i].ContentIndex = cloneValuePtr(item.ContentIndex)
		cloned[i].ConsumedSpan = cloneValuePtr(item.ConsumedSpan)
		cloned[i].Raw = cloneJSONRaw(item.Raw)
		cloned[i].Extra = cloneJSONRawMap(item.Extra)
		cloned[i].Protection.TextPaths = append([]string(nil), item.Protection.TextPaths...)
	}

	return cloned
}

func cloneOpenAIResponsesProtectableFragments(src []OpenAIResponsesProtectableFragment) []OpenAIResponsesProtectableFragment {
	if len(src) == 0 {
		return nil
	}

	return append([]OpenAIResponsesProtectableFragment(nil), src...)
}

func cloneOpenAIResponsesRawEvent(src *OpenAIResponsesRawEvent) *OpenAIResponsesRawEvent {
	if src == nil {
		return nil
	}

	cloned := *src
	cloned.SequenceNumber = cloneValuePtr(src.SequenceNumber)
	cloned.DataRaw = cloneJSONRaw(src.DataRaw)
	cloned.Raw = cloneJSONRaw(src.Raw)
	cloned.Extra = cloneJSONRawMap(src.Extra)

	return &cloned
}

func cloneOpenAIResponsesDirtySet(src OpenAIResponsesDirtySet) OpenAIResponsesDirtySet {
	if len(src.Scopes) == 0 {
		return OpenAIResponsesDirtySet{}
	}

	cloned := OpenAIResponsesDirtySet{Scopes: make(map[OpenAIResponsesDirtyScope]bool, len(src.Scopes))}
	for scope, dirty := range src.Scopes {
		cloned.Scopes[scope] = dirty
	}

	return cloned
}

func cloneJSONRaw(src json.RawMessage) json.RawMessage {
	return append(json.RawMessage(nil), src...)
}

func cloneJSONRawMap(src map[string]json.RawMessage) map[string]json.RawMessage {
	if len(src) == 0 {
		return nil
	}

	cloned := make(map[string]json.RawMessage, len(src))
	for key, value := range src {
		cloned[key] = cloneJSONRaw(value)
	}

	return cloned
}

func cloneValuePtr[T any](src *T) *T {
	if src == nil {
		return nil
	}

	cloned := *src

	return &cloned
}
