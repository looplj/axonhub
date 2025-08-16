package bedrock

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/aws/aws-sdk-go-v2/aws/protocol/eventstream"
	"github.com/aws/aws-sdk-go-v2/aws/protocol/eventstream/eventstreamapi"
	"github.com/tidwall/gjson"

	"github.com/looplj/axonhub/internal/pkg/httpclient"
)

// eventstreamChunk represents a chunk in the AWS eventstream format.
type eventstreamChunk struct {
	Bytes string `json:"bytes"`
	P     string `json:"p"`
}

// AWSEventStreamDecoder implements the StreamDecoder interface for AWS EventStream format.
type AWSEventStreamDecoder struct {
	eventstream.Decoder

	rc  io.ReadCloser
	evt *httpclient.StreamEvent
	err error
}

// NewAWSEventStreamDecoder creates a new AWS EventStream decoder.
func NewAWSEventStreamDecoder(ctx context.Context, rc io.ReadCloser) httpclient.StreamDecoder {
	return &AWSEventStreamDecoder{
		rc: rc,
	}
}

// Close closes the underlying reader.
func (d *AWSEventStreamDecoder) Close() error {
	return d.rc.Close()
}

// Err returns any error that occurred during decoding.
func (d *AWSEventStreamDecoder) Err() error {
	return d.err
}

// Next advances to the next event in the stream.
func (d *AWSEventStreamDecoder) Next() bool {
	if d.err != nil {
		return false
	}

	msg, err := d.Decoder.Decode(d.rc, nil)
	if err != nil {
		d.err = err
		return false
	}

	messageType := msg.Headers.Get(eventstreamapi.MessageTypeHeader)
	if messageType == nil {
		d.err = fmt.Errorf("%s event header not present", eventstreamapi.MessageTypeHeader)
		return false
	}

	switch messageType.String() {
	case eventstreamapi.EventMessageType:
		return d.handleEventMessage(msg)
	case eventstreamapi.ExceptionMessageType:
		return d.handleExceptionMessage(msg)
	case eventstreamapi.ErrorMessageType:
		return d.handleErrorMessage(msg)
	default:
		d.err = fmt.Errorf("unknown message type: %s", messageType.String())
		return false
	}
}

// handleEventMessage processes event messages.
func (d *AWSEventStreamDecoder) handleEventMessage(msg eventstream.Message) bool {
	eventType := msg.Headers.Get(eventstreamapi.EventTypeHeader)
	if eventType == nil {
		d.err = fmt.Errorf("%s event header not present", eventstreamapi.EventTypeHeader)
		return false
	}

	if eventType.String() == "chunk" {
		chunk := eventstreamChunk{}

		err := json.Unmarshal(msg.Payload, &chunk)
		if err != nil {
			d.err = err
			return false
		}

		decoded, err := base64.StdEncoding.DecodeString(chunk.Bytes)
		if err != nil {
			d.err = err
			return false
		}

		d.evt = &httpclient.StreamEvent{
			Type: gjson.GetBytes(decoded, "type").String(),
			Data: decoded,
		}
	}

	return true
}

// handleExceptionMessage processes exception messages.
func (d *AWSEventStreamDecoder) handleExceptionMessage(msg eventstream.Message) bool {
	exceptionType := msg.Headers.Get(eventstreamapi.ExceptionTypeHeader)
	if exceptionType == nil {
		d.err = fmt.Errorf("%s event header not present", eventstreamapi.ExceptionTypeHeader)
		return false
	}

	var errInfo struct {
		Code    string
		Type    string `json:"__type"`
		Message string
	}

	//nolint:musttag // Checked.
	err := json.Unmarshal(msg.Payload, &errInfo)
	if err != nil && !errors.Is(err, io.EOF) {
		d.err = fmt.Errorf("received exception %s: parsing exception payload failed: %w", exceptionType.String(), err)
		return false
	}

	errorCode := "UnknownError"

	errorMessage := errorCode
	if ev := exceptionType.String(); len(ev) > 0 {
		errorCode = ev
	} else if len(errInfo.Code) > 0 {
		errorCode = errInfo.Code
	} else if len(errInfo.Type) > 0 {
		errorCode = errInfo.Type
	}

	if len(errInfo.Message) > 0 {
		errorMessage = errInfo.Message
	}

	d.err = fmt.Errorf("received exception %s: %s", errorCode, errorMessage)

	return false
}

// handleErrorMessage processes error messages.
func (d *AWSEventStreamDecoder) handleErrorMessage(msg eventstream.Message) bool {
	errorCode := "UnknownError"
	errorMessage := errorCode

	if header := msg.Headers.Get(eventstreamapi.ErrorCodeHeader); header != nil {
		errorCode = header.String()
	}

	if header := msg.Headers.Get(eventstreamapi.ErrorMessageHeader); header != nil {
		errorMessage = header.String()
	}

	d.err = fmt.Errorf("received error %s: %s", errorCode, errorMessage)

	return false
}

// Current returns the current event.
func (d *AWSEventStreamDecoder) Current() *httpclient.StreamEvent {
	return d.evt
}

// Ensure AWSEventStreamDecoder implements StreamDecoder.
var _ httpclient.StreamDecoder = (*AWSEventStreamDecoder)(nil)
