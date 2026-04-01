# AxonClaw Prompts System

## Overview

AxonClaw now uses a unified prompt model centered on one editable default workspace guide. That prompt can be initialized from the server or from the built-in `AGENTS.md` template, then reused across normal execution, self-evolution, and conversation summarization.

## Prompt Files

### Editable Bootstrap Files

Stored in the workspace prompt directory (`.axonclaw/`), can be modified by the model:

| File | Purpose | Content Focus |
|------|---------|---------------|
| `IDENTITY.md` | Agent identity | Name, role, outward identity, stable self-description |
| `USER.md` | User context | User info, preferences, timezone |
| `SOUL.md` | Agent personality | Character, communication style, temperament, long-lived persona traits |
| `AGENTS.md` | Workspace operating guide | Project-specific instructions, operating rules, domain knowledge |

### Unified Default Workspace Guide

The built-in `AGENTS.md` template now contains the default operational guidance that used to be split across multiple internal prompts. It includes:

- execution protocol and tool priority
- workspace record maintenance guidance
- `memory-management` skill guidance
- self-evolution behavior
- conversation summarization behavior
- safety and trust constraints

## Initialization Flow

```
┌─────────────────────────────────────────────────────────────────┐
│                      Bootstrap Phase                             │
├─────────────────────────────────────────────────────────────────┤
│  1. Load existing files from workspace .axonclaw/               │
│  2. For each empty file:                                         │
│     - IDENTITY.md: Render template with AgentName, CreatedBy    │
│     - USER.md: Render template with AgentName, CreatedBy        │
│     - SOUL.md: Copy default template (no variables)             │
│     - AGENTS.md: Render server prompt or default template       │
│  3. Save rendered content to disk                               │
└─────────────────────────────────────────────────────────────────┘
```

Existing files are preserved as-is. Templates are only rendered when a bootstrap file is empty.

## Runtime Prompt Assembly

```
┌─────────────────────────────────────────────────────────────────┐
│                    System Prompts Order                          │
├─────────────────────────────────────────────────────────────────┤
│  1. AGENTS.md + Skills (editable default workspace guide)       │
│  2. IDENTITY.md (editable)                                      │
│  3. USER.md (editable)                                          │
│  4. SOUL.md (editable)                                          │
│  5. MEMORY.md + recent daily logs                               │
└─────────────────────────────────────────────────────────────────┘
```

## Prompt Wrapping Format

Editable files are wrapped with a semantic header that explains the file's role without exposing the on-disk path:

```markdown
# Your Identity

Use this file as the source of truth for who you are, what role you play, and the default style you should maintain.

[File content...]
```

This allows the AI to:
- Understand the file's purpose from the title and description
- Modify the editable prompt files through the normal workspace file tools without leaking internal storage paths into model context

## File Details

### IDENTITY.md - Stable Identity Record

Defines the agent's stable identity and outward role:

```markdown
## Name
Axon - Primary

## Role
Execution-focused workspace agent
```

### SOUL.md - Long-Lived Persona Record

Defines the agent's character and style:

```markdown
# SOUL.md

## Identity
You are an AI assistant managed by AxonHub.

## Personality
- Thoughtful and direct
- Prefers clarity over verbosity
- Willing to express opinions with reasoning

## Communication Style
- Concise and structured responses
- Use code snippets over long explanations when applicable
- Reply in the same language the user writes in

## Expertise
- Software development and engineering
- System architecture and design
- Problem solving and debugging

## Principles
- Correctness over cleverness
- Understand before acting
- Ask clarifying questions when requirements are ambiguous

## Boundaries
- Confirm before destructive operations
- Do not fabricate information
- Respect user's existing code style and conventions
```

### AGENTS.md - Workspace Operating Guide

Project-specific or domain-specific instructions, and the single default prompt entrypoint:

- Initialized from a server-provided template, or from a built-in default template when the server value is empty
- Can be customized by the model for specific projects
- Skills from server are appended at runtime
- Also carries the default guidance for self-evolution, summarization, and memory handling

## Template Variables

Available variables for template rendering (only during initialization):

| Variable | Description |
|----------|-------------|
| `{{.AgentName}}` | Agent's display name |
| `{{.CreatedByUserName}}` | User who created the agent |
| `{{.Date}}` | Current date (YYYY-MM-DD) |
| `{{.Timezone}}` | System timezone (UTC±X) |
| `{{.OS}}` | Human-readable operating system name, for example `macOS`, `Linux`, or `Windows` |
| `{{.Workspace}}` | Working directory path |
| `{{.ThreadID}}` | Current thread ID |
| `{{.AxonClawPath}}` | Path to axonclaw executable |
| `{{.SkillsRoot}}` | Skills directory path |
| `{{.AgentID}}` | Agent's unique ID |

## Skills Integration

Skills from the server are appended to `AGENTS.md` content at runtime:

```
[AGENTS.md content]

---

## Skill: skill-name

[Skill content...]

---

## Skill: another-skill

[Skill content...]
```

## Key Design Decisions

1. **Template rendering only at initialization**: Variables are resolved once when the file is created. After that, files are read as-is, allowing the model to edit them freely.

2. **No runtime re-rendering**: This ensures user/model modifications are preserved across sessions.

3. **No path disclosure in prompts**: Editable files are described by purpose only. Disk paths are intentionally omitted so internal prompt/config locations are not exposed to the model.

4. **One default prompt entrypoint**: The built-in operational guidance is merged into `AGENTS.md`, so users only need to customize one default prompt instead of maintaining separate instruction or self-evolution prompt files.

5. **Skills are dynamic**: Skills come from the server and are appended at runtime, not saved to disk. This allows the server to update skills without modifying local files.

6. **IDENTITY.md and SOUL.md are living memory layers**: The agent should actively maintain them from conversation. `IDENTITY.md` stores durable identity facts and role definition; `SOUL.md` stores long-lived temperament, style, and persona guidance.

7. **AGENTS.md always exists**: If the server does not provide a system prompt, AxonClaw falls back to a built-in `AGENTS.md` template so the editable workspace-guide layer is still present.

## Related Files

- `cmd/axonclaw/prompts/prompts.go` - Prompt building functions
- `cmd/axonclaw/prompts/bootstrap.go` - File loading/saving
- `cmd/axonclaw/prompts/templates.go` - Default templates
- `cmd/axonclaw/prompts/templates/AGENTS.md` - Default workspace guide template
- `cmd/axonclaw/prompts/templates/SOUL.md` - Default personality template
- `cmd/axonclaw/bootstrap/bootstrap.go` - Initialization logic
- `cmd/axonclaw/runner/runner.go` - Runtime assembly
- `frontend/src/features/agents/data/SYSTEM.md` - Default system prompt template for frontend agent creation
