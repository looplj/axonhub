package main

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

const (
	desktopHost     = "127.0.0.1"
	desktopPort     = 18090
	sqliteDSNParams = "?cache=shared&_fk=1&_pragma=journal_mode(WAL)"
	ownerSecretName = "owner.secret"
)

// desktopDataDir is the private directory for this desktop install.
// macOS uses ~/Library/Application Support/AxonHub, Windows uses
// %AppData%\AxonHub, and other systems use ~/.config/axonhub-desktop.
func desktopDataDir(goos, home, appData string) (string, error) {
	switch goos {
	case "darwin":
		if home == "" {
			return "", errors.New("home directory is not set")
		}
		return filepath.Join(home, "Library", "Application Support", "AxonHub"), nil
	case "windows":
		if appData == "" {
			return "", errors.New("AppData is not set")
		}
		return filepath.Join(appData, "AxonHub"), nil
	default:
		if home == "" {
			return "", errors.New("home directory is not set")
		}
		return filepath.Join(home, ".config", "axonhub-desktop"), nil
	}
}

func ensureDataDir() (string, error) {
	home, homeErr := os.UserHomeDir()
	appData := os.Getenv("AppData")
	if runtime.GOOS != "windows" && homeErr != nil {
		return "", fmt.Errorf("resolve home directory: %w", homeErr)
	}

	dir, err := desktopDataDir(runtime.GOOS, home, appData)
	if err != nil {
		return "", err
	}
	logs := filepath.Join(dir, "logs")
	if err := os.MkdirAll(logs, 0o700); err != nil {
		return "", fmt.Errorf("create desktop data directory: %w", err)
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return "", fmt.Errorf("restrict desktop data directory: %w", err)
	}
	return dir, nil
}

func sqliteDSN(dbPath string) string {
	return "file:" + filepath.ToSlash(dbPath) + sqliteDSNParams
}

func configYAML(dir string) string {
	dbPath := filepath.Join(dir, "axonhub.db")
	logPath := filepath.Join(dir, "logs", "axonhub.log")
	return strings.Join([]string{
		"server:",
		"  host: " + desktopHost,
		"  port: " + strconv.Itoa(desktopPort),
		"db:",
		"  dialect: sqlite3",
		"  dsn: " + strconv.Quote(sqliteDSN(dbPath)),
		"log:",
		"  output: file",
		"  file:",
		"    path: " + strconv.Quote(logPath),
		"",
	}, "\n")
}

// ensureConfig writes config.yml on the first launch and leaves an existing
// file untouched so later edits survive.
func ensureConfig(dir string) (string, error) {
	path := filepath.Join(dir, "config.yml")
	if _, err := os.Stat(path); err == nil {
		return path, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("stat desktop config: %w", err)
	}
	if err := os.WriteFile(path, []byte(configYAML(dir)), 0o600); err != nil {
		return "", fmt.Errorf("write desktop config: %w", err)
	}
	return path, nil
}

func ownerSecretPath(dir string) string {
	return filepath.Join(dir, ownerSecretName)
}

// ownerSecret reads the saved owner password. When create is set and the
// file is absent, it generates a password and stores it with mode 0600.
// The password is never written to logs.
func ownerSecret(path string, create bool) (string, error) {
	secret, err := readOwnerSecret(path)
	if err == nil {
		return secret, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	if !create {
		return "", errors.New("owner secret file is missing")
	}

	secret, err = newOwnerPassword()
	if err != nil {
		return "", err
	}
	if err := writeOwnerSecret(path, secret); err != nil {
		if !errors.Is(err, os.ErrExist) {
			return "", err
		}
		return readOwnerSecret(path)
	}
	return secret, nil
}

func readOwnerSecret(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", err
		}
		return "", fmt.Errorf("read owner secret: %w", err)
	}
	secret := strings.TrimSpace(string(data))
	if secret == "" {
		return "", errors.New("owner secret file is empty")
	}
	return secret, nil
}

func writeOwnerSecret(path, secret string) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	_, writeErr := file.WriteString(secret + "\n")
	closeErr := file.Close()
	if writeErr != nil {
		return fmt.Errorf("write owner secret: %w", writeErr)
	}
	if closeErr != nil {
		return fmt.Errorf("write owner secret: %w", closeErr)
	}
	return nil
}

func newOwnerPassword() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate owner password: %w", err)
	}
	return hex.EncodeToString(buf), nil
}
