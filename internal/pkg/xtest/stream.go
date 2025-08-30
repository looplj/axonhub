package xtest

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/llm"
	"github.com/looplj/axonhub/internal/pkg/httpclient"
)

// LoadStreamChunks loads stream chunks from a JSONL file in testdata directory.
func LoadStreamChunks(t *testing.T, filename string) ([]*httpclient.StreamEvent, error) {
	t.Helper()

	//nolint:gosec
	file, err := os.Open("testdata/" + filename)
	if err != nil {
		return nil, err
	}

	defer func() {
		err := file.Close()
		if err != nil {
			panic(err)
		}
	}()

	var (
		chunks []*httpclient.StreamEvent
		idx    int
	)

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		// Parse the line as a temporary struct to handle the Data field correctly
		var temp struct {
			LastEventID string `json:"LastEventID"`
			Type        string `json:"Type"`
			Data        string `json:"Data"` // Data is a JSON string in the test file
		}

		err := json.Unmarshal([]byte(line), &temp)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal stream chunk at line: %d %s %w", idx, line, err)
		}

		// Create the StreamEvent with Data as []byte
		streamEvent := &httpclient.StreamEvent{
			LastEventID: temp.LastEventID,
			Type:        temp.Type,
			Data:        []byte(temp.Data), // Convert string to []byte
		}

		chunks = append(chunks, streamEvent)
		idx++
	}

	return chunks, scanner.Err()
}

// LoadTestData loads test data from a JSON file in testdata directory.
func LoadTestData(t *testing.T, filename string, v any) error {
	t.Helper()

	//nolint:gosec
	file, err := os.Open("testdata/" + filename)
	if err != nil {
		return err
	}

	defer func() {
		err := file.Close()
		if err != nil {
			panic(err)
		}
	}()

	decoder := json.NewDecoder(file)

	return decoder.Decode(v)
}

func LoadResponses(t *testing.T, filename string) ([]*llm.Response, error) {
	t.Helper()

	expectedData, err := LoadStreamChunks(t, filename)
	if err != nil {
		return nil, err
	}

	var expectedResponses []*llm.Response

	for idx, line := range expectedData {
		if line != nil {
			// Check if this is a DONE event
			if bytes.Contains(line.Data, []byte(`[DONE]`)) {
				// This is a DONE event, add the DoneResponse
				expectedResponses = append(expectedResponses, llm.DoneResponse)
			} else {
				// Parse the Data field as llm.Response
				var resp llm.Response

				err = json.Unmarshal(line.Data, &resp)
				if err != nil {
					return nil, fmt.Errorf("failed to unmarshal response at index %d: %s %w", idx, string(line.Data), err)
				}

				require.NoError(t, err)

				expectedResponses = append(expectedResponses, &resp)
			}
		}
	}

	return expectedResponses, nil
}
