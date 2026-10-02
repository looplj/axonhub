package orchestrator

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"

	sdkmetric "go.opentelemetry.io/otel/sdk/metric"

	"github.com/looplj/axonhub/internal/ent"
	appmetrics "github.com/looplj/axonhub/internal/metrics"
	"github.com/looplj/axonhub/llm/httpclient"
)

func TestInboundFirstOutputDoesNotRecordRequestDuration(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	previous := appmetrics.Metrics
	t.Cleanup(func() {
		appmetrics.Metrics = previous
		_ = provider.Shutdown(context.Background())
	})
	require.NoError(t, appmetrics.SetupMetrics(provider, "inbound-test"))
	stream := &InboundPersistentStream{
		ctx:     t.Context(),
		request: &ent.Request{CreatedAt: time.Now().Add(-time.Second), ModelID: "requested", Stream: true},
		state:   &PersistenceState{},
		stream: &mockStream{events: []*httpclient.StreamEvent{
			{Data: []byte(`{"type":"response.created"}`)},
			{Data: []byte(`{"choices":[{"delta":{"role":"assistant","content":""}}]}`)},
			{Data: []byte(`{"choices":[{"delta":{"content":"hello"}}]}`)},
			{Data: []byte(`{"choices":[{"delta":{"content":" world"}}]}`)},
		}},
	}
	for i := 0; stream.Next(); i++ {
		stream.Current()
		require.Equal(t, i >= 2, stream.downstreamTTFTRecorded)
	}

	var collected metricdata.ResourceMetrics
	require.NoError(t, reader.Collect(t.Context(), &collected))
	var ttftCount uint64
	for _, scope := range collected.ScopeMetrics {
		for _, mt := range scope.Metrics {
			require.NotEqual(t, "axonhub_downstream_request_duration_seconds", mt.Name,
				"first output must not emit an end-to-end duration sample")
			if mt.Name == "axonhub_downstream_ttft_seconds" {
				for _, point := range mt.Data.(metricdata.Histogram[float64]).DataPoints {
					ttftCount += point.Count
				}
			}
		}
	}
	require.Equal(t, uint64(1), ttftCount)
}

func TestHasClientVisibleOutput(t *testing.T) {
	tests := []struct {
		name string
		kind string
		data string
		want bool
	}{
		{"empty", "", "", false},
		{"done", "", "[DONE]", false},
		{"created in JSON", "", `{"type":"response.created"}`, false},
		{"message start", "message_start", `{"message":{"role":"assistant"}}`, false},
		{"role only", "", `{"choices":[{"delta":{"role":"assistant","content":""}}]}`, false},
		{"usage only", "", `{"usage":{"completion_tokens":5}}`, false},
		{"error", "error", `{"error":{"message":"failed"}}`, false},
		{"text", "", `{"choices":[{"delta":{"content":"hello"}}]}`, true},
		{"final text", "", `{"choices":[{"delta":{"content":"hello"},"finish_reason":"stop"}]}`, true},
		{"tool", "", `{"choices":[{"delta":{"tool_calls":[{"function":{"name":"search"}}]}}]}`, true},
		{"responses delta", "", `{"type":"response.output_text.delta","delta":"hello"}`, true},
		{"empty delta", "response.output_text.delta", `{"delta":""}`, false},
		{"anthropic empty block", "content_block_start", `{"content_block":{"type":"text","text":""}}`, false},
		{"anthropic tool block", "content_block_start", `{"content_block":{"type":"tool_use","name":"search"}}`, true},
		{"thinking", "content_block_delta", `{"delta":{"thinking":"think"}}`, true},
		{"signature only", "content_block_delta", `{"delta":{"signature":"sig"}}`, false},
		{"gemini final output", "", `{"candidates":[{"content":{"parts":[{"text":"hello"}]},"finishReason":"STOP"}]}`, true},
		{"audio", "audio/mpeg", "binary", true},
		{"ai sdk text", "text-delta", `{"delta":"hello"}`, true},
	}
	require.False(t, hasClientVisibleOutput(nil))
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, hasClientVisibleOutput(&httpclient.StreamEvent{Type: tt.kind, Data: []byte(tt.data)}))
		})
	}
}
