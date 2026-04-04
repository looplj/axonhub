package claudecode

import (
	"regexp"
	"strings"
)

// PromptEnvConfig holds the canonical environment configuration for prompt rewriting.
type PromptEnvConfig struct {
	Platform    string // darwin
	Shell       string // zsh
	OSVersion   string // Darwin 24.4.0
	WorkingDir  string // /Users/jack/projects
}

// DefaultPromptEnv returns the default prompt environment configuration.
func DefaultPromptEnv() *PromptEnvConfig {
	return &PromptEnvConfig{
		Platform:   "darwin",
		Shell:      "zsh",
		OSVersion:  "Darwin 24.4.0",
		WorkingDir: "/Users/jack/projects",
	}
}

var (
	// Platform pattern: "Platform: linux" -> "Platform: darwin"
	platformPattern = regexp.MustCompile(`Platform:\s*\S+`)
	
	// Shell pattern: "Shell: bash" -> "Shell: zsh"
	shellPattern = regexp.MustCompile(`Shell:\s*\S+`)
	
	// OS Version pattern: "OS Version: Linux 6.5.0-xxx" -> "OS Version: Darwin 24.4.0"
	osVersionPattern = regexp.MustCompile(`OS Version:\s*[^\n<]+`)
	
	// Working directory pattern: "Working directory: /home/xxx" -> "Working directory: /Users/jack/projects"
	workingDirPattern = regexp.MustCompile(`((?:Primary )?[Ww]orking directory:\s*)\/\S+`)
	
	// Home directory path pattern: /Users/xxx/ or /home/xxx/
	homePathPattern = regexp.MustCompile(`\/(?:Users|home)\/[^/\s]+\/`)
)

// RewritePromptText rewrites environment fields in system prompts.
// This replaces platform, shell, OS version, and working directory with canonical values.
func RewritePromptText(text string, config *PromptEnvConfig, cchHash string) string {
	if config == nil {
		config = DefaultPromptEnv()
	}

	result := text

	// 1. Billing header fingerprint (if hash provided)
	if cchHash != "" {
		result = rewriteBillingHeader(result, cchHash)
	}

	// 2. <env> block environment fields
	result = platformPattern.ReplaceAllString(result, "Platform: "+config.Platform)
	result = shellPattern.ReplaceAllString(result, "Shell: "+config.Shell)
	result = osVersionPattern.ReplaceAllString(result, "OS Version: "+config.OSVersion)

	// 3. Working directory
	result = workingDirPattern.ReplaceAllString(result, "$1"+config.WorkingDir)

	// 4. Home directory paths
	canonicalHome := extractCanonicalHome(config.WorkingDir)
	if canonicalHome != "" {
		result = homePathPattern.ReplaceAllString(result, canonicalHome)
	}

	return result
}

// rewriteBillingHeader rewrites the cc_version hash in billing headers.
func rewriteBillingHeader(text string, cchHash string) string {
	// Pattern: cc_version=2.1.81.xxx
	pattern := regexp.MustCompile(`cc_version=[\d.]+\.[a-f0-9]{3}`)
	replacement := "cc_version=" + CCHVersion + "." + cchHash
	return pattern.ReplaceAllString(text, replacement)
}

// RewriteSystemReminders rewrites <system-reminder> blocks within message text.
// These are injected by Claude Code (env info, git status, etc.) — not user-authored.
// User-written text outside these tags is left untouched to preserve intent.
func RewriteSystemReminders(text string, config *PromptEnvConfig) string {
	// Match <system-reminder>...</system-reminder> blocks
	pattern := regexp.MustCompile(`(<system-reminder>)([\s\S]*?)(<\/system-reminder>)`)
	
	return pattern.ReplaceAllStringFunc(text, func(match string) string {
		// Extract the content between tags
		parts := pattern.FindStringSubmatch(match)
		if len(parts) != 4 {
			return match
		}
		
		openTag := parts[1]
		content := parts[2]
		closeTag := parts[3]
		
		// Rewrite the content (without CCH hash)
		rewritten := RewritePromptText(content, config, "")
		
		return openTag + rewritten + closeTag
	})
}

// extractCanonicalHome extracts the canonical home directory from working dir.
// Example: /Users/jack/projects -> /Users/jack/
func extractCanonicalHome(workingDir string) string {
	parts := strings.Split(workingDir, "/")
	if len(parts) >= 3 {
		return "/" + parts[1] + "/" + parts[2] + "/"
	}
	return "/Users/user/"
}