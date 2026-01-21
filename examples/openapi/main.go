package main

import (
	"context"
	"fmt"
	"net/http"
	"os"

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
	// 注意: 必须是 Service Account 类型，并且拥有 write_api_keys 权限
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
	// 该操作会为当前 Service Account 所属的项目创建一个新的 User 类型 Key
	name := "example-key-from-sdk"
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
