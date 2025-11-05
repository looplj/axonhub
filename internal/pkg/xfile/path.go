package xfile

import (
	"path/filepath"
	"runtime"
	"strings"
)

func CurDir() string {
	_, file, _, ok := runtime.Caller(1)
	if !ok {
		panic("runtime error")
	}

	return filepath.Dir(file)
}

func CallerDir() string {
	_, file, _, ok := runtime.Caller(2)
	if !ok {
		panic("runtime error")
	}

	return filepath.Dir(file)
}

func ProjectDir() string {
	dir := CurDir()
	return strings.TrimSuffix(dir, "/internal/pkg/xfile")
}
