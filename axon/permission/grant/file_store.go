package grant

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type FileStore struct {
	BaseDir string
}

type fileFormat struct {
	Version   int       `json:"version"`
	UpdatedAt time.Time `json:"updated_at"`
	Keys      []string  `json:"keys"`
}

func (s FileStore) Load(workspace string) (map[string]struct{}, error) {
	path, err := s.pathForWorkspace(workspace)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string]struct{}), nil
		}
		return nil, fmt.Errorf("grant: read %s: %w", path, err)
	}
	var f fileFormat
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("grant: parse %s: %w", path, err)
	}
	out := make(map[string]struct{}, len(f.Keys))
	for _, k := range f.Keys {
		out[k] = struct{}{}
	}
	return out, nil
}

func (s FileStore) Save(workspace string, keys map[string]struct{}) error {
	path, err := s.pathForWorkspace(workspace)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("grant: mkdir: %w", err)
	}
	var list []string
	for k := range keys {
		list = append(list, k)
	}
	f := fileFormat{
		Version:   1,
		UpdatedAt: time.Now(),
		Keys:      list,
	}
	data, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return fmt.Errorf("grant: marshal: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("grant: write: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("grant: rename: %w", err)
	}
	return nil
}

func (s FileStore) pathForWorkspace(workspace string) (string, error) {
	if s.BaseDir == "" {
		return "", fmt.Errorf("grant: BaseDir is empty")
	}
	ws := filepath.Clean(workspace)
	hash := workspaceHash(ws)
	return filepath.Join(s.BaseDir, "workspaces", hash+".json"), nil
}

func workspaceHash(ws string) string {
	// Keep deterministic short filename.
	// Reuse the same hashing logic as BuildKey without importing permission.
	// (This file store is independent of the key algorithm.)
	// sha256 hex shortened to 16 bytes (32 chars) to keep paths tidy.
	sum := sha256.Sum256([]byte(ws))
	return hex.EncodeToString(sum[:])[:32]
}
