// 职责: 通过宿主的 GraphQL schema 执行服务端插件任务操作。
// 关联文件:
//   internal/server/gql/graphql.go
//   internal/server/plugin/hostapi_graphql.go
//   internal/server/gql/plugin_call.go

package gql

import (
	"context"
	"encoding/json"

	"github.com/99designs/gqlgen/graphql"
	"github.com/99designs/gqlgen/graphql/executor"
	"github.com/99designs/gqlgen/graphql/handler/lru"
	"github.com/vektah/gqlparser/v2/ast"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/server/plugin"
)

func newTaskGraphQLExecutor(schema graphql.ExecutableSchema, client *ent.Client) plugin.GraphQLExecutor {
	exec := executor.New(schema)
	configurePluginGraphQL(exec, client)
	exec.SetQueryCache(lru.New[*ast.QueryDocument](1024))
	return func(ctx context.Context, query string, variables map[string]any, name string) (json.RawMessage, error) {
		ctx, cancel := context.WithTimeout(ctx, PluginGraphqlTimeout)
		defer cancel()
		ctx = graphql.StartOperationTrace(ctx)
		op, errs := exec.CreateOperationContext(ctx, &graphql.RawParams{Query: query, Variables: variables, OperationName: name})
		if len(errs) != 0 {
			return json.Marshal(exec.DispatchError(graphql.WithOperationContext(ctx, op), errs))
		}
		responses, responseCtx := exec.DispatchOperation(ctx, op)
		return json.Marshal(responses(responseCtx))
	}
}
