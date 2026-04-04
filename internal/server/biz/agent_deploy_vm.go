//nolint:gosec // G204: Subprocess launched with variable.
package biz

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/agenthost"
)

func (svc *AgentDeployService) deployToVM(ctx context.Context, runtime *ent.AgentHost, apiKey *ent.APIKey, name, directory, baseURL string) error {
	isLocalhost := runtime.Addr == "localhost" || runtime.Addr == "127.0.0.1"

	if isLocalhost {
		if err := os.MkdirAll(directory, 0o755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", directory, err)
		}

		if debugLocalPath != "" {
			if _, err := os.Stat(debugLocalPath); os.IsNotExist(err) {
				return fmt.Errorf("debug package not found at %s", debugLocalPath)
			}

			//nolint:gosec
			unzipCmd := fmt.Sprintf(
				"unzip -o %s -d %s && chmod +x %s/start.sh %s/stop.sh",
				shellQuote(debugLocalPath),
				shellQuote(directory),
				shellQuote(directory),
				shellQuote(directory),
			)
			if err := exec.CommandContext(ctx, "sh", "-c", unzipCmd).Run(); err != nil {
				return fmt.Errorf("failed to unzip debug package: %w", err)
			}

			//nolint:gosec
			startCmd := fmt.Sprintf(
				"cd %s && AXONCLAW_NAME=%s AXONCLAW_BASE_URL=%s AXONCLAW_API_KEY=%s ./start.sh",
				shellQuote(directory),
				shellQuote(name),
				shellQuote(baseURL),
				shellQuote(apiKey.Key),
			)
			if err := exec.CommandContext(ctx, "sh", "-c", startCmd).Run(); err != nil {
				return fmt.Errorf("failed to start debug axonclaw: %w", err)
			}

			return nil
		}

		//nolint:gosec
		deployCmd := fmt.Sprintf(
			"cd %s && curl -sSL https://raw.githubusercontent.com/looplj/axonhub/unstable/cmd/axonclaw/install.sh | AXONCLAW_NAME=%s AXONCLAW_BASE_URL=%s AXONCLAW_API_KEY=%s bash",
			shellQuote(directory),
			shellQuote(name),
			shellQuote(baseURL),
			shellQuote(apiKey.Key),
		)
		if err := exec.CommandContext(ctx, "bash", "-c", deployCmd).Run(); err != nil {
			return fmt.Errorf("failed to deploy axonclaw: %w", err)
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

	mkdirCmd := fmt.Sprintf("mkdir -p %s", directory)
	if err := session.Run(mkdirCmd); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", directory, err)
	}

	session2, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create SSH session: %w", err)
	}
	defer session2.Close()

	deployCmd := fmt.Sprintf(
		"cd %s && curl -sSL https://raw.githubusercontent.com/looplj/axonhub/unstable/cmd/axonclaw/install.sh | AXONCLAW_NAME=%s AXONCLAW_BASE_URL=%s AXONCLAW_API_KEY=%s bash",
		shellQuote(directory),
		shellQuote(name),
		shellQuote(baseURL),
		shellQuote(apiKey.Key),
	)

	output, err := session2.CombinedOutput(deployCmd)
	if err != nil {
		return fmt.Errorf("failed to deploy axonclaw: %w, output: %s", err, string(output))
	}

	return nil
}

