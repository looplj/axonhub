package permission

import (
	"path/filepath"
	"regexp"
	"strings"

	"github.com/looplj/axonhub/axon/permission/policy"
)

var hardDenyCommandPatterns = []*regexp.Regexp{
	regexp.MustCompile(`\brm\s+(-[a-zA-Z]*f[a-zA-Z]*\s+)?/`),
	regexp.MustCompile(`\brm\s+-rf\b`),
	regexp.MustCompile(`\bmkfs\b`),
	regexp.MustCompile(`\bwipefs\b`),
	regexp.MustCompile(`\bshutdown\b`),
	regexp.MustCompile(`\breboot\b`),
	regexp.MustCompile(`\bsudo\b`),
	regexp.MustCompile(`\bnc\s+-[a-z]*e\b`),
	regexp.MustCompile(`/dev/tcp/`),
}

func HardDeny(toolName string, resources []policy.Resource, workspace string) (ToolDecision, bool) {
	if toolName == "Bash" {
		for _, r := range resources {
			if r.Type != policy.ResourceCommand {
				continue
			}
			for _, p := range hardDenyCommandPatterns {
				if p.MatchString(r.Command) {
					return ToolDecision{
						Effect:    EffectDeny,
						RuleID:    "hard_deny.bash",
						Reason:    "command denied by hard deny filter",
						RiskLevel: RiskCritical,
						Resources: resources,
						Display: DecisionDisplay{
							Summary: "Hard deny: dangerous command pattern",
						},
					}, true
				}
			}
		}
	}

	// Deny clearly sensitive paths regardless of policy (defense-in-depth).
	for _, r := range resources {
		if r.Type != policy.ResourcePath {
			continue
		}
		if isSensitivePath(r.Path) {
			return ToolDecision{
				Effect:    EffectDeny,
				RuleID:    "hard_deny.sensitive_path",
				Reason:    "access to sensitive path denied",
				RiskLevel: RiskCritical,
				Resources: resources,
				Display: DecisionDisplay{
					Summary: "Hard deny: sensitive path",
				},
			}, true
		}
	}

	// Deny non-http(s) URL schemes in WebFetch by default.
	for _, r := range resources {
		if r.Type != policy.ResourceURL {
			continue
		}
		if r.Scheme != "" && r.Scheme != "http" && r.Scheme != "https" {
			return ToolDecision{
				Effect:    EffectDeny,
				RuleID:    "hard_deny.net.scheme",
				Reason:    "non-http(s) scheme denied",
				RiskLevel: RiskHigh,
				Resources: resources,
				Display: DecisionDisplay{
					Summary: "Hard deny: non-http(s) URL scheme",
				},
			}, true
		}
	}

	_ = workspace
	return ToolDecision{}, false
}

func isSensitivePath(p string) bool {
	cp := filepath.Clean(p)
	l := strings.ToLower(cp)

	denyPrefixes := []string{
		"/etc/",
		"/system/",
		"/private/etc/",
		"/private/var/db/",
		"/var/db/",
	}
	for _, pre := range denyPrefixes {
		if strings.HasPrefix(l, pre) {
			return true
		}
	}

	// macOS user secrets
	if strings.Contains(l, "/.ssh/") || strings.Contains(l, "/.gnupg/") {
		return true
	}

	return false
}
