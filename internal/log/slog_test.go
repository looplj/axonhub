package log

import (
	"context"
	"encoding/json"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type testLogOutput struct {
	output []map[string]interface{}
}

func (t *testLogOutput) Write(p []byte) (n int, err error) {
	var entry map[string]interface{}
	if err := json.Unmarshal(p, &entry); err != nil {
		return 0, err
	}

	t.output = append(t.output, entry)

	return len(p), nil
}

func (t *testLogOutput) String() string {
	return ""
}

func (t *testLogOutput) Sync() error {
	return nil
}

func (t *testLogOutput) Close() error {
	return nil
}

func (t *testLogOutput) Clear() {
	t.output = nil
}

func (t *testLogOutput) GetOutput() []map[string]interface{} {
	return t.output
}

func createTestLogger(level Level) (*Logger, *testLogOutput) {
	output := &testLogOutput{}

	config := Config{
		Name:      "test",
		Level:     level,
		Encoding:  "json",
		Debug:     false,
		SkipLevel: 0,
		LevelKey:  "level",
		TimeKey:   "time",
		CallerKey: "caller",
		NameKey:   "logger",
	}

	logger := New(config)

	// Replace the output to capture logs
	zapLogger := logger.logger
	newLogger := zapLogger.WithOptions(zap.WrapCore(func(core zapcore.Core) zapcore.Core {
		return zapcore.NewCore(
			zapcore.NewJSONEncoder(zapcore.EncoderConfig{
				MessageKey:     "message",
				LevelKey:       "level",
				TimeKey:        "time",
				NameKey:        "logger",
				CallerKey:      "caller",
				StacktraceKey:  "stacktrace",
				SkipLineEnding: false,
				LineEnding:     zapcore.DefaultLineEnding,
				EncodeLevel:    zapcore.CapitalLevelEncoder,
				EncodeTime:     zapcore.RFC3339TimeEncoder,
				EncodeDuration: zapcore.SecondsDurationEncoder,
				EncodeCaller:   zapcore.ShortCallerEncoder,
			}),
			zapcore.AddSync(output),
			zap.NewAtomicLevelAt(zapcore.DebugLevel), // Use DebugLevel to allow all levels, rely on custom filtering
		)
	}))

	logger.logger = newLogger

	return logger, output
}

func TestSlogAdapter_BasicLogging(t *testing.T) {
	logger, output := createTestLogger(DebugLevel)
	slogLogger := logger.AsSlog()

	// Test basic logging at different levels
	slogLogger.Debug("debug message")
	slogLogger.Info("info message")
	slogLogger.Warn("warn message")
	slogLogger.Error("error message")

	logs := output.GetOutput()
	require.Len(t, logs, 4, "Should have 4 log entries")

	// Verify debug log
	assert.Equal(t, "debug message", logs[0]["message"])
	assert.Equal(t, "DEBUG", logs[0]["level"])

	// Verify info log
	assert.Equal(t, "info message", logs[1]["message"])
	assert.Equal(t, "INFO", logs[1]["level"])

	// Verify warn log
	assert.Equal(t, "warn message", logs[2]["message"])
	assert.Equal(t, "WARN", logs[2]["level"])

	// Verify error log
	assert.Equal(t, "error message", logs[3]["message"])
	assert.Equal(t, "ERROR", logs[3]["level"])
}

func TestSlogAdapter_LevelFiltering(t *testing.T) {
	logger, output := createTestLogger(InfoLevel)
	slogLogger := logger.AsSlog()

	// Test that debug logs are filtered out
	slogLogger.Debug("debug message - should be filtered")
	slogLogger.Info("info message - should appear")
	slogLogger.Warn("warn message - should appear")
	slogLogger.Error("error message - should appear")

	logs := output.GetOutput()
	require.Len(t, logs, 3, "Should have 3 log entries (debug filtered out)")

	// Verify debug message is not present
	for _, log := range logs {
		assert.NotContains(t, log["message"], "debug message - should be filtered")
	}

	// Verify other messages are present
	assert.Equal(t, "info message - should appear", logs[0]["message"])
	assert.Equal(t, "warn message - should appear", logs[1]["message"])
	assert.Equal(t, "error message - should appear", logs[2]["message"])
}

func TestSlogAdapter_WithAttributes(t *testing.T) {
	logger, output := createTestLogger(DebugLevel)
	slogLogger := logger.AsSlog()

	// Test logging with attributes
	slogLogger.Info("message with attributes",
		slog.String("key1", "value1"),
		slog.Int("key2", 42),
		slog.Bool("key3", true),
	)

	logs := output.GetOutput()
	require.Len(t, logs, 1, "Should have 1 log entry")

	log := logs[0]
	assert.Equal(t, "message with attributes", log["message"])
	assert.Equal(t, "value1", log["key1"])
	assert.Equal(t, float64(42), log["key2"])
	assert.Equal(t, true, log["key3"])
}

func TestSlogAdapter_WithGroups(t *testing.T) {
	logger, output := createTestLogger(DebugLevel)
	slogLogger := logger.AsSlog()

	// Test logging with groups
	slogLogger.Info("message with groups",
		slog.Group("request",
			slog.String("method", "GET"),
			slog.String("path", "/api/test"),
		),
		slog.Group("response",
			slog.Int("status", 200),
			slog.Int("size", 1024),
		),
	)

	logs := output.GetOutput()
	require.Len(t, logs, 1, "Should have 1 log entry")

	log := logs[0]
	assert.Equal(t, "message with groups", log["message"])

	// Verify group attributes - they should be nested under the group keys
	if requestGroup, ok := log["request"].(map[string]interface{}); ok {
		assert.Equal(t, "GET", requestGroup["method"])
		assert.Equal(t, "/api/test", requestGroup["path"])
	} else {
		t.Errorf("Expected 'request' to be a map, got: %T", log["request"])
	}

	if responseGroup, ok := log["response"].(map[string]interface{}); ok {
		assert.Equal(t, float64(200), responseGroup["status"])
		assert.Equal(t, float64(1024), responseGroup["size"])
	} else {
		t.Errorf("Expected 'response' to be a map, got: %T", log["response"])
	}
}

func TestSlogAdapter_WithHandler(t *testing.T) {
	logger, output := createTestLogger(DebugLevel)

	// Test WithAttrs
	slogLogger := logger.AsSlog()
	slogLoggerWithAttrs := slogLogger.With(
		slog.String("service", "test-service"),
		slog.String("version", "1.0.0"),
	)

	slogLoggerWithAttrs.Info("message with handler attributes")

	logs := output.GetOutput()
	require.Len(t, logs, 1, "Should have 1 log entry")

	log := logs[0]
	assert.Equal(t, "message with handler attributes", log["message"])
	assert.Equal(t, "test-service", log["service"])
	assert.Equal(t, "1.0.0", log["version"])
}

func TestSlogAdapter_WithGroupsHandler(t *testing.T) {
	logger, output := createTestLogger(DebugLevel)

	// Test WithGroup
	slogLogger := logger.AsSlog()
	slogLoggerWithGroup := slogLogger.WithGroup("http")

	slogLoggerWithGroup.Info("message with group",
		slog.String("method", "POST"),
		slog.String("path", "/api/create"),
	)

	logs := output.GetOutput()
	require.Len(t, logs, 1, "Should have 1 log entry")

	log := logs[0]
	assert.Equal(t, "message with group", log["message"])
	assert.Equal(t, "POST", log["http.method"])
	assert.Equal(t, "/api/create", log["http.path"])
}

func TestSlogAdapter_ContextLevelForcing(t *testing.T) {
	logger, output := createTestLogger(InfoLevel)
	slogLogger := logger.AsSlog()

	// Test without forcing (debug should be filtered)
	slogLogger.Debug("debug message - should be filtered")
	slogLogger.Info("info message - should appear")

	logs := output.GetOutput()
	require.Len(t, logs, 1, "Should have 1 log entry (debug filtered out)")
	assert.Equal(t, "info message - should appear", logs[0]["message"])

	// Test with forcing debug level
	output.Clear()

	forcedCtx := ForceEnableLevel(context.Background(), DebugLevel)

	// Check if the context is working
	enabled := slogLogger.Enabled(forcedCtx, slog.LevelDebug)
	require.True(t, enabled, "Debug should be enabled with forced context")

	slogLogger.DebugContext(forcedCtx, "debug message - should appear with force")
	slogLogger.InfoContext(forcedCtx, "info message - should also appear")

	logs = output.GetOutput()
	require.Len(t, logs, 2, "Should have 2 log entries (debug forced)")
	assert.Equal(t, "debug message - should appear with force", logs[0]["message"])
	assert.Equal(t, "info message - should also appear", logs[1]["message"])
}

func TestSlogAdapter_EnabledCheck(t *testing.T) {
	logger, _ := createTestLogger(InfoLevel)
	slogLogger := logger.AsSlog()

	// Test enabled checks
	assert.False(t, slogLogger.Enabled(context.Background(), slog.LevelDebug), "Debug should be disabled")
	assert.True(t, slogLogger.Enabled(context.Background(), slog.LevelInfo), "Info should be enabled")
	assert.True(t, slogLogger.Enabled(context.Background(), slog.LevelWarn), "Warn should be enabled")
	assert.True(t, slogLogger.Enabled(context.Background(), slog.LevelError), "Error should be enabled")

	// Test with forced debug level
	forcedCtx := ForceEnableLevel(context.Background(), DebugLevel)
	assert.True(t, slogLogger.Enabled(forcedCtx, slog.LevelDebug), "Debug should be enabled with force")
}

func TestSlogAdapter_TimeField(t *testing.T) {
	logger, output := createTestLogger(DebugLevel)
	slogLogger := logger.AsSlog()

	startTime := time.Now()

	slogLogger.Info("message with time")

	logs := output.GetOutput()
	require.Len(t, logs, 1, "Should have 1 log entry")

	log := logs[0]
	assert.Contains(t, log, "time", "Log should contain time field")

	// Parse and verify time
	timeStr, ok := log["time"].(string)
	require.True(t, ok, "Time should be a string")

	parsedTime, err := time.Parse(time.RFC3339, timeStr)
	require.NoError(t, err, "Time should be in RFC3339 format")

	// Verify time is recent (within 1 second)
	assert.WithinDuration(t, startTime, parsedTime, time.Second, "Log time should be recent")
}

func TestSlogAdapter_AllAttributeTypes(t *testing.T) {
	logger, output := createTestLogger(DebugLevel)
	slogLogger := logger.AsSlog()

	// Test all supported attribute types
	slogLogger.Info("message with all types",
		slog.String("string", "test"),
		slog.Int64("int64", 123456789),
		slog.Uint64("uint64", 987654321),
		slog.Float64("float64", 3.14159),
		slog.Bool("bool", true),
		slog.Duration("duration", 5*time.Second),
		slog.Time("time", time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)),
		slog.Any("any", map[string]interface{}{"nested": "value"}),
	)

	logs := output.GetOutput()
	require.Len(t, logs, 1, "Should have 1 log entry")

	log := logs[0]
	assert.Equal(t, "test", log["string"])
	assert.Equal(t, float64(123456789), log["int64"])
	assert.Equal(t, float64(987654321), log["uint64"])
	assert.Equal(t, 3.14159, log["float64"])
	assert.Equal(t, true, log["bool"])
	assert.Equal(t, float64(5), log["duration"]) // Duration in seconds
	assert.Equal(t, "2023-01-01T00:00:00Z", log["time"])
	assert.Equal(t, map[string]interface{}{"nested": "value"}, log["any"])
}

