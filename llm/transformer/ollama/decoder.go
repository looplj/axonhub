package ollama

import (
	"bufio"
	"context"
	"io"
	"log/slog"

	"github.com/looplj/axonhub/llm/httpclient"
)

// init registers the NDJSON decoder for Ollama streaming responses.
func init() {
	httpclient.RegisterDecoder("application/x-ndjson", NewNDJSONDecoder)
	httpclient.RegisterDecoder("application/x-ndjson; charset=utf-8", NewNDJSONDecoder)
}

// NewNDJSONDecoder creates a new NDJSON (Newline Delimited JSON) decoder.
// Ollama returns streaming responses in NDJSON format, not SSE.
func NewNDJSONDecoder(ctx context.Context, rc io.ReadCloser) httpclient.StreamDecoder {
	return &ndjsonDecoder{
		ctx:    ctx,
		reader: bufio.NewReader(rc),
		body:   rc,
	}
}

// ndjsonDecoder implements StreamDecoder for NDJSON format.
type ndjsonDecoder struct {
	ctx     context.Context
	reader  *bufio.Reader
	body    io.ReadCloser
	current *httpclient.StreamEvent
	err     error
	closed  bool
}

// Ensure ndjsonDecoder implements StreamDecoder.
var _ httpclient.StreamDecoder = (*ndjsonDecoder)(nil)

// Next advances to the next JSON object in the stream.
func (d *ndjsonDecoder) Next() bool {
	if d.err != nil || d.closed {
		return false
	}

	// Check context cancellation
	select {
	case <-d.ctx.Done():
		d.err = d.ctx.Err()
		_ = d.Close()
		return false
	default:
	}

	// Read next line
	line, err := d.reader.ReadBytes('\n')
	if err != nil {
		if err == io.EOF {
			// Check if there's remaining data without newline
			if len(line) > 0 {
				d.current = &httpclient.StreamEvent{
					Data: line,
				}
				return true
			}
			slog.DebugContext(d.ctx, "NDJSON stream closed")
		} else {
			d.err = err
			slog.ErrorContext(d.ctx, "NDJSON read error", slog.Any("error", err))
		}
		_ = d.Close()
		return false
	}

	// Trim newline but keep the data
	if len(line) > 0 && line[len(line)-1] == '\n' {
		line = line[:len(line)-1]
	}
	if len(line) > 0 && line[len(line)-1] == '\r' {
		line = line[:len(line)-1]
	}

	d.current = &httpclient.StreamEvent{
		Data: line,
	}

	return true
}

// Current returns the current event.
func (d *ndjsonDecoder) Current() *httpclient.StreamEvent {
	return d.current
}

// Err returns any error that occurred during streaming.
func (d *ndjsonDecoder) Err() error {
	return d.err
}

// Close closes the stream.
func (d *ndjsonDecoder) Close() error {
	if d.closed {
		return nil
	}
	d.closed = true
	if d.body != nil {
		return d.body.Close()
	}
	return nil
}
