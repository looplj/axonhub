// Package dumper for internal debug use only.
package dumper

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
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

	d.cleanup()

	slog.Info("dumped struct to file", "path", fullPath)
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

	d.cleanup()

	slog.Info("dumped stream events to file", "path", fullPath, "count", len(events))
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

	d.cleanup()

	slog.Info("dumped bytes to file", "path", fullPath, "size", len(data))
}

// cleanup enforces retention policy on dump files: total size, backup count, and max age.
func (d *Dumper) cleanup() {
	entries, err := os.ReadDir(d.config.DumpPath)
	if err != nil {
		return
	}

	type fileEntry struct {
		path    string
		modTime time.Time
		size    int64
	}

	var files []fileEntry
	var totalSize int64
	maxBytes := int64(d.config.MaxSize) * 1024 * 1024
	cutoff := time.Now().Add(-d.config.MaxAge)

	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		files = append(files, fileEntry{
			path:    filepath.Join(d.config.DumpPath, e.Name()),
			modTime: info.ModTime(),
			size:    info.Size(),
		})
		totalSize += info.Size()
	}

	// Sort by modification time, oldest first.
	sort.Slice(files, func(i, j int) bool {
		return files[i].modTime.Before(files[j].modTime)
	})

	toDelete := make(map[string]struct{})

	// 1. Remove files exceeding MaxAge.
	for _, f := range files {
		if d.config.MaxAge > 0 && f.modTime.Before(cutoff) {
			toDelete[f.path] = struct{}{}
		}
	}

	// 2. Remove oldest files until total size is within MaxSize.
	if maxBytes > 0 {
		remaining := totalSize
		for _, f := range files {
			if _, ok := toDelete[f.path]; ok {
				continue
			}
			if remaining <= maxBytes {
				break
			}
			toDelete[f.path] = struct{}{}
			remaining -= f.size
		}
	}

	// 3. Remove oldest files if count exceeds MaxBackups.
	if d.config.MaxBackups > 0 {
		kept := 0
		for i := len(files) - 1; i >= 0; i-- {
			if _, ok := toDelete[files[i].path]; ok {
				continue
			}
			if kept >= d.config.MaxBackups {
				toDelete[files[i].path] = struct{}{}
				continue
			}
			kept++
		}
	}

	for path := range toDelete {
		if err := os.Remove(path); err != nil {
			slog.Warn("failed to remove old dump file", "path", path, "error", err)
		}
	}
}
