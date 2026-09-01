package biz

import (
	"fmt"
	"strings"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/transformer/gemini"
)

// SupportedAPIFormats lists the API formats that are recognized as valid endpoint api_format values.
var SupportedAPIFormats = map[string]struct{}{
	llm.APIFormatOpenAIChatCompletion.String():  {},
	llm.APIFormatOpenAICompletion.String():      {},
	llm.APIFormatOpenAIResponse.String():        {},
	llm.APIFormatOpenAIResponseCompact.String(): {},
	llm.APIFormatOpenAIEmbedding.String():       {},
	llm.APIFormatOpenAIImageGeneration.String(): {},
	llm.APIFormatOpenAIImageEdit.String():       {},
	llm.APIFormatOpenAIImageVariation.String():  {},
	llm.APIFormatOpenAIVideo.String():           {},
	llm.APIFormatOpenAISpeech.String():          {},
	llm.APIFormatOpenAITranscription.String():   {},
	llm.APIFormatOpenAITranslation.String():     {},
	llm.APIFormatOpenAIModeration.String():      {},
	llm.APIFormatOpenAIAlphaSearch.String():     {},
	llm.APIFormatAnthropicMessage.String():      {},
	llm.APIFormatGeminiContents.String():        {},
	llm.APIFormatGeminiEmbedding.String():       {},
	llm.APIFormatJinaRerank.String():            {},
	llm.APIFormatJinaEmbedding.String():         {},
}

// ValidateEndpoints validates channel endpoint configurations.
// Ensures api_format is non-empty, supported, and unique within the channel.
// Ensures path is empty, starts with "/", and is not a full URL.
func ValidateEndpoints(endpoints []objects.ChannelEndpoint) error {
	seen := make(map[string]bool, len(endpoints))
	for i, ep := range endpoints {
		if ep.APIFormat == "" {
			return fmt.Errorf("endpoint[%d]: api_format is required", i)
		}

		if _, ok := SupportedAPIFormats[ep.APIFormat]; !ok {
			return fmt.Errorf("endpoint[%d]: unsupported api_format %q", i, ep.APIFormat)
		}

		if seen[ep.APIFormat] {
			return fmt.Errorf("endpoint[%d]: duplicate api_format %q", i, ep.APIFormat)
		}

		seen[ep.APIFormat] = true

		if ep.Transport != "" && ep.Transport != objects.ChannelEndpointTransportHTTP && ep.Transport != objects.ChannelEndpointTransportWebSocket {
			return fmt.Errorf("endpoint[%d]: unsupported transport %q", i, ep.Transport)
		}

		if ep.Transport == objects.ChannelEndpointTransportWebSocket && !supportsWebSocketTransport(ep.APIFormat) {
			return fmt.Errorf("endpoint[%d]: websocket transport only supports api_format %q or %q", i, llm.APIFormatOpenAIResponse.String(), llm.APIFormatOpenAIResponseCompact.String())
		}

		if ep.Path != "" {
			if strings.HasPrefix(ep.Path, "http://") || strings.HasPrefix(ep.Path, "https://") {
				return fmt.Errorf("endpoint[%d]: path must not be a full URL, got %q", i, ep.Path)
			}

			if !strings.HasPrefix(ep.Path, "/") {
				return fmt.Errorf("endpoint[%d]: path must start with '/', got %q", i, ep.Path)
			}
		}
	}

	return nil
}

func supportsWebSocketTransport(apiFormat string) bool {
	return apiFormat == llm.APIFormatOpenAIResponse.String() || apiFormat == llm.APIFormatOpenAIResponseCompact.String()
}

