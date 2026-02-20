package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/google/uuid"
	"github.com/looplj/axonhub/axon/agent"
	axoncontext "github.com/looplj/axonhub/axon/context"
	"github.com/looplj/axonhub/axon/provider/anthropic"
	"github.com/looplj/axonhub/axon/tools"
)

const (
	defaultEndpoint          = "http://localhost:8090/agent/v1/graphql"
	defaultPollInterval      = 2 * time.Second
	defaultHeartbeatInterval = 10 * time.Second
	defaultThreadID          = "default"
	defaultMaxIterations     = 30
)

func main() {
	var (
		endpoint          = flag.String("endpoint", envOr("AXONCLAW_ENDPOINT", defaultEndpoint), "AxonHub Agent API GraphQL endpoint")
		baseURL           = flag.String("base-url", envOr("AXONCLAW_BASE_URL", ""), "AxonHub base URL (used for LLM calls); defaults to endpoint base")
		apiKey            = flag.String("api-key", envOr("AXONCLAW_API_KEY", ""), "Agent API key (type=agent)")
		agentID           = flag.String("agent-id", envOr("AXONCLAW_AGENT_ID", ""), "Agent ID (gid://axonhub/Agent/<id> or numeric)")
		modelOverride     = flag.String("model", envOr("AXONCLAW_MODEL", ""), "Optional model override when agent bootstrap doesn't specify one")
		threadID          = flag.String("thread", envOr("AXONCLAW_THREAD", defaultThreadID), "Thread ID to poll for messages")
		workspaceDir      = flag.String("workspace", envOr("AXONCLAW_WORKSPACE", ""), "Workspace directory (default: ./axonclaw-workspace)")
		configDirOverride = flag.String("config-dir", envOr("AXONCLAW_CONFIG_DIR", ""), "Config dir (default: ~/.axonclaw)")
		pollInterval      = flag.Duration("poll-interval", envOrDuration("AXONCLAW_POLL_INTERVAL", defaultPollInterval), "Polling interval for messages")
		heartbeatInterval = flag.Duration("heartbeat-interval", envOrDuration("AXONCLAW_HEARTBEAT_INTERVAL", defaultHeartbeatInterval), "Heartbeat interval")
		debug             = flag.Bool("debug", envOrBool("AXONCLAW_DEBUG", false), "Enable debug logging")
	)
	flag.Parse()

	if *apiKey == "" {
		fatalf("missing -api-key (or AXONCLAW_API_KEY)")
	}
	if *agentID == "" {
		fatalf("missing -agent-id (or AXONCLAW_AGENT_ID)")
	}
	if *threadID == "" {
		fatalf("missing -thread (or AXONCLAW_THREAD)")
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	if *debug {
		logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
			Level: slog.LevelDebug,
		}))
	}

	cfgDir, err := resolveConfigDir(*configDirOverride)
	if err != nil {
		fatalf("resolve config dir: %v", err)
	}
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		fatalf("create config dir: %v", err)
	}

	instanceID, err := loadOrCreateInstanceID(filepath.Join(cfgDir, "instance_id"))
	if err != nil {
		fatalf("load instance id: %v", err)
	}

	ws := *workspaceDir
	if ws == "" {
		ws = filepath.Join(mustGetwd(), "axonclaw-workspace")
	}
	ws, err = filepath.Abs(ws)
	if err != nil {
		fatalf("resolve workspace: %v", err)
	}
	if err := os.MkdirAll(ws, 0o755); err != nil {
		fatalf("create workspace: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	gid := normalizeAgentID(*agentID)

	gqlClient := newGQLClient(*endpoint, *apiKey, 30*time.Second)

	bootstrap, err := gqlClient.agentBootstrap(ctx, gid)
	if err != nil {
		fatalf("agentBootstrap failed: %v", err)
	}

	model := ""
	if bootstrap.Model != nil {
		model = strings.TrimSpace(*bootstrap.Model)
	}
	if model == "" {
		model = strings.TrimSpace(*modelOverride)
	}
	if model == "" {
		fatalf("missing model: agentBootstrap.model is empty and -model not set")
	}

	systemPrompt := buildSystemPrompt(bootstrap.SystemPrompt, bootstrap.Skills)

	llmBaseURL := strings.TrimRight(*baseURL, "/")
	if llmBaseURL == "" {
		llmBaseURL = strings.TrimRight(inferBaseURL(*endpoint), "/")
	}
	if llmBaseURL == "" {
		fatalf("cannot infer base URL; provide -base-url")
	}

	logger.Info("axonclaw starting",
		"agent_id", gid,
		"agent_name", bootstrap.AgentName,
		"instance_id", instanceID,
		"endpoint", *endpoint,
		"base_url", llmBaseURL,
		"model", model,
		"thread", *threadID,
		"workspace", ws,
	)

	provider := anthropic.New(llmBaseURL+"/anthropic", *apiKey)

	name := "axonclaw"
	platform := runtime.GOOS
	version := "dev"

	if _, err := gqlClient.registerAgentInstance(ctx, gid, instanceID, &name, &platform, &version); err != nil {
		fatalf("registerAgentInstance failed: %v", err)
	}

	threadWorkspace := filepath.Join(ws, "threads", *threadID)
	if err := os.MkdirAll(threadWorkspace, 0o755); err != nil {
		fatalf("create thread workspace: %v", err)
	}

	a := agent.New(agent.Config{
		Model:         model,
		MaxIterations: defaultMaxIterations,
		SystemPrompt:  systemPrompt,
	}, provider)
	registerToolsFromBootstrap(a, threadWorkspace, ws, cfgDir, bootstrap, logger)

	run(ctx, logger, gqlClient, a, gid, instanceID, *threadID, *pollInterval, *heartbeatInterval)
}

func run(
	ctx context.Context,
	logger *slog.Logger,
	gqlClient *gqlClient,
	a *agent.Agent,
	agentID string,
	instanceID string,
	threadID string,
	pollInterval time.Duration,
	heartbeatInterval time.Duration,
) {
	pollTicker := time.NewTicker(pollInterval)
	defer pollTicker.Stop()
	hbTicker := time.NewTicker(heartbeatInterval)
	defer hbTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.Info("axonclaw stopping", "reason", ctx.Err())
			return

		case <-hbTicker.C:
			ok, err := gqlClient.heartbeatAgentInstance(ctx, agentID, instanceID)
			if err != nil {
				logger.Warn("heartbeat failed", "error", err)
				continue
			}
			if !ok {
				logger.Warn("heartbeat returned false")
			}

		case <-pollTicker.C:
			msgs, err := gqlClient.pullAgentMessages(ctx, agentID, instanceID, threadID, nil, 50)
			if err != nil {
				logger.Warn("pullAgentMessages failed", "error", err)
				continue
			}
			if len(msgs) == 0 {
				continue
			}

			for _, msg := range msgs {
				if msg.Text == "" {
					logger.Debug("skip empty message", "message_id", msg.ID, "sequence", msg.Sequence)
					continue
				}

				reqCtx := axoncontext.WithThreadID(ctx, threadID)
				reqCtx = axoncontext.WithTraceID(reqCtx, uuid.New().String())
				resp, err := processAndCollect(reqCtx, a, msg.Text)
				if err != nil {
					logger.Warn("agent process failed", "error", err, "message_id", msg.ID, "sequence", msg.Sequence)
					continue
				}
				if strings.TrimSpace(resp) == "" {
					resp = "(no response)"
				}

				if _, err := gqlClient.pushAgentMessage(ctx, agentID, instanceID, threadID, resp); err != nil {
					logger.Warn("pushAgentMessage failed", "error", err, "message_id", msg.ID, "sequence", msg.Sequence)
					continue
				}

				if ok, err := gqlClient.ackAgentMessages(ctx, agentID, instanceID, []string{msg.ID}); err != nil {
					logger.Warn("ackAgentMessages failed", "error", err, "message_id", msg.ID, "sequence", msg.Sequence)
				} else if !ok {
					logger.Warn("ackAgentMessages returned false", "message_id", msg.ID, "sequence", msg.Sequence)
				}
			}
		}
	}
}

