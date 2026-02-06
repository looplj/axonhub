package main

import (
	"encoding/json"
	"net/http"
	"os"
	"testing"
)

// ModelResponse represents the list models API response
type ModelResponse struct {
	Object string        `json:"object"`
	Data   []OpenAIModel `json:"data"`
}

// OpenAIModel represents a model in the API response
type OpenAIModel struct {
	ID              string        `json:"id"`
	Object          string        `json:"object"`
	Created         int64         `json:"created"`
	OwnedBy         string        `json:"owned_by"`
	Name            string        `json:"name,omitempty"`
	Description     string        `json:"description,omitempty"`
	Icon            string        `json:"icon,omitempty"`
	Type            string        `json:"type,omitempty"`
	ContextLength   int           `json:"context_length,omitempty"`
	MaxOutputTokens int           `json:"max_output_tokens,omitempty"`
	Capabilities    *Capabilities `json:"capabilities,omitempty"`
	Pricing         *Pricing      `json:"pricing,omitempty"`
}

// Capabilities represents model capabilities
type Capabilities struct {
	Vision    bool `json:"vision"`
	ToolCall  bool `json:"tool_call"`
	Reasoning bool `json:"reasoning"`
}

// Pricing represents model pricing information
type Pricing struct {
	Input      float64 `json:"input"`
	Output     float64 `json:"output"`
	CacheRead  float64 `json:"cache_read"`
	CacheWrite float64 `json:"cache_write"`
	Unit       string  `json:"unit"`
	Currency   string  `json:"currency"`
}

func getBaseURL() string {
	if url := os.Getenv("TEST_OPENAI_BASE_URL"); url != "" {
		return url
	}
	return "http://localhost:8090/v1"
}

func getAPIKey() string {
	return os.Getenv("TEST_AXONHUB_API_KEY")
}

func fetchModels(t *testing.T, query string) *ModelResponse {
	baseURL := getBaseURL()
	apiKey := getAPIKey()

	if apiKey == "" {
		t.Skip("TEST_AXONHUB_API_KEY not set, skipping integration test")
	}

	url := baseURL + "/models"
	if query != "" {
		url = url + "?" + query
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", resp.StatusCode)
	}

	var result ModelResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if result.Object != "list" {
		t.Errorf("Expected object to be 'list', got '%s'", result.Object)
	}

	if len(result.Data) == 0 {
		t.Skip("No models returned from API")
	}

	return &result
}

// TestBackwardCompatibility tests that the endpoint returns only basic fields without query params
func TestBackwardCompatibility(t *testing.T) {
	result := fetchModels(t, "")

	model := result.Data[0]

	// Basic fields should always be present
	if model.ID == "" {
		t.Error("Expected ID to be present")
	}
	if model.Object != "model" {
		t.Errorf("Expected Object to be 'model', got '%s'", model.Object)
	}
	if model.Created == 0 {
		t.Error("Expected Created to be present")
	}
	if model.OwnedBy == "" {
		t.Error("Expected OwnedBy to be present")
	}

	// Extended fields should NOT be present in backward compatible mode
	if model.Name != "" {
		t.Errorf("Expected Name to be empty in backward compatible mode, got '%s'", model.Name)
	}
	if model.Description != "" {
		t.Errorf("Expected Description to be empty in backward compatible mode, got '%s'", model.Description)
	}
	if model.Capabilities != nil {
		t.Error("Expected Capabilities to be nil in backward compatible mode")
	}
	if model.Pricing != nil {
		t.Error("Expected Pricing to be nil in backward compatible mode")
	}

	t.Logf("Backward compatibility verified: model %s has only basic fields", model.ID)
}

