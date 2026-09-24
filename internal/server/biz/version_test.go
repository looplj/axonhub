package biz

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestIsNewerVersion(t *testing.T) {
	tests := []struct {
		name    string
		current string
		latest  string
		want    bool
	}{
		{
			name:    "latest version is newer - major version",
			current: "v1.0.0",
			latest:  "v2.0.0",
			want:    true,
		},
		{
			name:    "latest version is newer - minor version",
			current: "v1.0.0",
			latest:  "v1.1.0",
			want:    true,
		},
		{
			name:    "latest version is newer - patch version",
			current: "v1.0.0",
			latest:  "v1.0.1",
			want:    true,
		},
		{
			name:    "latest version is same",
			current: "v1.0.0",
			latest:  "v1.0.0",
			want:    false,
		},
		{
			name:    "latest version is older",
			current: "v2.0.0",
			latest:  "v1.0.0",
			want:    false,
		},
		{
			name:    "versions without v prefix - latest newer",
			current: "1.0.0",
			latest:  "1.1.0",
			want:    true,
		},
		{
			name:    "mixed v prefix - current has v, latest doesn't",
			current: "v1.0.0",
			latest:  "1.1.0",
			want:    true,
		},
		{
			name:    "mixed v prefix - latest has v, current doesn't",
			current: "1.0.0",
			latest:  "v1.1.0",
			want:    true,
		},
		{
			name:    "complex version comparison",
			current: "v1.2.3",
			latest:  "v2.0.0",
			want:    true,
		},
		{
			name:    "same major, higher minor",
			current: "v1.5.0",
			latest:  "v1.6.0",
			want:    true,
		},
		{
			name:    "same major and minor, higher patch",
			current: "v1.5.2",
			latest:  "v1.5.3",
			want:    true,
		},
		{
			name:    "invalid current version",
			current: "invalid",
			latest:  "v1.0.0",
			want:    false,
		},
		{
			name:    "invalid latest version",
			current: "v1.0.0",
			latest:  "invalid",
			want:    false,
		},
		{
			name:    "both invalid versions",
			current: "invalid",
			latest:  "invalid",
			want:    false,
		},
		{
			name:    "empty current version",
			current: "",
			latest:  "v1.0.0",
			want:    false,
		},
		{
			name:    "empty latest version",
			current: "v1.0.0",
			latest:  "",
			want:    false,
		},
		{
			name:    "both empty versions",
			current: "",
			latest:  "",
			want:    false,
		},
		{
			name:    "prerelease versions - current is prerelease",
			current: "v1.0.0-beta",
			latest:  "v1.0.0",
			want:    true,
		},
		{
			name:    "prerelease versions - latest is prerelease",
			current: "v1.0.0",
			latest:  "v1.0.1-beta",
			want:    true,
		},
		{
			name:    "beta sequence is newer",
			current: "v1.0.0-beta5",
			latest:  "v1.0.0-beta6",
			want:    true,
		},
		{
			name:    "beta counter crosses ten boundary",
			current: "v1.0.0-beta9",
			latest:  "v1.0.0-beta10",
			want:    true,
		},
		{
			name:    "beta counter crosses ten boundary in reverse",
			current: "v1.0.0-beta10",
			latest:  "v1.0.0-beta9",
			want:    false,
		},
		{
			name:    "beta counter crosses hundred boundary",
			current: "v1.0.0-beta99",
			latest:  "v1.0.0-beta100",
			want:    true,
		},
		{
			name:    "dotted beta counter crosses ten boundary",
			current: "v1.0.0-beta.9",
			latest:  "v1.0.0-beta.10",
			want:    true,
		},
		{
			name:    "glued and dotted beta counters are equivalent",
			current: "v1.0.0-beta10",
			latest:  "v1.0.0-beta.10",
			want:    false,
		},
		{
			name:    "rc counter crosses ten boundary",
			current: "v1.0.0-rc9",
			latest:  "v1.0.0-rc10",
			want:    true,
		},
		{
			name:    "unstable build of the same beta outranks the beta",
			current: "v1.0.0-beta10",
			latest:  "v1.0.0-beta10-unstable.20260907",
			want:    true,
		},
		{
			name:    "unstable build precedes the next beta",
			current: "v1.0.0-beta9-unstable.20260907",
			latest:  "v1.0.0-beta10",
			want:    true,
		},
		{
			name:    "stable release outranks its beta",
			current: "v1.0.0-beta10",
			latest:  "v1.0.0",
			want:    true,
		},
		{
			name:    "build metadata",
			current: "v1.0.0+build.1",
			latest:  "v1.0.0+build.2",
			want:    false, // build metadata doesn't affect version comparison
		},
		{
			name:    "version with many digits",
			current: "v1.2.3",
			latest:  "v1.2.3.4",
			want:    false, // semver only supports 3-part versions
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsNewerVersion(tt.current, tt.latest)
			require.Equal(t, tt.want, got, "IsNewerVersion(%q, %q) = %v, want %v", tt.current, tt.latest, got, tt.want)
		})
	}
}