func processAndCollect(ctx context.Context, a *agent.Agent, text string) (string, error) {
	before := a.Messages()
	beforeLen := len(before)

	t := text
	if err := a.Process(ctx, agent.Content{Text: &t}); err != nil {
		return "", err
	}

	after := a.Messages()
	if len(after) <= beforeLen {
		return "", nil
	}

	last := ""
	for i := beforeLen; i < len(after); i++ {
		if after[i].Role == agent.RoleAssistant && after[i].Content != nil {
			last = after[i].Content.String()
		}
	}
	return last, nil
}

func registerToolsFromBootstrap(
	a *agent.Agent,
	threadWorkspace string,
	workspaceRoot string,
	configDir string,
	bootstrap agentBootstrap,
	logger *slog.Logger,
) {
	enabledBuiltin := map[string]bool{}
	for _, t := range bootstrap.BuiltinTools {
		if t.Name == "" {
			continue
		}
		if t.Enabled {
			enabledBuiltin[t.Name] = true
		}
	}

	// MVP: provide a sensible builtin set even if server didn't specify builtinTools.
	if len(enabledBuiltin) == 0 {
		for _, name := range []string{"Read", "Write", "Edit", "Bash", "Grep", "Glob", "Skill"} {
			enabledBuiltin[name] = true
		}
	}

	if enabledBuiltin["Read"] {
		a.RegisterTool(tools.NewReadTool(threadWorkspace, true))
	}
	if enabledBuiltin["Write"] {
		a.RegisterTool(tools.NewWriteTool(threadWorkspace, true))
	}
	if enabledBuiltin["Edit"] {
		a.RegisterTool(tools.NewEditTool(threadWorkspace, true))
	}
	if enabledBuiltin["Bash"] {
		a.RegisterTool(tools.NewBashTool(threadWorkspace, true))
	}
	if enabledBuiltin["Grep"] {
		a.RegisterTool(tools.NewGrepTool(threadWorkspace, true))
	}
	if enabledBuiltin["Glob"] {
		a.RegisterTool(tools.NewGlobTool(threadWorkspace, true))
	}
	if enabledBuiltin["Skill"] {
		a.RegisterTool(tools.NewSkillTool(filepath.Join(workspaceRoot, "skills"), filepath.Join(configDir, "skills")))
	}

	known := map[string]struct{}{}
	for name := range enabledBuiltin {
		known[name] = struct{}{}
	}
	for _, t := range bootstrap.Tools {
		if t.Name == "" {
			continue
		}
		if _, ok := known[t.Name]; ok {
			continue
		}

		def, err := convertRemoteToolDefinition(t)
		if err != nil {
			logger.Warn("skip invalid tool schema from bootstrap", "tool", t.Name, "error", err)
			continue
		}
		a.RegisterTool(&unimplementedTool{def: def})
	}
}