// ValidateModelProtocols validates the channel settings' per-model protocol overrides.
// Each entry requires a non-empty model (unique within the channel), at least one
// api_format, and every api_format on an enabled entry must already be available
// on the channel — i.e. present in the type's default endpoints or the
// user-configured endpoints. A manually disabled entry for a model that still
// exists may keep its protocol choices while inactive.
func ValidateModelProtocols(settings *objects.ChannelSettings, channelType channel.Type, endpoints []objects.ChannelEndpoint) error {
	if settings == nil || len(settings.ModelProtocols) == 0 {
		return nil
	}

	available := make(map[string]struct{})
	for _, ep := range DefaultEndpointsForChannelType(channelType) {
		available[ep.APIFormat] = struct{}{}
	}

	for _, ep := range endpoints {
		available[ep.APIFormat] = struct{}{}
	}

	seen := make(map[string]struct{}, len(settings.ModelProtocols))
	for i, mp := range settings.ModelProtocols {
		if mp.Model == "" {
			return fmt.Errorf("modelProtocols[%d]: model is required", i)
		}

		if _, dup := seen[mp.Model]; dup {
			return fmt.Errorf("modelProtocols[%d]: duplicate model %q", i, mp.Model)
		}

		seen[mp.Model] = struct{}{}

		if len(mp.APIFormats) == 0 {
			return fmt.Errorf("modelProtocols[%d] (%s): at least one api_format is required", i, mp.Model)
		}

		// Manually disabled overrides no longer need to track endpoint changes while
		// off. Overrides for removed models are deleted by model-list normalization.
		if !mp.IsEnabled() {
			continue
		}

		for _, format := range mp.APIFormats {
			if _, ok := available[format]; !ok {
				return fmt.Errorf("modelProtocols[%d] (%s): api_format %q is not configured on this channel", i, mp.Model, format)
			}
		}
	}

	return nil
}

// RemoveRemovedModelProtocolOverrides deletes overrides for models that are no
// longer exposed by the channel. Once a model disappears, its protocol override
// disappears with it instead of remaining as an inactive stale record. The model
// list includes direct models and derived request names (prefixes, auto-trimmed
// names, and mappings), matching runtime model lookup.
func RemoveRemovedModelProtocolOverrides(settings *objects.ChannelSettings, supportedModels []string) bool {
	if settings == nil || len(settings.ModelProtocols) == 0 {
		return false
	}

	available := modelProtocolAvailableModels(settings, supportedModels)
	kept := make([]objects.ModelProtocol, 0, len(settings.ModelProtocols))
	changed := false
	for _, protocol := range settings.ModelProtocols {
		if _, present := available[protocol.Model]; present {
			kept = append(kept, protocol)
			continue
		}

		changed = true
	}

	if changed {
		settings.ModelProtocols = kept
	}

	return changed
}

func modelProtocolAvailableModels(settings *objects.ChannelSettings, supportedModels []string) map[string]struct{} {
	probe := new(Channel)
	probe.Channel = new(ent.Channel)
	probe.Channel.SupportedModels = supportedModels
	probe.Channel.Settings = settings
	entries := probe.GetModelEntries()
	available := make(map[string]struct{}, len(entries)+len(supportedModels))
	for model := range entries {
		available[model] = struct{}{}
	}
	// Hidden-original/mapped settings only affect exposure. Direct models remain
	// valid override targets because runtime matching also considers actual names.
	for _, model := range supportedModels {
		available[model] = struct{}{}
		if settings.LowercaseModelID {
			available[strings.ToLower(model)] = struct{}{}
		}
	}

	return available
}

var openAICompatibleDefaultEndpoints = []objects.ChannelEndpoint{
	{APIFormat: llm.APIFormatOpenAIChatCompletion.String()},
	{APIFormat: llm.APIFormatOpenAIEmbedding.String()},
	{APIFormat: llm.APIFormatOpenAIImageGeneration.String()},
	{APIFormat: llm.APIFormatOpenAIImageEdit.String()},
	{APIFormat: llm.APIFormatOpenAIImageVariation.String()},
	{APIFormat: llm.APIFormatOpenAIVideo.String()},
	{APIFormat: llm.APIFormatOpenAIModeration.String()},
}

