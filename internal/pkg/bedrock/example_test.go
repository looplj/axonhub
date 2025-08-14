package bedrock_test

import (
	"context"
	"fmt"
	"io"

	"github.com/looplj/axonhub/internal/pkg/bedrock"
	"github.com/looplj/axonhub/internal/pkg/httpclient"
)

// Example_bedrockStreaming demonstrates how to use Bedrock streaming with the AWS EventStream decoder.
func Example_bedrockStreaming() {
	// Create a new Bedrock executor
	executor, err := bedrock.NewExecutor("us-east-1", "accessKeyID", "secretAccessKey")
	if err != nil {
		fmt.Printf("Error creating executor: %v\n", err)
		return
	}

	// Create a streaming request
	req := &httpclient.Request{
		Method: "POST",
		URL:    "https://bedrock-runtime.us-east-1.amazonaws.com/model/anthropic.claude-3-sonnet-20240229-v1:0/invoke-with-response-stream",
		Headers: map[string][]string{
			"Content-Type": {"application/json"},
			"Accept":       {"application/vnd.amazon.eventstream"},
		},
		Body: []byte(`{"messages":[{"role":"user","content":"Hello"}],"max_tokens":100}`),
	}

	// Execute the streaming request
	stream, err := executor.DoStream(context.Background(), req)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	defer stream.Close()

	// Process the stream
	for stream.Next() {
		event := stream.Current()
		fmt.Printf("Event: %+v\n", event)
	}

	if err := stream.Err(); err != nil {
		fmt.Printf("Stream error: %v\n", err)
	}
}

// Example_customDecoder demonstrates how to register a custom decoder.
func Example_customDecoder() {
	// Create a new Bedrock executor
	executor, err := bedrock.NewExecutor("us-west-2", "accessKeyID", "secretAccessKey")
	if err != nil {
		fmt.Printf("Error creating executor: %v\n", err)
		return
	}

	// Create a custom decoder (example)
	// In a real implementation, you would create a decoder that implements
	// the httpclient.StreamDecoder interface

	// Create a request
	req := &httpclient.Request{
		Method: "POST",
		URL:    "https://bedrock-runtime.us-west-2.amazonaws.com/model/anthropic.claude-3-haiku-20240307-v1:0/invoke",
		Headers: map[string][]string{
			"Content-Type": {"application/json"},
		},
		Body: []byte(`{"messages":[{"role":"user","content":"What is AI?"}],"max_tokens":50}`),
	}

	// Execute the request
	resp, err := executor.Do(context.Background(), req)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Response: %s\n", string(resp.Body))
}

// customDecoder is an example custom decoder implementation.
type customDecoder struct {
	rc      io.ReadCloser
	current *httpclient.StreamEvent
	err     error
}

func (c *customDecoder) Next() bool {
	// Implement your custom decoding logic here
	return false
}

func (c *customDecoder) Current() *httpclient.StreamEvent {
	return c.current
}

func (c *customDecoder) Err() error {
	return c.err
}

func (c *customDecoder) Close() error {
	return c.rc.Close()
}