type unimplementedTool struct {
	def agent.ToolDefinition
}

func (t *unimplementedTool) Definition() agent.ToolDefinition { return t.def }

func (t *unimplementedTool) Execute(_ context.Context, _ json.RawMessage) agent.ToolResult {
	return agent.ToolResult{Error: fmt.Errorf("tool %q is not implemented in axonclaw", t.def.Name)}
}

func convertRemoteToolDefinition(in agentToolDefinition) (agent.ToolDefinition, error) {
	var schema jsonschema.Schema
	if len(in.Parameters) > 0 && string(in.Parameters) != "null" {
		if err := json.Unmarshal(in.Parameters, &schema); err != nil {
			return agent.ToolDefinition{}, err
		}
	}

	if schema.Type == "" {
		schema = jsonschema.Schema{
			Schema:               "https://json-schema.org/draft/2020-12/schema",
			Type:                 "object",
			AdditionalProperties: &jsonschema.Schema{},
		}
	}

	return agent.ToolDefinition{
		Name:        in.Name,
		Description: in.Description,
		Parameters:  schema,
	}, nil
}

func buildSystemPrompt(systemPrompt string, skills []agentSkillDefinition) string {
	var sb strings.Builder
	sb.WriteString(systemPrompt)

	for _, sk := range skills {
		if sk.Name == "" || sk.Content == nil || strings.TrimSpace(*sk.Content) == "" {
			continue
		}
		sb.WriteString("\n\n---\n\n")
		sb.WriteString("## Skill: ")
		sb.WriteString(sk.Name)
		sb.WriteString("\n\n")
		sb.WriteString(*sk.Content)
	}

	return sb.String()
}

func normalizeAgentID(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return ""
	}
	if strings.HasPrefix(v, "gid://") {
		return v
	}
	if n, err := strconv.Atoi(v); err == nil && n > 0 {
		return fmt.Sprintf("gid://axonhub/Agent/%d", n)
	}
	// Assume already an ID string accepted by GraphQL server.
	return v
}

func inferBaseURL(endpoint string) string {
	// http(s)://host/path -> http(s)://host
	if endpoint == "" {
		return ""
	}
	u := endpoint
	if i := strings.Index(u, "://"); i >= 0 {
		scheme := u[:i+3]
		rest := u[i+3:]
		if j := strings.IndexByte(rest, '/'); j >= 0 {
			return scheme + rest[:j]
		}
		return scheme + rest
	}
	return ""
}

