// 职责: 将端点协议隔离错误返回为明确的HTTP422，不改变其他GraphQL错误语义。
// 关联文件:
//
//	internal/server/gql/graphql.go
//	internal/objects/channel_realtime.go
//
// 注意事项: 不缓冲响应体；只在目标错误且尚未输出JSON时设置状态码。

// Package gql serves the GraphQL API and its HTTP integration.
package gql

import (
	"context"
	"net/http"

	"github.com/99designs/gqlgen/graphql"

	"github.com/looplj/axonhub/internal/objects"
)

type endpointResponseWriterKey struct{}

func withEndpointHTTPStatus(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), endpointResponseWriterKey{}, w)))
	})
}

func endpointProtocolErrorStatus(ctx context.Context, next graphql.ResponseHandler) *graphql.Response {
	response := next(ctx)
	if response != nil {
		for _, err := range response.Errors {
			if err.Extensions["code"] == objects.ErrCodeEndpointRealtimeNotAllowed {
				if w, ok := ctx.Value(endpointResponseWriterKey{}).(http.ResponseWriter); ok {
					w.WriteHeader(http.StatusUnprocessableEntity)
				}
				break
			}
		}
	}
	return response
}
