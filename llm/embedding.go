package llm

import (
	"encoding/json"
	"fmt"
)

const (
	EmbeddingInputTypeString        = "string"
	EmbeddingInputTypeStringArray   = "string_array"
	EmbeddingInputTypeIntArray      = "int_array"
	EmbeddingInputTypeIntArrayArray = "int_array_array"
)

type EmbeddingInput struct {
	String        string    `json:"string,omitempty"`
	StringArray   []string  `json:"string_array,omitempty"`
	IntArray      []int64   `json:"int_array,omitempty"`
	IntArrayArray [][]int64 `json:"int_array_array,omitempty"`
}

func (e EmbeddingInput) MarshalJSON() ([]byte, error) {
	if e.String != "" {
		return json.Marshal(e.String)
	}

	if len(e.StringArray) > 0 {
		return json.Marshal(e.StringArray)
	}

	if len(e.IntArray) > 0 {
		return json.Marshal(e.IntArray)
	}

	if len(e.IntArrayArray) > 0 {
		return json.Marshal(e.IntArrayArray)
	}

	return json.Marshal(nil)
}

func (e *EmbeddingInput) UnmarshalJSON(data []byte) error {
	var str string

	err := json.Unmarshal(data, &str)
	if err == nil {
		e.String = str
		return nil
	}

	var strArray []string

	err = json.Unmarshal(data, &strArray)
	if err == nil {
		e.StringArray = strArray
		return nil
	}

	var intArray []int64

	err = json.Unmarshal(data, &intArray)
	if err == nil {
		e.IntArray = intArray
		return nil
	}

	var intArrayArray [][]int64

	err = json.Unmarshal(data, &intArrayArray)
	if err == nil {
		e.IntArrayArray = intArrayArray
		return nil
	}

	return fmt.Errorf("invalid embedding input type")
}

func (e EmbeddingInput) GetType() string {
	if e.String != "" {
		return EmbeddingInputTypeString
	}

	if len(e.StringArray) > 0 {
		return EmbeddingInputTypeStringArray
	}

	if len(e.IntArray) > 0 {
		return EmbeddingInputTypeIntArray
	}

	if len(e.IntArrayArray) > 0 {
		return EmbeddingInputTypeIntArrayArray
	}

	return ""
}

// EmbeddingRequest represents the unified embedding request model.
// Based on OpenAI embedding request format for compatibility.
type EmbeddingRequest struct {
	// Input is the text to embed. Can be string, []string, []int (tokens), or [][]int (multiple token arrays).
	Input EmbeddingInput `json:"input"`

	// Task is the task to embed.
	// For jina embedding, it can be:
	// text-matching
	// retrieval.query
	// retrieval.passag
	// separation
	// classification
	// none
	Task string `json:"task,omitempty"`

	// The format to return the embeddings in. Can be either `float` or
	// [`base64`](https://pypi.org/project/pybase64/).
	//
	// Any of "float", "base64".
	EncodingFormat string `json:"encoding_format,omitempty"`

	// Dimensions is the number of dimensions for the output embeddings.
	Dimensions *int `json:"dimensions,omitempty"`

	// User is a unique identifier for the end-user.
	User string `json:"user,omitempty"`
}

// EmbeddingResponse represents the unified embedding response model.
type EmbeddingResponse struct {
	// ID is the response identifier (some providers return this).
	ID string `json:"id,omitempty"`

	// Object is the object type, typically "list".
	Object string `json:"object"`

	// Data contains the embedding results.
	Data []EmbeddingData `json:"data"`

	// Usage contains token usage information.
	Usage *EmbeddingUsage `json:"usage,omitempty"`
}

// EmbeddingData represents a single embedding result.
type EmbeddingData struct {
	// Object is the object type, typically "embedding".
	Object string `json:"object"`

	// Embedding is the embedding vector. Can be []float64 or base64 encoded string.
	Embedding Embedding `json:"embedding"`

	// Index is the index of the input this embedding corresponds to.
	Index int `json:"index"`
}

type Embedding struct {
	Embedding []float64 `json:"embedding,omitempty"`
	Base64    string    `json:"base64,omitempty"`
}

func (e Embedding) MarshalJSON() ([]byte, error) {
	if len(e.Embedding) > 0 {
		return json.Marshal(e.Embedding)
	}

	if e.Base64 != "" {
		return json.Marshal(e.Base64)
	}

	return json.Marshal(nil)
}

func (e *Embedding) UnmarshalJSON(data []byte) error {
	var str string

	err := json.Unmarshal(data, &str)
	if err == nil {
		e.Base64 = str
		return nil
	}

	var floatArray []float64

	err = json.Unmarshal(data, &floatArray)
	if err == nil {
		e.Embedding = floatArray
		return nil
	}

	return fmt.Errorf("invalid embedding type")
}

// EmbeddingUsage represents token usage for embedding requests.
type EmbeddingUsage struct {
	// PromptTokens is the number of tokens in the input.
	PromptTokens int64 `json:"prompt_tokens"`

	// TotalTokens is the total number of tokens used.
	TotalTokens int64 `json:"total_tokens"`
}
