package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/samber/lo"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/looplj/axonhub/axon/agent"
)

const (
	FileName = "mcp.json"

	defaultConnectTimeout = 15 * time.Second
	defaultRequestTimeout = 2 * time.Minute
)

const (
	TransportStdio = "stdio"
	TransportHTTP  = "http"
	TransportSSE   = "sse"
)

type ServerConfig struct {
	Disabled       bool              `json:"disabled,omitempty"`
	Type           string            `json:"type,omitempty"`
	Command        string            `json:"command,omitempty"`
	Args           []string          `json:"args,omitempty"`
	Env            map[string]string `json:"env,omitempty"`
	URL            string            `json:"url,omitempty"`
	Headers        map[string]string `json:"headers,omitempty"`
	ToolPrefix     string            `json:"tool_prefix,omitempty"`
	RequestTimeout time.Duration     `json:"-"`
	ConnectTimeout time.Duration     `json:"-"`
}

func (c ServerConfig) IsEnabled() bool {
	return !c.Disabled
}

func (c ServerConfig) TransportType() string {
	t := strings.ToLower(strings.TrimSpace(c.Type))
	if t == "" {
		if strings.TrimSpace(c.URL) != "" {
			return TransportHTTP
		}

		return TransportStdio
	}

	return t
}

func (c ServerConfig) MarshalJSON() ([]byte, error) {
	type Alias ServerConfig

	aux := struct {
		Alias

		RequestTimeout string `json:"request_timeout,omitempty"`
		ConnectTimeout string `json:"connect_timeout,omitempty"`
	}{
		Alias: Alias(c),
	}
	if c.RequestTimeout > 0 {
		aux.RequestTimeout = c.RequestTimeout.String()
	}

	if c.ConnectTimeout > 0 {
		aux.ConnectTimeout = c.ConnectTimeout.String()
	}

	return json.Marshal(aux)
}

func (c *ServerConfig) UnmarshalJSON(data []byte) error {
	type Alias ServerConfig

	aux := struct {
		*Alias

		RequestTimeout string `json:"request_timeout,omitempty"`
		ConnectTimeout string `json:"connect_timeout,omitempty"`
	}{
		Alias: (*Alias)(c),
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	c.Command = strings.TrimSpace(c.Command)

	if aux.RequestTimeout != "" {
		d, err := time.ParseDuration(aux.RequestTimeout)
		if err != nil {
			return fmt.Errorf("parse request_timeout: %w", err)
		}

		c.RequestTimeout = d
	}

	if aux.ConnectTimeout != "" {
		d, err := time.ParseDuration(aux.ConnectTimeout)
		if err != nil {
			return fmt.Errorf("parse connect_timeout: %w", err)
		}

		c.ConnectTimeout = d
	}

	return nil
}

type ManagerOptions struct {
	Logger        *slog.Logger
	ConfigDir     string
	ClientName    string
	ClientVersion string
}

type Manager struct {
	logger        *slog.Logger
	configDir     string
	clientName    string
	clientVersion string
	sessions      []*namedSession
}

type namedSession struct {
	serverName string
	session    *mcpsdk.ClientSession
}

func NewManager(opts ManagerOptions) *Manager {
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}

	clientName := opts.ClientName
	if clientName == "" {
		clientName = "axon"
	}

	clientVersion := opts.ClientVersion
	if clientVersion == "" {
		clientVersion = "v1.0.0"
	}

	return &Manager{
		logger:        logger,
		configDir:     opts.ConfigDir,
		clientName:    clientName,
		clientVersion: clientVersion,
	}
}

