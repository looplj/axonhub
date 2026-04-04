package claw

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/looplj/axonhub/axon/agent"
	"github.com/looplj/axonhub/axon/bus"

	axoncontext "github.com/looplj/axonhub/axon/context"
)

func AppendArchiveMessage(ctx context.Context, dir string, msg agent.Message) error {
	threadID := resolveArchiveThreadID(ctx)
	if threadID == "" {
		threadID = "unknown-thread"
	}

	archiveDir := filepath.Join(dir, "archives")
	if err := os.MkdirAll(archiveDir, 0o755); err != nil {
		return fmt.Errorf("create archives directory: %w", err)
	}

	path := filepath.Join(
		archiveDir,
		fmt.Sprintf("%s_%s.md", time.Now().Format("2006-01-02"), sanitizeArchiveThreadID(threadID)),
	)

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open archive file: %w", err)
	}
	defer f.Close()

	if _, err := f.WriteString(RenderMessage(time.Now().UTC(), msg, MessageRenderOptions{TimePrefix: true})); err != nil {
		return fmt.Errorf("append archive message: %w", err)
	}

	return nil
}

func resolveArchiveThreadID(ctx context.Context) string {
	if meta, ok := bus.MetadataFromContext(ctx); ok && strings.TrimSpace(meta.ThreadID) != "" {
		return strings.TrimSpace(meta.ThreadID)
	}

	return strings.TrimSpace(axoncontext.ThreadID(ctx))
}

func sanitizeArchiveThreadID(threadID string) string {
	replacer := strings.NewReplacer("/", "_", "\\", "_", ":", "_", " ", "_")

	cleaned := replacer.Replace(strings.TrimSpace(threadID))
	if cleaned == "" {
		return "unknown-thread"
	}

	return cleaned
}
