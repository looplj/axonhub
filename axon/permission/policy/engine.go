package policy

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

type Engine struct {
	doc Document

	compiledRules []compiledRule
}

type compiledRule struct {
	rule         Rule
	commandRegex []*regexp.Regexp
	pathRegex    []*regexp.Regexp
}

type ResourceType string

const (
	ResourcePath    ResourceType = "path"
	ResourceCommand ResourceType = "command"
	ResourceURL     ResourceType = "url"
	ResourceDomain  ResourceType = "domain"
)

type Resource struct {
	Type ResourceType

	Path             string
	WorkspaceRel     string
	OutsideWorkspace bool

	Command string
	Cwd     string

	URL    string
	Domain string
	Scheme string
}

type Decision struct {
	Effect    Effect
	RuleID    string
	Reason    string
	RiskLevel string
}

func New(doc Document) (*Engine, error) {
	e := &Engine{doc: doc}
	for _, r := range doc.Rules {
		cr := compiledRule{rule: r}
		for _, pat := range r.When.Resource.CommandMatches {
			re, err := regexp.Compile(pat)
			if err != nil {
				return nil, fmt.Errorf("policy: rule %s invalid command_matches regex %q: %w", r.ID, pat, err)
			}
			cr.commandRegex = append(cr.commandRegex, re)
		}
		for _, pat := range r.When.Resource.PathMatches {
			re, err := compilePathGlob(pat)
			if err != nil {
				return nil, fmt.Errorf("policy: rule %s invalid path_matches glob %q: %w", r.ID, pat, err)
			}
			cr.pathRegex = append(cr.pathRegex, re)
		}
		e.compiledRules = append(e.compiledRules, cr)
	}
	return e, nil
}

// Evaluate returns the policy decision for a tool request.
// Default mode is controlled by Document.Defaults.Mode:
// - require_approval_by_default (default)
// - deny_by_default
// - allow_by_default
func (e *Engine) Evaluate(capabilities []string, resources []Resource) Decision {
	var allow *Decision
	var require *Decision

	for _, cr := range e.compiledRules {
		if !matchCapabilities(cr.rule.When.CapabilityIn, capabilities) {
			continue
		}
		if !matchResources(cr, resources) {
			continue
		}

		dec := Decision{RuleID: cr.rule.ID, Reason: cr.rule.Reason, RiskLevel: cr.rule.RiskLevel}

		switch cr.rule.Effect {
		case EffectDeny:
			dec.Effect = EffectDeny
			if dec.RiskLevel == "" {
				dec.RiskLevel = "high"
			}
			if dec.Reason == "" {
				dec.Reason = "denied by policy rule"
			}
			return dec
		case EffectAllow:
			dec.Effect = EffectAllow
			if dec.RiskLevel == "" {
				dec.RiskLevel = "low"
			}
			if dec.Reason == "" {
				dec.Reason = "allowed by policy rule"
			}
			allow = &dec
		case EffectRequireApproval:
			dec.Effect = EffectRequireApproval
			if dec.RiskLevel == "" {
				dec.RiskLevel = "high"
			}
			if dec.Reason == "" {
				dec.Reason = "approval required by policy rule"
			}
			require = &dec
		default:
			// ignore unknown
		}
	}

	// Capability allowlist is low precedence: rules override it.
	for _, a := range e.doc.Allow {
		for _, c := range capabilities {
			if c == a.Capability {
				return Decision{Effect: EffectAllow, RuleID: "allow.capability", Reason: "allowed by capability allowlist", RiskLevel: "low"}
			}
		}
	}

	if allow != nil {
		return *allow
	}
	if require != nil {
		return *require
	}

	switch normalizeMode(e.doc.Defaults.Mode) {
	case "allow_by_default":
		return Decision{Effect: EffectAllow, RuleID: "default.allow", Reason: "allowed by default policy", RiskLevel: "low"}
	case "deny_by_default":
		return Decision{Effect: EffectDeny, RuleID: "default.deny", Reason: "denied by default policy", RiskLevel: "high"}
	default:
		return Decision{Effect: EffectRequireApproval, RuleID: "default.require_approval", Reason: "approval required by default", RiskLevel: "high"}
	}
}

func matchCapabilities(ruleCaps []string, caps []string) bool {
	if len(ruleCaps) == 0 {
		return true
	}
	for _, rc := range ruleCaps {
		for _, c := range caps {
			if c == rc {
				return true
			}
		}
	}
	return false
}

func matchResources(cr compiledRule, resources []Resource) bool {
	w := cr.rule.When.Resource

	for _, r := range resources {
		if w.OutsideWorkspace != nil && r.Type == ResourcePath {
			if r.OutsideWorkspace != *w.OutsideWorkspace {
				return false
			}
		}
	}

	if len(w.SchemeIn) > 0 {
		ok := false
		for _, r := range resources {
			if r.Type == ResourceURL {
				for _, s := range w.SchemeIn {
					if r.Scheme == s {
						ok = true
						break
					}
				}
			}
		}
		if !ok {
			return false
		}
	}

	if len(w.DomainIn) > 0 {
		ok := false
		for _, r := range resources {
			if r.Type == ResourceDomain || r.Type == ResourceURL {
				for _, d := range w.DomainIn {
					if r.Domain == d {
						ok = true
						break
					}
				}
			}
		}
		if !ok {
			return false
		}
	}

	if len(w.PathMatches) > 0 {
		ok := false
		for _, r := range resources {
			if r.Type != ResourcePath {
				continue
			}
			target := r.WorkspaceRel
			if target == "" {
				target = r.Path
			}
			target = filepath.ToSlash(target)
			for _, re := range cr.pathRegex {
				if re.MatchString(target) {
					ok = true
					break
				}
			}
		}
		if !ok {
			return false
		}
	}

	if len(cr.commandRegex) > 0 {
		ok := false
		for _, r := range resources {
			if r.Type != ResourceCommand {
				continue
			}
			for _, re := range cr.commandRegex {
				if re.MatchString(r.Command) {
					ok = true
					break
				}
			}
		}
		if !ok {
			return false
		}
	}

	return true
}

func normalizeMode(mode string) string {
	m := strings.ToLower(strings.TrimSpace(mode))
	switch m {
	case "", "require_approval_by_default":
		return "require_approval_by_default"
	case "deny_by_default":
		return "deny_by_default"
	case "allow_by_default":
		return "allow_by_default"
	default:
		// Unknown modes fall back to the safest practical behavior:
		// require approval.
		return "require_approval_by_default"
	}
}

// compilePathGlob converts a glob to a regexp.
// Supported:
// - `*` matches any chars except `/`
// - `?` matches any single char except `/`
// - `**` matches any chars including `/`
//
// Patterns and targets are matched against slash-separated paths.
func compilePathGlob(pattern string) (*regexp.Regexp, error) {
	p := filepath.ToSlash(strings.TrimSpace(pattern))
	if p == "" {
		return regexp.Compile("^$") // never match
	}

	var b strings.Builder
	b.WriteString("^")
	for i := 0; i < len(p); i++ {
		ch := p[i]
		if ch == '*' {
			if i+1 < len(p) && p[i+1] == '*' {
				b.WriteString(".*")
				i++
				continue
			}
			b.WriteString(`[^/]*`)
			continue
		}
		if ch == '?' {
			b.WriteString(`[^/]`)
			continue
		}
		switch ch {
		case '.', '+', '(', ')', '|', '^', '$', '{', '}', '[', ']', '\\':
			b.WriteByte('\\')
		}
		b.WriteByte(ch)
	}
	b.WriteString("$")

	return regexp.Compile(b.String())
}
