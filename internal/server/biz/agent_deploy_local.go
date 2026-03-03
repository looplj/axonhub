//nolint:gosec // G204: Subprocess launched with variable.
package biz

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"

	"github.com/looplj/axonhub/internal/ent"
)

func isWindows() bool {
	return runtime.GOOS == "windows"
}

func (svc *AgentDeployService) deployToLocal(ctx context.Context, runtime *ent.AgentRuntime, apiKey *ent.APIKey, name, directory, baseURL string) error {
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", directory, err)
	}

	if debugLocalPath != "" {
		if _, err := os.Stat(debugLocalPath); os.IsNotExist(err) {
			return fmt.Errorf("debug package not found at %s", debugLocalPath)
		}

		if isWindows() {
			return svc.deployToLocalWindows(ctx, apiKey, name, directory, baseURL)
		}

		unzipCmd := fmt.Sprintf("unzip -o %s -d %s && chmod +x %s/start.sh %s/stop.sh", debugLocalPath, directory, directory, directory)
		if err := exec.CommandContext(ctx, "sh", "-c", unzipCmd).Run(); err != nil {
			return fmt.Errorf("failed to unzip debug package: %w", err)
		}

		startCmd := fmt.Sprintf("cd %s && AXONCLAW_NAME=%s AXONCLAW_BASE_URL=%s AXONCLAW_API_KEY=%s ./start.sh", directory, name, baseURL, apiKey.Key)
		if err := exec.CommandContext(ctx, "sh", "-c", startCmd).Run(); err != nil {
			return fmt.Errorf("failed to start debug axonclaw: %w", err)
		}

		return nil
	}

	return svc.localInstallLatest(ctx, apiKey, name, directory, baseURL)
}

func (svc *AgentDeployService) deployToLocalWindows(ctx context.Context, apiKey *ent.APIKey, name, directory, baseURL string) error {
	expandCmd := fmt.Sprintf("Expand-Archive -Path '%s' -DestinationPath '%s' -Force", debugLocalPath, directory)
	if err := exec.CommandContext(ctx, "powershell", "-Command", expandCmd).Run(); err != nil {
		return fmt.Errorf("failed to expand archive: %w", err)
	}

	startCmd := fmt.Sprintf("cd %s; $env:AXONCLAW_NAME='%s'; $env:AXONCLAW_BASE_URL='%s'; $env:AXONCLAW_API_KEY='%s'; .\\start.bat", directory, name, baseURL, apiKey.Key)
	if err := exec.CommandContext(ctx, "powershell", "-Command", startCmd).Run(); err != nil {
		return fmt.Errorf("failed to start axonclaw: %w", err)
	}

	return nil
}

func (svc *AgentDeployService) localStop(ctx context.Context, directory string) error {
	if isWindows() {
		cmd := exec.CommandContext(ctx, "powershell", "-Command", fmt.Sprintf("cd %s; .\\stop.bat", directory))

		cmd.Dir = directory
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("stop axonclaw: %w", err)
		}

		return nil
	}

	cmd := exec.CommandContext(ctx, "./stop.sh")

	cmd.Dir = directory
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("stop axonclaw: %w", err)
	}

	return nil
}

func (svc *AgentDeployService) localStart(ctx context.Context, apiKey *ent.APIKey, name, directory, baseURL string) error {
	if isWindows() {
		cmd := exec.CommandContext(ctx, "powershell", "-Command", fmt.Sprintf("cd %s; $env:AXONCLAW_NAME='%s'; $env:AXONCLAW_BASE_URL='%s'; $env:AXONCLAW_API_KEY='%s'; .\\start.bat", directory, name, baseURL, apiKey.Key))

		cmd.Dir = directory
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("start axonclaw: %w", err)
		}

		return nil
	}

	cmd := exec.CommandContext(ctx, "./start.sh")
	cmd.Dir = directory

	cmd.Env = append(os.Environ(),
		"AXONCLAW_NAME="+name,
		"AXONCLAW_BASE_URL="+baseURL,
		"AXONCLAW_API_KEY="+apiKey.Key,
	)

	setProcessGroup(cmd)

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("start axonclaw: %w", err)
	}

	return nil
}

func (svc *AgentDeployService) localRestart(ctx context.Context, apiKey *ent.APIKey, name, directory, baseURL string) error {
	if isWindows() {
		cmd := exec.CommandContext(ctx, "powershell", "-Command", fmt.Sprintf("cd %s; $env:AXONCLAW_NAME='%s'; $env:AXONCLAW_BASE_URL='%s'; $env:AXONCLAW_API_KEY='%s'; .\\restart.bat", directory, name, baseURL, apiKey.Key))

		cmd.Dir = directory
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("restart axonclaw: %w", err)
		}

		return nil
	}

	cmd := exec.CommandContext(ctx, "./restart.sh")
	cmd.Dir = directory

	cmd.Env = append(os.Environ(),
		"AXONCLAW_NAME="+name,
		"AXONCLAW_BASE_URL="+baseURL,
		"AXONCLAW_API_KEY="+apiKey.Key,
	)

	setProcessGroup(cmd)

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("restart axonclaw: %w", err)
	}

	return nil
}

func (svc *AgentDeployService) localInstallLatest(ctx context.Context, apiKey *ent.APIKey, name, directory, baseURL string) error {
	if debugLocalPath != "" {
		if _, err := os.Stat(debugLocalPath); os.IsNotExist(err) {
			return fmt.Errorf("debug package not found at %s", debugLocalPath)
		}

		if isWindows() {
			expandCmd := fmt.Sprintf("Expand-Archive -Path '%s' -DestinationPath '%s' -Force", debugLocalPath, directory)
			if err := exec.CommandContext(ctx, "powershell", "-Command", expandCmd).Run(); err != nil {
				return fmt.Errorf("failed to expand debug archive: %w", err)
			}

			return nil
		}

		unzipCmd := fmt.Sprintf("unzip -o %s -d %s && chmod +x %s/start.sh %s/stop.sh", debugLocalPath, directory, directory, directory)
		if err := exec.CommandContext(ctx, "sh", "-c", unzipCmd).Run(); err != nil {
			return fmt.Errorf("failed to unzip debug package: %w", err)
		}

		return nil
	}

	if isWindows() {
		installCmd := fmt.Sprintf("cd %s; $env:AXONCLAW_NAME='%s'; $env:AXONCLAW_BASE_URL='%s'; $env:AXONCLAW_API_KEY='%s'; Invoke-Expression (Invoke-WebRequest -Uri 'https://raw.githubusercontent.com/looplj/axonhub/unstable/cmd/axonclaw/install.ps1' -UseBasicParsing).Content", directory, name, baseURL, apiKey.Key)
		cmd := exec.CommandContext(ctx, "powershell", "-Command", installCmd)

		cmd.Dir = directory
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("install latest axonclaw: %w", err)
		}

		return nil
	}

	cmd := exec.CommandContext(ctx, "sh", "-c", "curl -sSL https://raw.githubusercontent.com/looplj/axonhub/unstable/cmd/axonclaw/install.sh | sh")
	cmd.Dir = directory

	cmd.Env = append(os.Environ(),
		"AXONCLAW_NAME="+name,
		"AXONCLAW_BASE_URL="+baseURL,
		"AXONCLAW_API_KEY="+apiKey.Key,
	)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("install latest axonclaw: %w", err)
	}

	return nil
}
