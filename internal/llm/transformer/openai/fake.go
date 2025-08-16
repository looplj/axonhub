package openai

import (
	"bufio"
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/looplj/axonhub/internal/llm/pipeline"
	"github.com/looplj/axonhub/internal/pkg/httpclient"
	"github.com/looplj/axonhub/internal/pkg/streams"
)

//go:embed testdata/*
var testdataFS embed.FS

// FakeTransformer implements pipeline.ChannelCustomizedExecutor for testing purposes.
// It returns fixed responses from testdata files.
type FakeTransformer struct{}

// NewFakeTransformer creates a new FakeTransformer instance.
func NewFakeTransformer() *FakeTransformer {
	return &FakeTransformer{}
}

// CustomizeExecutor returns a fake executor that returns fixed responses.
func (f *FakeTransformer) CustomizeExecutor(executor pipeline.Executor) pipeline.Executor {
	return &fakeExecutor{}
}

// fakeExecutor implements pipeline.Executor and returns fixed responses.
type fakeExecutor struct{}

// Do returns a fixed HTTP response from testdata/openai-stop.response.json.
func (e *fakeExecutor) Do(ctx context.Context, req *httpclient.Request) (*httpclient.Response, error) {
	// Read the fixed response from testdata
	testDataPath := filepath.Join("testdata", "openai-stop.response.json")

	responseData, err := testdataFS.ReadFile(testDataPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read test response: %w", err)
	}

	// Create a fake HTTP response
	resp := &httpclient.Response{
		StatusCode: 200,
		Headers:    map[string][]string{"Content-Type": {"application/json"}},
		Body:       responseData,
	}

	return resp, nil
}

// DoStream returns a fixed stream response from testdata/openai-stop.stream.jsonl.
func (e *fakeExecutor) DoStream(ctx context.Context, req *httpclient.Request) (streams.Stream[*httpclient.StreamEvent], error) {
	// Read the fixed stream response from testdata
	testDataPath := filepath.Join("testdata", "openai-stop.stream.jsonl")

	streamData, err := testdataFS.ReadFile(testDataPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read test stream: %w", err)
	}

	// Parse the JSONL file into stream events
	var events []*httpclient.StreamEvent

	scanner := bufio.NewScanner(bytes.NewReader(streamData))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		// Parse the line as a raw event with string Data field
		var rawEvent struct {
			LastEventID string `json:"LastEventID"`
			Type        string `json:"Type"`
			Data        string `json:"Data"`
		}
		if err := json.Unmarshal([]byte(line), &rawEvent); err != nil {
			return nil, fmt.Errorf("failed to parse stream event: %w", err)
		}

		// Convert to StreamEvent with []byte Data field
		event := &httpclient.StreamEvent{
			LastEventID: rawEvent.LastEventID,
			Type:        rawEvent.Type,
			Data:        []byte(rawEvent.Data),
		}
		events = append(events, event)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to scan stream data: %w", err)
	}

	// Create a stream from the events
	return streams.SliceStream(events), nil
}
