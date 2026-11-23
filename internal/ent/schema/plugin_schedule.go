// 职责: 定义插件任务的设置、执行租约和运行记录。
// 关联文件:
//
//	internal/ent/schema/plugin.go
//	internal/server/plugin/tasks.go
//
// 遵循的 Rules:
//
//	.agents/rules/ent-graphql.md
//	.agents/rules/database-indexes.md
package schema

import (
	"entgo.io/contrib/entgql"
	"entgo.io/ent"
	"entgo.io/ent/privacy"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type PluginSchedule struct{ ent.Schema }

func (PluginSchedule) Fields() []ent.Field {
	return []ent.Field{
		field.Int("installation_id").Immutable().Positive(),
		field.String("key").Immutable().MaxLen(32).NotEmpty(),
		field.String("override_cron").MaxLen(128).Default(""),
		field.String("override_timezone").MaxLen(64).Default(""),
		field.Bool("paused").Default(false),
		field.Int64("last_slot").Default(0),
		field.Int("active_run_id").Default(0),
		field.Int64("lease_until").Default(0),
	}
}

func (PluginSchedule) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("installation_id", "key").StorageKey("plugin_schedules_by_installation_key").Unique(),
	}
}

func (PluginSchedule) Annotations() []schema.Annotation {
	return []schema.Annotation{entgql.Skip(entgql.SkipAll)}
}

func (PluginSchedule) Policy() ent.Policy {
	return privacy.Policy{
		Query:    privacy.QueryPolicy{privacy.AlwaysDenyRule()},
		Mutation: privacy.MutationPolicy{privacy.AlwaysDenyRule()},
	}
}

type PluginScheduleRun struct{ ent.Schema }

func (PluginScheduleRun) Fields() []ent.Field {
	return []ent.Field{
		field.Int("schedule_id").Immutable().Positive(),
		field.Time("scheduled_at").Immutable().Optional().Nillable(),
		field.Time("started_at").Immutable(),
		field.Time("finished_at").Optional().Nillable(),
		field.String("status").MaxLen(16).NotEmpty(),
	}
}

func (PluginScheduleRun) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("schedule_id", "scheduled_at").StorageKey("plugin_schedule_runs_by_schedule_slot").Unique(),
		index.Fields("schedule_id", "id").StorageKey("plugin_schedule_runs_by_schedule_id"),
	}
}

func (PluginScheduleRun) Annotations() []schema.Annotation {
	return []schema.Annotation{entgql.Skip(entgql.SkipAll)}
}

func (PluginScheduleRun) Policy() ent.Policy {
	return privacy.Policy{
		Query:    privacy.QueryPolicy{privacy.AlwaysDenyRule()},
		Mutation: privacy.MutationPolicy{privacy.AlwaysDenyRule()},
	}
}
