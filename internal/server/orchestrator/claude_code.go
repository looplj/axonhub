package orchestrator

import (
	"context"

	entchannel "github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/pipeline"
	"github.com/looplj/axonhub/llm/transformer/anthropic/claudecode"
)

type billingSystemMessageMiddleware struct {
	pipeline.DummyMiddleware

	state *PersistenceState
}

var _ pipeline.OutboundLlmRequestMiddleware = (*billingSystemMessageMiddleware)(nil)

// newBillingSystemMessageMiddleware decides whether the Claude Code billing
// metadata survives the outbound transform for the channel selected for the
// current attempt.
func newBillingSystemMessageMiddleware(state *PersistenceState) pipeline.OutboundLlmRequestMiddleware {
	return &billingSystemMessageMiddleware{state: state}
}

func (m *billingSystemMessageMiddleware) Name() string {
	return "claudecode-billing-system-message"
}

func (m *billingSystemMessageMiddleware) OnOutboundLlmRequest(
	_ context.Context,
	request *llm.Request,
	_ llm.APIFormat,
) (*llm.Request, error) {
	var channel *biz.Channel
	if m.state != nil && m.state.CurrentCandidate != nil {
		channel = m.state.CurrentCandidate.Channel
	}

	if keepsBillingSystemMessages(channel) {
		return request, nil
	}

	return claudecode.RemoveBillingSystemMessages(request), nil
}

// keepsBillingSystemMessages reports whether the channel wants the billing
// system message forwarded untouched.
//
// The auto mode can only look at the hop between AxonHub and the channel, which
// misclassifies a relay that itself forwards to the official Claude Code
// endpoint: such a channel is not OAuth from here, yet the upstream rejects
// requests whose identity marker has been stripped. keep and strip exist so an
// operator can settle that case explicitly.
func keepsBillingSystemMessages(channel *biz.Channel) bool {
	if channel == nil {
		return false
	}

	if channel.Settings != nil {
		switch channel.Settings.ClaudeCodeBillingHeader {
		case objects.ClaudeCodeBillingHeaderKeep:
			return true
		case objects.ClaudeCodeBillingHeaderStrip:
			return false
		case objects.ClaudeCodeBillingHeaderAuto:
		}
	}

	return channel.Type == entchannel.TypeClaudecode && channel.Credentials.IsOAuth()
}

// billingStripWouldBeUndone reports whether replaying the raw inbound body would
// put a billing system message back on a channel pinned to strip.
//
// OnOutboundLlmRequest rewrites the unified request and deliberately leaves
// RawRequest.Body alone, so pass-through would otherwise resurrect the block.
// llmReq is the unfiltered inbound request, which makes the removal helper its
// own detector: an unchanged pointer means there was nothing to remove and
// pass-through stays byte-exact.
//
// Only the explicit strip mode is covered. Under auto the outcome still depends
// on pass-through, which is exactly the ambiguity the mode exists to resolve;
// changing auto here would alter behaviour for channels that never opted in.
func billingStripWouldBeUndone(channel *biz.Channel, llmReq *llm.Request) bool {
	if channel == nil || channel.Settings == nil || llmReq == nil {
		return false
	}

	if channel.Settings.ClaudeCodeBillingHeader != objects.ClaudeCodeBillingHeaderStrip {
		return false
	}

	return claudecode.RemoveBillingSystemMessages(llmReq) != llmReq
}