func envOr(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func envOrBool(key string, fallback bool) bool {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	v = strings.ToLower(v)
	return v == "1" || v == "true" || v == "yes" || v == "on"
}

func envOrDuration(key string, fallback time.Duration) time.Duration {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return fallback
	}
	return d
}

func resolveConfigDir(override string) (string, error) {
	if strings.TrimSpace(override) != "" {
		return filepath.Abs(override)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".axonclaw"), nil
}

func loadOrCreateInstanceID(path string) (string, error) {
	if b, err := os.ReadFile(path); err == nil {
		id := strings.TrimSpace(string(b))
		if id != "" {
			return id, nil
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}

	id := uuid.New().String()
	if err := os.WriteFile(path, []byte(id+"\n"), 0o600); err != nil {
		return "", err
	}
	return id, nil
}

func mustGetwd() string {
	dir, err := os.Getwd()
	if err != nil {
		fatalf("getwd: %v", err)
	}
	return dir
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "Error: "+format+"\n", args...)
	os.Exit(1)
}

// --- GraphQL client (minimal) ---

type gqlClient struct {
	endpoint   string
	httpClient *http.Client
}

func newGQLClient(endpoint, apiKey string, timeout time.Duration) *gqlClient {
	httpClient := &http.Client{
		Timeout: timeout,
		Transport: &authHeaderTransport{
			apiKey: apiKey,
			base:   http.DefaultTransport,
		},
	}
	return &gqlClient{
		endpoint:   endpoint,
		httpClient: httpClient,
	}
}

type authHeaderTransport struct {
	apiKey string
	base   http.RoundTripper
}

func (t *authHeaderTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set("Authorization", "Bearer "+t.apiKey)
	return t.base.RoundTrip(req)
}

type gqlRequest struct {
	Query     string `json:"query"`
	Variables any    `json:"variables,omitempty"`
}

type gqlError struct {
	Message string `json:"message"`
}

type gqlResponse[T any] struct {
	Data   T         `json:"data"`
	Errors []gqlError `json:"errors,omitempty"`
}

func (c *gqlClient) do(ctx context.Context, query string, variables any, out any) error {
	payload, err := json.Marshal(gqlRequest{Query: query, Variables: variables})
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("http %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("decode graphql response: %w", err)
	}
	return nil
}

type agentBootstrap struct {
	AgentID      string               `json:"agentID"`
	AgentName    string               `json:"agentName"`
	Model        *string              `json:"model"`
	SystemPrompt string               `json:"systemPrompt"`
	Tools        []agentToolDefinition `json:"tools"`
	Skills       []agentSkillDefinition `json:"skills"`
	BuiltinTools []agentBuiltinTool   `json:"builtinTools"`
}

type agentToolDefinition struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
	Config      json.RawMessage `json:"config"`
}

type agentSkillDefinition struct {
	Name       string  `json:"name"`
	Content    *string `json:"content"`
	Entrypoint *string `json:"entrypoint"`
	Args       *string `json:"args"`
}

type agentBuiltinTool struct {
	Name    string          `json:"name"`
	Enabled bool            `json:"enabled"`
	Order   int             `json:"order"`
	Config  json.RawMessage `json:"config"`
}

func (c *gqlClient) agentBootstrap(ctx context.Context, agentID string) (agentBootstrap, error) {
	const q = `
query AgentBootstrap($agentID: ID!) {
  agentBootstrap(agentID: $agentID) {
    agentID
    agentName
    model
    systemPrompt
    tools { name description parameters config }
    skills { name content entrypoint args }
    builtinTools { name enabled order config }
  }
}`

	var resp gqlResponse[struct {
		AgentBootstrap agentBootstrap `json:"agentBootstrap"`
	}]

	if err := c.do(ctx, q, map[string]any{"agentID": agentID}, &resp); err != nil {
		return agentBootstrap{}, err
	}
	if len(resp.Errors) > 0 {
		return agentBootstrap{}, fmt.Errorf("graphql: %s", resp.Errors[0].Message)
	}
	return resp.Data.AgentBootstrap, nil
}

