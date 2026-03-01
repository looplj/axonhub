//nolint:gosec // G204: Subprocess launched with variable.
package biz

import (
	"context"
	"fmt"
	"os/exec"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/looplj/axonhub/internal/ent"
)

func dockerContainerName(name string) string {
	return fmt.Sprintf("axonclaw-%s", name)
}

func (svc *AgentDeployService) deployToDocker(ctx context.Context, runtime *ent.AgentRuntime, apiKey *ent.APIKey, name, baseURL string) error {
	isLocalhost := runtime.Host == "localhost" || runtime.Host == "127.0.0.1"
	containerName := fmt.Sprintf("axonclaw-%s", name)

	imageName := "axonclaw/axonclaw:latest"
	if debugDockerImage != "" {
		imageName = debugDockerImage
	}

	if isLocalhost {
		//nolint:gosec
		stopCmd := exec.CommandContext(ctx, "sh", "-c", fmt.Sprintf("docker stop %s 2>/dev/null || true", containerName))
		if err := stopCmd.Run(); err != nil {
			return fmt.Errorf("failed to stop existing container: %w", err)
		}

		//nolint:gosec
		rmCmd := exec.CommandContext(ctx, "sh", "-c", fmt.Sprintf("docker rm %s 2>/dev/null || true", containerName))
		if err := rmCmd.Run(); err != nil {
			return fmt.Errorf("failed to remove existing container: %w", err)
		}

		if debugDockerImage == "" {
			//nolint:gosec
			pullCmd := exec.CommandContext(ctx, "sh", "-c", fmt.Sprintf("docker pull %s", imageName))
			if err := pullCmd.Run(); err != nil {
				return fmt.Errorf("failed to pull latest image: %w", err)
			}
		}

		//nolint:gosec
		runCmd := exec.CommandContext(ctx, "sh", "-c", fmt.Sprintf("docker run -d --name %s --restart unless-stopped -e AXONCLAW_NAME=%s -e AXONCLAW_BASE_URL=%s -e AXONCLAW_API_KEY=%s %s", containerName, name, baseURL, apiKey.Key, imageName))
		if err := runCmd.Run(); err != nil {
			return fmt.Errorf("failed to start Docker container: %w", err)
		}

		time.Sleep(2 * time.Second)

		//nolint:gosec
		checkCmd := exec.CommandContext(ctx, "sh", "-c", fmt.Sprintf("docker inspect --format='{{.State.Running}}' %s", containerName))

		output, err := checkCmd.Output()
		if err != nil {
			return fmt.Errorf("failed to check container status: %w", err)
		}

		if string(output) != "true\n" {
			//nolint:gosec
			logsCmd := exec.CommandContext(ctx, "sh", "-c", fmt.Sprintf("docker logs %s", containerName))
			logsOutput, _ := logsCmd.CombinedOutput()

			return fmt.Errorf("container is not running. Logs: %s", string(logsOutput))
		}

		return nil
	}

	config := &ssh.ClientConfig{
		User: runtime.User,
		Auth: []ssh.AuthMethod{
			ssh.Password(runtime.Password),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         30 * time.Second,
	}

	client, err := ssh.Dial("tcp", runtime.Host, config)
	if err != nil {
		return fmt.Errorf("failed to connect to Docker host %s: %w", runtime.Host, err)
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

	pullCmd := fmt.Sprintf("docker pull %s", imageName)
	if err := session3.Run(pullCmd); err != nil {
		return fmt.Errorf("failed to pull latest image: %w", err)
	}

	session4, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create SSH session: %w", err)
	}
	defer session4.Close()

	runCmd := fmt.Sprintf("docker run -d --name %s --restart unless-stopped -e AXONCLAW_NAME=%s -e AXONCLAW_BASE_URL=%s -e AXONCLAW_API_KEY=%s %s", containerName, name, baseURL, apiKey.Key, imageName)
	if err := session4.Run(runCmd); err != nil {
		return fmt.Errorf("failed to start Docker container: %w", err)
	}

	session5, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create SSH session: %w", err)
	}
	defer session5.Close()

	time.Sleep(2 * time.Second)

	checkCmd := fmt.Sprintf("docker inspect --format='{{.State.Running}}' %s", containerName)

	output, err := session5.CombinedOutput(checkCmd)
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

func (svc *AgentDeployService) dockerStop(ctx context.Context, runtime *ent.AgentRuntime, containerName string) error {
	isLocalhost := runtime.Host == "localhost" || runtime.Host == "127.0.0.1"
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

func (svc *AgentDeployService) dockerStart(ctx context.Context, runtime *ent.AgentRuntime, containerName string) error {
	isLocalhost := runtime.Host == "localhost" || runtime.Host == "127.0.0.1"
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

func (svc *AgentDeployService) dockerRestart(ctx context.Context, runtime *ent.AgentRuntime, containerName string) error {
	isLocalhost := runtime.Host == "localhost" || runtime.Host == "127.0.0.1"
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

func (svc *AgentDeployService) dockerRedeploy(ctx context.Context, runtime *ent.AgentRuntime, apiKey *ent.APIKey, name, containerName, baseURL string) error {
	imageName := "axonclaw/axonclaw:latest"
	if debugDockerImage != "" {
		imageName = debugDockerImage
	}

	isLocalhost := runtime.Host == "localhost" || runtime.Host == "127.0.0.1"
	if isLocalhost {
		_ = exec.CommandContext(ctx, "docker", "stop", containerName).Run()
		_ = exec.CommandContext(ctx, "docker", "rm", containerName).Run()

		if debugDockerImage == "" {
			if err := exec.CommandContext(ctx, "docker", "pull", imageName).Run(); err != nil {
				return fmt.Errorf("docker pull: %w", err)
			}
		}

		runArgs := []string{
			"run", "-d",
			"--name", containerName,
			"--restart", "unless-stopped",
			"-e", "AXONCLAW_NAME=" + name,
			"-e", "AXONCLAW_BASE_URL=" + baseURL,
			"-e", "AXONCLAW_API_KEY=" + apiKey.Key,
			imageName,
		}
		if err := exec.CommandContext(ctx, "docker", runArgs...).Run(); err != nil {
			return fmt.Errorf("docker run: %w", err)
		}

		return nil
	}

	client, err := sshDial(runtime)
	if err != nil {
		return err
	}
	defer client.Close()

	runSSH := func(cmd string) error {
		s, err := client.NewSession()
		if err != nil {
			return fmt.Errorf("create ssh session: %w", err)
		}
		defer s.Close()

		return s.Run(cmd)
	}

	if debugDockerImage == "" {
		if err := runSSH(fmt.Sprintf("docker pull %s", shellQuote(imageName))); err != nil {
			return fmt.Errorf("docker pull: %w", err)
		}
	}

	_ = runSSH(fmt.Sprintf("docker stop %s 2>/dev/null || true", shellQuote(containerName)))
	_ = runSSH(fmt.Sprintf("docker rm %s 2>/dev/null || true", shellQuote(containerName)))

	runCmd := fmt.Sprintf(
		"docker run -d --name %s --restart unless-stopped -e AXONCLAW_NAME=%s -e AXONCLAW_BASE_URL=%s -e AXONCLAW_API_KEY=%s %s",
		shellQuote(containerName),
		shellQuote(name),
		shellQuote(baseURL),
		shellQuote(apiKey.Key),
		shellQuote(imageName),
	)
	if err := runSSH(runCmd); err != nil {
		return fmt.Errorf("docker run: %w", err)
	}

	return nil
}
