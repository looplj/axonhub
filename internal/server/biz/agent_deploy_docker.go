//nolint:gosec // G204: Subprocess launched with variable.
package biz

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/pkg/xcontext"
)

func dockerContainerName(name string) string {
	return fmt.Sprintf("axonclaw_%s", name)
}

func (svc *AgentDeployService) deployToDocker(ctx context.Context, runtime *ent.AgentHost, apiKey *ent.APIKey, name, baseURL string) error {
	isLocalhost := runtime.Addr == "localhost" || runtime.Addr == "127.0.0.1"
	containerName := fmt.Sprintf("axonclaw-%s", name)

	imageName := "looplj/axonclaw:latest"
	if debugDockerImage != "" {
		imageName = debugDockerImage
	}

	deployCtx, deployCancel := xcontext.DetachWithTimeout(ctx, 5*time.Minute)
	defer deployCancel()

	if isLocalhost {
		_ = exec.CommandContext(deployCtx, "docker", "stop", containerName).Run()
		_ = exec.CommandContext(deployCtx, "docker", "rm", containerName).Run()

		dockerEnv := append([]string{}, os.Environ()...)
		dockerEnv = overrideEnv(dockerEnv, "AXONCLAW_NAME", name)
		dockerEnv = overrideEnv(dockerEnv, "AXONCLAW_BASE_URL", baseURL)
		dockerEnv = overrideEnv(dockerEnv, "AXONCLAW_API_KEY", apiKey.Key)

		runArgs := []string{
			"run", "-d",
			"--name", containerName,
			"--restart", "unless-stopped",
			"-e", "AXONCLAW_NAME",
			"-e", "AXONCLAW_BASE_URL",
			"-e", "AXONCLAW_API_KEY",
			imageName,
		}
		cmd := exec.CommandContext(deployCtx, "docker", runArgs...)

		cmd.Env = dockerEnv
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("failed to start Docker container: %w, output: %s", err, string(output))
		}

		time.Sleep(2 * time.Second)

		checkArgs := []string{"inspect", "--format={{.State.Running}}", containerName}

		output, err := exec.CommandContext(deployCtx, "docker", checkArgs...).Output()
		if err != nil {
			return fmt.Errorf("failed to check container status: %w", err)
		}

		if string(output) != "true\n" {
			logsOutput, _ := exec.CommandContext(deployCtx, "docker", "logs", containerName).CombinedOutput()

			return fmt.Errorf("container is not running. Logs: %s", string(logsOutput))
		}

		return nil
	}

	client, err := sshDial(runtime)
	if err != nil {
		return err
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create SSH session: %w", err)
	}
	defer session.Close()

	stopCmd := fmt.Sprintf("docker stop %s 2>/dev/null || true", containerName)
	if err := session.Run(stopCmd); err != nil {
		return fmt.Errorf("failed to stop existing container: %w", err)
	}

	session2, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create SSH session: %w", err)
	}
	defer session2.Close()

	rmCmd := fmt.Sprintf("docker rm %s 2>/dev/null || true", containerName)
	if err := session2.Run(rmCmd); err != nil {
		return fmt.Errorf("failed to remove existing container: %w", err)
	}

	session3, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create SSH session: %w", err)
	}
	defer session3.Close()

	runCmd := fmt.Sprintf(
		"docker run -d --name %s --restart unless-stopped -e AXONCLAW_NAME=%s -e AXONCLAW_BASE_URL=%s -e AXONCLAW_API_KEY=%s %s",
		shellQuote(containerName),
		shellQuote(name),
		shellQuote(baseURL),
		shellQuote(apiKey.Key),
		shellQuote(imageName),
	)
	if output, err := session3.CombinedOutput(runCmd); err != nil {
		return fmt.Errorf("failed to start Docker container: %w, output: %s", err, string(output))
	}

	session4, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create SSH session: %w", err)
	}
	defer session4.Close()

	time.Sleep(2 * time.Second)

	checkCmd := fmt.Sprintf("docker inspect --format='{{.State.Running}}' %s", containerName)

	output, err := session4.CombinedOutput(checkCmd)
	if err != nil {
		return fmt.Errorf("failed to check container status: %w", err)
	}

	if string(output) != "true\n" {
		logsSession, _ := client.NewSession()
		if logsSession != nil {
			defer logsSession.Close()

			logsCmd := fmt.Sprintf("docker logs %s", containerName)
			logsOutput, _ := logsSession.CombinedOutput(logsCmd)

			return fmt.Errorf("container is not running. Logs: %s", string(logsOutput))
		}

		return fmt.Errorf("container is not running")
	}

	return nil
}

