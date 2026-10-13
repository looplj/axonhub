package biz

import (
	"slices"
	"strings"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/llm"
)

func isResponsesEndpoint(format string) bool {
	return format == llm.APIFormatOpenAIResponse.String() || format == llm.APIFormatOpenAIResponseWebSocket.String()
}

// NormalizeEndpoints returns canonical copies; it never changes persisted input.
// Legacy explicit transport takes precedence over the inherited Base URL scheme.
func NormalizeEndpoints(endpoints []objects.ChannelEndpoint, baseURL string) []objects.ChannelEndpoint {
	normalized := slices.Clone(endpoints)
	for i, ep := range normalized {
		if !isResponsesEndpoint(ep.APIFormat) {
			continue
		}
		resolvedBase := ep.BaseURL
		if resolvedBase == "" {
			resolvedBase = baseURL
		}
		if ep.APIFormat == llm.APIFormatOpenAIResponse.String() {
			legacy := ep
			legacy.BaseURL = resolvedBase
			if endpointTransport(legacy) == objects.ChannelEndpointTransportWebSocket {
				ep.APIFormat = llm.APIFormatOpenAIResponseWebSocket.String()
			} else if ep.Transport == objects.ChannelEndpointTransportHTTP {
				// Keep explicit HTTP authoritative after omitting the legacy field.
				base := strings.TrimSpace(resolvedBase)
				switch {
				case strings.HasPrefix(strings.ToLower(base), "wss://"):
					ep.BaseURL = "https://" + base[len("wss://"):]
				case strings.HasPrefix(strings.ToLower(base), "ws://"):
					ep.BaseURL = "http://" + base[len("ws://"):]
				}
			}
		}
		ep.Transport = ""
		normalized[i] = ep
	}
	return normalized
}

// DefaultEndpoints applies stored Responses-family intent before the normal merge.
// Codex's family entries are authoritative; other providers retain their defaults.
func (c *Channel) DefaultEndpoints() []objects.ChannelEndpoint {
	if c.Channel == nil {
		return nil
	}
	defaults := slices.Clone(DefaultEndpointsForChannelType(c.Type))
	if c.Type != channel.TypeCodex {
		for i, ep := range defaults {
			if ep.APIFormat == llm.APIFormatOpenAIResponse.String() {
				defaults[i].APIFormat = legacyResponsesFormat(c.Channel)
			}
		}
		return defaults
	}
	allowed := make(map[string]bool)
	for _, ep := range NormalizeEndpoints(c.Endpoints, c.BaseURL) {
		if isResponsesEndpoint(ep.APIFormat) {
			allowed[ep.APIFormat] = true
		}
	}
	if len(allowed) == 0 {
		allowed[llm.APIFormatOpenAIResponseWebSocket.String()] = true
		allowed[llm.APIFormatOpenAIResponse.String()] = endpointTransport(objects.ChannelEndpoint{BaseURL: c.BaseURL}) != objects.ChannelEndpointTransportWebSocket
	}
	return slices.DeleteFunc(defaults, func(ep objects.ChannelEndpoint) bool {
		return isResponsesEndpoint(ep.APIFormat) && !allowed[ep.APIFormat]
	})
}

func legacyResponsesFormat(c *ent.Channel) string {
	for _, ep := range c.Endpoints {
		if ep.APIFormat == llm.APIFormatOpenAIResponse.String() {
			return NormalizeEndpoints([]objects.ChannelEndpoint{ep}, c.BaseURL)[0].APIFormat
		}
	}
	for _, ep := range c.Endpoints {
		if ep.APIFormat == llm.APIFormatOpenAIResponseWebSocket.String() {
			return llm.APIFormatOpenAIResponse.String()
		}
	}
	return NormalizeEndpoints([]objects.ChannelEndpoint{{APIFormat: llm.APIFormatOpenAIResponse.String()}}, c.BaseURL)[0].APIFormat
}

func normalizeModelProtocols(settings *objects.ChannelSettings, source *ent.Channel) *objects.ChannelSettings {
	if settings == nil || len(settings.ModelProtocols) == 0 || legacyResponsesFormat(source) != llm.APIFormatOpenAIResponseWebSocket.String() {
		return settings
	}
	copy := *settings
	copy.ModelProtocols = slices.Clone(settings.ModelProtocols)
	for i, mp := range copy.ModelProtocols {
		copy.ModelProtocols[i].APIFormats = slices.Clone(mp.APIFormats)
		for j, format := range mp.APIFormats {
			if format == llm.APIFormatOpenAIResponse.String() {
				copy.ModelProtocols[i].APIFormats[j] = llm.APIFormatOpenAIResponseWebSocket.String()
			}
		}
	}
	return &copy
}