func (m *Manager) Close() error {
	if m == nil {
		return nil
	}

	m.logger.Debug("closing mcp manager", "session_count", len(m.sessions))

	var errOut error

	for _, s := range m.sessions {
		if s == nil || s.session == nil {
			continue
		}

		m.logger.Debug("closing mcp server session", "server", s.serverName)

		if err := s.session.Close(); err != nil {
			m.logger.Debug("close mcp server session failed", "server", s.serverName, "error", err)
			errOut = errors.Join(errOut, fmt.Errorf("close mcp server %q: %w", s.serverName, err))
		}
	}

	m.sessions = nil
	m.logger.Debug("mcp manager closed")

	return errOut
}

func (m *Manager) RegisterTools(a *agent.Agent, workspace string, known map[string]struct{}) {
	m.logger.Debug("registering mcp tools", "config_path", m.ConfigPath(), "workspace", workspace)

	servers, err := m.LoadServers()
	if err != nil {
		m.logger.Warn("load mcp config failed", "path", m.ConfigPath(), "error", err)
		return
	}

	if len(servers) == 0 {
		m.logger.Debug("no mcp servers configured")
		return
	}

	serverNames := lo.Keys(servers)
	sort.Strings(serverNames)
	m.logger.Debug("found mcp servers in config", "count", len(serverNames), "servers", serverNames)

	for _, serverName := range serverNames {
		serverCfg := servers[serverName]
		if !serverCfg.IsEnabled() {
			m.logger.Debug("mcp server disabled, skipping", "server", serverName)
			continue
		}

		m.logger.Debug("connecting to mcp server",
			"server", serverName,
			"transport", serverCfg.TransportType(),
			"command", serverCfg.Command,
			"args", serverCfg.Args,
			"url", serverCfg.URL)

		connectTimeout := serverCfg.ConnectTimeout
		if connectTimeout <= 0 {
			connectTimeout = defaultConnectTimeout
		}

		requestTimeout := serverCfg.RequestTimeout
		if requestTimeout <= 0 {
			requestTimeout = defaultRequestTimeout
		}

		session, err := m.connectServer(serverName, serverCfg, workspace, connectTimeout)
		if err != nil {
			m.logger.Warn("connect mcp server failed", "server", serverName, "error", err)
			continue
		}

		m.logger.Debug("mcp server connected, listing tools", "server", serverName)

		tools, err := listTools(session, connectTimeout)
		if err != nil {
			_ = session.Close()

			m.logger.Warn("list mcp tools failed", "server", serverName, "error", err)

			continue
		}

		m.logger.Debug("mcp server tools listed", "server", serverName, "tool_count", len(tools))

		prefix := strings.TrimSpace(serverCfg.ToolPrefix)
		if prefix == "" {
			prefix = sanitizeToolNameSegment(serverName) + "__"
		}

		m.logger.Debug("registering mcp tools with prefix", "server", serverName, "prefix", prefix)

		registeredCount := 0

		for _, tool := range tools {
			if tool == nil || strings.TrimSpace(tool.Name) == "" {
				m.logger.Debug("skipping empty mcp tool", "server", serverName)
				continue
			}

			localName := prefix + sanitizeToolNameSegment(tool.Name)
			if _, exists := known[localName]; exists {
				m.logger.Warn("skip mcp tool due to name conflict", "server", serverName, "tool", tool.Name, "local_name", localName)
				continue
			}

			def, err := convertToolDefinition(localName, tool)
			if err != nil {
				m.logger.Warn("skip invalid mcp tool schema", "server", serverName, "tool", tool.Name, "error", err)
				continue
			}

			m.logger.Debug("registering mcp tool",
				"server", serverName,
				"remote_tool", tool.Name,
				"local_name", localName,
				"description", def.Description)

			a.RegisterTool(&mcpTool{
				def:            def,
				remoteToolName: tool.Name,
				serverName:     serverName,
				session:        session,
				timeout:        requestTimeout,
				logger:         m.logger,
			})

			known[localName] = struct{}{}
			registeredCount++
		}

		if registeredCount == 0 {
			_ = session.Close()

			m.logger.Warn("mcp server connected but no tools were registered", "server", serverName)

			continue
		}

		m.sessions = append(m.sessions, &namedSession{
			serverName: serverName,
			session:    session,
		})
		m.logger.Info("mcp server connected", "server", serverName, "transport", serverCfg.TransportType(), "tools", registeredCount)
	}
}