// openAIFullDefaultEndpoints includes the audio endpoints on top of the compatible set.
// Audio defaults are only granted to channel types confirmed to support the OpenAI
// /audio/* APIs; other compatible channels can opt in via custom endpoints.
var openAIFullDefaultEndpoints = append(
	append([]objects.ChannelEndpoint{}, openAICompatibleDefaultEndpoints...),
	objects.ChannelEndpoint{APIFormat: llm.APIFormatOpenAISpeech.String()},
	objects.ChannelEndpoint{APIFormat: llm.APIFormatOpenAITranscription.String()},
	objects.ChannelEndpoint{APIFormat: llm.APIFormatOpenAITranslation.String()},
)

var openAIChatOnlyDefaultEndpoints = []objects.ChannelEndpoint{
	{APIFormat: llm.APIFormatOpenAIChatCompletion.String()},
}

// defaultEndpointsForChannelType defines the built-in default endpoints for
// each channel type.
//
// A default endpoint is a first-class built-in capability surface owned by the
// channel type. The first endpoint is the primary endpoint and backs
// Channel.Outbound for backward compatibility. Additional entries are peer
// default endpoints, each mapped to exactly one API format / outbound
// transformer pair.
//
// Only include endpoints that are intentionally part of the channel type's
// built-in contract. User-configured custom endpoints remain external overrides
// and are not modeled here.
var defaultEndpointsForChannelType = map[channel.Type][]objects.ChannelEndpoint{
	channel.TypeOpenai:          openAIFullDefaultEndpoints,
	channel.TypeOpenaiResponses: {{APIFormat: llm.APIFormatOpenAIResponse.String()}},
	channel.TypeAtlascloud:      openAICompatibleDefaultEndpoints,
	channel.TypeQiniu:           {{APIFormat: llm.APIFormatOpenAIChatCompletion.String()}},
	channel.TypeQiniuAnthropic:  {{APIFormat: llm.APIFormatAnthropicMessage.String()}},
	channel.TypeCline:           openAIChatOnlyDefaultEndpoints,
	channel.TypeCodex: {
		{APIFormat: llm.APIFormatOpenAIResponse.String()},
		{APIFormat: llm.APIFormatOpenAIAlphaSearch.String()},
		{APIFormat: llm.APIFormatOpenAIImageGeneration.String()},
		{APIFormat: llm.APIFormatOpenAIImageEdit.String()},
	},
	channel.TypeFenno: {
		{APIFormat: llm.APIFormatOpenAIResponse.String()},
	},
	channel.TypeVercel:       openAICompatibleDefaultEndpoints,
	channel.TypeAnthropic:    {{APIFormat: llm.APIFormatAnthropicMessage.String()}},
	channel.TypeAnthropicAWS: {{APIFormat: llm.APIFormatAnthropicMessage.String()}},
	channel.TypeAnthropicGcp: {{APIFormat: llm.APIFormatAnthropicMessage.String()}},
	channel.TypeGeminiOpenai: {{APIFormat: llm.APIFormatOpenAIChatCompletion.String()}},
	channel.TypeGemini: {
		{APIFormat: llm.APIFormatGeminiContents.String()},
		{APIFormat: llm.APIFormatGeminiEmbedding.String()},
	},
	channel.TypeGeminiVertex: {
		{APIFormat: llm.APIFormatGeminiContents.String()},
		{APIFormat: llm.APIFormatGeminiEmbedding.String()},
	},
	channel.TypeDeepseek:          {{APIFormat: llm.APIFormatOpenAIChatCompletion.String()}, {APIFormat: llm.APIFormatOpenAICompletion.String()}},
	channel.TypeDeepseekAnthropic: {{APIFormat: llm.APIFormatAnthropicMessage.String()}},
	channel.TypeDeepinfra:         openAICompatibleDefaultEndpoints,
	channel.TypeFireworks:         {{APIFormat: llm.APIFormatOpenAIChatCompletion.String()}},
	channel.TypeDoubao: {
		{APIFormat: llm.APIFormatOpenAIChatCompletion.String()},
		{APIFormat: llm.APIFormatSeedanceVideo.String()},
	},
	channel.TypeDoubaoAnthropic:   {{APIFormat: llm.APIFormatAnthropicMessage.String()}},
	channel.TypeMoonshot:          {{APIFormat: llm.APIFormatOpenAIChatCompletion.String()}},
	channel.TypeMoonshotAnthropic: {{APIFormat: llm.APIFormatAnthropicMessage.String()}},
	channel.TypeZhipu:             {{APIFormat: llm.APIFormatOpenAIChatCompletion.String()}},
	channel.TypeZai:               {{APIFormat: llm.APIFormatOpenAIChatCompletion.String()}},
	channel.TypeZhipuAnthropic:    {{APIFormat: llm.APIFormatAnthropicMessage.String()}},
	channel.TypeZaiAnthropic:      {{APIFormat: llm.APIFormatAnthropicMessage.String()}},
	channel.TypeAnthropicFake:     {{APIFormat: llm.APIFormatAnthropicMessage.String()}},
	channel.TypeOpenaiFake:        {{APIFormat: llm.APIFormatOpenAIChatCompletion.String()}},
	channel.TypeOpenrouter: {
		{APIFormat: llm.APIFormatOpenAIChatCompletion.String()},
		{APIFormat: llm.APIFormatOpenAISpeech.String()},
		{APIFormat: llm.APIFormatOpenAITranscription.String()},
		{APIFormat: llm.APIFormatOpenAITranslation.String()},
	},
	channel.TypeXiaomi:          openAIChatOnlyDefaultEndpoints,
	channel.TypeXiaomiAnthropic: {{APIFormat: llm.APIFormatAnthropicMessage.String()}},
	channel.TypeXai: {
		{APIFormat: llm.APIFormatOpenAIChatCompletion.String()},
		{APIFormat: llm.APIFormatOpenAIResponse.String()},
	},
	channel.TypeXaiResponses:        {{APIFormat: llm.APIFormatOpenAIResponse.String()}},
	channel.TypeXaiSubscription:     {{APIFormat: llm.APIFormatOpenAIResponse.String()}},
	channel.TypePpio:                openAICompatibleDefaultEndpoints,
	channel.TypeSiliconflow:         openAICompatibleDefaultEndpoints,
	channel.TypeVolcengine:          {{APIFormat: llm.APIFormatOpenAIChatCompletion.String()}},
	channel.TypeVolcengineAnthropic: {{APIFormat: llm.APIFormatAnthropicMessage.String()}},
	channel.TypeLongcat:             {{APIFormat: llm.APIFormatOpenAIChatCompletion.String()}},
	channel.TypeLongcatAnthropic:    {{APIFormat: llm.APIFormatAnthropicMessage.String()}},
	channel.TypeMinimax:             openAIChatOnlyDefaultEndpoints,
	channel.TypeMinimaxAnthropic:    {{APIFormat: llm.APIFormatAnthropicMessage.String()}},
	channel.TypeAihubmix:            openAICompatibleDefaultEndpoints,
	channel.TypeAihubmixAnthropic:   {{APIFormat: llm.APIFormatAnthropicMessage.String()}},
	channel.TypeBurncloud:           openAICompatibleDefaultEndpoints,
	channel.TypeModelscope:          {{APIFormat: llm.APIFormatOpenAIChatCompletion.String()}},
	channel.TypeBailian:             {{APIFormat: llm.APIFormatOpenAIChatCompletion.String()}},
	channel.TypeBailianAnthropic:    {{APIFormat: llm.APIFormatAnthropicMessage.String()}},
	channel.TypeMoonshotCoding:      {{APIFormat: llm.APIFormatAnthropicMessage.String()}},
	channel.TypeJina: {
		{APIFormat: llm.APIFormatJinaRerank.String()},
		{APIFormat: llm.APIFormatJinaEmbedding.String()},
	},
	channel.TypeGithub:           openAICompatibleDefaultEndpoints,
	channel.TypeGithubCopilot:    {{APIFormat: llm.APIFormatOpenAIChatCompletion.String()}},
	channel.TypeClaudecode:       {{APIFormat: llm.APIFormatAnthropicMessage.String()}},
	channel.TypeCerebras:         {{APIFormat: llm.APIFormatOpenAIChatCompletion.String()}},
	channel.TypeAntigravity:      {{APIFormat: llm.APIFormatGeminiContents.String()}},
	channel.TypeNanogpt:          openAIFullDefaultEndpoints,
	channel.TypeNanogptResponses: {{APIFormat: llm.APIFormatOpenAIResponse.String()}},
	channel.TypeOpencodeGo:       openAIChatOnlyDefaultEndpoints,
	channel.TypeOllama:           {{APIFormat: llm.APIFormatOllamaChat.String()}},
	channel.TypeOllamaAnthropic:  {{APIFormat: llm.APIFormatAnthropicMessage.String()}},
	channel.TypeEvolink:          openAICompatibleDefaultEndpoints,
	channel.TypeEvolinkAnthropic: {{APIFormat: llm.APIFormatAnthropicMessage.String()}},
	channel.TypeGroq: {
		{APIFormat: llm.APIFormatOpenAIChatCompletion.String()},
		{APIFormat: llm.APIFormatOpenAISpeech.String()},
		{APIFormat: llm.APIFormatOpenAITranscription.String()},
		{APIFormat: llm.APIFormatOpenAITranslation.String()},
	},
}

