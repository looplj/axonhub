package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/samber/lo"
	"gopkg.in/yaml.v3"

	"github.com/looplj/axonhub/internal/server/db"
)

const dbPingTimeout = 10 * time.Second

// dbConfig is the db section of the desktop config.yml.
type dbConfig struct {
	Dialect string `yaml:"dialect"`
	DSN     string `yaml:"dsn"`
}

// desktopSettings is the server port, the database, and whether the
// server listens beyond this machine, as the settings page edits them.
type desktopSettings struct {
	Port int
	LAN  bool
	DB   dbConfig
}

// dbKind is a database the settings page offers, with the dialect it
// writes and a DSN to start from.
type dbKind struct {
	Label   string
	Dialect string
	Example string
}

var dbKinds = []dbKind{
	{Label: "SQLite", Dialect: "sqlite3", Example: "file:/path/to/axonhub.db?cache=shared&_fk=1"},
	{Label: "MySQL", Dialect: "mysql", Example: "user:password@tcp(127.0.0.1:3306)/axonhub?charset=utf8mb4&parseTime=True"},
	{Label: "PostgreSQL", Dialect: "postgres", Example: "postgres://user:password@127.0.0.1:5432/axonhub?sslmode=disable"},
}

// dbKindIndex is the settings choice for a dialect, the aliases the server
// accepts included. An unknown dialect is SQLite, the default.
func dbKindIndex(dialect string) int {
	switch strings.ToLower(strings.TrimSpace(dialect)) {
	case "mysql", "tidb":
		return 1
	case "postgres", "pgx", "postgresdb", "pg", "postgresql":
		return 2
	default:
		return 0
	}
}

func readDBConfig(path string) (dbConfig, error) {
	settings, err := readDesktopSettings(path)
	if err != nil {
		return dbConfig{}, err
	}
	return settings.DB, nil
}

func readDesktopSettings(path string) (desktopSettings, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return desktopSettings{}, fmt.Errorf("read desktop config: %w", err)
	}
	var file struct {
		Server struct {
			// A string, so a quoted port reads as the server reads it.
			Port string `yaml:"port"`
			Host string `yaml:"host"`
		} `yaml:"server"`
		DB dbConfig `yaml:"db"`
	}
	if err := yaml.Unmarshal(data, &file); err != nil {
		return desktopSettings{}, fmt.Errorf("parse desktop config: %w", err)
	}
	// A missing or unreadable port is the default, so it never blocks
	// reading the database settings.
	port, err := strconv.Atoi(strings.TrimSpace(file.Server.Port))
	if err != nil || port < 1 || port > 65535 {
		port = desktopPort
	}
	if file.DB.Dialect == "" {
		file.DB.Dialect = "sqlite3"
	}
	return desktopSettings{Port: port, LAN: lanHost(strings.TrimSpace(file.Server.Host)), DB: file.DB}, nil
}

// writeDBConfig sets db.dialect and db.dsn in config.yml. Other keys and
// comments stay as they are. The file is replaced in one rename, so a
// failed write leaves the old config.
func writeDBConfig(path string, cfg dbConfig) error {
	return writeDesktopFile(path, 0, "", cfg)
}

// writeDesktopSettings sets the listen port, the listen host and the
// database. The host is every interface with LAN on and loopback with it
// off. Other keys and comments stay as they are.
func writeDesktopSettings(path string, settings desktopSettings) error {
	if settings.Port < 1 || settings.Port > 65535 {
		return errors.New(formPortInvalid)
	}
	return writeDesktopFile(path, settings.Port, lo.Ternary(settings.LAN, desktopLANHost, desktopHost), settings.DB)
}

func writeDesktopFile(path string, port int, host string, cfg dbConfig) error {
	data, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("read desktop config: %w", err)
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return fmt.Errorf("parse desktop config: %w", err)
	}
	if doc.Kind == 0 {
		doc = yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{{Kind: yaml.MappingNode}}}
	}
	root := doc.Content[0]
	if root.Kind != yaml.MappingNode {
		return errors.New("desktop config is not a mapping")
	}
	if port > 0 {
		serverNode := mappingValue(root, "server", yaml.MappingNode)
		setInt(mappingValue(serverNode, "port", yaml.ScalarNode), port)
	}
	if host != "" {
		serverNode := mappingValue(root, "server", yaml.MappingNode)
		mappingValue(serverNode, "host", yaml.ScalarNode).SetString(host)
	}
	section := mappingValue(root, "db", yaml.MappingNode)
	mappingValue(section, "dialect", yaml.ScalarNode).SetString(cfg.Dialect)
	mappingValue(section, "dsn", yaml.ScalarNode).SetString(cfg.DSN)

	var out bytes.Buffer
	enc := yaml.NewEncoder(&out)
	enc.SetIndent(2)
	if err := enc.Encode(&doc); err != nil {
		return fmt.Errorf("encode desktop config: %w", err)
	}
	if err := enc.Close(); err != nil {
		return fmt.Errorf("encode desktop config: %w", err)
	}

	tmp, err := os.CreateTemp(filepath.Dir(path), ".config-*.yml")
	if err != nil {
		return fmt.Errorf("write desktop config: %w", err)
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(out.Bytes()); err != nil {
		tmp.Close()
		return fmt.Errorf("write desktop config: %w", err)
	}
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return fmt.Errorf("write desktop config: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("write desktop config: %w", err)
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return fmt.Errorf("write desktop config: %w", err)
	}
	return nil
}

// mappingValue returns the value of key in m, adding it as an empty node
// of kind when it is missing or of another kind.
func mappingValue(m *yaml.Node, key string, kind yaml.Kind) *yaml.Node {
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			value := m.Content[i+1]
			if value.Kind != kind {
				*value = yaml.Node{Kind: kind}
			}
			return value
		}
	}
	value := &yaml.Node{Kind: kind}
	m.Content = append(m.Content, &yaml.Node{Kind: yaml.ScalarNode, Value: key}, value)
	return value
}

func setInt(node *yaml.Node, value int) {
	node.Kind = yaml.ScalarNode
	node.Tag = "!!int"
	node.Value = strconv.Itoa(value)
	node.Style = 0
}

// parseListenPort reads a settings port. A blank value is the default.
func parseListenPort(value string) (int, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return desktopPort, nil
	}
	port, err := strconv.Atoi(value)
	if err != nil || port < 1 || port > 65535 {
		return 0, errors.New(formPortInvalid)
	}
	return port, nil
}

// pingDB checks that the server could open cfg, within dbPingTimeout.
func pingDB(ctx context.Context, cfg dbConfig) error {
	if strings.TrimSpace(cfg.DSN) == "" {
		return errors.New(formDSNEmpty)
	}
	ctx, cancel := context.WithTimeout(ctx, dbPingTimeout)
	defer cancel()
	return db.Ping(ctx, cfg.Dialect, cfg.DSN)
}
