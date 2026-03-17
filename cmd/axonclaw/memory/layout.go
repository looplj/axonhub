package memory

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/looplj/axonhub/cmd/axonclaw/conf"
)

const longTermMemoryFileName = "MEMORY.md"

type Layout struct {
	workspace string
}

func NewLayout(workspace string) Layout {
	return Layout{workspace: workspace}
}

func (l Layout) Workspace() string {
	return l.workspace
}

func (l Layout) RootDir() string {
	return filepath.Join(l.workspace, conf.DefaultDir)
}

func (l Layout) LongTermPath() string {
	return filepath.Join(l.RootDir(), longTermMemoryFileName)
}

func (l Layout) DailyDir() string {
	return filepath.Join(l.RootDir(), "memory")
}

func (l Layout) DailyPath(date time.Time) string {
	return filepath.Join(l.DailyDir(), date.Format("2006-01-02")+".md")
}

func (l Layout) TodayPath() string {
	return l.DailyPath(time.Now())
}

func (l Layout) CacheDir() string {
	return filepath.Join(l.RootDir(), "cache")
}

func (l Layout) IndexPath() string {
	return filepath.Join(l.CacheDir(), "memory.sqlite")
}

func (l Layout) PluginsDir() string {
	return filepath.Join(l.RootDir(), "plugins")
}

func (l Layout) MemoryFiles() ([]string, error) {
	var files []string

	if _, err := os.Stat(l.LongTermPath()); err == nil {
		files = append(files, l.LongTermPath())
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("stat long-term memory: %w", err)
	}

	entries, err := os.ReadDir(l.DailyDir())
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("read daily memory directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".md" {
			continue
		}
		files = append(files, filepath.Join(l.DailyDir(), entry.Name()))
	}

	sort.Strings(files)

	return files, nil
}

func (l Layout) DisplayPath(path string) string {
	rel, err := filepath.Rel(l.workspace, path)
	if err != nil {
		return filepath.ToSlash(path)
	}

	return filepath.ToSlash(rel)
}

func (l Layout) ScopeForPath(path string) string {
	if filepath.Clean(path) == filepath.Clean(l.LongTermPath()) {
		return "longterm"
	}

	if strings.HasPrefix(filepath.Clean(path), filepath.Clean(l.DailyDir())+string(filepath.Separator)) {
		return "daily"
	}

	return "memory"
}
