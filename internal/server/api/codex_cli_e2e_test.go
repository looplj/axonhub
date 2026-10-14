package api

import (
	"bytes"
	"context"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent/request"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/streams"
	"github.com/looplj/axonhub/llm/transformer/openai/responses"
)

func TestCodexCLIRealBinaryThroughResponsesGateway(t *testing.T) {
	// Given a real AxonHub Responses handler backed by deterministic upstream SSE.
	fixture := newStreamDeliveryE2EFixture(t, llm.APIFormatOpenAIResponse, codexCLIResponseStream(), false)
	h := fixture.handler(t)
	h.ChatCompletionOrchestrator.Inbound = responses.NewInboundTransformer()
	engine := gin.New()
	engine.Use(fixture.testContext)
	done := make(chan struct{})
	engine.POST("/v1/responses", func(c *gin.Context) {
		defer close(done)
		h.ChatCompletion(c)
	})
	server := startLoopbackServer(t, engine)

	// When the unmodified released Codex CLI sends one non-interactive request.
	workspace := t.TempDir()
	stdout, stderr, exitCode := runOfficialCodexCLI(t, workspace, server.URL+"/v1")

	// Then the CLI observed a completed Responses stream and AxonHub persisted it.
	require.Equal(t, 0, exitCode, "codex stdout=%s stderr=%s", stdout, stderr)
	require.Contains(t, stdout, "turn.completed")
	fixture.assertDatabase(t, request.StatusCompleted, true)
}

func runOfficialCodexCLI(t *testing.T, workspace, baseURL string) (string, string, int) {
	t.Helper()
	cmd, stdout, stderr := officialCodexCLICommand(t, workspace, baseURL, "Reply with exactly OK.")
	err := cmd.Run()
	if err == nil {
		return stdout.String(), stderr.String(), 0
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		return stdout.String(), stderr.String(), exitErr.ExitCode()
	}
	t.Fatalf("run official Codex CLI: %v", err)
	return "", "", -1
}