func (m *Manager) connectServer(serverName string, cfg ServerConfig, workspace string, timeout time.Duration) (*mcpsdk.ClientSession, error) {
	mcpClient := mcpsdk.NewClient(&mcpsdk.Implementation{
		Name:    m.clientName,
		Version: m.clientVersion,
	}, &mcpsdk.ClientOptions{
		ToolListChangedHandler: func(_ context.Context, _ *mcpsdk.ToolListChangedRequest) {
			m.logger.Info("mcp tools changed on server; restart to reload mcp tools", "server", serverName)
		},
	})

	connectCtx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	transportType := cfg.TransportType()
	switch transportType {
	case TransportStdio:
		return m.connectStdio(connectCtx, mcpClient, cfg, workspace)
	case TransportHTTP:
		return m.connectHTTP(connectCtx, mcpClient, cfg)
	case TransportSSE:
		return m.connectSSE(connectCtx, mcpClient, cfg)
	default:
		return nil, fmt.Errorf("unsupported transport type: %s", transportType)
	}
}

func (m *Manager) connectStdio(ctx context.Context, client *mcpsdk.Client, cfg ServerConfig, workspace string) (*mcpsdk.ClientSession, error) {
	command := strings.TrimSpace(cfg.Command)
	if command == "" {
		return nil, fmt.Errorf("command is required for stdio transport")
	}

	m.logger.Debug("connecting via stdio transport",
		"command", command,
		"args", cfg.Args,
		"workspace", workspace,
		"env_count", len(cfg.Env))

	cmd := exec.CommandContext(ctx, command, cfg.Args...)
	cmd.Dir = workspace
	cmd.Env = mergeEnvs(cfg.Env)

	session, err := client.Connect(ctx, &mcpsdk.CommandTransport{Command: cmd}, nil)
	if err != nil {
		m.logger.Debug("stdio connect failed", "command", command, "error", err)
		return nil, fmt.Errorf("stdio connect failed: %w", err)
	}

	m.logger.Debug("stdio connect succeeded", "command", command)

	return session, nil
}

func (m *Manager) connectHTTP(ctx context.Context, client *mcpsdk.Client, cfg ServerConfig) (*mcpsdk.ClientSession, error) {
	url := strings.TrimSpace(cfg.URL)
	if url == "" {
		return nil, fmt.Errorf("url is required for http transport")
	}

	m.logger.Debug("connecting via http transport", "url", url, "headers_count", len(cfg.Headers))

	transport := &mcpsdk.StreamableClientTransport{
		Endpoint: url,
	}
	if len(cfg.Headers) > 0 {
		transport.HTTPClient = &http.Client{
			Transport: &headerTransport{
				base:    http.DefaultTransport,
				headers: cfg.Headers,
			},
		}
	}

	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		m.logger.Debug("http connect failed", "url", url, "error", err)
		return nil, fmt.Errorf("http connect failed: %w", err)
	}

	m.logger.Debug("http connect succeeded", "url", url)

	return session, nil
}

func (m *Manager) connectSSE(ctx context.Context, client *mcpsdk.Client, cfg ServerConfig) (*mcpsdk.ClientSession, error) {
	url := strings.TrimSpace(cfg.URL)
	if url == "" {
		return nil, fmt.Errorf("url is required for sse transport")
	}

	m.logger.Debug("connecting via sse transport", "url", url, "headers_count", len(cfg.Headers))

	transport := &mcpsdk.SSEClientTransport{
		Endpoint: url,
	}
	if len(cfg.Headers) > 0 {
		transport.HTTPClient = &http.Client{
			Transport: &headerTransport{
				base:    http.DefaultTransport,
				headers: cfg.Headers,
			},
		}
	}

	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		m.logger.Debug("sse connect failed", "url", url, "error", err)
		return nil, fmt.Errorf("sse connect failed: %w", err)
	}

	m.logger.Debug("sse connect succeeded", "url", url)

	return session, nil
}

