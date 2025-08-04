package log

import (
	"github.com/samber/lo"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func withNameFilter(includeNames, excludeNames []string) zap.Option {
	includesMap := lo.SliceToMap(includeNames, func(item string) (string, bool) {
		return item, true
	})
	excludesMap := lo.SliceToMap(excludeNames, func(item string) (string, bool) {
		return item, true
	})
	return zap.WrapCore(func(base zapcore.Core) zapcore.Core {
		return &zapNameFilterCore{
			Core:          base,
			includeNames:  includesMap,
			excludeNamess: excludesMap,
		}
	})
}

type zapNameFilterCore struct {
	zapcore.Core
	includeNames  map[string]bool
	excludeNamess map[string]bool
}

func (c *zapNameFilterCore) Check(
	ent zapcore.Entry,
	ce *zapcore.CheckedEntry,
) *zapcore.CheckedEntry {
	name := ent.LoggerName
	excludes := c.excludeNamess
	includes := c.includeNames

	if len(includes) > 0 {
		if !includes[name] {
			return ce
		}
	} else if len(excludes) > 0 {
		if excludes[name] {
			return ce
		}
	}
	return c.Core.Check(ent, ce)
}