func officialCodexCLICommand(t *testing.T, workspace, baseURL, prompt string) (*exec.Cmd, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	binary := "/root/.local/share/axonhub-codex-testing/bin/codex-0.162.1-86f268d8"
	if _, err := os.Stat(binary); err != nil {
		t.Fatalf("official isolated Codex binary unavailable: %v", err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 45*time.Second)
	t.Cleanup(cancel)
	cmd := exec.CommandContext(ctx, binary, "--no-daemon", "exec", "--ignore-user-config", "--ignore-rules", "--skip-git-repo-check", "--ephemeral", "--sandbox", "read-only", "--json", prompt)
	cmd.Dir = workspace
	cmd.Env = append(os.Environ(),
		"CODEX_HOME=/root/.local/share/axonhub-codex-testing/codex-home",
		"AXONHUB_TEST_KEY=synthetic",
	)
	cmd.Args = append(cmd.Args,
		"-c", "model_provider=axonhub-test",
		"-c", "model_providers.axonhub-test={name='AxonHub test',base_url='"+baseURL+"',env_key='AXONHUB_TEST_KEY',wire_api='responses',requires_openai_auth=false,supports_websockets=false}",
		"-m", "gpt-test",
	)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	return cmd, &stdout, &stderr
}

func codexCLIResponseStream() string {
	return strings.Join([]string{
		"event: response.created\ndata: {\"type\":\"response.created\",\"response\":{\"id\":\"resp-codex\",\"status\":\"in_progress\",\"output\":[]}}\n\n",
		"event: response.output_item.added\ndata: {\"type\":\"response.output_item.added\",\"output_index\":0,\"item\":{\"type\":\"message\",\"id\":\"msg-codex\",\"role\":\"assistant\",\"content\":[]}}\n\n",
		"event: response.output_text.delta\ndata: {\"type\":\"response.output_text.delta\",\"item_id\":\"msg-codex\",\"output_index\":0,\"content_index\":0,\"delta\":\"OK\"}\n\n",
		"event: response.output_text.done\ndata: {\"type\":\"response.output_text.done\",\"item_id\":\"msg-codex\",\"output_index\":0,\"content_index\":0,\"text\":\"OK\"}\n\n",
		"event: response.completed\ndata: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp-codex\",\"status\":\"completed\",\"output\":[{\"type\":\"message\",\"id\":\"msg-codex\",\"role\":\"assistant\",\"content\":[{\"type\":\"output_text\",\"text\":\"OK\"}]}],\"usage\":{\"input_tokens\":12,\"output_tokens\":34,\"total_tokens\":46}}}\n\n",
	}, "")
}

const codexCLIToolFollowupResponseMarker = "__codex_cli_tool_followup_response__"

func codexCLIToolCallResponseStream() string {
	return strings.Join([]string{
		`event: response.created` + "\ndata: {\"type\":\"response.created\",\"response\":{\"id\":\"resp-tool\",\"status\":\"in_progress\",\"output\":[]}}\n\n",
		`event: response.output_item.added` + "\ndata: {\"type\":\"response.output_item.added\",\"output_index\":0,\"item\":{\"type\":\"function_call\",\"id\":\"call-tool\",\"call_id\":\"call-tool\",\"name\":\"exec_command\",\"status\":\"in_progress\"}}\n\n",
		`event: response.function_call_arguments.delta` + "\ndata: {\"type\":\"response.function_call_arguments.delta\",\"item_id\":\"call-tool\",\"output_index\":0,\"delta\":\"{\\\"cmd\\\":\\\"printf tool-result\\\"}\"}\n\n",
		`event: response.function_call_arguments.done` + "\ndata: {\"type\":\"response.function_call_arguments.done\",\"item_id\":\"call-tool\",\"output_index\":0,\"name\":\"exec_command\",\"arguments\":\"{\\\"cmd\\\":\\\"printf tool-result\\\"}\"}\n\n",
		`event: response.output_item.done` + "\ndata: {\"type\":\"response.output_item.done\",\"output_index\":0,\"item\":{\"type\":\"function_call\",\"id\":\"call-tool\",\"call_id\":\"call-tool\",\"name\":\"exec_command\",\"arguments\":\"{\\\"cmd\\\":\\\"printf tool-result\\\"}\",\"status\":\"completed\"}}\n\n",
		`event: response.completed` + "\ndata: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp-tool\",\"status\":\"completed\",\"output\":[{\"type\":\"function_call\",\"id\":\"call-tool\",\"call_id\":\"call-tool\",\"name\":\"exec_command\",\"arguments\":\"{\\\"cmd\\\":\\\"printf tool-result\\\"}\"}],\"usage\":{\"input_tokens\":12,\"output_tokens\":8,\"total_tokens\":20}}}\n\n",
	}, "")
}

func codexCLIFollowupResponseStream() string {
	return strings.Join([]string{
		`event: response.created` + "\ndata: {\"type\":\"response.created\",\"response\":{\"id\":\"resp-followup\",\"status\":\"in_progress\",\"output\":[]}}\n\n",
		`event: response.output_item.added` + "\ndata: {\"type\":\"response.output_item.added\",\"output_index\":0,\"item\":{\"type\":\"message\",\"id\":\"msg-followup\",\"role\":\"assistant\",\"content\":[]}}\n\n",
		`event: response.output_text.delta` + "\ndata: {\"type\":\"response.output_text.delta\",\"item_id\":\"msg-followup\",\"output_index\":0,\"content_index\":0,\"delta\":\"follow-up complete\"}\n\n",
		`event: response.output_text.done` + "\ndata: {\"type\":\"response.output_text.done\",\"item_id\":\"msg-followup\",\"output_index\":0,\"content_index\":0,\"text\":\"follow-up complete\"}\n\n",
		`event: response.completed` + "\ndata: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp-followup\",\"status\":\"completed\",\"output\":[{\"type\":\"message\",\"id\":\"msg-followup\",\"role\":\"assistant\",\"content\":[{\"type\":\"output_text\",\"text\":\"follow-up complete\"}]}],\"usage\":{\"input_tokens\":20,\"output_tokens\":10,\"total_tokens\":30}}}\n\n",
	}, "")
}

func startLoopbackServer(t *testing.T, handler *gin.Engine) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(func() {
		server.CloseClientConnections()
		server.Close()
	})
	return server
}

