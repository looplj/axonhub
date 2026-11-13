// 职责: 按原始顺序关联请求或渠道执行使用的正文块。
// 关联文件:
//
//	internal/ent/schema/request_body_chunk.go
//	internal/ent/schema/request.go
//	internal/ent/schema/request_execution.go
//	internal/server/biz/request_body_chunks.go
//	internal/server/gc/gc.go
//
// 注意事项: 生产迁移不启用外键，删除所有者后的引用由 GC 清理。
package schema

import (
	"entgo.io/contrib/entgql"
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/privacy"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type RequestBodyChunkRef struct{ ent.Schema }

func (RequestBodyChunkRef) Fields() []ent.Field {
	return []ent.Field{
		field.Int("request_id").Optional().Nillable().Immutable(),
		field.Int("execution_id").Optional().Nillable().Immutable(),
		field.Int("chunk_id").Immutable(),
		field.Int("position").NonNegative().Immutable(),
	}
}

func (RequestBodyChunkRef) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("request_id", "position").Unique().StorageKey("request_body_chunk_refs_by_request_position"),
		index.Fields("execution_id", "position").Unique().StorageKey("request_body_chunk_refs_by_execution_position"),
		index.Fields("chunk_id").StorageKey("request_body_chunk_refs_by_chunk"),
	}
}

func (RequestBodyChunkRef) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("request", Request.Type).Ref("request_body_chunks").Field("request_id").Unique().Immutable().Annotations(entsql.OnDelete(entsql.Cascade)),
		edge.From("execution", RequestExecution.Type).Ref("request_body_chunks").Field("execution_id").Unique().Immutable().Annotations(entsql.OnDelete(entsql.Cascade)),
		edge.From("chunk", RequestBodyChunk.Type).Ref("references").Field("chunk_id").Unique().Required().Immutable().Annotations(entsql.OnDelete(entsql.Restrict)),
	}
}

func (RequestBodyChunkRef) Annotations() []schema.Annotation {
	return []schema.Annotation{entgql.Skip(entgql.SkipAll)}
}

func (RequestBodyChunkRef) Policy() ent.Policy {
	return privacy.Policy{
		Query:    privacy.QueryPolicy{privacy.AlwaysDenyRule()},
		Mutation: privacy.MutationPolicy{privacy.AlwaysDenyRule()},
	}
}
