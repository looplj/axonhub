package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"

	"github.com/samber/lo"
	"github.com/tmaxmax/go-sse"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	fmt.Printf("Command arguments: %v\n", os.Args)
	fmt.Printf("Total arguments: %d\n", len(os.Args))

	for i, arg := range os.Args {
		fmt.Printf("  [%d]: %s\n", i, arg)
	}

	fmt.Println()

	command := os.Args[1]
	switch command {
	case "convert":
		runConvert(os.Args[2:])
	case "capture":
		runCapture(os.Args[2:])
	default:
		fmt.Printf("Unknown command: %s\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("Usage: axonhub-tool <command> [arguments]")
	fmt.Println("\nCommands:")
	fmt.Println("  convert  Convert JSON responses file to JSONL stream events")
	fmt.Println("  capture  Capture SSE stream from an upstream provider and save to JSONL")
	fmt.Println("\nUse 'axonhub-tool <command> -h' for more information about a command.")
}

func runConvert(args []string) {
	fs := flag.NewFlagSet("convert", flag.ExitOnError)
	input := fs.String("input", "", "Input JSON file containing []llm.Response")
	output := fs.String("output", "", "Output JSONL file containing stream events")
	fs.Parse(args)

	// Support positional arguments if flags are not provided
	if *input == "" && fs.NArg() > 0 {
		*input = fs.Arg(0)
	}

	if *output == "" && fs.NArg() > 1 {
		*output = fs.Arg(1)
	}

	if *input == "" || *output == "" {
		fmt.Println("Error: input and output files are required")
		fs.Usage()
		os.Exit(1)
	}

	responses, err := readJSONStreamFile(*input)
	if err != nil {
		log.Fatalf("Failed to read input file: %v", err)
	}

	streamEvents := convertJSONStreamEventsToStreamEvents(responses)

	err = writeStreamEventsFile(*output, streamEvents)
	if err != nil {
		log.Fatalf("Failed to write output file: %v", err)
	}

	fmt.Printf("Successfully converted %d responses to %d stream events in %s\n", len(responses), len(streamEvents), *output)
}

func getPresetPayload(payloadType string) string {
	switch payloadType {
	case "messages":
		return `{
  "messages": [
    {
      "content": "What is the current weather in New York?",
      "role": "user"
    }
  ],
  "model": "deepseek-chat",
  "stream": true,
  "max_tokens": 1024,
  "tools": [
    {  
        "type": "custom",
        "name": "get_current_weather",
        "description": "Get the current weather for a specified location",
        "input_schema": {
          "type": "object",
          "properties": {
            "location": {
              "type": "string",
              "description": "The city and state, e.g. San Francisco, CA"
            },
            "unit": {
              "type": "string",
              "enum": ["celsius", "fahrenheit"]
            }
          },
          "required": ["location"]
        }
      }
  ]
}`
	case "chat":
		return `{
  "messages": [
    {
      "content": "What is the current weather in New York?",
      "role": "user"
    }
  ],
  "model": "deepseek-chat",
  "stream": true,
  "tools": [
    {
      "type": "function",
      "function": {
        "name": "get_current_weather",
        "description": "Get the current weather for a specified location",
        "parameters": {
          "type": "object",
          "properties": {
            "location": {
              "type": "string",
              "description": "The city and state, e.g. San Francisco, CA"
            },
            "unit": {
              "type": "string",
              "enum": ["celsius", "fahrenheit"]
            }
          },
          "required": ["location"]
        }
      }
    }
  ]
}`
	case "responses":
		return `{
  "input": "What is the current weather in New York?",
  "model": "deepseek-chat",
  "stream": true,
  "tools": [
    {
      "type": "function",
        "name": "get_current_weather",
        "description": "Get the current weather for a specified location",
        "strict": true,
        "parameters": {
          "type": "object",
          "properties": {
            "location": {
              "type": "string",
              "description": "The city name to get weather for"
            }
          },
          "required": ["location"],
          "additionalProperties": false
        }
      }
  ]
}`
	default:
		return ""
	}
}

func runCapture(args []string) {
	fs := flag.NewFlagSet("capture", flag.ExitOnError)
	url := fs.String("url", "", "Upstream SSE URL")
	key := fs.String("key", "", "API Key (Authorization header)")
	payload := fs.String("payload", "", "Payload: preset type (messages/chat/responses) or JSON file path")
	output := fs.String("output", "captured.stream.jsonl", "Output JSONL file")
	model := fs.String("model", "", "Model name to override in payload")
	fs.Parse(args)

	if *url == "" || *key == "" {
		fmt.Println("Error: URL and API Key are required")
		fs.Usage()
		os.Exit(1)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	var body io.Reader

	if *payload != "" {
		presetPayload := getPresetPayload(*payload)
		if presetPayload != "" {
			payloadJSON := presetPayload
			if model != nil && *model != "" {
				payloadJSON = strings.ReplaceAll(payloadJSON, "deepseek-chat", *model)
			}

			body = strings.NewReader(payloadJSON)
		} else {
			data, err := os.ReadFile(*payload)
			if err != nil {
				log.Fatalf("Failed to read payload file: %v", err)
			}

			payloadJSON := string(data)

			if model != nil && *model != "" {
				var payloadMap map[string]any
				if err := json.Unmarshal(data, &payloadMap); err == nil {
					payloadMap["model"] = *model
					if updatedJSON, err := json.Marshal(payloadMap); err == nil {
						payloadJSON = string(updatedJSON)
					}
				}
			}

			body = strings.NewReader(payloadJSON)
		}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, *url, body)
	if err != nil {
		log.Fatalf("Failed to create request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")

	if *key != "" {
		if !strings.HasPrefix(*key, "Bearer ") && !strings.Contains(*key, " ") {
			req.Header.Set("Authorization", "Bearer "+*key)
		} else {
			req.Header.Set("Authorization", *key)
		}
	}

	fmt.Printf("Capturing stream from %s...\n", *url)

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Fatalf("Request error: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(res.Body)
		log.Fatalf("Response errored with code %s: %s", res.Status, string(data))
	}

	var events []StreamEvent
	// Use go-sse to read the stream
	for ev, err := range sse.Read(res.Body, nil) {
		if err != nil {
			log.Printf("Error while reading SSE stream: %v", err)
			break
		}

		// Convert sse.Event to our StreamEvent
		event := StreamEvent{
			LastEventID: ev.LastEventID,
			Type:        ev.Type,
			Data:        ev.Data,
		}
		events = append(events, event)

		fmt.Printf("Captured event: type=%s, data_len=%d\n", ev.Type, len(ev.Data))
	}

	if len(events) == 0 {
		fmt.Println("No events captured.")
		return
	}

	err = writeStreamEventsFile(*output, events)
	if err != nil {
		log.Fatalf("Failed to write output file: %v", err)
	}

	fmt.Printf("Successfully captured %d events to %s\n", len(events), *output)
}

func readJSONStreamFile(filename string) ([]JSONStreamEvent, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to open file %s: %w", filename, err)
	}
	defer file.Close()

	var events []JSONStreamEvent

	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&events); err != nil {
		return nil, fmt.Errorf("failed to decode JSON from %s: %w", filename, err)
	}

	return events, nil
}

type StreamEvent struct {
	LastEventID string `json:"LastEventID"`
	Type        string `json:"Type"`
	Data        string `json:"Data"` // Data is a JSON string in the test file
}

type JSONStreamEvent struct {
	LastEventID string          `json:"LastEventID"`
	Type        string          `json:"Type"`
	Data        json.RawMessage `json:"Data"` // Data is a JSON string in the test file
}

func convertJSONStreamEventsToStreamEvents(events []JSONStreamEvent) []StreamEvent {
	return lo.Map(events, func(event JSONStreamEvent, _ int) StreamEvent {
		var buf bytes.Buffer

		data := string(event.Data)
		if err := json.Compact(&buf, event.Data); err == nil {
			data = buf.String()
		}

		return StreamEvent{
			LastEventID: event.LastEventID,
			Type:        event.Type,
			Data:        data,
		}
	})
}

func writeStreamEventsFile(filename string, events []StreamEvent) error {
	// Create the directory if it doesn't exist
	dir := filepath.Dir(filename)
	if dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create file %s: %w", filename, err)
	}
	defer file.Close()

	writer := bufio.NewWriter(file)
	defer writer.Flush()

	// Write each event as a separate line (JSONL format)
	for _, event := range events {
		eventJSON, err := json.Marshal(event)
		if err != nil {
			log.Printf("Warning: failed to encode stream event: %v", err)
			continue
		}

		// Write the JSON line followed by a newline
		if _, err := writer.Write(eventJSON); err != nil {
			return fmt.Errorf("failed to write event to file: %w", err)
		}

		if _, err := writer.WriteString("\n"); err != nil {
			return fmt.Errorf("failed to write newline to file: %w", err)
		}
	}

	return nil
}
