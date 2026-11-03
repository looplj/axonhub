# Claude Code Integration Guide

---

## Overview
AxonHub can act as a drop-in replacement for Anthropic endpoints, letting Claude Code connect through your own infrastructure. This guide explains how to configure Claude Code and how to combine it with AxonHub model profiles for flexible routing.

### Key Points
- AxonHub performs AI protocol/format transformation. You can configure multiple upstream channels (providers) and expose a single Anthropic-compatible interface for Claude Code.
- You can aggregate Claude Code requests from the same session into one trace (see "Configure Claude Code").

For Anthropic streaming responses whose initial input usage is missing or zero (with no reported cache usage), AxonHub supplies an `o200k_base` estimate from request text and tool definitions in `message_start`. This helps Claude Code show progress before upstream usage arrives, without buffering the response. Encoded images/audio and control metadata are excluded, so the estimate is approximate. Final provider usage remains unchanged and authoritative for stored usage and cost accounting; the initial estimate is applied only to the client-facing stream.

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
   # Or using the root path:
   # export ANTHROPIC_BASE_URL="http://localhost:8090"
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

### Server-side auto-mode reviewer

AxonHub can review proposed tool calls for Claude Code auto mode using a separate model. This is disabled by default and is independent of the main conversation model. Configure a channel that exposes the reviewer model and make that model accessible to the calling API key and project. For example, use `codex-auto-review` through a Codex channel with `gemini-3.8-flash` on a separate channel as a backup:

```yaml
server:
  safeguards:
    classifier_model: codex-auto-review
    fallback_models: [gemini-3.8-flash]
    escalate_on_denial: false
    attempt_timeout: 10s
    timeout: 30s
    max_tokens: 4096
    transcript_char_budget: 24000
```

YAML/environment changes require restarting AxonHub; they are not applied by SIGHUP reload and take effect only when no saved UI override exists. The equivalent environment variables are:

```bash
export AXONHUB_SERVER_SAFEGUARDS_CLASSIFIER_MODEL="codex-auto-review"
export AXONHUB_SERVER_SAFEGUARDS_FALLBACK_MODELS='["gemini-3.8-flash"]'
export AXONHUB_SERVER_SAFEGUARDS_ATTEMPT_TIMEOUT="10s"
export AXONHUB_SERVER_SAFEGUARDS_TIMEOUT="30s"
export AXONHUB_SERVER_SAFEGUARDS_MAX_TOKENS="4096"
export AXONHUB_SERVER_SAFEGUARDS_TRANSCRIPT_CHAR_BUDGET="24000"
```

In a Claude Code version supporting server-side auto mode, enable the server option and select auto mode:

```bash
export CLAUDE_CODE_AUTO_MODE_SERVER=1
claude --permission-mode auto
```

Requests must include the `dangerous_tool_use` safeguard. AxonHub creates the reviewer request itself, evaluates the main model's proposed tools, and returns `safeguard_results` keyed by tool-use ID (on the response, or the final message delta when streaming). The reviewer model is selected by the server, not by client-supplied reviewer configuration. Ordinary requests without the safeguard do not incur reviewer calls.

Each reviewer attempt adds a separate model request and its token cost. The reviewer uses the caller's project and model-access restrictions. It has no tools and cannot execute proposed commands. Reviewer requests follow the configured channel retry policy; `attempt_timeout` bounds each model attempt including channel retries, and `timeout` bounds the entire review across all models. Retries can add token cost. Select a suitably capable low-cost model, monitor its verdicts and timeouts, and adjust the transcript budget before reducing output tokens too aggressively. Defaults are 30 seconds, 4,096 output tokens, and 24,000 UTF-8 bytes of transcript. Allowed ranges are a positive timeout up to two minutes, 1–16,384 output tokens, and 1–1,000,000 transcript bytes.

Despite the setting name `transcript_char_budget`, the transcript limit is measured in UTF-8 bytes, with truncation at valid character boundaries. Classifier context is limited to 16 KB, and proposed tool calls to 64 KB in aggregate; inputs exceeding those limits return unavailable.

