package orchestrator

// isLocalAdmissionError identifies local capacity rejections before dispatch.
// They must not penalize upstream performance, credentials, circuit health, or
// cooldown. Their retry/failover and execution-persistence policies may differ:
// plugin overload stops failover and precedes creation of this attempt's record;
// channel queue/RPM rejection may fail over and has a record to finish.
func isLocalAdmissionError(err error) bool {
	return isChannelQueueError(err) || isLocalRPMExhaustedError(err) || isPluginOverloaded(err)
}