func (svc *AgentDeployService) dockerStop(ctx context.Context, runtime *ent.AgentHost, containerName string) error {
	isLocalhost := runtime.Addr == "localhost" || runtime.Addr == "127.0.0.1"
	if isLocalhost {
		cmd := exec.CommandContext(ctx, "docker", "stop", containerName)
		_ = cmd.Run()

		return nil
	}

	client, err := sshDial(runtime)
	if err != nil {
		return err
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("create ssh session: %w", err)
	}
	defer session.Close()

	stopCmd := fmt.Sprintf("docker stop %s 2>/dev/null || true", shellQuote(containerName))
	if err := session.Run(stopCmd); err != nil {
		return fmt.Errorf("docker stop: %w", err)
	}

	return nil
}

func (svc *AgentDeployService) dockerStart(ctx context.Context, runtime *ent.AgentHost, containerName string) error {
	isLocalhost := runtime.Addr == "localhost" || runtime.Addr == "127.0.0.1"
	if isLocalhost {
		if err := exec.CommandContext(ctx, "docker", "start", containerName).Run(); err != nil {
			return fmt.Errorf("docker start: %w", err)
		}

		return nil
	}

	client, err := sshDial(runtime)
	if err != nil {
		return err
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("create ssh session: %w", err)
	}
	defer session.Close()

	startCmd := fmt.Sprintf("docker start %s", shellQuote(containerName))
	if err := session.Run(startCmd); err != nil {
		return fmt.Errorf("docker start: %w", err)
	}

	return nil
}

func (svc *AgentDeployService) dockerRestart(ctx context.Context, runtime *ent.AgentHost, containerName string) error {
	isLocalhost := runtime.Addr == "localhost" || runtime.Addr == "127.0.0.1"
	if isLocalhost {
		if err := exec.CommandContext(ctx, "docker", "restart", containerName).Run(); err != nil {
			return fmt.Errorf("docker restart: %w", err)
		}

		return nil
	}

	client, err := sshDial(runtime)
	if err != nil {
		return err
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("create ssh session: %w", err)
	}
	defer session.Close()

	restartCmd := fmt.Sprintf("docker restart %s", shellQuote(containerName))
	if err := session.Run(restartCmd); err != nil {
		return fmt.Errorf("docker restart: %w", err)
	}

	return nil
}

func (svc *AgentDeployService) dockerRedeploy(ctx context.Context, runtime *ent.AgentHost, apiKey *ent.APIKey, name, containerName, baseURL string) error {
	imageName := "looplj/axonclaw:latest"
	if debugDockerImage != "" {
		imageName = debugDockerImage
	}

	redeployCtx, redeployCancel := context.WithTimeout(ctx, 5*time.Minute)
	defer redeployCancel()

	isLocalhost := runtime.Addr == "localhost" || runtime.Addr == "127.0.0.1"
	if isLocalhost {
		_ = exec.CommandContext(redeployCtx, "docker", "stop", containerName).Run()
		_ = exec.CommandContext(redeployCtx, "docker", "rm", containerName).Run()

		dockerEnv := append([]string{}, os.Environ()...)
		dockerEnv = overrideEnv(dockerEnv, "AXONCLAW_NAME", name)
		dockerEnv = overrideEnv(dockerEnv, "AXONCLAW_BASE_URL", baseURL)
		dockerEnv = overrideEnv(dockerEnv, "AXONCLAW_API_KEY", apiKey.Key)

		runArgs := []string{
			"run", "-d",
			"--name", containerName,
			"--restart", "unless-stopped",
			"-e", "AXONCLAW_NAME",
			"-e", "AXONCLAW_BASE_URL",
			"-e", "AXONCLAW_API_KEY",
			imageName,
		}
		cmd := exec.CommandContext(redeployCtx, "docker", runArgs...)

		cmd.Env = dockerEnv
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("docker run: %w, output: %s", err, string(output))
		}

		return nil
	}

	client, err := sshDial(runtime)
	if err != nil {
		return err
	}
	defer client.Close()

	runSSH := func(cmd string) ([]byte, error) {
		s, err := client.NewSession()
		if err != nil {
			return nil, fmt.Errorf("create ssh session: %w", err)
		}
		defer s.Close()

		return s.CombinedOutput(cmd)
	}

	_, _ = runSSH(fmt.Sprintf("docker stop %s 2>/dev/null || true", shellQuote(containerName)))
	_, _ = runSSH(fmt.Sprintf("docker rm %s 2>/dev/null || true", shellQuote(containerName)))

	runCmd := fmt.Sprintf(
		"docker run -d --name %s --restart unless-stopped -e AXONCLAW_NAME=%s -e AXONCLAW_BASE_URL=%s -e AXONCLAW_API_KEY=%s %s",
		shellQuote(containerName),
		shellQuote(name),
		shellQuote(baseURL),
		shellQuote(apiKey.Key),
		shellQuote(imageName),
	)
	if output, err := runSSH(runCmd); err != nil {
		return fmt.Errorf("docker run: %w, output: %s", err, string(output))
	}

	return nil
}
