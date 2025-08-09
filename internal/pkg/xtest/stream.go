package xtest

import (
	"bufio"
	"encoding/json"
	"os"
	"strings"

	"github.com/looplj/axonhub/internal/pkg/httpclient"
)

// LoadStreamChunks loads stream chunks from a JSONL file in testdata directory.
func LoadStreamChunks(filename string) ([]*httpclient.StreamEvent, error) {
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

	var chunks []*httpclient.StreamEvent

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
			return nil, err
		}

		// Create the StreamEvent with Data as []byte
		streamEvent := &httpclient.StreamEvent{
			LastEventID: temp.LastEventID,
			Type:        temp.Type,
			Data:        []byte(temp.Data), // Convert string to []byte
		}

		chunks = append(chunks, streamEvent)
	}

	return chunks, scanner.Err()
}

// LoadTestData loads test data from a JSON file in testdata directory.
func LoadTestData(filename string, v interface{}) error {
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