func TestCodexCLIPostTerminalDownstreamBarrier(t *testing.T) {
	fixture := newStreamDeliveryE2EFixture(t, llm.APIFormatOpenAIResponse, codexCLIResponseStream(), false)
	h := fixture.handler(t)
	h.ChatCompletionOrchestrator.Inbound = responses.NewInboundTransformer()
	receipt := make(chan string, 1)
	done := make(chan struct{})
	h.StreamWriter = func(c *gin.Context, source streams.Stream[*httpclient.StreamEvent]) {
		barrier := &codexCLIDownstreamBarrier{Stream: source, observer: source.(streams.DeliveryAcknowledger), c: c, receipt: receipt}
		WriteSSEStream(c, barrier)
	}
	engine := gin.New()
	engine.Use(fixture.testContext)
	engine.POST("/v1/responses", func(c *gin.Context) {
		defer close(done)
		h.ChatCompletion(c)
	})
	server := startLoopbackServer(t, engine)
	stdout, stderr, code := runOfficialCodexCLI(t, t.TempDir(), server.URL+"/v1")
	t.Logf("CLI exit=%d terminal=%t", code, strings.Contains(stdout, "turn.completed"))
	require.Equal(t, 0, code, "stderr=%s", stderr)
	require.Contains(t, stdout, "turn.completed")
	require.Equal(t, "ack -> downstream-canceled -> barrier-release", <-receipt)
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("gateway finalization did not finish")
	}
	fixture.assertDatabase(t, request.StatusCompleted, true)
	rows, err := fixture.client.UsageLog.Query().All(fixture.ctx)
	require.NoError(t, err)
	require.Equal(t, int64(12), rows[0].PromptTokens)
	require.Equal(t, int64(34), rows[0].CompletionTokens)
	t.Logf("Request/Execution completed; usage=12/34 count=1 cost=%v", rows[0].TotalCost)
}

func TestCodexCLIPreTerminalInterruptCancelsUpstream(t *testing.T) {
	fixture := newStreamDeliveryE2EFixture(t, llm.APIFormatOpenAIResponse, streamDeliveryBlockingResponseMarker, false)
	h := fixture.handler(t)
	h.ChatCompletionOrchestrator.Inbound = responses.NewInboundTransformer()
	engine := gin.New()
	engine.Use(fixture.testContext)
	done := make(chan struct{})
	engine.POST("/v1/responses", func(c *gin.Context) {
		defer close(done)
		h.ChatCompletion(c)
	})
	server := startLoopbackServer(t, engine)
	cmd, stdout, stderr := officialCodexCLICommand(t, t.TempDir(), server.URL+"/v1", "Continue working until interrupted.")
	require.NoError(t, cmd.Start())
	select {
	case <-fixture.upstreamStarted:
	case <-time.After(10 * time.Second):
		t.Fatal("synthetic upstream did not receive the real CLI request")
	}
	require.NoError(t, cmd.Process.Signal(os.Interrupt))
	err := cmd.Wait()
	require.Error(t, err, "the interrupted CLI unexpectedly exited successfully")
	t.Logf("pre-terminal interrupt exit=%v stdout=%s stderr=%s", err, stdout.String(), stderr.String())
	select {
	case <-done:
	case <-time.After(15 * time.Second):
		t.Fatal("gateway did not finish cancellation finalization")
	}
	fixture.assertDatabase(t, request.StatusCanceled, false)
}

type codexCLIDownstreamBarrier struct {
	streams.Stream[*httpclient.StreamEvent]
	observer streams.DeliveryAcknowledger
	c        *gin.Context
	receipt  chan string
}

func (s *codexCLIDownstreamBarrier) AcknowledgeDelivery() {
	s.observer.AcknowledgeDelivery()
	if !s.observer.DeliveryAcknowledged() {
		s.receipt <- "ack rejected"
		return
	}
	select {
	case <-s.c.Request.Context().Done():
		s.receipt <- "ack -> downstream-canceled -> barrier-release"
	case <-time.After(10 * time.Second):
		s.receipt <- "BLOCKED watchdog: CLI did not cancel held response"
	}
}

func (s *codexCLIDownstreamBarrier) DeliveryAcknowledged() bool {
	return s.observer.DeliveryAcknowledged()
}