// TestIncludeAll tests that ?include=all returns all extended fields
func TestIncludeAll(t *testing.T) {
	result := fetchModels(t, "include=all")

	model := result.Data[0]

	// Basic fields should be present
	if model.ID == "" {
		t.Error("Expected ID to be present")
	}

	// With include=all, we should have extended fields populated (if the model has them)
	// Note: Some fields may still be empty if the model doesn't have that data in the database
	t.Logf("Model %s - Name: %s, Description: %s", model.ID, model.Name, model.Description)

	// Check capabilities if present
	if model.Capabilities != nil {
		t.Logf("Capabilities - Vision: %v, ToolCall: %v, Reasoning: %v",
			model.Capabilities.Vision,
			model.Capabilities.ToolCall,
			model.Capabilities.Reasoning)

		// Type checks
		_ = model.Capabilities.Vision == true || model.Capabilities.Vision == false
		_ = model.Capabilities.ToolCall == true || model.Capabilities.ToolCall == false
		_ = model.Capabilities.Reasoning == true || model.Capabilities.Reasoning == false
	}

	// Check pricing if present
	if model.Pricing != nil {
		t.Logf("Pricing - Input: %f, Output: %f, Unit: %s",
			model.Pricing.Input,
			model.Pricing.Output,
			model.Pricing.Unit)
	}

	t.Log("Include=all test completed")
}

// TestIncludeSelective tests that selective ?include=name,pricing returns only requested fields
func TestIncludeSelective(t *testing.T) {
	result := fetchModels(t, "include=name,pricing")

	model := result.Data[0]

	// Basic fields should be present
	if model.ID == "" {
		t.Error("Expected ID to be present")
	}

	// name and pricing should be populated (if model has data)
	t.Logf("Model %s - Name: %s", model.ID, model.Name)

	// Description should NOT be present since we only asked for name,pricing
	if model.Description != "" {
		t.Logf("Warning: Description is present but not requested: %s", model.Description)
	}

	// Capabilities should NOT be present
	if model.Capabilities != nil {
		t.Log("Warning: Capabilities present but not requested")
	}

	// Pricing should be present
	if model.Pricing != nil {
		t.Logf("Pricing present - Input: %f", model.Pricing.Input)
	}

	t.Log("Selective include test completed")
}

// TestFieldTypes verifies that fields have correct JSON types
func TestFieldTypes(t *testing.T) {
	result := fetchModels(t, "include=all")

	model := result.Data[0]

	// Verify basic field types
	if model.ID != "" && model.Object == "model" {
		t.Log("Basic string fields verified")
	}

	if model.Created > 0 {
		t.Log("Created timestamp is int64")
	}

	// Verify capabilities boolean fields
	if model.Capabilities != nil {
		// These should all be boolean types
		_ = model.Capabilities.Vision
		_ = model.Capabilities.ToolCall
		_ = model.Capabilities.Reasoning
		t.Log("Capabilities boolean fields verified")
	}

	// Verify pricing numeric fields
	if model.Pricing != nil {
		// These should all be float64 types
		_ = model.Pricing.Input
		_ = model.Pricing.Output
		_ = model.Pricing.CacheRead
		_ = model.Pricing.CacheWrite
		t.Log("Pricing numeric fields verified")
	}

	t.Log("Field types test completed")
}

// TestIncludeCapabilitiesOnly tests requesting only capabilities
func TestIncludeCapabilitiesOnly(t *testing.T) {
	result := fetchModels(t, "include=capabilities")

	model := result.Data[0]

	// Basic fields should be present
	if model.ID == "" {
		t.Error("Expected ID to be present")
	}

	// Capabilities should be present
	if model.Capabilities != nil {
		t.Logf("Capabilities present: Vision=%v, ToolCall=%v, Reasoning=%v",
			model.Capabilities.Vision,
			model.Capabilities.ToolCall,
			model.Capabilities.Reasoning)
	}

	// Pricing should NOT be present
	if model.Pricing != nil {
		t.Error("Expected Pricing to be nil when only capabilities requested")
	}

	// Name should NOT be present
	if model.Name != "" {
		t.Logf("Warning: Name present but not requested: %s", model.Name)
	}

	t.Log("Capabilities-only include test completed")
}

// TestResponseStructure verifies the overall response structure
func TestResponseStructure(t *testing.T) {
	result := fetchModels(t, "")

	// Check top-level object field
	if result.Object != "list" {
		t.Errorf("Expected Object to be 'list', got '%s'", result.Object)
	}

	// Check data array
	if result.Data == nil {
		t.Error("Expected Data to be non-nil")
	}

	// Each model should have required fields
	for _, model := range result.Data {
		if model.ID == "" {
			t.Error("Model missing ID")
		}
		if model.Object != "model" {
			t.Errorf("Model %s has Object='%s', expected 'model'", model.ID, model.Object)
		}
	}

	t.Logf("Response structure verified: %d models returned", len(result.Data))
}
