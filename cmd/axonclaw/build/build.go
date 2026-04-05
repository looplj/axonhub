package build

var (
	Version   = "dev"
	BuildTime = "unknown"
	GitCommit = "unknown"
)

func GetVersion() string {
	return Version
}

func GetBuildTime() string {
	return BuildTime
}

func GetGitCommit() string {
	return GitCommit
}
