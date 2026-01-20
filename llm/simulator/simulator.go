package simulator

import (
	"context"
	"fmt"
	"net/http"

	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/transformer"
)

// Simulator simulates the flow from API layer to httpclient.
type Simulator struct {
	Inbound  transformer.Inbound
	Outbound transformer.Outbound
}

// NewSimulator creates a new simulator.
func NewSimulator(inbound transformer.Inbound, outbound transformer.Outbound) *Simulator {
	return &Simulator{
		Inbound:  inbound,
		Outbound: outbound,
	}
}

// Simulate simulates the processing of an AI request.
// It takes a raw http.Request (representing the client request),
// runs it through the inbound and outbound transformers,
// and returns the final http.Request that would be sent to the AI provider.
func (s *Simulator) Simulate(ctx context.Context, req *http.Request) (*http.Request, error) {
	// 1. Convert http.Request to httpclient.Request (Simulate API Layer receiving request)
	inboundHCReq, err := httpclient.ReadHTTPRequest(req)
	if err != nil {
		return nil, fmt.Errorf("failed to read inbound http request: %w", err)
	}

	// 2. Inbound Transformation (Simulate Inbound Transformer)
	llmReq, err := s.Inbound.TransformRequest(ctx, inboundHCReq)
	if err != nil {
		return nil, fmt.Errorf("inbound transformation failed: %w", err)
	}

	llmReq.RawRequest = inboundHCReq

	// 3. Outbound Transformation (Simulate Outbound Transformer)
	outboundHCReq, err := s.Outbound.TransformRequest(ctx, llmReq)
	if err != nil {
		return nil, fmt.Errorf("outbound transformation failed: %w", err)
	}

	outboundHCReq = httpclient.MergeInboundRequest(outboundHCReq, inboundHCReq)

	outboundHCReq, err = httpclient.FinalizeAuthHeaders(outboundHCReq)
	if err != nil {
		return nil, fmt.Errorf("finalize auth headers failed: %w", err)
	}

	// 4. Convert httpclient.Request to http.Request (Simulate HTTP Client building request)
	finalReq, err := httpclient.BuildHttpRequest(ctx, outboundHCReq)
	if err != nil {
		return nil, fmt.Errorf("failed to build final http request: %w", err)
	}

	return finalReq, nil
}