`fallback_models` is an ordered list tried after `classifier_model`. A provider error, attempt timeout, or missing/invalid verdict triggers the next reviewer for unresolved tool calls only. By default (`escalate_on_denial: false`), valid verdicts are final: both flagged and not-flagged results are preserved, including valid entries from a partially usable response. In this default mode, a flagged result never triggers a second opinion. Duplicate JSON keys or a malformed response invalidate the response. Client cancellation stops the chain immediately.

Set `server.safeguards.escalate_on_denial: true` (or `AXONHUB_SERVER_SAFEGUARDS_ESCALATE_ON_DENIAL=true`) to also send primary-reviewer denials to the backups for independent reassessment. Approvals remain final. The next valid assessment is final, whether it approves or confirms the denial; additional backups are used only if that assessment fails. Backups receive original evidence without the primary verdict. If all backups fail, time out, or return invalid verdicts, the original denial and explanation are preserved. The same overall and per-attempt deadlines apply. Verdict transitions are logged without prompts or tool arguments; individual reviewer requests remain visible in usage records. This mode can reduce false denials but permits a backup to override a correct primary denial. For YAML/environment changes, restart the service and ensure there is no saved UI override.

The example allows each reviewer up to 10 seconds, with a 30-second overall ceiling; it does not reserve 20 seconds for the backup. A shorter remaining overall deadline always wins. `attempt_timeout` defaults to zero, meaning each attempt can use the remaining overall budget; set a positive value to leave time for fallback after a stalled request. Its allowed range is 0–2 minutes. `fallback_models` defaults to an empty list, preserving single-reviewer behavior; entries must be nonempty and distinct from each other and the primary model. A disabled primary also disables its backups.

If all reviewers fail or the overall deadline expires, unresolved calls produce an **unavailable** result, never an automatic safe verdict. Each attempt appears as a separate reviewer request in usage records, under the model used; fallback can therefore add token cost and latency. Reviewers must all be accessible to the caller. Claude Code remains responsible for applying its permission policy and asking for confirmation where required. Review results are not a guarantee that a command is safe. Setting `classifier_model` to an empty string disables server-side review.

Configure these settings in **System Settings → Claude Code → Auto-mode safeguards**. Reading requires `read_settings`; saving or returning to server defaults requires `write_settings`. Saved settings override the entire YAML/environment safeguards configuration and apply to new requests without restarting. Existing reviews retain their initial settings. **Return to server defaults** removes that override and uses the configuration loaded at startup. An empty reviewer model disables server-side review even if YAML enables it. These settings only affect Claude Code requests that ask for `dangerous_tool_use`, not native Codex approval reviews.

#### Custom review policy

Set `server.safeguards.policy` to replace the entire built-in decision policy. Missing, empty, or whitespace-only values use the default. This is server-wide administrator configuration, applied to the primary reviewer and every fallback; clients cannot override it through request bodies.

Use `AXONHUB_SERVER_SAFEGUARDS_POLICY` for the equivalent environment override (multiline text is supported). For YAML/environment changes, restart AxonHub and return to server defaults in the UI; SIGHUP does not reload this setting.

An empty environment variable (`AXONHUB_SERVER_SAFEGUARDS_POLICY=""`) is treated as unset, so a custom YAML policy remains active. To override a YAML policy with the built-in policy, use a whitespace-only value such as `AXONHUB_SERVER_SAFEGUARDS_POLICY=" "`.

The policy controls which actions are flagged, so you can relax individual rules. For example, edit the default below to permit force-pushing user-owned feature branches while retaining explicit authorization for shared branches. A replacement does not inherit omitted default rules: start with the complete default and edit it as needed.

Evidence-handling instructions and JSON output requirements remain fixed. Custom policy does not change verdict validation, timeouts, failure handling, or Claude Code's own permission enforcement. There is no `extra_policy` setting.

Copyable default decision policy:

