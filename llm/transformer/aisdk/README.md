# AI SDK Transformers

This package implements transformers for the AI SDK protocol, supporting both text streaming and data streaming formats.

## Transformers

### TextTransformer (`text.go`)
- Implements the original AI SDK text streaming protocol
- Uses simple text-based streaming format
- Backward compatible with existing implementations

### DataStreamTransformer (`datastream.go`)
- Implements the AI SDK Data Stream Protocol
- Uses Server-Sent Events (SSE) format
- Supports structured streaming with multiple part types

## Factory (`factory.go`)
The factory automatically selects the appropriate transformer based on request headers:

- **Data Stream**: When `X-Vercel-Ai-Ui-Message-Stream: v1` header is present
- **Text Stream**: Default fallback for backward compatibility

## Supported Stream Parts

The Data Stream Protocol supports the following stream parts:

### Text Parts
- `text-start`: Beginning of a text block
- `text-delta`: Incremental text content
- `text-end`: Completion of a text block

### Tool Parts
- `tool-input-start`: Beginning of tool input streaming
- `tool-input-delta`: Incremental tool input chunks
- `tool-input-available`: Tool input complete and ready for execution
- `tool-output-available`: Result of tool execution

### Control Parts
- `start`: Beginning of a new message
- `finish-step`: Completion of a step
- `finish`: Completion of a message
- `error`: Error information

## Usage

The transformer is automatically selected based on request headers:

```go
// Automatic selection based on headers
transformer := aisdk.NewTransformer(request.Header)

// Explicit selection
textTransformer := aisdk.NewTransformerByType(aisdk.TransformerTypeText)
dataStreamTransformer := aisdk.NewTransformerByType(aisdk.TransformerTypeDataStream)
```

## Headers

### Data Stream Protocol
When using the data stream protocol, the following headers are automatically set:
- `x-vercel-ai-ui-message-stream: v1`
- `Content-Type: text/event-stream`
- `Cache-Control: no-cache`
- `Connection: keep-alive`

### Text Stream Protocol
For backward compatibility, text streaming uses:
- `Content-Type: text/plain; charset=utf-8`
- `X-Vercel-AI-Data-Stream: v1`

## Testing

Comprehensive tests are provided for both transformers:
- `text_test.go`: Tests for text transformer
- `datastream_test.go`: Tests for data stream transformer
- `factory_test.go`: Tests for factory functions
