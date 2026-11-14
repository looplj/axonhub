// 职责: 保存按项目隔离、按内容去重的请求正文块。
// 关联文件:
//
//	internal/ent/schema/request_body_chunk_ref.go
//	internal/server/biz/request_body_chunks.go
//
// 注意事项: 不对 GraphQL 暴露内容；仍被引用的块不得删除。
package schema

import (
	"time"

	"entgo.io/contrib/entgql"
	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/privacy"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type RequestBodyChunk struct{ ent.Schema }

func (RequestBodyChunk) Fields() []ent.Field {
	return []ent.Field{
		field.Int("project_id").Immutable(),
		field.String("hash").MaxLen(64).NotEmpty().Immutable(),
		field.Bytes("data").Immutable().SchemaType(map[string]string{dialect.MySQL: "mediumblob"}),
		field.Time("created_at").Default(time.Now).Immutable().Annotations(entsql.DefaultExpr("CURRENT_TIMESTAMP")),
	}
}

func (RequestBodyChunk) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("project_id", "hash").Unique().StorageKey("request_body_chunks_by_project_hash"),
	}
}

func (RequestBodyChunk) Edges() []ent.Edge {
	return []ent.Edge{edge.To("references", RequestBodyChunkRef.Type).Annotations(entsql.OnDelete(entsql.Restrict))}
}

func (RequestBodyChunk) Annotations() []schema.Annotation {
	return []schema.Annotation{entgql.Skip(entgql.SkipAll)}
}

func (RequestBodyChunk) Policy() ent.Policy {
	return privacy.Policy{
		Query:    privacy.QueryPolicy{privacy.AlwaysDenyRule()},
		Mutation: privacy.MutationPolicy{privacy.AlwaysDenyRule()},
	}
}
