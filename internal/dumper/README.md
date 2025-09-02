# Dumper Module

The dumper module provides functionality to dump data to files when errors occur. It supports dumping generic structs as JSON and stream events as JSONL (JSON Lines).

## Configuration

Add the following to your `config.yml` to enable and configure the dumper:

```yaml
dumper:
  enabled: true                 # Enable data dumping on errors
  dump_path: "./dumps"          # Directory to dump data to
  max_size: 100                 # Maximum size of dump files in MB
  max_age: "24h"                # Maximum age of dump files to keep
  max_backups: 10               # Maximum number of old dump files to retain
```

Or use environment variables:
```bash
export AXONHUB_DUMPER_ENABLED=true
export AXONHUB_DUMPER_DUMP_PATH="./dumps"
export AXONHUB_DUMPER_MAX_SIZE=100
export AXONHUB_DUMPER_MAX_AGE="24h"
export AXONHUB_DUMPER_MAX_BACKUPS=10
```

## Usage

### Creating a Dumper Instance

```go
// Create a logger
logger := log.New(log.Config{
    Level: 0, // Debug level
})

// Create dumper config
config := dumper.Config{
    Enabled:    true,
    DumpPath:   "./dumps",
    MaxSize:    100,
    MaxAge:     24 * time.Hour,
    MaxBackups: 10,
}

// Create a dumper
dumper := dumper.New(config, logger)
```

### Dumping Structs as JSON

```go
// Assuming you have a dumper instance
err := dumper.DumpStruct(context, myDataStruct, "error_data")
if err != nil {
    // Handle error
}
```

### Dumping Stream Events as JSONL

```go
// Assuming you have a dumper instance and a slice of events
events := []interface{}{
    map[string]interface{}{"event": "start", "timestamp": time.Now().Unix()},
    map[string]interface{}{"event": "process", "data": "processing user data"},
    map[string]interface{}{"event": "complete", "result": "success"},
}

err := dumper.DumpStreamEvents(context, events, "stream_events")
if err != nil {
    // Handle error
}
```

## File Naming

Dumped files are named with the pattern: `{filename}_{timestamp}.{extension}`
- For structs: `{filename}_{YYYYMMDD_HHMMSS}.json`
- For stream events: `{filename}_{YYYYMMDD_HHMMSS}.jsonl`

## Notes

- The dumper is thread-safe
- When disabled, dump operations return nil without error
- The dump directory is created automatically if it doesn't exist