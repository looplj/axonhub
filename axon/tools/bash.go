package tools

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/google/jsonschema-go/jsonschema"

	_ "embed"

	"github.com/looplj/axonhub/axon/agent"
)

//go:embed bash.md
var bashDescription string

var denyPatterns = []*regexp.Regexp{
	// Dangerous file operations
	regexp.MustCompile(`\brm\s+(-[a-zA-Z]*f[a-zA-Z]*\s+)?/`),
	regexp.MustCompile(`\brm\s+-rf\b`),
	regexp.MustCompile(`\bmkfs\b`),
	regexp.MustCompile(`\bformat\b`),
	regexp.MustCompile(`\bdd\s+.*of=/`),
	regexp.MustCompile(`>\s*/dev/sda`),
	regexp.MustCompile(`>\s*/dev/hda`),
	regexp.MustCompile(`>\s*/dev/nvme`),

	// System control
	regexp.MustCompile(`\bshutdown\b`),
	regexp.MustCompile(`\breboot\b`),
	regexp.MustCompile(`\binit\s+0\b`),
	regexp.MustCompile(`\bhalt\b`),
	regexp.MustCompile(`\bpoweroff\b`),
	regexp.MustCompile(`\bsystemctl\s+(poweroff|reboot|halt)\b`),

	// Fork bomb and resource exhaustion
	regexp.MustCompile(`:\(\)\s*\{\s*:\|:\s*&\s*\}\s*;`),

	// Privilege escalation
	regexp.MustCompile(`\bsudo\b`),
	regexp.MustCompile(`\bsu\s+-`),
	regexp.MustCompile(`\bsu\s+root\b`),
	regexp.MustCompile(`\bchmod\s+.*\+s\b`),
	regexp.MustCompile(`\bchown\s+root\b`),

	// Sensitive file access
	regexp.MustCompile(`\bcat\s+.*(/etc/shadow|/etc/passwd)\b`),
	regexp.MustCompile(`\b(vi|vim|nano|emacs)\s+.*/etc/`),

	// Network attacks
	regexp.MustCompile(`\biptables\s+-F\b`),
	regexp.MustCompile(`\bufw\s+disable\b`),

	// Dangerous commands
	regexp.MustCompile(`\b:(\s*)>\s*/`),
	regexp.MustCompile(`\btruncate\s+.*-s\s*0\s+/`),
	regexp.MustCompile(`\bshred\b`),
	regexp.MustCompile(`\bwipefs\b`),

	// Crypto mining / reverse shell patterns
	regexp.MustCompile(`\bcurl\s+.*\|\s*(ba)?sh\b`),
	regexp.MustCompile(`\bwget\s+.*\|\s*(ba)?sh\b`),
	regexp.MustCompile(`\bnc\s+-[a-z]*e\b`),
	regexp.MustCompile(`/dev/tcp/`),
}

// rtkRewritablePrefixes lists command prefixes that rtk is known to support rewriting.
// Commands not starting with any of these prefixes skip the rtk rewrite call entirely.
var rtkRewritablePrefixes = []string{
	"git", "gh",
	"cat", "head", "tail",
	"rg", "grep",
	"ls", "find",
	"cargo", "go",
	"npm", "pnpm", "pip",
	"vitest", "jest", "pytest",
	"tsc", "eslint", "biome",
	"prettier", "playwright", "prisma",
	"ruff", "golangci-lint",
	"docker", "kubectl",
	"curl", "wget",
}

const (
	bashTimeout       = 60 * time.Second
	maxOutputLen      = 10000
	rtkMinMajor       = 0
	rtkMinMinor       = 23
	rtkRewriteTimeout = 3 * time.Second
)

type BashTool struct {
	workingDir string
	restrict   bool
	rtkPath    string
}

func NewBashTool(workingDir string, restrict bool, rtkAware bool) *BashTool {
	t := &BashTool{
		workingDir: workingDir,
		restrict:   restrict,
	}
	if rtkAware {
		t.rtkPath = detectRtk()
	}

	return t
}

