package pipeline

import (
	"errors"

	"github.com/looplj/axonhub/llm"
)

// ErrEmptyResponse indicates the response contains no meaningful content.
// This error triggers channel retry when empty response detection is enabled.
var ErrEmptyResponse = errors.New("empty response detected")

// ErrStreamFirstEventTimeout indicates a streaming response did not produce the first event in time.
var ErrStreamFirstEventTimeout = errors.New("stream first event timeout")

// ErrNonStreamResponseTimeout indicates a non-streaming response did not complete in time.
var ErrNonStreamResponseTimeout = errors.New("non-stream response timeout")

// ErrEmptyStreamChunks indicates an auto-upgraded streaming request produced no inbound chunks.
var ErrEmptyStreamChunks = errors.New("empty stream chunks")

// ErrEmptyAggregatedBody indicates inbound chunk aggregation produced an empty body.
var ErrEmptyAggregatedBody = errors.New("empty aggregated body")

func hasMessageContent(msg *llm.Message) bool {
	if msg == nil {
		return false
	}

	if msg.Content.Content != nil && *msg.Content.Content != "" {
		return true
	}

	if len(msg.Content.MultipleContent) > 0 {
		return true
	}

	if len(msg.ToolCalls) > 0 {
		return true
	}

	if msg.ReasoningContent != nil && *msg.ReasoningContent != "" {
		return true
	}

	if msg.Reasoning != nil && *msg.Reasoning != "" {
		return true
	}

	if msg.Refusal != "" {
		return true
	}

	if msg.Audio != nil {
		return true
	}

	return false
}

// hasResponseContent checks if an llm.Response contains meaningful content.
func hasResponseContent(resp *llm.Response) bool {
	if resp == nil || resp == llm.DoneResponse || resp.Object == "[DONE]" {
		return false
	}

	if resp.Embedding != nil && len(resp.Embedding.Data) > 0 {
		return true
	}

	if resp.Rerank != nil && len(resp.Rerank.Results) > 0 {
		return true
	}

	if resp.Image != nil && len(resp.Image.Data) > 0 {
		return true
	}

	if resp.Video != nil &&
		(resp.Video.ID != "" || resp.Video.Status != "" || resp.Video.VideoURL != "" || resp.Video.Error != nil) {
		return true
	}

	if resp.Compact != nil && len(resp.Compact.Output) > 0 {
		return true
	}

	if resp.Speech != nil && len(resp.Speech.Audio) > 0 {
		return true
	}

	if resp.Transcription != nil && (resp.Transcription.Text != "" || len(resp.Transcription.Raw) > 0) {
		return true
	}

	// Only audio deltas count as content. A bare "speech.audio.done" event with
	// no audio chunks must still be treated as empty so empty-response detection
	// can retry instead of completing a request with audio_bytes=0.
	if resp.SpeechStreamEvent != nil && resp.SpeechStreamEvent.AudioBase64 != "" {
		return true
	}

	if resp.SpeechAudioChunk != nil && len(resp.SpeechAudioChunk.Audio) > 0 {
		return true
	}

	if resp.TranscriptionStreamEvent != nil && (resp.TranscriptionStreamEvent.Delta != "" || resp.TranscriptionStreamEvent.Text != "" || resp.TranscriptionStreamEvent.Type != "") {
		return true
	}

	if resp.Completion != nil {
		for _, choice := range resp.Completion.Choices {
			if choice.Text != "" {
				return true
			}
		}
	}

	for _, choice := range resp.Choices {
		if hasMessageContent(choice.Delta) || hasMessageContent(choice.Message) {
			return true
		}
	}

	return false
}