type headerTransport struct {
	base    http.RoundTripper
	headers map[string]string
}

func (t *headerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	for key, value := range t.headers {
		req.Header.Set(key, os.ExpandEnv(value))
	}

	return t.base.RoundTrip(req)
}

type Config struct {
	McpServers map[string]ServerConfig `json:"mcpServers"`
}

func (m *Manager) ConfigPath() string {
	return filepath.Join(m.configDir, FileName)
}

func (m *Manager) LoadServers() (map[string]ServerConfig, error) {
	path := m.ConfigPath()
	m.logger.Debug("loading mcp servers config", "path", path)

	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			m.logger.Debug("mcp config file not found", "path", path)
			return map[string]ServerConfig{}, nil
		}

		m.logger.Debug("failed to read mcp config file", "path", path, "error", err)

		return nil, err
	}

	if strings.TrimSpace(string(b)) == "" {
		m.logger.Debug("mcp config file is empty", "path", path)
		return map[string]ServerConfig{}, nil
	}

	var doc Config
	if err := json.Unmarshal(b, &doc); err != nil {
		m.logger.Debug("failed to parse mcp config", "path", path, "error", err)
		return nil, fmt.Errorf("parse mcp config %s: %w", path, err)
	}

	m.logger.Debug("loaded mcp servers config", "path", path, "server_count", len(doc.McpServers))

	return doc.McpServers, nil
}

func (m *Manager) SaveServers(servers map[string]ServerConfig) error {
	doc := Config{McpServers: servers}

	b, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal mcp config: %w", err)
	}

	b = append(b, '\n')

	if err := os.MkdirAll(m.configDir, 0o755); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}

	return os.WriteFile(m.ConfigPath(), b, 0o600)
}

func listTools(session *mcpsdk.ClientSession, timeout time.Duration) ([]*mcpsdk.Tool, error) {
	listCtx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	tools := make([]*mcpsdk.Tool, 0)

	for tool, err := range session.Tools(listCtx, nil) {
		if err != nil {
			return nil, err
		}

		tools = append(tools, tool)
	}

	return tools, nil
}

func mergeEnvs(extra map[string]string) []string {
	out := append([]string{}, os.Environ()...)
	if len(extra) == 0 {
		return out
	}

	keys := lo.Keys(extra)
	sort.Strings(keys)

	for _, k := range keys {
		key := strings.TrimSpace(k)
		if key == "" {
			continue
		}

		value := os.ExpandEnv(extra[k])
		out = append(out, key+"="+value)
	}

	return out
}

func sanitizeToolNameSegment(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "mcp"
	}

	var b strings.Builder

	for _, r := range value {
		if (r >= 'a' && r <= 'z') ||
			(r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') ||
			r == '_' || r == '-' || r == '.' {
			b.WriteRune(r)
			continue
		}

		b.WriteRune('_')
	}

	name := strings.Trim(b.String(), "_")
	if name == "" {
		return "mcp"
	}

	return name
}

func convertToolDefinition(localName string, in *mcpsdk.Tool) (agent.ToolDefinition, error) {
	schema := jsonschema.Schema{}

	if in.InputSchema != nil {
		raw, err := json.Marshal(in.InputSchema)
		if err != nil {
			return agent.ToolDefinition{}, err
		}

		if len(raw) > 0 && string(raw) != "null" {
			if err := json.Unmarshal(raw, &schema); err != nil {
				return agent.ToolDefinition{}, err
			}
		}
	}

	if schema.Type == "" {
		schema = jsonschema.Schema{
			Schema:               "https://json-schema.org/draft/2020-12/schema",
			Type:                 "object",
			AdditionalProperties: &jsonschema.Schema{},
		}
	}

	desc := strings.TrimSpace(in.Description)
	if desc == "" {
		desc = fmt.Sprintf("MCP tool %q", in.Name)
	}

	return agent.ToolDefinition{
		Name:        localName,
		Description: desc,
		Parameters:  schema,
	}, nil
}

