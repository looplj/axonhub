package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	openapigraphql "examples/graphql"

	"github.com/Khan/genqlient/graphql"
)

func main() {
	// AxonHub 的 OpenAPI GraphQL 地址
	// 注意: 这是一个专用的管理端点，需要 Service Account 类型的 API Key
	endpoint := "http://localhost:8090/openapi/v1/graphql"
	if envEndpoint := os.Getenv("AXONHUB_ENDPOINT"); envEndpoint != "" {
		endpoint = envEndpoint
	}

	// 你的 API Key
	// 注意: 必须是 Service Account 类型。createLLMAPIKey 需要 write_api_keys；
	// 末尾可选的 apiKeyQuotaUsages 查询额外需要 read_api_keys。
	apiKey := os.Getenv("AXONHUB_API_KEY")
	if apiKey == "" {
		fmt.Println("请设置 AXONHUB_API_KEY 环境变量 (需要 Service Account Key)")
		os.Exit(1)
	}

	// 创建带有认证头的 HTTP 客户端
	httpClient := &http.Client{
		Transport: &headerTransport{
			apiKey: apiKey,
			base:   http.DefaultTransport,
		},
	}

	// 初始化 genqlient 客户端
	client := graphql.NewClient(endpoint, httpClient)

	// 调用生成的 CreateAPIKey 方法
	// 该操作会为当前 Service Account 所属的项目创建一个新的 User 类型 Key。
	// 注意: Key 名称在项目内必须唯一（名称可作为查询标识符），重名会被拒绝，
	// 因此示例用时间戳保证每次运行的名称不同。
	name := fmt.Sprintf("example-key-from-sdk-%d", time.Now().Unix())
	fmt.Printf("正在创建 API Key: %s...\n", name)

	resp, err := openapigraphql.CreateAPIKey(context.Background(), client, name)
	if err != nil {
		fmt.Fprintf(os.Stderr, "调用失败: %v\n", err)
		fmt.Println("\n可能的原因:")
		fmt.Println("1. 服务器未启动 (默认 8090 端口)")
		fmt.Println("2. API Key 不是 Service Account 类型")
		fmt.Println("3. API Key 缺少 write_api_keys 权限")
		os.Exit(1)
	}

	if resp.CreateLLMAPIKey != nil {
		fmt.Printf("成功创建 API Key!\n")
		fmt.Printf("名称: %s\n", resp.CreateLLMAPIKey.Name)
		fmt.Printf("密钥: %s\n", resp.CreateLLMAPIKey.Key)
		fmt.Printf("权限: %v\n", resp.CreateLLMAPIKey.Scopes)
		fmt.Println("\n现在你可以使用这个新生成的 Key 来进行常规的 LLM 调用了。")
	} else {
		fmt.Println("创建成功但返回数据为空")
	}

	// 演示: 按名称反查刚创建的 Key 的详情 (id / key / scopes / profiles)。
	// 这是 name 与 id 并存能力的核心用法——手里只有名称时，先解析出 id/key，
	// 再去调用其他接口。需要 read_api_keys 权限。
	lookupAPIKeyByName(context.Background(), client, name)

	// 可选: 查询某个 Key 的模版额度用量。
	// 需要 Service Account Key 拥有 read_api_keys 权限，且目标 Key 在同一项目内。
	queryQuotaUsage(context.Background(), client)
}

// lookupAPIKeyByName 演示 apiKey 查询: 按 id / key / name 三选一定位一把 Key。
// name 在调用方所属项目内唯一，跨项目的 Key 一律不可见（表现为查不到）。
func lookupAPIKeyByName(ctx context.Context, client graphql.Client, name string) {
	fmt.Printf("\n正在按名称查询 API Key: %s...\n", name)

	resp, err := openapigraphql.GetAPIKey(ctx, client, nil, nil, &name)
	if err != nil {
		fmt.Fprintf(os.Stderr, "查询失败: %v\n", err)
		fmt.Println("可能的原因: 缺少 read_api_keys 权限 / 目标 Key 不在当前项目")
		return
	}

	if resp.ApiKey == nil {
		fmt.Println("查询成功但返回数据为空")
		return
	}

	fmt.Printf("解析结果: id=%s name=%s scopes=%v\n",
		resp.ApiKey.Id, resp.ApiKey.Name, resp.ApiKey.Scopes)
}

// queryQuotaUsage 演示 apiKeyQuotaUsages 查询。通过环境变量三选一指定目标 Key:
//   - AXONHUB_QUERY_KEY_ID:   目标 Key 的 GUID (形如 gid://axonhub/APIKey/123)
//   - AXONHUB_QUERY_KEY:      目标 Key 的明文字符串
//   - AXONHUB_QUERY_KEY_NAME: 目标 Key 的名称 (项目内唯一)
//
// 都未设置时跳过本演示。注意: 该查询应使用 POST，避免明文 Key 落入 URL。
func queryQuotaUsage(ctx context.Context, client graphql.Client) {
	keyID := os.Getenv("AXONHUB_QUERY_KEY_ID")
	keyVal := os.Getenv("AXONHUB_QUERY_KEY")
	keyName := os.Getenv("AXONHUB_QUERY_KEY_NAME")

	if keyID == "" && keyVal == "" && keyName == "" {
		fmt.Println("\n(设置 AXONHUB_QUERY_KEY_ID / AXONHUB_QUERY_KEY / AXONHUB_QUERY_KEY_NAME 之一可查询某个 Key 的额度用量)")
		return
	}

	var (
		apiKeyID *string
		key      *string
		name     *string
	)

	switch {
	case keyID != "":
		apiKeyID = &keyID
	case keyVal != "":
		key = &keyVal
	case keyName != "":
		name = &keyName
	}

	fmt.Println("\n正在查询额度用量...")

	resp, err := openapigraphql.APIKeyQuotaUsages(ctx, client, apiKeyID, key, name)
	if err != nil {
		fmt.Fprintf(os.Stderr, "查询失败: %v\n", err)
		fmt.Println("可能的原因: 缺少 read_api_keys 权限 / 目标 Key 不在当前项目 / 参数无效")
		return
	}

	if len(resp.ApiKeyQuotaUsages) == 0 {
		fmt.Println("该 Key 没有启用额度的 profile。")
		return
	}

	for _, usage := range resp.ApiKeyQuotaUsages {
		// usage is non-null per the schema, but it is a pointer in the generated
		// client (use_struct_references), so guard against a malformed response.
		if usage.Usage == nil {
			fmt.Printf("- profile=%s (响应缺少 usage)\n", usage.ProfileName)
			continue
		}

		fmt.Printf("- profile=%s requests=%d totalTokens=%d totalCost=%s\n",
			usage.ProfileName,
			usage.Usage.RequestCount,
			usage.Usage.TotalTokens,
			usage.Usage.TotalCost,
		)
	}
}

// headerTransport 用于在每个请求中自动注入 Authorization 头
type headerTransport struct {
	apiKey string
	base   http.RoundTripper
}

func (t *headerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set("Authorization", "Bearer "+t.apiKey)
	return t.base.RoundTrip(req)
}
