package conf

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

func ReadYAMLFile(path string) (map[string]any, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]any{}, nil
		}
		return nil, err
	}
	if len(bytesTrimSpace(b)) == 0 {
		return map[string]any{}, nil
	}

	var data map[string]any
	if err := yaml.Unmarshal(b, &data); err != nil {
		return nil, err
	}
	if data == nil {
		data = map[string]any{}
	}
	return data, nil
}

func WriteYAMLFile(path string, data map[string]any) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	b, err := yaml.Marshal(data)
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o600)
}

func SetYAMLKey(path string, key string, value string) error {
	data, err := ReadYAMLFile(path)
	if err != nil {
		return err
	}
	data[key] = value
	return WriteYAMLFile(path, data)
}

func GetYAMLString(path string, key string) (string, bool, error) {
	data, err := ReadYAMLFile(path)
	if err != nil {
		return "", false, err
	}
	v, ok := data[key]
	if !ok {
		return "", false, nil
	}
	s, ok := v.(string)
	if ok {
		return s, true, nil
	}
	return fmt.Sprintf("%v", v), true, nil
}

func bytesTrimSpace(b []byte) []byte {
	return []byte(strings.TrimSpace(string(b)))
}

