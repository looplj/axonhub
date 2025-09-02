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

	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/pkg/httpclient"
	"go.uber.org/fx"
)

// Dumper is responsible for dumping data to files when errors occur.
type Dumper struct {
	config Config
	logger *log.Logger
	mu     sync.Mutex
}

// New creates a new Dumper instance.
func New(config Config, logger *log.Logger) *Dumper {
	return &Dumper{
		config: config,
		logger: logger,
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
	if err := os.MkdirAll(d.config.DumpPath, 0755); err != nil {
		d.logger.Error(ctx, "Failed to create dump directory", log.NamedError("error", err))
		return
	}

	// Create dump file
	timestamp := time.Now().Format("20060102_150405")
	fullPath := filepath.Join(d.config.DumpPath, fmt.Sprintf("%s_%s.json", filename, timestamp))

	file, err := os.Create(fullPath)
	if err != nil {
		d.logger.Error(ctx, "Failed to create dump file", log.NamedError("error", err), log.String("path", fullPath))
		return
	}
	defer file.Close()

	// Marshal data to JSON
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		d.logger.Error(ctx, "Failed to marshal data to JSON", log.NamedError("error", err))
		return
	}

	// Write to file
	if _, err := file.Write(jsonData); err != nil {
		d.logger.Error(ctx, "Failed to write data to dump file", log.NamedError("error", err), log.String("path", fullPath))
		return
	}

	d.logger.Info(ctx, "Successfully dumped struct to file", log.String("path", fullPath))
}

// DumpStreamEvents dumps a slice of interface{} as JSONL (JSON Lines) to a file.
func (d *Dumper) DumpStreamEvents(ctx context.Context, events []*httpclient.StreamEvent, filename string) {
	if !d.config.Enabled {
		return
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	// Ensure dump directory exists
	if err := os.MkdirAll(d.config.DumpPath, 0755); err != nil {
		d.logger.Error(ctx, "Failed to create dump directory", log.NamedError("error", err))
		return
	}

	// Create dump file
	timestamp := time.Now().Format("20060102_150405")
	fullPath := filepath.Join(d.config.DumpPath, fmt.Sprintf("%s_%s.jsonl", filename, timestamp))

	file, err := os.Create(fullPath)
	if err != nil {
		d.logger.Error(ctx, "Failed to create dump file", log.NamedError("error", err), log.String("path", fullPath))
		return
	}
	defer file.Close()

	// Create a buffered writer for better performance
	writer := bufio.NewWriter(file)
	defer writer.Flush()

	// Write each event as a JSON line
	for i, event := range events {
		jsonData, err := httpclient.EncodeStreamEventToJSON(event)
		if err != nil {
			d.logger.Error(ctx, "Failed to marshal stream event to JSON", log.NamedError("error", err), log.Int("index", i))
			return
		}

		if _, err := writer.Write(append(jsonData, '\n')); err != nil {
			d.logger.Error(ctx, "Failed to write stream event to dump file", log.NamedError("error", err), log.Int("index", i), log.String("path", fullPath))
			return
		}
	}

	d.logger.Info(ctx, "Successfully dumped stream events to file", log.String("path", fullPath), log.Int("count", len(events)))
}

// Module is the fx module for the dumper.
func Module() fx.Option {
	return fx.Options(
		fx.Provide(New),
	)
}
