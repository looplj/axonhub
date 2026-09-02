package orchestrator

import (
	"context"

	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/llm"
)

// SelectAPIFormat selects the most appropriate APIFormat from a channel's resolved endpoints
// based on the request type and inbound API format. Prefers an endpoint whose API format
// matches the inbound request format so that pass-through can be enabled when identical
// formats are used. Falls back to the first capable endpoint, then the first endpoint.
func SelectAPIFormat(endpoints []objects.ChannelEndpoint, req *llm.Request) string {
	if len(endpoints) == 0 {
		return ""
	}

	preferredFormat := string(req.APIFormat)
	allowed := llm.CapableAPIFormats(req.RequestType)

	if allowed != nil {
		if preferredFormat != "" {
			for _, ep := range endpoints {
				if _, ok := allowed[ep.APIFormat]; ok && ep.APIFormat == preferredFormat {
					return ep.APIFormat
				}
			}
		}

		for _, ep := range endpoints {
			if _, ok := allowed[ep.APIFormat]; ok {
				return ep.APIFormat
			}
		}

		if req.RequestType == llm.RequestTypeAlphaSearch {
			return ""
		}
	}

	return endpoints[0].APIFormat
}

// FilterEndpointsByAPIFormats restricts endpoints to the given api formats. The
// result keeps the priority order of allowed and preserves the relative order of
// endpoints sharing a format. Returns nil when allowed is empty or nothing matches.
func FilterEndpointsByAPIFormats(endpoints []objects.ChannelEndpoint, allowed []string) []objects.ChannelEndpoint {
	if len(allowed) == 0 || len(endpoints) == 0 {
		return nil
	}

	byFormat := make(map[string][]objects.ChannelEndpoint, len(endpoints))
	for _, ep := range endpoints {
		byFormat[ep.APIFormat] = append(byFormat[ep.APIFormat], ep)
	}

	filtered := make([]objects.ChannelEndpoint, 0, len(endpoints))
	seen := make(map[string]struct{}, len(allowed))
	for _, format := range allowed {
		if _, dup := seen[format]; dup {
			continue
		}

		seen[format] = struct{}{}
		filtered = append(filtered, byFormat[format]...)
	}

	if len(filtered) == 0 {
		return nil
	}

	return filtered
}

// forcedAPIFormatsForCandidate returns the union of ModelProtocols overrides that
// apply to this candidate. Overrides may be keyed by the request model, any
// channel-side model entry, or its mapped actual model. Formats keep configuration
// order; duplicates are removed.
func forcedAPIFormatsForCandidate(ch *biz.Channel, entries []biz.ChannelModelEntry, requestModel string) []string {
	if ch == nil || ch.Channel == nil || ch.Settings == nil || len(ch.Settings.ModelProtocols) == 0 {
		return nil
	}

	names := make([]string, 0, len(entries)*2+1)
	if requestModel != "" {
		names = append(names, requestModel)
	}

	for _, entry := range entries {
		if entry.RequestModel != "" && entry.RequestModel != requestModel {
			names = append(names, entry.RequestModel)
		}

		if entry.ActualModel == "" || entry.ActualModel == requestModel {
			continue
		}

		names = append(names, entry.ActualModel)
	}

	var forced []string

	seen := make(map[string]struct{})
	for _, name := range names {
		for _, format := range ch.ForcedAPIFormats(name) {
			if _, dup := seen[format]; dup {
				continue
			}

			seen[format] = struct{}{}
			forced = append(forced, format)
		}
	}

	return forced
}

// applyForcedAPIFormats narrows the channel's endpoint surface to the per-model
// protocol overrides for this candidate. When the overrides reference no configured
// endpoint (configuration drift), all endpoints are kept so the request can still
// be served.
func applyForcedAPIFormats(
	ctx context.Context,
	ch *biz.Channel,
	entries []biz.ChannelModelEntry,
	requestModel string,
	endpoints []objects.ChannelEndpoint,
) []objects.ChannelEndpoint {
	forced := forcedAPIFormatsForCandidate(ch, entries, requestModel)
	if len(forced) == 0 {
		return endpoints
	}

	if filtered := FilterEndpointsByAPIFormats(endpoints, forced); len(filtered) > 0 {
		return filtered
	}

	log.Debug(ctx, "model protocol override matches no configured endpoint, falling back to all endpoints",
		log.String("channel", ch.Name),
		log.String("model", requestModel),
		log.Strings("forced_formats", forced),
	)

	return endpoints
}
