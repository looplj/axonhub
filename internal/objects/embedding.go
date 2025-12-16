package objects

// EmbeddingRequest 表示 OpenAI 兼容的 embedding 请求载荷。
type EmbeddingRequest struct {
	// Input 要嵌入的文本。根据 OpenAI 规范，可以是字符串或字符串/token 数组。
	Input any `json:"input"`
	// Model 要使用的模型 ID。
	Model string `json:"model"`
	// EncodingFormat 返回的 embedding 格式："float"（默认）或 "base64"。
	EncodingFormat string `json:"encoding_format,omitempty"`
	// Dimensions 结果 embedding 的维度数量。
	Dimensions *int `json:"dimensions,omitempty"`
	// User 代表最终用户的唯一标识符。
	User string `json:"user,omitempty"`
}

// EmbeddingResponse 表示 OpenAI 兼容的 embedding 响应载荷。
type EmbeddingResponse struct {
	ID     string      `json:"id,omitempty"` // 某些提供商会返回 ID
	Object string      `json:"object"`
	Data   []Embedding `json:"data"`
	Model  string      `json:"model"`
	Usage  Usage       `json:"usage"`
}

// Embedding 是响应中的单个 embedding 向量条目。
type Embedding struct {
	Object    string `json:"object"`
	Embedding any    `json:"embedding"`
	Index     int    `json:"index"`
}

// Usage 表示 embedding 的 prompt/total token 使用量。
type Usage struct {
	PromptTokens int `json:"prompt_tokens"`
	TotalTokens  int `json:"total_tokens"`
}
