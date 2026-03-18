package prompts

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	SoulFileName      = "SOUL.md"
	IdentityFileName  = "IDENTITY.md"
	UserFileName      = "USER.md"
	AgentsFileName    = "AGENTS.md"
	HeartbeatFileName = "HEARTBEAT.md"
	MemoryFileName    = "MEMORY.md"
	MemoryDirName     = "memory"
)

type Bootstrap struct {
	Soul      MarkdownFile
	Identity  MarkdownFile
	User      MarkdownFile
	System    MarkdownFile
	Heartbeat MarkdownFile
	Memory    MarkdownFile   // MEMORY.md long-term memory
	DailyLogs []MarkdownFile // Recent daily logs (today, yesterday)
}

type MarkdownFile struct {
	Content string
	Path    string
}

func (f MarkdownFile) IsEmpty() bool {
	return strings.TrimSpace(f.Content) == ""
}

type InitParams struct {
	Env                PromptEnv
	ServerSystemPrompt string
}

func Load(configDir string, initParams *InitParams) (*Bootstrap, error) {
	soul, err := LoadFile(configDir, SoulFileName)
	if err != nil {
		return nil, fmt.Errorf("load %s: %w", SoulFileName, err)
	}

	identity, err := LoadFile(configDir, IdentityFileName)
	if err != nil {
		return nil, fmt.Errorf("load %s: %w", IdentityFileName, err)
	}

	user, err := LoadFile(configDir, UserFileName)
	if err != nil {
		return nil, fmt.Errorf("load %s: %w", UserFileName, err)
	}

	system, err := LoadFile(configDir, AgentsFileName)
	if err != nil {
		return nil, fmt.Errorf("load %s: %w", AgentsFileName, err)
	}

	heartbeat, err := LoadFile(configDir, HeartbeatFileName)
	if err != nil {
		return nil, fmt.Errorf("load %s: %w", HeartbeatFileName, err)
	}

	memoryFile, err := LoadFile(configDir, MemoryFileName)
	if err != nil {
		return nil, fmt.Errorf("load %s: %w", MemoryFileName, err)
	}

	dailyLogs := loadRecentDailyLogs(configDir)

	boot := &Bootstrap{
		Soul:      *soul,
		Identity:  *identity,
		User:      *user,
		System:    *system,
		Heartbeat: *heartbeat,
		Memory:    *memoryFile,
		DailyLogs: dailyLogs,
	}

	if initParams != nil {
		if err := initPromptFiles(configDir, boot, initParams); err != nil {
			return nil, err
		}
	}

	return boot, nil
}

func initPromptFiles(configDir string, boot *Bootstrap, params *InitParams) error {
	if boot.Identity.IsEmpty() {
		content, err := RenderTemplate(DefaultIdentityTemplate, params.Env)
		if err != nil {
			return fmt.Errorf("render identity template: %w", err)
		}

		if err := SaveFile(configDir, IdentityFileName, content); err != nil {
			return fmt.Errorf("save identity: %w", err)
		}

		boot.Identity.Content = content
	}

	if boot.User.IsEmpty() {
		content, err := RenderTemplate(DefaultUserTemplate, params.Env)
		if err != nil {
			return fmt.Errorf("render user template: %w", err)
		}

		if err := SaveFile(configDir, UserFileName, content); err != nil {
			return fmt.Errorf("save user: %w", err)
		}

		boot.User.Content = content
	}

	if boot.Soul.IsEmpty() {
		if err := SaveFile(configDir, SoulFileName, DefaultSoulTemplate); err != nil {
			return fmt.Errorf("save soul: %w", err)
		}

		boot.Soul.Content = DefaultSoulTemplate
	}

	if boot.System.IsEmpty() {
		systemTemplate := params.ServerSystemPrompt
		if strings.TrimSpace(systemTemplate) == "" {
			systemTemplate = DefaultSystemTemplate
		}

		rendered, err := RenderTemplate(systemTemplate, params.Env)
		if err != nil {
			return fmt.Errorf("render system prompt: %w", err)
		}

		if err := SaveFile(configDir, AgentsFileName, rendered); err != nil {
			return fmt.Errorf("save system: %w", err)
		}

		boot.System.Content = rendered
	}

	if boot.Heartbeat.IsEmpty() {
		if err := SaveFile(configDir, HeartbeatFileName, DefaultHeartbeatTemplate); err != nil {
			return fmt.Errorf("save heartbeat: %w", err)
		}

		boot.Heartbeat.Content = DefaultHeartbeatTemplate
	}

	return nil
}

func LoadFile(configDir, name string) (*MarkdownFile, error) {
	path := filepath.Join(configDir, name)

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &MarkdownFile{Path: path}, nil
		}

		return nil, err
	}

	return &MarkdownFile{
		Content: strings.TrimSpace(string(data)),
		Path:    path,
	}, nil
}

func InitFile(configDir, name, template string) (string, error) {
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		return "", fmt.Errorf("create config directory: %w", err)
	}

	path := filepath.Join(configDir, name)
	if _, err := os.Stat(path); err == nil {
		return "", fmt.Errorf("bootstrap file already exists: %s", path)
	}

	if err := os.WriteFile(path, []byte(template), 0o600); err != nil {
		return "", fmt.Errorf("write %s: %w", name, err)
	}

	return path, nil
}

func SaveFile(configDir, name, content string) error {
	//nolint:gosec //Checked.
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}

	path := filepath.Join(configDir, name)

	return os.WriteFile(path, []byte(content), 0o600)
}

func Path(configDir, name string) string {
	return filepath.Join(configDir, name)
}

// loadRecentDailyLogs loads today's and yesterday's daily log files.
func loadRecentDailyLogs(configDir string) []MarkdownFile {
	memDir := filepath.Join(configDir, MemoryDirName)
	now := time.Now()

	dates := []time.Time{now, now.AddDate(0, 0, -1)}

	var logs []MarkdownFile

	for _, d := range dates {
		name := d.Format("2006-01-02") + ".md"
		path := filepath.Join(memDir, name)

		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		content := strings.TrimSpace(string(data))
		if content != "" {
			logs = append(logs, MarkdownFile{
				Content: content,
				Path:    path,
			})
		}
	}

	return logs
}
