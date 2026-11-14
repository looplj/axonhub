// 职责: 验证 GraphQL 字段裁剪后仍能读取去重及外部存储的请求正文。
// 关联文件:
//
//	internal/server/gql/ent.resolvers.go
//	internal/server/biz/request.go
//	internal/ent/schema/request.go
//	internal/ent/schema/request_execution.go
//
// 注意事项: 正文读取必须加载存储标记及外部执行文件所需的父请求 ID。

package gql

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/pkg/xcache"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
)

func TestGraphQLChunkedRequestBodyFieldSelection(t *testing.T) {
	for _, mode := range []string{"database", "external"} {
		t.Run(mode, func(t *testing.T) {
			client := enttest.NewEntClient(t, "sqlite3", "file:"+t.Name()+"?mode=memory&_fk=0")
			t.Cleanup(func() { _ = client.Close() })
			ctx := authz.WithTestBypass(ent.NewContext(context.Background(), client))
			project := client.Project.Create().SetName("chunked-body").SaveX(ctx)
			ctx = contexts.WithProjectID(ctx, project.ID)
			client.DataStorage.Create().SetName("primary").SetDescription("test").SetPrimary(true).
				SetType("database").SetSettings(&objects.DataStorageSettings{}).SaveX(ctx)
			system := biz.NewSystemService(biz.SystemServiceParams{Ent: client})
			if mode == "external" {
				dir := t.TempDir()
				ds := client.DataStorage.Create().SetName("external").SetDescription("test").SetPrimary(false).
					SetType("fs").SetSettings(&objects.DataStorageSettings{Directory: &dir}).SaveX(ctx)
				require.NoError(t, system.SetDefaultDataStorageID(ctx, ds.ID))
			}
			channels := biz.NewChannelServiceForTest(client)
			usage := biz.NewUsageLogService(client, system, channels)
			storage := biz.NewDataStorageService(biz.DataStorageServiceParams{SystemService: system, Client: client})
			service := biz.NewRequestService(client, xcache.Config{}, system, usage, storage, biz.NewLiveStreamRegistry())
			body := []byte(`{"model":"test","input":"` + strings.Repeat("exact conversation history ", 1500) + `","big":9007199254740993}`)
			req, err := service.CreateRequest(ctx, &llm.Request{Model: "test"}, &httpclient.Request{JSONBody: body}, llm.APIFormatOpenAIResponse)
			require.NoError(t, err)
			channel := client.Channel.Create().SetName("test").SetType("openai").SetBaseURL("https://example.test").
				SetCredentials(objects.ChannelCredentials{APIKey: "test"}).SetSupportedModels([]string{"test"}).SetDefaultTestModel("test").SaveX(ctx)
			execution, err := service.CreateRequestExecution(ctx, &biz.Channel{Channel: channel}, "test", req,
				httpclient.Request{Body: body}, llm.APIFormatOpenAIResponse, false)
			require.NoError(t, err)
			require.Equal(t, mode == "database", req.RequestBodyChunked)
			require.Equal(t, mode == "database", execution.RequestBodyChunked)
			stored, err := service.LoadRequestExecutionRequestBody(ctx, execution)
			require.NoError(t, err)
			require.Equal(t, body, []byte(stored))

			schema := NewExecutableSchema(Config{Resolvers: &Resolver{client: client, requestService: service}})
			server := handler.NewDefaultServer(schema)
			for _, selection := range []string{"requestBody", "requestID requestBody"} {
				t.Run(selection, func(t *testing.T) {
					payload, err := json.Marshal(map[string]string{"query": `{requests(first:1){edges{node{requestBody executions{edges{node{` + selection + `}}}}}}}`})
					require.NoError(t, err)
					httpRequest := httptest.NewRequest(http.MethodPost, "/graphql", bytes.NewReader(payload)).WithContext(ctx)
					httpRequest.Header.Set("Content-Type", "application/json")
					response := httptest.NewRecorder()
					server.ServeHTTP(response, httpRequest)
					var result struct {
						Errors json.RawMessage `json:"errors"`
						Data   struct {
							Requests struct {
								Edges []struct {
									Node struct {
										RequestBody json.RawMessage `json:"requestBody"`
										Executions  struct {
											Edges []struct {
												Node struct {
													RequestBody json.RawMessage `json:"requestBody"`
												} `json:"node"`
											} `json:"edges"`
										} `json:"executions"`
									} `json:"node"`
								} `json:"edges"`
							} `json:"requests"`
						} `json:"data"`
					}
					require.NoError(t, json.Unmarshal(response.Body.Bytes(), &result), response.Body.String())
					require.Empty(t, result.Errors, response.Body.String())
					require.Len(t, result.Data.Requests.Edges, 1)
					node := result.Data.Requests.Edges[0].Node
					require.JSONEq(t, string(body), string(node.RequestBody))
					require.Len(t, node.Executions.Edges, 1)
					require.JSONEq(t, string(body), string(node.Executions.Edges[0].Node.RequestBody))
				})
			}
		})
	}
}