func (svc *AgentDeployService) vmStop(ctx context.Context, runtime *ent.AgentHost, directory string) error {
	isLocalhost := runtime.Addr == "localhost" || runtime.Addr == "127.0.0.1"
	if isLocalhost {
		cmd := exec.CommandContext(ctx, "./stop.sh") //nolint:gosec

		cmd.Dir = directory
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("stop axonclaw: %w", err)
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

	stopCmd := fmt.Sprintf("cd %s && ./stop.sh", shellQuote(directory))
	if err := session.Run(stopCmd); err != nil {
		return fmt.Errorf("stop axonclaw: %w", err)
	}

	return nil
}

func (svc *AgentDeployService) vmStart(ctx context.Context, runtime *ent.AgentHost, apiKey *ent.APIKey, name, directory, baseURL string) error {
	isLocalhost := runtime.Addr == "localhost" || runtime.Addr == "127.0.0.1"
	if isLocalhost {
		cmd := exec.CommandContext(ctx, "./start.sh") //nolint:gosec
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

	startCmd := fmt.Sprintf(
		"cd %s && AXONCLAW_NAME=%s AXONCLAW_BASE_URL=%s AXONCLAW_API_KEY=%s ./start.sh",
		shellQuote(directory),
		shellQuote(name),
		shellQuote(baseURL),
		shellQuote(apiKey.Key),
	)
	if err := session.Run(startCmd); err != nil {
		return fmt.Errorf("start axonclaw: %w", err)
	}

	return nil
}

func (svc *AgentDeployService) vmRestart(ctx context.Context, runtime *ent.AgentHost, apiKey *ent.APIKey, name, directory, baseURL string) error {
	isLocalhost := runtime.Addr == "localhost" || runtime.Addr == "127.0.0.1"
	if isLocalhost {
		cmd := exec.CommandContext(ctx, "./restart.sh") //nolint:gosec
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

	restartCmd := fmt.Sprintf(
		"cd %s && AXONCLAW_NAME=%s AXONCLAW_BASE_URL=%s AXONCLAW_API_KEY=%s ./restart.sh",
		shellQuote(directory),
		shellQuote(name),
		shellQuote(baseURL),
		shellQuote(apiKey.Key),
	)
	if err := session.Run(restartCmd); err != nil {
		return fmt.Errorf("restart axonclaw: %w", err)
	}

	return nil
}

func (svc *AgentDeployService) vmInstallLatest(ctx context.Context, runtime *ent.AgentHost, apiKey *ent.APIKey, name, directory, baseURL string) error {
	isLocalhost := runtime.Addr == "localhost" || runtime.Addr == "127.0.0.1"

	if isLocalhost {
		if debugLocalPath != "" {
			if _, err := os.Stat(debugLocalPath); os.IsNotExist(err) {
				return fmt.Errorf("debug package not found at %s", debugLocalPath)
			}

			//nolint:gosec
			unzipCmd := fmt.Sprintf(
				"unzip -o %s -d %s && chmod +x %s/start.sh %s/stop.sh",
				shellQuote(debugLocalPath),
				shellQuote(directory),
				shellQuote(directory),
				shellQuote(directory),
			)
			if err := exec.CommandContext(ctx, "sh", "-c", unzipCmd).Run(); err != nil {
				return fmt.Errorf("failed to unzip debug package: %w", err)
			}

			return nil
		}

		cmd := exec.CommandContext(ctx, "bash", "-c", "curl -sSL https://raw.githubusercontent.com/looplj/axonhub/unstable/cmd/axonclaw/install.sh | bash") //nolint:gosec
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

	installCmd := fmt.Sprintf(
		"cd %s && curl -sSL https://raw.githubusercontent.com/looplj/axonhub/unstable/cmd/axonclaw/install.sh | AXONCLAW_NAME=%s AXONCLAW_BASE_URL=%s AXONCLAW_API_KEY=%s bash",
		shellQuote(directory),
		shellQuote(name),
		shellQuote(baseURL),
		shellQuote(apiKey.Key),
	)

	output, err := session.CombinedOutput(installCmd)
	if err != nil {
		return fmt.Errorf("install latest axonclaw: %w, output: %s", err, string(output))
	}

	return nil
}

func sshDial(runtime *ent.AgentHost) (*ssh.Client, error) {
	authMethods, err := buildSSHAuthMethods(runtime)
	if err != nil {
		return nil, err
	}

	config := &ssh.ClientConfig{
		User: runtime.User,
		Auth: authMethods,
		//nolint:gosec // ignore G202, it's a test environment.
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         30 * time.Second,
	}

	host := runtime.Addr
	if !strings.Contains(host, ":") {
		host = host + ":22"
	}

	client, err := ssh.Dial("tcp", host, config)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to host %s: %w", host, err)
	}

	return client, nil
}

func buildSSHAuthMethods(runtime *ent.AgentHost) ([]ssh.AuthMethod, error) {
	if runtime.AuthMethod == agenthost.AuthMethodSSHKey {
		signer, err := ssh.ParsePrivateKey([]byte(runtime.SSHPrivateKey))
		if err != nil {
			return nil, fmt.Errorf("failed to parse SSH private key: %w", err)
		}

		return []ssh.AuthMethod{ssh.PublicKeys(signer)}, nil
	}

	return []ssh.AuthMethod{ssh.Password(runtime.Password)}, nil
}