type mcpTool struct {
	def            agent.ToolDefinition
	remoteToolName string
	serverName     string
	session        *mcpsdk.ClientSession
	timeout        time.Duration
	logger         *slog.Logger
}

func (t *mcpTool) Definition() agent.ToolDefinition {
	return t.def
}

func (t *mcpTool) Execute(ctx context.Context, arguments json.RawMessage) agent.ToolResult {
	if t.logger != nil {
		t.logger.Debug("executing mcp tool",
			"server", t.serverName,
			"remote_tool", t.remoteToolName,
			"local_name", t.def.Name,
			"arguments", string(arguments))
	}

	if t.session == nil {
		if t.logger != nil {
			t.logger.Debug("mcp tool execution failed: session is nil", "server", t.serverName, "tool", t.remoteToolName)
		}

		return agent.ToolResult{Error: fmt.Errorf("mcp server %q is not connected", t.serverName)}
	}

	callCtx := ctx

	cancel := func() {}
	if t.timeout > 0 {
		callCtx, cancel = context.WithTimeout(ctx, t.timeout)
	}
	defer cancel()

	args := map[string]any{}
	if len(arguments) > 0 {
		if err := json.Unmarshal(arguments, &args); err != nil {
			if t.logger != nil {
				t.logger.Debug("mcp tool arguments parse failed", "server", t.serverName, "tool", t.remoteToolName, "error", err)
			}

			return agent.ToolResult{Error: fmt.Errorf("invalid arguments: %w", err)}
		}
	}

	result, err := t.session.CallTool(callCtx, &mcpsdk.CallToolParams{
		Name:      t.remoteToolName,
		Arguments: args,
	})
	if err != nil {
		if t.logger != nil {
			t.logger.Debug("mcp tool call failed", "server", t.serverName, "tool", t.remoteToolName, "error", err)
		}

		return agent.ToolResult{Error: fmt.Errorf("mcp call %q on %q failed: %w", t.remoteToolName, t.serverName, err)}
	}

	text := callToolResultToText(result)
	if result.IsError {
		if t.logger != nil {
			t.logger.Debug("mcp tool returned error", "server", t.serverName, "tool", t.remoteToolName, "response", text)
		}

		return agent.ToolResult{Error: fmt.Errorf("mcp tool %q returned error: %s", t.remoteToolName, text)}
	}

	if t.logger != nil {
		t.logger.Debug("mcp tool execution succeeded",
			"server", t.serverName,
			"tool", t.remoteToolName,
			"response_length", len(text))
	}

	return agent.ToolResult{
		Content: agent.Content{Text: &text},
	}
}

func callToolResultToText(result *mcpsdk.CallToolResult) string {
	if result == nil {
		return "{}"
	}

	parts := make([]string, 0, len(result.Content)+1)
	for _, c := range result.Content {
		if c == nil {
			continue
		}

		switch v := c.(type) {
		case *mcpsdk.TextContent:
			if strings.TrimSpace(v.Text) != "" {
				parts = append(parts, v.Text)
			}
		default:
			raw, err := json.Marshal(v)
			if err != nil {
				parts = append(parts, fmt.Sprintf("%v", v))
			} else {
				parts = append(parts, string(raw))
			}
		}
	}

	if result.StructuredContent != nil {
		raw, err := json.Marshal(result.StructuredContent)
		if err == nil && string(raw) != "null" && string(raw) != "{}" {
			parts = append(parts, string(raw))
		}
	}

	if len(parts) == 0 {
		return "{}"
	}

	return strings.Join(parts, "\n")
}
