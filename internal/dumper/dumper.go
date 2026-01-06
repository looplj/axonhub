// Package dumper for internal debug use only.
package dumper

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/looplj/axonhub/llm/httpclient"
)

// Dumper is responsible for dumping data to files when errors occur.
type Dumper struct {
	config Config
	mu     sync.Mutex
}

// New creates a new Dumper instance.
func New(config Config) *Dumper {
	return &Dumper{
		config: config,
	}
}

// DumpStruct dumps any struct as JSON to a file.
func (d *Dumper) DumpStruct(ctx context.Context, data any, filename string) {
	if !d.config.Enabled {
		return
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	// Ensure dump directory exists
	if err := os.MkdirAll(d.config.DumpPath, 0o750); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create dump directory: %v\n", err)
		return
	}

	// Create dump file
	timestamp := time.Now().Format("20060102_150405")
	fullPath := filepath.Join(d.config.DumpPath, fmt.Sprintf("%s_%s.json", filename, timestamp))

	//nolint:gosec // Checked.
	file, err := os.Create(fullPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create dump file %s: %v\n", fullPath, err)
		return
	}

	defer func() {
		if err := file.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to close dump file %s: %v\n", fullPath, err)
		}
	}()

	// Marshal data to JSON
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to marshal data to JSON: %v\n", err)
		return
	}

	// Write to file
	if _, err := file.Write(jsonData); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to write data to dump file %s: %v\n", fullPath, err)
		return
	}

	fmt.Printf("Successfully dumped struct to file: %s\n", fullPath)
}

// DumpStreamEvents dumps a slice of interface{} as JSONL (JSON Lines) to a file.
func (d *Dumper) DumpStreamEvents(ctx context.Context, events []*httpclient.StreamEvent, filename string) {
	if !d.config.Enabled {
		return
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	// Ensure dump directory exists
	if err := os.MkdirAll(d.config.DumpPath, 0o750); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create dump directory: %v\n", err)
		return
	}

	// Create dump file
	timestamp := time.Now().Format("20060102_150405")
	fullPath := filepath.Join(d.config.DumpPath, fmt.Sprintf("%s_%s.jsonl", filename, timestamp))

	//nolint:gosec // Checked.
	file, err := os.Create(fullPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create dump file %s: %v\n", fullPath, err)
		return
	}

	defer func() {
		if err := file.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to close dump file %s: %v\n", fullPath, err)
		}
	}()

	// Create a buffered writer for better performance
	writer := bufio.NewWriter(file)

	defer func() {
		if err := writer.Flush(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to flush dump file %s: %v\n", fullPath, err)
		}
	}()

	// Write each event as a JSON line
	for i, event := range events {
		jsonData, err := httpclient.EncodeStreamEventToJSON(event)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to marshal stream event to JSON at index %d: %v\n", i, err)
			return
		}

		if _, err := writer.Write(append(jsonData, '\n')); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to write stream event to dump file %s at index %d: %v\n", fullPath, i, err)
			return
		}
	}

	fmt.Printf("Successfully dumped stream events to file: %s (count: %d)\n", fullPath, len(events))
}

// DumpBytes dumps raw byte data to a file.
func (d *Dumper) DumpBytes(ctx context.Context, data []byte, filename string) {
	if !d.config.Enabled {
		return
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	// Ensure dump directory exists
	if err := os.MkdirAll(d.config.DumpPath, 0o750); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create dump directory: %v\n", err)
		return
	}

	// Create dump file
	timestamp := time.Now().Format("20060102_150405")
	fullPath := filepath.Join(d.config.DumpPath, fmt.Sprintf("%s_%s.bin", filename, timestamp))

	//nolint:gosec // Checked.
	file, err := os.Create(fullPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create dump file %s: %v\n", fullPath, err)
		return
	}

	defer func() {
		if err := file.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to close dump file %s: %v\n", fullPath, err)
		}
	}()

	// Write bytes to file
	if _, err := file.Write(data); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to write bytes to dump file %s: %v\n", fullPath, err)
		return
	}

	fmt.Printf("Successfully dumped bytes to file: %s (size: %d)\n", fullPath, len(data))
}
