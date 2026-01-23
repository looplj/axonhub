package schema

import (
	"entgo.io/contrib/entgql"
	"entgo.io/ent"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	"github.com/looplj/axonhub/internal/ent/schema/schematype"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/scopes"
)

type Channel struct {
	ent.Schema
}

func (Channel) Mixin() []ent.Mixin {
	return []ent.Mixin{
		TimeMixin{},
		schematype.SoftDeleteMixin{},
	}
}

func (Channel) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("name", "deleted_at").
			StorageKey("channels_by_name").
			Unique(),
	}
}

func (Channel) Fields() []ent.Field {
	return []ent.Field{
		field.Enum("type").
			Values(
				"openai",
				"openai_responses",
				"codex",
				"vercel",
				"anthropic",
				"anthropic_aws",
				"anthropic_gcp",
				"gemini_openai",
				"gemini",
				"gemini_vertex",
				"deepseek",
				"deepseek_anthropic",
				"deepinfra",
				"doubao",
				"doubao_anthropic",
				"moonshot",
				"moonshot_anthropic",
				"zhipu",
				"zai",
				"zhipu_anthropic",
				"zai_anthropic",
				"anthropic_fake",
				"openai_fake",
				"openrouter",
				"xai",
				"ppio",
				"siliconflow",
				"volcengine",
				"longcat",
				"longcat_anthropic",
				"minimax",
				"minimax_anthropic",
				"aihubmix",
				"burncloud",
				"modelscope",
				"bailian",
				"jina",
				"github",
				"claudecode",
				"cerebras",
			).
			Immutable().
			Annotations(
				entgql.OrderField("TYPE"),
			),
		field.String("base_url").Optional(),
		field.String("name").
			Annotations(
				entgql.OrderField("NAME"),
			),
		field.Enum("status").Values("enabled", "disabled", "archived").Default("disabled").
			Annotations(
				entgql.Skip(entgql.SkipMutationCreateInput),
				entgql.OrderField("STATUS"),
			),
		field.JSON("credentials", &objects.ChannelCredentials{}).
			Sensitive().
			Default(&objects.ChannelCredentials{}),
		field.Strings("supported_models"),
		field.Bool("auto_sync_supported_models").Default(false),
		field.Strings("tags").Optional().Default([]string{}),
		field.String("default_test_model"),
		field.JSON("policies", objects.ChannelPolicies{}).
			Default(objects.ChannelPolicies{
				Stream: objects.CapabilityPolicyUnlimited,
			}).
			Annotations(
				entgql.Directives(forceResolver()),
			).
			Optional(),
		field.JSON("settings", &objects.ChannelSettings{}).
			Default(&objects.ChannelSettings{
				ModelMappings: []objects.ModelMapping{},
			}).Optional().Annotations(),
		field.Int("ordering_weight").Default(0).Comment("Ordering weight for display sorting").
			Annotations(
				entgql.OrderField("ORDERING_WEIGHT"),
			),
		field.String("error_message").
			Optional().Nillable().
			Annotations(
				entgql.Skip(entgql.SkipMutationCreateInput),
			),
		field.String("remark").
			Optional().Nillable().
			Comment("User-defined remark or note for the channel"),
	}
}

func (Channel) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("requests", Request.Type).
			Annotations(
				entgql.Skip(entgql.SkipMutationCreateInput, entgql.SkipMutationUpdateInput),
				entgql.RelayConnection(),
			),
		edge.To("executions", RequestExecution.Type).
			Annotations(
				entgql.Skip(entgql.SkipMutationCreateInput, entgql.SkipMutationUpdateInput),
				entgql.RelayConnection(),
			),
		edge.To("usage_logs", UsageLog.Type).
			Annotations(
				entgql.Skip(entgql.SkipMutationCreateInput, entgql.SkipMutationUpdateInput),
				entgql.RelayConnection(),
			),
		edge.To("channel_performance", ChannelPerformance.Type).
			Unique().
			Annotations(
				entgql.Skip(entgql.SkipMutationCreateInput, entgql.SkipMutationUpdateInput),
			),
		edge.To("channel_probes", ChannelProbe.Type).
			Annotations(
				entgql.Skip(entgql.SkipMutationCreateInput, entgql.SkipMutationUpdateInput),
			),
		edge.To("channel_model_prices", ChannelModelPrice.Type).
			Annotations(
				entgql.Skip(entgql.SkipMutationCreateInput, entgql.SkipMutationUpdateInput),
			),
	}
}

func (Channel) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entgql.QueryField(),
		entgql.RelayConnection(),
		entgql.Mutations(entgql.MutationCreate(), entgql.MutationUpdate()),
	}
}

// Policy 定义 Channel 的权限策略.
func (Channel) Policy() ent.Policy {
	return scopes.Policy{
		Query: scopes.QueryPolicy{
			scopes.APIKeyScopeQueryRule(scopes.ScopeReadChannels),
			scopes.OwnerRule(), // owner 用户可以访问所有渠道
			scopes.UserReadScopeRule(scopes.ScopeReadChannels), // 需要 channels 读取权限
		},
		Mutation: scopes.MutationPolicy{
			scopes.OwnerRule(), // owner 用户可以修改所有渠道
			scopes.UserWriteScopeRule(scopes.ScopeWriteChannels), // 需要 channels 写入权限
		},
	}
}
