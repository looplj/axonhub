package anthropic

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/looplj/axonhub/internal/llm/pipeline"
	"github.com/looplj/axonhub/internal/llm/transformer"
	"github.com/looplj/axonhub/internal/pkg/httpclient"
	"github.com/looplj/axonhub/internal/pkg/streams"
)

//go:embed testdata/*
var testdataFS embed.FS

// FakeTransformer implements a fake transformer that returns fixed responses from testdata.
type FakeTransformer struct {
	transformer.Outbound
}

// NewFakeTransformer creates a new fake transformer.
func NewFakeTransformer() *FakeTransformer {
	return &FakeTransformer{}
}

// CustomizeExecutor returns a fake executor that serves fixed responses.
func (f *FakeTransformer) CustomizeExecutor(executor pipeline.Executor) pipeline.Executor {
	return &fakeExecutor{}
}

// Ensure FakeTransformer implements the ChannelCustomizedExecutor interface.
var _ pipeline.ChannelCustomizedExecutor = (*FakeTransformer)(nil)

// fakeExecutor implements the Executor interface with fixed responses.
type fakeExecutor struct{}

// Do returns a fixed non-streaming response.
func (e *fakeExecutor) Do(ctx context.Context, request *httpclient.Request) (*httpclient.Response, error) {
	// Read the fixed response from testdata
	responseData, err := testdataFS.ReadFile("testdata/anthropic-stop.response.json")
	if err != nil {
		return nil, fmt.Errorf("failed to read test response: %w", err)
	}

	return &httpclient.Response{
		StatusCode: http.StatusOK,
		Headers: http.Header{
			"Content-Type": []string{"application/json"},
		},
		Body: responseData,
	}, nil
}

// DoStream returns a fixed streaming response.
func (e *fakeExecutor) DoStream(ctx context.Context, request *httpclient.Request) (streams.Stream[*httpclient.StreamEvent], error) {
	// Read the fixed stream data from testdata
	streamData, err := testdataFS.ReadFile("testdata/anthropic-stop.stream.jsonl")
	if err != nil {
		return nil, fmt.Errorf("failed to read test stream: %w", err)
	}

	// Parse the JSONL stream data
	lines := strings.Split(strings.TrimSpace(string(streamData)), "\n")
	events := make([]*httpclient.StreamEvent, 0, len(lines))

	for _, line := range lines {
		if line == "" {
			continue
		}

		// Parse into a temporary struct to handle the Data field as string
		var rawEvent struct {
			LastEventID string `json:"LastEventID"`
			Type        string `json:"Type"`
			Data        string `json:"Data"`
		}
		if err := json.Unmarshal([]byte(line), &rawEvent); err != nil {
			return nil, fmt.Errorf("failed to parse stream event: %w", err)
		}

		// Convert to StreamEvent with Data as []byte
		event := &httpclient.StreamEvent{
			LastEventID: rawEvent.LastEventID,
			Type:        rawEvent.Type,
			Data:        []byte(rawEvent.Data),
		}

		events = append(events, event)
	}

	// Return a slice stream with the parsed events
	return streams.SliceStream(events), nil
}
