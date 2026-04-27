package biz

// ChannelLimiterForgetter lets the channel service notify the orchestrator's
// per-channel limiter manager when a channel is updated or deleted, so any
// previously cached limiter entry is dropped and the next request rebuilds it
// from the latest configuration.
//
// The interface lives in biz to avoid an import cycle with orchestrator. The
// orchestrator's ChannelLimiterManager satisfies it directly via Forget.
type ChannelLimiterForgetter interface {
	Forget(channelID int)
}

// SetChannelLimiterForgetter wires the optional limiter hook. Called once at
// startup from the orchestrator fx module.
func (svc *ChannelService) SetChannelLimiterForgetter(f ChannelLimiterForgetter) {
	svc.limiterForgetter = f
}

// forgetLimiter is the internal helper called from Update / Delete paths. Safe
// to call when no forgetter has been registered.
func (svc *ChannelService) forgetLimiter(channelID int) {
	if svc.limiterForgetter == nil {
		return
	}

	svc.limiterForgetter.Forget(channelID)
}
