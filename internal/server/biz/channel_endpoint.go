package biz

import (
	"fmt"
	"strings"

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

var openAICompatibleDefaultEndpoints = []objects.ChannelEndpoint{
	{APIFormat: llm.APIFormatOpenAIChatCompletion.String()},
	{APIFormat: llm.APIFormatOpenAIEmbedding.String()},
	{APIFormat: llm.APIFormatOpenAIImageGeneration.String()},
	{APIFormat: llm.APIFormatOpenAIImageEdit.String()},
	{APIFormat: llm.APIFormatOpenAIImageVariation.String()},
	{APIFormat: llm.APIFormatOpenAIVideo.String()},
}

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
	channel.TypeOpenai:          openAICompatibleDefaultEndpoints,
	channel.TypeOpenaiResponses: {{APIFormat: llm.APIFormatOpenAIResponse.String()}},
	channel.TypeCodex:           {{APIFormat: llm.APIFormatOpenAIResponse.String()}},
	channel.TypeVercel:          openAICompatibleDefaultEndpoints,
	channel.TypeAnthropic:       {{APIFormat: llm.APIFormatAnthropicMessage.String()}},
	channel.TypeAnthropicAWS:    {{APIFormat: llm.APIFormatAnthropicMessage.String()}},
	channel.TypeAnthropicGcp:    {{APIFormat: llm.APIFormatAnthropicMessage.String()}},
	channel.TypeGeminiOpenai:    {{APIFormat: llm.APIFormatOpenAIChatCompletion.String()}},
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
	channel.TypeDoubaoAnthropic:     {{APIFormat: llm.APIFormatAnthropicMessage.String()}},
	channel.TypeMoonshot:            {{APIFormat: llm.APIFormatOpenAIChatCompletion.String()}},
	channel.TypeMoonshotAnthropic:   {{APIFormat: llm.APIFormatAnthropicMessage.String()}},
	channel.TypeZhipu:               {{APIFormat: llm.APIFormatOpenAIChatCompletion.String()}},
	channel.TypeZai:                 {{APIFormat: llm.APIFormatOpenAIChatCompletion.String()}},
	channel.TypeZhipuAnthropic:      {{APIFormat: llm.APIFormatAnthropicMessage.String()}},
	channel.TypeZaiAnthropic:        {{APIFormat: llm.APIFormatAnthropicMessage.String()}},
	channel.TypeAnthropicFake:       {{APIFormat: llm.APIFormatAnthropicMessage.String()}},
	channel.TypeOpenaiFake:          {{APIFormat: llm.APIFormatOpenAIChatCompletion.String()}},
	channel.TypeOpenrouter:          {{APIFormat: llm.APIFormatOpenAIChatCompletion.String()}},
	channel.TypeXiaomi:              openAIChatOnlyDefaultEndpoints,
	channel.TypeXiaomiAnthropic:     {{APIFormat: llm.APIFormatAnthropicMessage.String()}},
	channel.TypeXai:                 {{APIFormat: llm.APIFormatOpenAIChatCompletion.String()}},
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
	channel.TypeNanogpt:          openAICompatibleDefaultEndpoints,
	channel.TypeNanogptResponses: {{APIFormat: llm.APIFormatOpenAIResponse.String()}},
	channel.TypeOllama:           {{APIFormat: llm.APIFormatOllamaChat.String()}},
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

func (c *Channel) platformTypeForGeminiEndpoint() string {
	if c == nil || c.Channel == nil {
		return ""
	}

	if c.Type == channel.TypeGeminiVertex {
		return gemini.PlatformVertex
	}

	return ""
}