func TestIsAxonHubTag(t *testing.T) {
	tests := []struct {
		name string
		tag  string
		want bool
	}{
		{
			name: "standard axonhub tag",
			tag:  "v1.0.0",
			want: true,
		},
		{
			name: "axonhub prerelease tag",
			tag:  "v1.0.0-beta",
			want: true,
		},
		{
			name: "axonclaw prefixed tag",
			tag:  "axonclaw/v1.0.0",
			want: false,
		},
		{
			name: "other service prefixed tag",
			tag:  "other-service/v2.0.0",
			want: false,
		},
		{
			name: "empty tag",
			tag:  "",
			want: false,
		},
		{
			name: "non-version tag",
			tag:  "release-2024",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isAxonHubTag(tt.tag)
			require.Equal(t, tt.want, got, "isAxonHubTag(%q) = %v, want %v", tt.tag, got, tt.want)
		})
	}
}

func TestIsPreReleaseTag(t *testing.T) {
	tests := []struct {
		name string
		tag  string
		want bool
	}{
		{
			name: "beta tag",
			tag:  "v1.0.0-beta",
			want: true,
		},
		{
			name: "rc tag",
			tag:  "v1.0.0-rc",
			want: true,
		},
		{
			name: "alpha tag",
			tag:  "v1.0.0-alpha",
			want: true,
		},
		{
			name: "dev tag",
			tag:  "v1.0.0-dev",
			want: true,
		},
		{
			name: "preview tag",
			tag:  "v1.0.0-preview",
			want: true,
		},
		{
			name: "snapshot tag",
			tag:  "v1.0.0-snapshot",
			want: true,
		},
		{
			name: "stable tag",
			tag:  "v1.0.0",
			want: false,
		},
		{
			name: "uppercase beta",
			tag:  "v1.0.0-BETA",
			want: true,
		},
		{
			name: "mixed case",
			tag:  "v1.0.0-Beta",
			want: true,
		},
		{
			name: "beta with number",
			tag:  "v1.0.0-beta.1",
			want: true,
		},
		{
			name: "rc with number",
			tag:  "v1.0.0-rc.1",
			want: true,
		},
		{
			name: "empty tag",
			tag:  "",
			want: false,
		},
		{
			name: "tag without prerelease",
			tag:  "release",
			want: false,
		},
		{
			name: "tag containing beta but not as prerelease",
			tag:  "betatest",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isPreReleaseTag(tt.tag)
			require.Equal(t, tt.want, got, "isPreReleaseTag(%q) = %v, want %v", tt.tag, got, tt.want)
		})
	}
}