func DefaultEndpointsForChannelType(t channel.Type) []objects.ChannelEndpoint {
	if eps, ok := defaultEndpointsForChannelType[t]; ok {
		return eps
	}

	return nil
}

func mergeEndpoints(defaultEndpoints, userEndpoints []objects.ChannelEndpoint) []objects.ChannelEndpoint {
	if len(defaultEndpoints) == 0 && len(userEndpoints) == 0 {
		return nil
	}

	merged := make([]objects.ChannelEndpoint, 0, len(defaultEndpoints)+len(userEndpoints))

	overrides := make(map[string]objects.ChannelEndpoint, len(userEndpoints))

	for _, ep := range userEndpoints {
		if ep.APIFormat == "" {
			continue
		}

		overrides[ep.APIFormat] = ep
	}

	for _, ep := range defaultEndpoints {
		if ep.APIFormat == "" {
			continue
		}

		if override, ok := overrides[ep.APIFormat]; ok {
			merged = append(merged, override)

			delete(overrides, ep.APIFormat)

			continue
		}

		merged = append(merged, ep)
	}

	for _, ep := range userEndpoints {
		if ep.APIFormat == "" {
			continue
		}

		if _, ok := overrides[ep.APIFormat]; !ok {
			continue
		}

		merged = append(merged, ep)

		delete(overrides, ep.APIFormat)
	}

	return merged
}

// ResolveEndpoints returns the runtime-effective endpoints used for API format
// selection. Built-in default endpoints define the channel's capability
// surface, and user-configured endpoints override matching api_format entries
// or append additional ones.
func (c *Channel) ResolveEndpoints() []objects.ChannelEndpoint {
	if c.Channel == nil {
		return nil
	}

	return mergeEndpoints(DefaultEndpointsForChannelType(c.Type), c.Endpoints)
}

// ForcedAPIFormats returns the api formats force-specified for the given request
// model via ChannelSettings.ModelProtocols, or nil when the model has no override.
// Results are in configured priority order.
func (c *Channel) ForcedAPIFormats(model string) []string {
	if c == nil || c.Channel == nil || c.Settings == nil || model == "" {
		return nil
	}

	for _, mp := range c.Settings.ModelProtocols {
		if mp.Model == model && mp.IsEnabled() {
			return mp.APIFormats
		}
	}

	return nil
}

func (c *Channel) platformTypeForGeminiEndpoint() string {
	if c == nil || c.Channel == nil {
		return ""
	}

	if c.Type == channel.TypeGeminiVertex {
		return gemini.PlatformVertex
	}

	return ""
}