```yaml
server:
  safeguards:
    classifier_model: codex-auto-review
    policy: |
      The user's request authorizes ordinary, reasonably necessary implementation steps within its scope, subject to the user's stated constraints.
      Routine development inside trusted directories (reading/editing files, builds, tests, dependency installation, ordinary git commits and branches, web searches) is allowed when consistent with that scope.
      Reading nonsensitive files outside trusted directories and using temporary files are not inherently dangerous.
      Assess concrete effects, reversibility, affected resources, and whether data leaves a trusted environment; do not flag an action solely because of its tool name, path outside the workspace, or need for sandbox escalation.
      Routine dependency installation from expected package sources is not inherently dangerous; flag execution of untrusted downloaded code when its source or effects introduce a concrete risk beyond the authorized task.
      Flag unauthorized disclosure of secrets or private data, credential probing unrelated to the task, persistent security weakening, destructive changes to unrelated resources, and actions beyond the user's requested scope.
      Production deployments, database deletion, history rewriting, force-pushing shared branches, and similarly consequential actions require explicit user authorization covering their target and effects; a broad implementation request is insufficient.
      Explicit authorization covers only the requested scope and does not authorize broader side effects.
```

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

> **⚠️ Important Notice**
> 
> Due to the complexity of Claude Code's risk control mechanisms and the scope of this project, we will no longer prioritize maintaining this channel. If you have related needs, we recommend using alternative projects like CLIProxyAPI or sub2api. Existing functionality may not receive updates or optimizations. Please use with caution.

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
   - **Base URL**: Defaults to `https://api.anthropic.com/v1`. You can point this to a reverse proxy or compatible gateway; AxonHub will send requests to `{baseURL}/messages` (or `{baseURL}/v1/messages` depending on whether your base URL ends with `/v1`).
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

- **Channel test fails**: Verify the configured base URL is reachable and the endpoint is compatible with Anthropic Messages API
- **Authentication errors**: Verify the token from `claude setup-token` is correct and hasn't expired
- **Network issues**: If using a remote gateway/proxy, check firewall rules and network connectivity
- **Model not available**: Confirm the requested model is listed in the channel's `supported_models`

---

## Provider Quota Tracking

AxonHub automatically tracks quota usage for Claude Code provider channels, displaying the current status with battery icons in the interface.

### How It Works

- **Automatic Polling**: AxonHub periodically polls your Claude Code account to check quota status
- **Storage**: Quota data is stored in the database and updated based on the configured check interval
- **Visual Indicators**: Battery icons show your remaining quota at a glance.

### Quota Buckets

Claude Code uses two quota windows:
- **5-hour window**: Usage limits over a 5-minute rolling period
- **7-day window**: Usage limits over a 7-day rolling period

The system shows both bucket percentages and indicates which bucket is currently limiting your access.

### Configuration

Adjust the quota check interval in `config.yml`:

```yaml
provider_quota:
  check_interval: "5m"           # Check every 5 minutes (default)
```

Or via environment variable:

```bash
export AXONHUB_PROVIDER_QUOTA_CHECK_INTERVAL="30m"
```

Supported intervals: `1m`, `2m`, `3m`, `4m`, `5m`, `6m`, `10m`, `12m`, `15m`, `20m`, `30m`, `1h`, `2h`, etc.

**Recommendations:**
- **Development**: Use shorter intervals (e.g., `5m`) for quick feedback
- **Production**: Use `5m` (default) for timely quota detection; increase to `10m` or `20m` to reduce API calls

### Refreshing Quota Data

You can manually trigger a quota refresh by clicking the refresh icon in the quota status popover.

### Viewing Quota Status

1. Look for the battery icon next to the settings gear in the header
2. Click the battery icon to view detailed quota information including:
   - 5-hour and 7-day window usage percentages
   - Which quota bucket is currently limiting access
   - Time until the quota window resets

---

### Related Documentation
- [Tracing Guide](tracing.md)
- [OpenAI API](../api-reference/openai-api.md)
- [Codex Integration Guide](codex-integration.md)
- [Channel Management Guide](channel-management.md)
- README sections on [Usage Guide](../../../README.en-US.md#usage-guide)