// detectRtk checks if rtk is installed and meets the minimum version requirement.
func detectRtk() string {
	path, err := exec.LookPath("rtk")
	if err != nil {
		return ""
	}

	out, err := exec.CommandContext(context.Background(), path, "--version").Output()
	if err != nil {
		return ""
	}

	re := regexp.MustCompile(`(\d+)\.(\d+)\.\d+`)

	matches := re.FindStringSubmatch(string(out))
	if len(matches) < 3 {
		return ""
	}

	major, _ := strconv.Atoi(matches[1])

	minor, _ := strconv.Atoi(matches[2])
	if major < rtkMinMajor || (major == rtkMinMajor && minor < rtkMinMinor) {
		return ""
	}

	return path
}

// rtkRewrite attempts to rewrite a command via rtk. Returns the original command on any failure.
func (t *BashTool) rtkRewrite(command string) string {
	if t.rtkPath == "" {
		return command
	}

	// Extract the leading command name to check against the whitelist.
	cmdName := strings.Fields(command)
	if len(cmdName) == 0 {
		return command
	}

	leading := cmdName[0]

	rewritable := slices.Contains(rtkRewritablePrefixes, leading)

	if !rewritable {
		return command
	}

	ctx, cancel := context.WithTimeout(context.Background(), rtkRewriteTimeout)
	defer cancel()

	out, err := exec.CommandContext(ctx, t.rtkPath, "rewrite", command).Output()
	if err != nil {
		return command
	}

	rewritten := strings.TrimSpace(string(out))
	if rewritten == "" || rewritten == command {
		return command
	}

	// Re-check deny patterns on the rewritten command.
	for _, p := range denyPatterns {
		if p.MatchString(rewritten) {
			return command
		}
	}

	return rewritten
}

type bashInput struct {
	Command string `json:"command"`
	Cwd     string `json:"cwd,omitempty"`
}

var bashParameters = jsonschema.Schema{
	Schema: "https://json-schema.org/draft/2020-12/schema",
	Type:   "object",
	Properties: map[string]*jsonschema.Schema{
		"command": {
			Type:        "string",
			MinLength:   new(1),
			Description: "The shell command to execute",
		},
		"cwd": {
			Type:        "string",
			Description: "Working directory for the command",
		},
	},
	Required: []string{"command"},
}

func (t *BashTool) Definition() agent.ToolDefinition {
	return agent.ToolDefinition{
		Name:        "Bash",
		Description: bashDescription,
		Parameters:  bashParameters,
	}
}

func (t *BashTool) Execute(ctx context.Context, input bashInput) agent.ToolResult {
	for _, p := range denyPatterns {
		if p.MatchString(input.Command) {
			return ErrorResult(fmt.Errorf("command denied by safety filter: %s", input.Command))
		}
	}

	cwd := t.workingDir
	if input.Cwd != "" {
		resolved, err := validatePath(input.Cwd, t.workingDir, t.restrict)
		if err != nil {
			return ErrorResult(err)
		}
		cwd = resolved
	}

	command := t.rtkRewrite(input.Command)

	ctx, cancel := context.WithTimeout(ctx, bashTimeout)
	defer cancel()

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(ctx, "powershell", "-Command", command)
	} else {
		cmd = exec.CommandContext(ctx, "sh", "-c", command)
	}
	cmd.Dir = cwd

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	var sb strings.Builder
	if stdout.Len() > 0 {
		sb.WriteString(truncate(stdout.String(), maxOutputLen))
	}
	if stderr.Len() > 0 {
		if sb.Len() > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString("STDERR:\n")
		sb.WriteString(truncate(stderr.String(), maxOutputLen))
	}

	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return ErrorResult(fmt.Errorf("command timed out after %s", bashTimeout))
		}
		if sb.Len() > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString(fmt.Sprintf("Exit error: %s", err))
	}

	if sb.Len() == 0 {
		return TextResult("(no output)")
	}

	return TextResult(sb.String())
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "\n... (truncated)"
}
