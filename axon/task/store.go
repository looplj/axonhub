package task

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	tasksFileName        = "tasks.json"
	deletedTasksFileName = "deleted_tasks.json"
)

var (
	ErrTaskNotFound     = errors.New("task not found")
	ErrTaskExists       = errors.New("task already exists")
	ErrSystemTaskDelete = errors.New("system task cannot be deleted")
)

type Store struct {
	dir         string
	path        string
	deletedPath string
	mu          sync.Mutex
}

type UpdateFunc func(tasks []Task) (updated []Task, changed bool, err error)

func NewStore(dir string) (*Store, error) {
	if dir == "" {
		return nil, fmt.Errorf("task store directory is required")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create task store dir: %w", err)
	}

	path := filepath.Join(dir, tasksFileName)
	if _, err := os.Stat(path); err != nil {
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("stat task store: %w", err)
		}
		if err := os.WriteFile(path, []byte("[]\n"), 0o644); err != nil {
			return nil, fmt.Errorf("init task store: %w", err)
		}
	}

	deletedPath := filepath.Join(dir, deletedTasksFileName)
	if _, err := os.Stat(deletedPath); err != nil {
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("stat deleted task store: %w", err)
		}
		if err := os.WriteFile(deletedPath, []byte("[]\n"), 0o644); err != nil {
			return nil, fmt.Errorf("init deleted task store: %w", err)
		}
	}

	return &Store{
		dir:         dir,
		path:        path,
		deletedPath: deletedPath,
	}, nil
}

func (s *Store) Path() string {
	return s.path
}

func (s *Store) DeletedPath() string {
	return s.deletedPath
}

func (s *Store) Load() ([]Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.loadLocked()
}

func (s *Store) List() ([]Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	tasks, err := s.loadLocked()
	if err != nil {
		return nil, err
	}

	visible := make([]Task, 0, len(tasks))
	for _, t := range tasks {
		if t.Hidden {
			continue
		}

		visible = append(visible, t)
	}

	return visible, nil
}

func (s *Store) Save(tasks []Task) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.saveLocked(tasks)
}

func (s *Store) Update(fn UpdateFunc) error {
	if fn == nil {
		return fmt.Errorf("update function is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	tasks, err := s.loadLocked()
	if err != nil {
		return err
	}

	updated, changed, err := fn(tasks)
	if err != nil {
		return err
	}
	if !changed {
		return nil
	}

	return s.saveLocked(updated)
}

func (s *Store) Get(id string) (*Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	tasks, err := s.loadLocked()
	if err != nil {
		return nil, err
	}

	for i := range tasks {
		if tasks[i].ID == id {
			return &tasks[i], nil
		}
	}
	return nil, ErrTaskNotFound
}

func (s *Store) Add(t Task) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tasks, err := s.loadLocked()
	if err != nil {
		return err
	}

	for i := range tasks {
		if tasks[i].ID == t.ID {
			return fmt.Errorf("%w: %s", ErrTaskExists, t.ID)
		}
	}

	tasks = append(tasks, t)
	return s.saveLocked(tasks)
}

func (s *Store) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tasks, err := s.loadLocked()
	if err != nil {
		return err
	}

	idx := -1
	for i := range tasks {
		if tasks[i].ID == id {
			idx = i
			break
		}
	}
	if idx < 0 {
		return ErrTaskNotFound
	}

	if tasks[idx].System {
		return fmt.Errorf("%w: %s", ErrSystemTaskDelete, id)
	}

	deletedTask := tasks[idx]
	tasks = append(tasks[:idx], tasks[idx+1:]...)

	if err := s.saveLocked(tasks); err != nil {
		return err
	}

	return s.appendToDeletedLocked(deletedTask, "deleted")
}

func (s *Store) SetEnabled(id string, enabled bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tasks, err := s.loadLocked()
	if err != nil {
		return err
	}

	idx := -1
	for i := range tasks {
		if tasks[i].ID == id {
			idx = i
			break
		}
	}
	if idx < 0 {
		return ErrTaskNotFound
	}

	tasks[idx].Enabled = enabled
	if enabled {
		tasks[idx].Runtime.NextRunAt = ""
	}
	return s.saveLocked(tasks)
}

func (s *Store) MoveToDeleted(id string, reason string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tasks, err := s.loadLocked()
	if err != nil {
		return err
	}

	idx := -1
	for i := range tasks {
		if tasks[i].ID == id {
			idx = i
			break
		}
	}
	if idx < 0 {
		return ErrTaskNotFound
	}

	if tasks[idx].System {
		return fmt.Errorf("%w: %s", ErrSystemTaskDelete, id)
	}

	deletedTask := tasks[idx]
	tasks = append(tasks[:idx], tasks[idx+1:]...)

	if err := s.saveLocked(tasks); err != nil {
		return err
	}

	return s.appendToDeletedLocked(deletedTask, reason)
}

func (s *Store) LoadDeleted() ([]DeletedTask, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.loadDeletedLocked()
}

type DeletedTask struct {
	Task      Task   `json:"task"`
	Reason    string `json:"reason"`
	DeletedAt string `json:"deleted_at"`
}

func (s *Store) appendToDeletedLocked(t Task, reason string) error {
	deleted, err := s.loadDeletedLocked()
	if err != nil {
		return err
	}

	deleted = append(deleted, DeletedTask{
		Task:      t,
		Reason:    reason,
		DeletedAt: time.Now().UTC().Format(time.RFC3339),
	})

	return s.saveDeletedLocked(deleted)
}

func (s *Store) loadLocked() ([]Task, error) {
	data, err := os.ReadFile(s.path)
	if err != nil {
		return nil, fmt.Errorf("read task store: %w", err)
	}

	var tasks []Task
	if len(data) > 0 {
		if err := json.Unmarshal(data, &tasks); err != nil {
			return nil, fmt.Errorf("decode task store: %w", err)
		}
	}
	if tasks == nil {
		return []Task{}, nil
	}
	return tasks, nil
}

func (s *Store) saveLocked(tasks []Task) error {
	if tasks == nil {
		tasks = []Task{}
	}
	data, err := json.MarshalIndent(tasks, "", "  ")
	if err != nil {
		return fmt.Errorf("encode task store: %w", err)
	}
	data = append(data, '\n')

	tmpPath := s.path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0o644); err != nil {
		return fmt.Errorf("write task store tmp: %w", err)
	}
	if err := os.Rename(tmpPath, s.path); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("commit task store: %w", err)
	}
	return nil
}

func (s *Store) loadDeletedLocked() ([]DeletedTask, error) {
	data, err := os.ReadFile(s.deletedPath)
	if err != nil {
		return nil, fmt.Errorf("read deleted task store: %w", err)
	}

	var deleted []DeletedTask
	if len(data) > 0 {
		if err := json.Unmarshal(data, &deleted); err != nil {
			return nil, fmt.Errorf("decode deleted task store: %w", err)
		}
	}
	if deleted == nil {
		return []DeletedTask{}, nil
	}
	return deleted, nil
}

func (s *Store) saveDeletedLocked(deleted []DeletedTask) error {
	if deleted == nil {
		deleted = []DeletedTask{}
	}
	data, err := json.MarshalIndent(deleted, "", "  ")
	if err != nil {
		return fmt.Errorf("encode deleted task store: %w", err)
	}
	data = append(data, '\n')

	tmpPath := s.deletedPath + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0o644); err != nil {
		return fmt.Errorf("write deleted task store tmp: %w", err)
	}
	if err := os.Rename(tmpPath, s.deletedPath); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("commit deleted task store: %w", err)
	}
	return nil
}
