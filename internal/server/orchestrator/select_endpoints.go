package orchestrator

import (
	"github.com/looplj/axonhub/internal/objects"
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
	}

	return endpoints[0].APIFormat
}
