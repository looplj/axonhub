package llm

// LossyDowngrade records a protocol field that could not be represented when
// converting across protocol families. It is diagnostic data only; transformers
// must not use it as native field storage.
type LossyDowngrade struct {
	SourceProtocol APIFormat              `json:"source_protocol"`
	SourceField    string                 `json:"source_field"`
	TargetProtocol APIFormat              `json:"target_protocol"`
	Reason         string                 `json:"reason"`
	Severity       LossyDowngradeSeverity `json:"severity"`
}

type LossyDowngradeSeverity string

const (
	LossyDowngradeSeverityWarning LossyDowngradeSeverity = "warning"
)

const (
	LossyDowngradeReasonNoEquivalentSemantics = "no_equivalent_semantics"
)

// AddLossyDowngrade appends a diagnostic unless the same source field has
// already been reported for the same source/target/reason tuple.
func AddLossyDowngrade(req *Request, diagnostic LossyDowngrade) {
	if req == nil || diagnostic.SourceField == "" || diagnostic.SourceProtocol == "" || diagnostic.TargetProtocol == "" || diagnostic.Reason == "" || diagnostic.Severity == "" {
		return
	}

	diagnostics := EnsureDiagnosticsProviderExtensions(req)
	if diagnostics == nil {
		return
	}

	for _, existing := range diagnostics.LossyDowngrades {
		if existing == diagnostic {
			return
		}
	}

	diagnostics.LossyDowngrades = append(diagnostics.LossyDowngrades, diagnostic)
}

// AddLossyDowngradeIfPresent records a standard cross-protocol downgrade diagnostic
// when present is true.
func AddLossyDowngradeIfPresent(req *Request, sourceProtocol APIFormat, sourceField string, targetProtocol APIFormat, present bool) {
	if !present {
		return
	}

	AddLossyDowngrade(req, LossyDowngrade{
		SourceProtocol: sourceProtocol,
		SourceField:    sourceField,
		TargetProtocol: targetProtocol,
		Reason:         LossyDowngradeReasonNoEquivalentSemantics,
		Severity:       LossyDowngradeSeverityWarning,
	})
}

func LossyDowngrades(req *Request) []LossyDowngrade {
	if req == nil || req.ProviderExtensions == nil || req.ProviderExtensions.Diagnostics == nil {
		return nil
	}

	return req.ProviderExtensions.Diagnostics.LossyDowngrades
}
