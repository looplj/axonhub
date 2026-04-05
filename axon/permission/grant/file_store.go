package grant

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/afero"
)

type FileStore struct {
	BaseDir string
	fsys    afero.Fs
}

type fileFormat struct {
	Version   int       `json:"version"`
	UpdatedAt time.Time `json:"updated_at"`
	Keys      []string  `json:"keys"`
}

func NewFileStore(baseDir string) FileStore {
	return FileStore{BaseDir: baseDir, fsys: afero.NewOsFs()}
}

func NewFileStoreWithFS(baseDir string, fsys afero.Fs) FileStore {
	return FileStore{BaseDir: baseDir, fsys: fsys}
}

func (s FileStore) LoadWorkspace(workspace string) (map[string]struct{}, error) {
	path := s.workspacePath(workspace)
	return s.loadFile(path)
}

func (s FileStore) SaveWorkspace(workspace string, keys map[string]struct{}) error {
	path := s.workspacePath(workspace)
	return s.saveFile(path, keys)
}

func (s FileStore) LoadGlobal() (map[string]struct{}, error) {
	path := s.globalPath()
	return s.loadFile(path)
}

func (s FileStore) SaveGlobal(keys map[string]struct{}) error {
	path := s.globalPath()
	return s.saveFile(path, keys)
}

func (s FileStore) loadFile(path string) (map[string]struct{}, error) {
	data, err := afero.ReadFile(s.fsys, path)
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

func (s FileStore) saveFile(path string, keys map[string]struct{}) error {
	if err := s.fsys.MkdirAll(filepath.Dir(path), 0o755); err != nil {
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
	if err := afero.WriteFile(s.fsys, tmp, data, 0o600); err != nil {
		return fmt.Errorf("grant: write: %w", err)
	}
	if err := s.fsys.Rename(tmp, path); err != nil {
		return fmt.Errorf("grant: rename: %w", err)
	}
	return nil
}

func (s FileStore) workspacePath(workspace string) string {
	ws := filepath.Clean(workspace)
	hash := workspaceHash(ws)
	return filepath.Join(s.BaseDir, "workspaces", hash+".json")
}

func (s FileStore) globalPath() string {
	return filepath.Join(s.BaseDir, "global.json")
}

func workspaceHash(ws string) string {
	sum := sha256.Sum256([]byte(ws))
	return hex.EncodeToString(sum[:])[:32]
}
