package build

import (
	"fmt"
	"runtime"
	"strings"
	"time"

	_ "embed"
)

//go:embed VERSION
var rawVersion []byte

// Build information.
var (
	Version   = ""
	Commit    = ""
	BuildTime = ""
	GoVersion = runtime.Version()
	Platform  = fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH)
	StartTime = time.Now()
)

//nolint:gochecknoinits // init version.
func init() {
	// The version can be set by goreleaser.
	// If not set, use the version in the VERSION file for local development and docker build.
	if Version == "" {
		Version = strings.TrimSpace(string(rawVersion))
	}
}

// Info contains build information.
type Info struct {
	Version   string `json:"version"`
	Commit    string `json:"commit,omitempty"`
	BuildTime string `json:"build_time,omitempty"`
	GoVersion string `json:"go_version"`
	Platform  string `json:"platform"`
	Uptime    string `json:"uptime"`
}

// GetBuildInfo returns build information.
func GetBuildInfo() Info {
	return Info{
		Version:   Version,
		Commit:    Commit,
		BuildTime: BuildTime,
		GoVersion: GoVersion,
		Platform:  Platform,
		Uptime:    time.Since(StartTime).String(),
	}
}

// String returns string representation of build info.
func (i Info) String() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Version: %s\n", i.Version))

	if i.Commit != "" {
		sb.WriteString(fmt.Sprintf("Commit: %s\n", i.Commit))
	}

	if i.BuildTime != "" {
		sb.WriteString(fmt.Sprintf("Build Time: %s\n", i.BuildTime))
	}

	sb.WriteString(fmt.Sprintf("Go Version: %s\n", i.GoVersion))
	sb.WriteString(fmt.Sprintf("Platform: %s\n", i.Platform))
	sb.WriteString(fmt.Sprintf("Uptime: %s\n", i.Uptime))

	return sb.String()
}
