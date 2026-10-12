package biz

import (
	"strings"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/transformer"
	"github.com/looplj/axonhub/llm/transformer/openai"
	"github.com/looplj/axonhub/llm/transformer/openai/codex"
	"github.com/looplj/axonhub/llm/transformer/opencode"
)

func isRelayImageChannel(typ channel.Type) bool {
	switch typ {
	case channel.TypeCodex, channel.TypeFenno, channel.TypeOpenaiResponses, channel.TypeZenmuxResponses,
		channel.TypeNanogptResponses, channel.TypeXaiResponses, channel.TypeBailianResponses, channel.TypeOpencodeGo:
		return true
	default:
		return false
	}
}

func isNativeImageFormat(format string) bool {
	return format == llm.APIFormatOpenAIImageGeneration.String() || format == llm.APIFormatOpenAIImageEdit.String()
}

func appendNativeImageEndpoints(endpoints []objects.ChannelEndpoint, baseURL string) []objects.ChannelEndpoint {
	result := append([]objects.ChannelEndpoint(nil), endpoints...)
	for _, format := range []llm.APIFormat{llm.APIFormatOpenAIImageGeneration, llm.APIFormatOpenAIImageEdit} {
		found := false
		for _, ep := range endpoints {
			if ep.APIFormat == format.String() {
				found = true
				break
			}
		}
		if !found {
			result = append(result, objects.ChannelEndpoint{APIFormat: format.String(), BaseURL: baseURL})
		}
	}
	return result
}

func resolveRelayImageEndpoints(c *Channel, endpoints []objects.ChannelEndpoint) []objects.ChannelEndpoint {
	var responseBaseURL string
	customResponse := false
	for _, ep := range c.Endpoints {
		if ep.APIFormat == llm.APIFormatOpenAIResponse.String() {
			responseBaseURL = ep.BaseURL
		}
		if len(endpoints) > 0 && ep.APIFormat == endpoints[0].APIFormat {
			customResponse = true
		}
	}
	if !isRelayImageChannel(c.Type) {
		if !customResponse || len(endpoints) == 0 || (endpoints[0].APIFormat != llm.APIFormatOpenAIResponse.String() && endpoints[0].APIFormat != llm.APIFormatOpenAIResponseCompact.String()) {
			return endpoints
		}
		for _, ep := range endpoints {
			if _, capable := llm.CapableAPIFormats(llm.RequestTypeImage)[ep.APIFormat]; capable {
				return endpoints
			}
		}
		return appendNativeImageEndpoints(endpoints, endpoints[0].BaseURL)
	}
	for i, ep := range endpoints {
		if !isNativeImageFormat(ep.APIFormat) {
			continue
		}
		explicit := false
		for _, configured := range c.Endpoints {
			if configured.APIFormat == ep.APIFormat {
				explicit = true
				break
			}
		}
		if !explicit {
			endpoints[i].BaseURL = responseBaseURL
		}
	}
	return endpoints
}

func buildRelayImageOutbound(c *ent.Channel, ch *Channel, ep objects.ChannelEndpoint) (transformer.Outbound, error) {
	baseURL := c.BaseURL
	if ep.BaseURL != "" {
		baseURL = ep.BaseURL
	}
	if c.Type == channel.TypeCodex || c.Type == channel.TypeFenno {
		primary := ch.Outbound.(*codex.OutboundTransformer)
		return codex.NewImageOutboundTransformer(codex.ImageParams{TokenProvider: primary.TokenProvider(), BaseURL: baseURL, EndpointPath: ep.Path})
	}
	baseURL = strings.Replace(strings.Replace(baseURL, "wss://", "https://", 1), "ws://", "http://", 1)
	out, err := openai.NewOutboundTransformerWithConfig(&openai.Config{PlatformType: openai.PlatformOpenAI, BaseURL: baseURL, EndpointPath: ep.Path, APIKeyProvider: getAPIKeyProvider(ch)})
	if err != nil {
		return nil, err
	}
	if c.Type == channel.TypeOpencodeGo {
		out = opencode.WithSessionHeader(out)
	}
	return out, nil
}
