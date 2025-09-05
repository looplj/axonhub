package build

import (
	"fmt"
	"runtime"
	"strings"
	"time"
)

// Build information.
var (
	Version   = "dev"
	Commit    = ""
	BuildTime = ""
	GoVersion = runtime.Version()
	Platform  = fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH)
)

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
	startTime := time.Now()

	return Info{
		Version:   Version,
		Commit:    Commit,
		BuildTime: BuildTime,
		GoVersion: GoVersion,
		Platform:  Platform,
		Uptime:    startTime.Format(time.RFC3339),
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
