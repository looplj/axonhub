package runtime

import (
	"github.com/samber/lo"

	axonconf "github.com/looplj/axonhub/axon/conf"
	cliconf "github.com/looplj/axonhub/cmd/axoncli/conf"
)

func DiffConfig(oldV, newV cliconf.Config) axonconf.ChangeSet {
	var changed []string
	if oldV.BaseURL != newV.BaseURL {
		changed = append(changed, "base_url")
	}
	if oldV.APIKey != newV.APIKey {
		changed = append(changed, "api_key")
	}
	if oldV.Model != newV.Model {
		changed = append(changed, "model")
	}
	if oldV.TraceHeader != newV.TraceHeader {
		changed = append(changed, "trace_header")
	}
	if oldV.ThreadHeader != newV.ThreadHeader {
		changed = append(changed, "thread_header")
	}

	sensitive := lo.Filter(changed, func(k string, _ int) bool {
		return k == "api_key"
	})

	return axonconf.ChangeSet{
		ChangedKeys:   changed,
		SensitiveKeys: sensitive,
	}
}
