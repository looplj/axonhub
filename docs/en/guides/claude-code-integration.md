# Claude Code Integration Guide

---

## Overview
AxonHub can act as a drop-in replacement for Anthropic endpoints, letting Claude Code connect through your own infrastructure. This guide explains how to configure Claude Code and how to combine it with AxonHub model profiles for flexible routing.

### Key Points
- AxonHub performs AI protocol/format transformation. You can configure multiple upstream channels (providers) and expose a single Anthropic-compatible interface for Claude Code.
- You can aggregate Claude Code requests from the same session into one trace (see "Configure Claude Code").

### Prerequisites
- AxonHub instance reachable from your development machine.
- Valid AxonHub API key with project access.
- Access to Claude Code (Anthropic) application.
- Optional: one or more model profiles configured in the AxonHub console.

### Configure Claude Code
1. Open your shell environment and export the AxonHub credentials:
   ```bash
   export ANTHROPIC_AUTH_TOKEN="<your-axonhub-api-key>"
   export ANTHROPIC_BASE_URL="http://localhost:8090/anthropic"
   ```
2. Launch Claude Code. It will read the environment variables and route all Anthropic requests through AxonHub.
3. (Optional) Confirm the integration by triggering a chat completion and checking AxonHub traces.

#### Trace aggregation (important)
To aggregate requests from the same Claude Code session into a single trace, enable the following in `config.yml`:

```yaml
server:
  trace:
    claude_code_trace_enabled: true
```

**Note**: Enabling this also ensures that requests from the same trace are prioritized to be sent to the same upstream channel, significantly improving provider-side cache hit rates (e.g., Anthropic Prompt Caching).

#### Tips
- Keep your API key secret; store it in a shell profile or secret manager.
- If your AxonHub endpoint uses HTTPS with a self-signed certificate, configure trust settings in your OS.

### Working with Model Profiles
AxonHub model profiles remap incoming model names to provider-specific equivalents:
- Create a profile in the AxonHub console and add mapping rules (exact name or regex).
- Assign the profile to your API key.
- Switch active profiles to alter Claude Code/Codex behavior without changing tool settings.

<table>
  <tr align="center">
    <td align="center">
      <a href="../../screenshots/axonhub-profiles.png">
        <img src="../../screenshots/axonhub-profiles.png" alt="Model Profiles" width="250"/>
      </a>
      <br/>
      Model Profiles
    </td>
  </tr>
</table>

#### Example
- Request `claude-sonnet-4-5` → mapped to `deepseek-reasoner` for getting more accurate responses.
- Request `claude-haiku-4-5` → mapped to `deepseek-chat` for reducing costs.

### Troubleshooting
- **Claude Code cannot connect**: verify `ANTHROPIC_BASE_URL` points to the `/anthropic` path and that your firewall allows outbound calls.
- **Unexpected model responses**: review active profile mappings in the AxonHub console; disable or adjust rules if necessary.

---

## Using Claude Code as a Provider Channel

AxonHub can also use your Claude Code subscription as a backend provider, allowing non-Claude Code tools to leverage Claude Code's capabilities. This is useful when you want to route requests from other applications (OpenAI-compatible clients, custom tools, etc.) through Claude Code.

### Prerequisites
- Claude Code CLI installed (https://claude.com/claude-code)
- Valid Anthropic account with Claude Code subscription
- AxonHub instance with channel management access

### Getting an Authentication Token

To configure Claude Code as a provider channel, you need a long-lived authentication token:

1. Run the token setup command:
   ```bash
   claude setup-token
   ```

2. You will be prompted to authenticate with your Anthropic account through your browser

3. Upon successful authentication, a long-lived token starting with `sk-ant` will be printed to your terminal:
   ```
   Your authentication token: sk-ant-api03-xyz...
   ```

4. Copy this token - you'll use it in the AxonHub channel configuration

### Configuring the Channel

1. Navigate to the **Channels** section in the AxonHub management interface

2. Create a new channel with the following configuration:
   - **Type**: `claude-code`
   - **Name**: A descriptive name (e.g., "Claude Code Provider")
   - **Base URL**: This field will be overridden to the standard Claude Code base URL
   - **API Key**: The token from `claude setup-token` (starts with `sk-ant`)
   - **Supported Models**: Add the Claude models you want to expose:
     - `claude-haiku-4-5`
     - `claude-sonnet-4-5`
     - `claude-opus-4-5`

     Note: These are the unversioned 'latest' variants. You can also use specific versioned model names (e.g., `claude-sonnet-4-5-20250514`) if you prefer to pin to a specific version.

3. Test the connection using the **Test** button

4. Enable the channel once the test succeeds

### Use Cases

- **Multi-Tool Access**: Allow multiple applications to share your Claude Code subscription through AxonHub
- **Cost Management**: Use Claude Code alongside other providers with load balancing and failover
- **Extended Context**: Route requests requiring large context windows through Claude Code
- **Model Flexibility**: Combine Claude Code with other providers using model profiles for intelligent routing

### Troubleshooting

- **Channel test fails**: Ensure Claude Code server is running and accessible at the configured base URL
- **Authentication errors**: Verify the token from `claude setup-token` is correct and hasn't expired
- **Network issues**: If using a remote Claude Code instance, check firewall rules and network connectivity
- **Model not available**: Confirm the requested model is listed in the channel's `supported_models`

---

### Related Documentation
- [Tracing Guide](tracing.md)
- [Chat Completions](../api-reference/unified-api.md#openai-chat-completions-api)
- [Codex Integration Guide](codex-integration.md)
- [Channel Management Guide](channel-management.md)
- README sections on [Usage Guide](../../../README.md#usage-guide)
