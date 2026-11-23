// 职责: 校验插件任务声明、计算触发时间并比较升级前后的定时配置。
// 关联文件:
//
//	internal/objects/plugin.go
//	internal/server/plugin/manifest.go
//	internal/server/plugin/tasks.go

package plugin

import (
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/aptible/supercronic/cronexpr"

	"github.com/looplj/axonhub/internal/objects"
)

var taskKeyPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,31}$`)

func validateTasks(tasks map[string]objects.PluginTaskSpec) error {
	if len(tasks) > 16 {
		return fmt.Errorf("tasks may contain at most 16 entries")
	}
	for _, key := range slices.Sorted(maps.Keys(tasks)) {
		if !taskKeyPattern.MatchString(key) {
			return fmt.Errorf("task key %q must match %s", key, taskKeyPattern)
		}
		spec := tasks[key].Schedule
		if _, _, err := parsePluginSchedule(spec.Cron, spec.Timezone); err != nil {
			return fmt.Errorf("tasks.%s.schedule: %w", key, err)
		}
	}
	return nil
}

func parsePluginSchedule(cron, timezone string) (*cronexpr.Expression, *time.Location, error) {
	if len(cron) > 128 || len(strings.Fields(cron)) != 5 {
		return nil, nil, fmt.Errorf("cron must contain five fields and fit in 128 bytes")
	}
	expr, err := cronexpr.ParseStrict(cron)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid cron: %w", err)
	}
	if timezone == "" {
		timezone = "UTC"
	}
	if timezone == "Local" || len(timezone) > 64 {
		return nil, nil, fmt.Errorf("timezone must be an IANA timezone or UTC")
	}
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid timezone: %w", err)
	}
	return expr, loc, nil
}

func changedTasks(old, next map[string]objects.PluginTaskSpec) []string {
	var changed []string
	for _, key := range slices.Sorted(maps.Keys(next)) {
		spec := next[key].Schedule
		prior, exists := old[key]
		if !exists || prior.Schedule.Cron != spec.Cron || scheduleTimezone(prior.Schedule.Timezone) != scheduleTimezone(spec.Timezone) {
			changed = append(changed, "tasks: "+key+" ("+spec.Cron+" "+scheduleTimezone(spec.Timezone)+")")
		}
	}
	return changed
}

func scheduleTimezone(timezone string) string {
	if timezone == "" {
		return "UTC"
	}
	return timezone
}