func TestSelectLatestGitHubRelease(t *testing.T) {
	now := time.Date(2026, time.July, 30, 12, 0, 0, 0, time.UTC)
	releases := []GitHubRelease{
		{TagName: "v1.0.0-beta6", PublishedAt: now.Add(-time.Hour)},
		{TagName: "v1.0.0-rc1", PublishedAt: now.Add(-2 * time.Hour)},
		{TagName: "v0.9.43", PublishedAt: now.Add(-3 * time.Hour)},
	}

	t.Run("stable check skips beta releases", func(t *testing.T) {
		got, err := selectLatestGitHubRelease(releases, false, now)
		require.NoError(t, err)
		require.Equal(t, "v0.9.43", got)
	})

	t.Run("beta check includes beta releases", func(t *testing.T) {
		got, err := selectLatestGitHubRelease(releases, true, now)
		require.NoError(t, err)
		require.Equal(t, "v1.0.0-beta6", got)
	})

	t.Run("selects higher stable release after lower beta release", func(t *testing.T) {
		got, err := selectLatestGitHubRelease([]GitHubRelease{
			{TagName: "v1.9.0-beta1", PublishedAt: now.Add(-time.Hour)},
			{TagName: "v2.0.0", PublishedAt: now.Add(-2 * time.Hour)},
		}, true, now)
		require.NoError(t, err)
		require.Equal(t, "v2.0.0", got)
	})

	t.Run("selects highest eligible stable release", func(t *testing.T) {
		got, err := selectLatestGitHubRelease([]GitHubRelease{
			{TagName: "v0.9.42", PublishedAt: now.Add(-time.Hour)},
			{TagName: "v0.9.43", PublishedAt: now.Add(-2 * time.Hour)},
		}, false, now)
		require.NoError(t, err)
		require.Equal(t, "v0.9.43", got)
	})

	t.Run("beta check still excludes other prereleases", func(t *testing.T) {
		got, err := selectLatestGitHubRelease([]GitHubRelease{
			{TagName: "v1.0.0-rc1", PublishedAt: now.Add(-time.Hour)},
			{TagName: "v0.9.43", PublishedAt: now.Add(-2 * time.Hour)},
		}, true, now)
		require.NoError(t, err)
		require.Equal(t, "v0.9.43", got)
	})

	t.Run("skips drafts recent releases and other service tags", func(t *testing.T) {
		got, err := selectLatestGitHubRelease([]GitHubRelease{
			{TagName: "v1.0.0-beta6", Draft: true, PublishedAt: now.Add(-time.Hour)},
			{TagName: "v1.0.0-beta5", PublishedAt: now.Add(-releaseCooldownDuration / 2)},
			{TagName: "axonclaw/v2.0.0", PublishedAt: now.Add(-time.Hour)},
			{TagName: "v1.0.0-beta4", Prerelease: true, PublishedAt: now.Add(-2 * time.Hour)},
		}, true, now)
		require.NoError(t, err)
		require.Equal(t, "v1.0.0-beta4", got)
	})

	t.Run("selects the highest beta across the ten boundary", func(t *testing.T) {
		got, err := selectLatestGitHubRelease([]GitHubRelease{
			{TagName: "v1.0.0-beta9", Prerelease: true, PublishedAt: now.Add(-96 * time.Hour)},
			{TagName: "v1.0.0-beta10", Prerelease: true, PublishedAt: now.Add(-24 * time.Hour)},
		}, true, now)
		require.NoError(t, err)
		require.Equal(t, "v1.0.0-beta10", got)
	})

	t.Run("returns an error when no release is eligible", func(t *testing.T) {
		_, err := selectLatestGitHubRelease([]GitHubRelease{
			{TagName: "v1.0.0-beta6", PublishedAt: now.Add(-time.Hour)},
		}, false, now)
		require.EqualError(t, err, "no eligible release found")
	})
}

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		name string
		a    string
		b    string
		want int
	}{
		{name: "equal", a: "v1.0.0", b: "v1.0.0", want: 0},
		{name: "major", a: "v2.0.0", b: "v1.0.0", want: 1},
		{name: "minor", a: "v1.1.0", b: "v1.2.0", want: -1},
		{name: "patch", a: "v1.0.2", b: "v1.0.1", want: 1},
		{name: "beta below ten", a: "v1.0.0-beta5", b: "v1.0.0-beta6", want: -1},
		{name: "beta across ten", a: "v1.0.0-beta10", b: "v1.0.0-beta9", want: 1},
		{name: "beta across hundred", a: "v1.0.0-beta100", b: "v1.0.0-beta99", want: 1},
		{name: "glued equals dotted", a: "v1.0.0-beta10", b: "v1.0.0-beta.10", want: 0},
		{name: "padded counter equals bare counter", a: "v1.0.0-beta010", b: "v1.0.0-beta10", want: 0},
		{name: "long counters do not overflow", a: "v1.0.0-beta99999999999999999999", b: "v1.0.0-beta99999999999999999998", want: 1},
		{name: "bare name precedes counter", a: "v1.0.0-beta", b: "v1.0.0-beta1", want: -1},
		{name: "alpha precedes beta", a: "v1.0.0-alpha10", b: "v1.0.0-beta1", want: -1},
		{name: "beta precedes rc", a: "v1.0.0-beta10", b: "v1.0.0-rc1", want: -1},
		{name: "stable outranks beta", a: "v1.0.0", b: "v1.0.0-beta10", want: 1},
		{name: "build metadata ignored", a: "v1.0.0+build.2", b: "v1.0.0+build.1", want: 0},
		{name: "missing v prefix", a: "1.0.0-beta10", b: "v1.0.0-beta9", want: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := CompareVersions(tt.a, tt.b)
			require.NoError(t, err)
			require.Equal(t, tt.want, got, "CompareVersions(%q, %q)", tt.a, tt.b)

			// The comparison must be antisymmetric.
			reverse, err := CompareVersions(tt.b, tt.a)
			require.NoError(t, err)
			require.Equal(t, -tt.want, reverse, "CompareVersions(%q, %q)", tt.b, tt.a)
		})
	}

	t.Run("reports invalid versions", func(t *testing.T) {
		_, err := CompareVersions("invalid", "v1.0.0")
		require.Error(t, err)

		_, err = CompareVersions("v1.0.0", "")
		require.Error(t, err)
	})
}
