package biz

import (
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/llm"
)

// hasCapableEndpointForModel reports whether any structurally matched channel
// exposes an endpoint that can serve the configured model type.
//
// This is a list-time capability check. It intentionally ignores request-specific
// When conditions, stream policy, and quota so /v1/models stays stable.
func hasCapableEndpointForModel(m *ent.Model, connections []*ModelChannelConnection) bool {
	if m == nil || len(connections) == 0 {
		return false
	}

	allowed := llm.CapableAPIFormats(llm.RequestTypeForModelType(m.Type.String()))
	if len(allowed) == 0 {
		// Unknown model types keep the existing association-only visibility.
		return true
	}

	for _, conn := range connections {
		if conn == nil || conn.Channel == nil {
			continue
		}

		endpoints := mergeEndpoints(DefaultEndpointsForChannelType(conn.Channel.Type), conn.Channel.Endpoints)
		for _, ep := range endpoints {
			if _, ok := allowed[ep.APIFormat]; ok {
				return true
			}
		}
	}

	return false
}
