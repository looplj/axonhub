package main

import (
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/looplj/axonhub/llm/client"
	"github.com/looplj/axonhub/llm/transformer/openai"
	"github.com/looplj/axonhub/server/api"
)

func main() {
	// Create transformers
	inboundTransformer := openai.NewInboundTransformer()
	outboundTransformer := openai.NewOutboundTransformer()

	// Create HTTP client
	httpClient := client.NewHttpClient(30 * time.Second)

	// Create chat completion processor
	processor := api.NewChatCompletionProcessor(
		inboundTransformer,
		outboundTransformer,
		httpClient,
	)

	// Setup Gin router
	r := gin.Default()

	// Add chat completion endpoint
	r.POST("/v1/chat/completions", func(c *gin.Context) {
		if err := processor.Process(c); err != nil {
			log.Printf("Error processing chat completion: %v", err)
		}
	})

	// Start server
	log.Println("Starting server on :8080")
	log.Fatal(http.ListenAndServe(":8080", r))
}