func (c *gqlClient) registerAgentInstance(ctx context.Context, agentID, instanceID string, name, platform, version *string) (string, error) {
	const q = `
mutation RegisterAgentInstance($input: RegisterAgentInstanceInput!) {
  registerAgentInstance(input: $input) {
    instanceID
  }
}`
	input := map[string]any{
		"agentID":    agentID,
		"instanceID": instanceID,
		"name":       name,
		"platform":   platform,
		"version":    version,
	}

	var resp gqlResponse[struct {
		RegisterAgentInstance struct {
			InstanceID string `json:"instanceID"`
		} `json:"registerAgentInstance"`
	}]
	if err := c.do(ctx, q, map[string]any{"input": input}, &resp); err != nil {
		return "", err
	}
	if len(resp.Errors) > 0 {
		return "", fmt.Errorf("graphql: %s", resp.Errors[0].Message)
	}
	return resp.Data.RegisterAgentInstance.InstanceID, nil
}

func (c *gqlClient) heartbeatAgentInstance(ctx context.Context, agentID, instanceID string) (bool, error) {
	const q = `
mutation HeartbeatAgentInstance($input: HeartbeatAgentInstanceInput!) {
  heartbeatAgentInstance(input: $input)
}`

	var resp gqlResponse[struct {
		HeartbeatAgentInstance bool `json:"heartbeatAgentInstance"`
	}]
	if err := c.do(ctx, q, map[string]any{
		"input": map[string]any{"agentID": agentID, "instanceID": instanceID},
	}, &resp); err != nil {
		return false, err
	}
	if len(resp.Errors) > 0 {
		return false, fmt.Errorf("graphql: %s", resp.Errors[0].Message)
	}
	return resp.Data.HeartbeatAgentInstance, nil
}

type agentMessage struct {
	ID       string `json:"id"`
	ThreadID string `json:"threadID"`
	Text     string `json:"text"`
	Sequence int    `json:"sequence"`
}

func (c *gqlClient) pullAgentMessages(ctx context.Context, agentID, instanceID, threadID string, afterSequence *int, limit int) ([]agentMessage, error) {
	const q = `
query PullAgentMessages($input: PullAgentMessagesInput!) {
  pullAgentMessages(input: $input) {
    id
    threadID
    text
    sequence
  }
}`

	input := map[string]any{
		"agentID":    agentID,
		"instanceID": instanceID,
		"threadID":   threadID,
		"limit":      limit,
	}
	if afterSequence != nil {
		input["afterSequence"] = *afterSequence
	}

	var resp gqlResponse[struct {
		PullAgentMessages []agentMessage `json:"pullAgentMessages"`
	}]
	if err := c.do(ctx, q, map[string]any{"input": input}, &resp); err != nil {
		return nil, err
	}
	if len(resp.Errors) > 0 {
		return nil, fmt.Errorf("graphql: %s", resp.Errors[0].Message)
	}
	return resp.Data.PullAgentMessages, nil
}

func (c *gqlClient) ackAgentMessages(ctx context.Context, agentID, instanceID string, messageIDs []string) (bool, error) {
	const q = `
mutation AckAgentMessages($input: AckAgentMessagesInput!) {
  ackAgentMessages(input: $input)
}`

	var resp gqlResponse[struct {
		AckAgentMessages bool `json:"ackAgentMessages"`
	}]

	if err := c.do(ctx, q, map[string]any{
		"input": map[string]any{
			"agentID":    agentID,
			"instanceID": instanceID,
			"messageIDs": messageIDs,
		},
	}, &resp); err != nil {
		return false, err
	}
	if len(resp.Errors) > 0 {
		return false, fmt.Errorf("graphql: %s", resp.Errors[0].Message)
	}
	return resp.Data.AckAgentMessages, nil
}

func (c *gqlClient) pushAgentMessage(ctx context.Context, agentID, instanceID, threadID, text string) (string, error) {
	const q = `
mutation PushAgentMessage($input: PushAgentMessageInput!) {
  pushAgentMessage(input: $input) { id }
}`

	var resp gqlResponse[struct {
		PushAgentMessage struct {
			ID string `json:"id"`
		} `json:"pushAgentMessage"`
	}]

	if err := c.do(ctx, q, map[string]any{
		"input": map[string]any{
			"agentID":    agentID,
			"instanceID": instanceID,
			"threadID":   threadID,
			"text":       text,
		},
	}, &resp); err != nil {
		return "", err
	}
	if len(resp.Errors) > 0 {
		return "", fmt.Errorf("graphql: %s", resp.Errors[0].Message)
	}
	return resp.Data.PushAgentMessage.ID, nil
}
