package dumper

import (
	"context"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/pkg/httpclient"
)

func TestDumper_DumpStruct(t *testing.T) {
	// Create a temporary directory for testing
	tempDir := t.TempDir()

	// Create a logger for testing
	logger := log.New(log.Config{
		Level: 0, // Debug level
	})

	// Create a dumper with test config
	config := Config{
		Enabled:    true,
		DumpPath:   tempDir,
		MaxSize:    100,
		MaxAge:     24 * time.Hour,
		MaxBackups: 10,
	}

	dumper := New(config, logger)

	// Test data to dump
	testData := struct {
		Name  string `json:"name"`
		Value int    `json:"value"`
	}{
		Name:  "test",
		Value: 42,
	}

	// Test dumping struct
	dumper.DumpStruct(context.Background(), testData, "test_struct")

	// Check if file was created
	files, err := os.ReadDir(tempDir)
	require.NoError(t, err)
	assert.Len(t, files, 1)
	assert.Contains(t, files[0].Name(), "test_struct")
	assert.Contains(t, files[0].Name(), ".json")
}

func TestDumper_DumpStreamEvents(t *testing.T) {
	// Create a temporary directory for testing
	tempDir := t.TempDir()

	// Create a logger for testing
	logger := log.New(log.Config{
		Level: 0, // Debug level
	})

	// Create a dumper with test config
	config := Config{
		Enabled:    true,
		DumpPath:   tempDir,
		MaxSize:    100,
		MaxAge:     24 * time.Hour,
		MaxBackups: 10,
	}

	dumper := New(config, logger)

	// Test data to dump
	events := []*httpclient.StreamEvent{
		{Type: "start", Data: []byte(strconv.FormatInt(time.Now().Unix(), 10))},
		{Type: "process", Data: []byte("test data")},
		{Type: "end", Data: []byte("success")},
	}

	// Test dumping stream events
	dumper.DumpStreamEvents(context.Background(), events, "test_events")

	// Check if file was created
	files, err := os.ReadDir(tempDir)
	require.NoError(t, err)
	assert.Len(t, files, 1)
	assert.Contains(t, files[0].Name(), "test_events")
	assert.Contains(t, files[0].Name(), ".jsonl")
}

func TestDumper_Disabled(t *testing.T) {
	// Create a temporary directory for testing
	tempDir := t.TempDir()

	// Create a logger for testing
	logger := log.New(log.Config{
		Level: 0, // Debug level
	})

	// Create a dumper with test config (disabled)
	config := Config{
		Enabled:    false,
		DumpPath:   tempDir,
		MaxSize:    100,
		MaxAge:     24 * time.Hour,
		MaxBackups: 10,
	}

	dumper := New(config, logger)

	// Test data to dump
	testData := struct {
		Name  string `json:"name"`
		Value int    `json:"value"`
	}{
		Name:  "test",
		Value: 42,
	}

	// Test dumping struct when disabled
	dumper.DumpStruct(context.Background(), testData, "test_struct")

	// Check that no file was created
	files, err := os.ReadDir(tempDir)
	require.NoError(t, err)
	assert.Len(t, files, 0)
}
