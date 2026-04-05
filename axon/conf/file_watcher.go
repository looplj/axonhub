package conf

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

type FileWatcher struct {
	paths    []string
	debounce time.Duration
}

type FileChange struct {
	Path string
	At   time.Time
}

func NewFileWatcher(paths []string, debounce time.Duration) *FileWatcher {
	if debounce <= 0 {
		debounce = 200 * time.Millisecond
	}
	return &FileWatcher{paths: paths, debounce: debounce}
}

func (w *FileWatcher) Start(ctx context.Context) (<-chan FileChange, func() error, error) {
	fw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, nil, err
	}

	dirs := map[string]struct{}{}
	names := map[string]struct{}{}
	for _, p := range w.paths {
		if p == "" {
			continue
		}
		abs, err := filepath.Abs(p)
		if err != nil {
			fw.Close()
			return nil, nil, fmt.Errorf("conf: invalid watch path %q: %w", p, err)
		}
		dirs[filepath.Dir(abs)] = struct{}{}
		names[abs] = struct{}{}
	}

	for dir := range dirs {
		if err := fw.Add(dir); err != nil {
			fw.Close()
			return nil, nil, err
		}
	}

	out := make(chan FileChange, 16)
	stopOnce := sync.Once{}
	stop := func() error {
		var e error
		stopOnce.Do(func() {
			e = fw.Close()
		})
		return e
	}

	go func() {
		defer close(out)
		var (
			lastAt  time.Time
			lastKey string
		)
		for {
			select {
			case <-ctx.Done():
				_ = stop()
				return
			case ev, ok := <-fw.Events:
				if !ok {
					return
				}
				if ev.Name == "" {
					continue
				}
				abs, err := filepath.Abs(ev.Name)
				if err != nil {
					continue
				}
				if _, ok := names[abs]; !ok {
					continue
				}
				now := time.Now()
				if lastKey == abs && now.Sub(lastAt) < w.debounce {
					continue
				}
				lastKey = abs
				lastAt = now
				select {
				case out <- FileChange{Path: abs, At: now}:
				default:
				}
			case <-fw.Errors:
			}
		}
	}()

	return out, stop, nil
}