func TestSlogAdapter_EmptyAttributes(t *testing.T) {
	logger, output := createTestLogger(DebugLevel)
	slogLogger := logger.AsSlog()

	// Test with no attributes
	slogLogger.Info("message with no attributes")

	logs := output.GetOutput()
	require.Len(t, logs, 1, "Should have 1 log entry")

	log := logs[0]
	assert.Equal(t, "message with no attributes", log["message"])
	assert.Contains(t, log, "level", "Should have level field")
	assert.Contains(t, log, "time", "Should have time field")
}

func TestSlogAdapter_MultipleLoggers(t *testing.T) {
	output1 := &testLogOutput{}
	output2 := &testLogOutput{}

	logger1 := New(Config{
		Name:     "logger1",
		Level:    DebugLevel,
		Encoding: "json",
	})

	logger2 := New(Config{
		Name:     "logger2",
		Level:    InfoLevel,
		Encoding: "json",
	})

	// Replace outputs
	logger1.logger = logger1.logger.WithOptions(zap.WrapCore(func(core zapcore.Core) zapcore.Core {
		return zapcore.NewCore(
			zapcore.NewJSONEncoder(zapcore.EncoderConfig{
				MessageKey:     "message",
				LevelKey:       "level",
				TimeKey:        "time",
				NameKey:        "logger",
				CallerKey:      "caller",
				StacktraceKey:  "stacktrace",
				SkipLineEnding: false,
				LineEnding:     zapcore.DefaultLineEnding,
				EncodeLevel:    zapcore.CapitalLevelEncoder,
				EncodeTime:     zapcore.RFC3339TimeEncoder,
				EncodeDuration: zapcore.SecondsDurationEncoder,
				EncodeCaller:   zapcore.ShortCallerEncoder,
			}),
			zapcore.AddSync(output1),
			zap.NewAtomicLevelAt(zapcore.DebugLevel),
		)
	}))

	logger2.logger = logger2.logger.WithOptions(zap.WrapCore(func(core zapcore.Core) zapcore.Core {
		return zapcore.NewCore(
			zapcore.NewJSONEncoder(zapcore.EncoderConfig{
				MessageKey:     "message",
				LevelKey:       "level",
				TimeKey:        "time",
				NameKey:        "logger",
				CallerKey:      "caller",
				StacktraceKey:  "stacktrace",
				SkipLineEnding: false,
				LineEnding:     zapcore.DefaultLineEnding,
				EncodeLevel:    zapcore.CapitalLevelEncoder,
				EncodeTime:     zapcore.RFC3339TimeEncoder,
				EncodeDuration: zapcore.SecondsDurationEncoder,
				EncodeCaller:   zapcore.ShortCallerEncoder,
			}),
			zapcore.AddSync(output2),
			zap.NewAtomicLevelAt(zapcore.InfoLevel),
		)
	}))

	slogLogger1 := logger1.AsSlog()
	slogLogger2 := logger2.AsSlog()

	// Both loggers should log debug
	slogLogger1.Debug("debug from logger1")
	slogLogger2.Debug("debug from logger2")

	// Only logger1 should have debug log
	logs1 := output1.GetOutput()
	logs2 := output2.GetOutput()

	assert.Len(t, logs1, 1, "Logger1 should have debug log")
	assert.Len(t, logs2, 0, "Logger2 should not have debug log")

	if len(logs1) > 0 {
		assert.Equal(t, "debug from logger1", logs1[0]["message"])
	}
}
