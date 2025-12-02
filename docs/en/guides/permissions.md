# Fine-grained Permission

## Overview
AxonHub provides role-based access control (RBAC) so that organizations can tailor API access, feature visibility, and resource quotas to specific teams or workloads. Fine-grained rules allow administrators to enforce least-privilege policies, protect sensitive data, and monitor usage across projects.

## Key Concepts
- **Roles** – Collections of permissions that define a user or API key's capabilities.
- **Scopes** – Granular privileges such as managing channels, issuing API keys, or viewing traces.
- **Projects** – Logical containers that tie together datasets, model profiles, and API activity.
- **API Keys** – Tokens issued per project or user that inherit role scopes and can be rotated at any time.

## Common Policies
1. **Separation of Duties** – Assign operational teams read-only access to traces while keeping configuration changes limited to administrators.
2. **Quota Guardrails** – Combine rate limits and per-model cost ceilings to prevent runaway spend.
3. **Environment Isolation** – Create dedicated projects for staging and production, mapping distinct model profiles and upstream credentials.

## Best Practices
- Rotate API keys regularly and revoke unused credentials from the admin console.
- Use service accounts with minimal scopes for automation pipelines and CI/CD flows.
- Enable auditing to capture every administrative change for compliance investigations.

## Related Resources
- [Chat Completions API](../api-reference/unified-api.md#openai-chat-completions-api)
- [Claude Code & Codex Integration](claude-code-integration.md)
