# AxonClaw Prompts System

## Overview

AxonClaw uses a multi-layer prompt system that combines local personality files with server-provided configuration. All prompts are assembled at startup and injected into the agent's system context.

## Prompt Layers

### Layer 1: Personality Files (Editable)

Stored in the workspace prompt directory (`.axonclaw/`), can be modified by the model:

| File | Purpose | Content Focus |
|------|---------|---------------|
| `IDENTITY.md` | Agent identity | Name, role, outward identity, stable self-description |
| `USER.md` | User context | User info, preferences, timezone |
| `SOUL.md` | Agent personality | Character, communication style, temperament, long-lived persona traits |
| `SYSTEM.md` | Custom system prompt | Project-specific instructions, domain knowledge |

### Layer 2: Instruction Prompt (Read-only)

Built-in instructions embedded in the binary, **cannot be modified**:

| Component | Purpose |
|-----------|---------|
| Skills & Extended Capabilities | How to use skills first, fallback to raw tools |
| Bash Usage Guidelines | When to use specialized tools vs Bash |
| Response Protocol | SendMessage tool usage for user communication |
| Stopping & Pausing | How to handle stop/pause commands |
| Language | Reply in user's language |
| AxonClaw Command Reference | Help tool, command discovery |
| Agent Reset | Reset tool usage |
| Inter-Agent Communication | Discover and message peer agents |
| Scheduled Tasks | Task scheduling with cron/interval/at |
| Environment | Runtime environment variables |

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
│     - SYSTEM.md: Render server prompt or default template       │
│  3. Save rendered content to disk                               │
└─────────────────────────────────────────────────────────────────┘
```

Existing files are preserved as-is. Templates are only rendered when a bootstrap file is empty.

## Runtime Prompt Assembly

```
┌─────────────────────────────────────────────────────────────────┐
│                    System Prompts Order                          │
├─────────────────────────────────────────────────────────────────┤
│  1. SYSTEM.md + Skills (editable)                               │
│  2. INSTRUCTION.md (read-only)                                  │
│  3. IDENTITY.md (editable)                                      │
│  4. USER.md (editable)                                          │
│  5. SOUL.md (editable)                                          │
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

**Note**: INSTRUCTION.md is read-only and embedded in the binary, so it is added directly without an editable wrapper.

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

### INSTRUCTION.md - Built-in Instructions

Read-only instructions for tool usage:

- **Skills First**: Always check for skills before using raw tools
- **Tool Preferences**: Use specialized tools (Read, Write, Glob, Grep) over Bash
- **Response Protocol**: Use SendMessage to communicate with users
- **Agent Commands**: AxonClaw CLI commands for discover, tasks, memory, etc.
- **Environment**: Runtime variables (OS, workspace, AxonClaw path, skills root)

### SYSTEM.md - Custom System Prompt

Project-specific or domain-specific instructions:

- Initialized from a server-provided template, or from a built-in default template when the server value is empty
- Can be customized by the model for specific projects
- Skills from server are appended at runtime

## Template Variables

Available variables for template rendering (only during initialization):

| Variable | Description |
|----------|-------------|
| `{{.AgentName}}` | Agent's display name |
| `{{.CreatedByUserName}}` | User who created the agent |
| `{{.Date}}` | Current date (YYYY-MM-DD) |
| `{{.Timezone}}` | System timezone (UTC±X) |
| `{{.OS}}` | Operating system |
| `{{.Workspace}}` | Working directory path |
| `{{.ThreadID}}` | Current thread ID |
| `{{.AxonClawPath}}` | Path to axonclaw executable |
| `{{.SkillsRoot}}` | Skills directory path |
| `{{.AgentID}}` | Agent's unique ID |

## Skills Integration

Skills from the server are appended to `SYSTEM.md` content at runtime:

```
[SYSTEM.md content]

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

4. **Instruction is read-only**: Built-in tool usage instructions are embedded in the binary and cannot be modified, ensuring consistent behavior across all agents.

5. **Skills are dynamic**: Skills come from the server and are appended at runtime, not saved to disk. This allows the server to update skills without modifying local files.

6. **IDENTITY.md and SOUL.md are living memory layers**: The agent should actively maintain them from conversation. `IDENTITY.md` stores durable identity facts and role definition; `SOUL.md` stores long-lived temperament, style, and persona guidance.

7. **SYSTEM.md always exists**: If the server does not provide a system prompt, AxonClaw falls back to a built-in `SYSTEM.md` template so the editable system layer is still present.

## Related Files

- `cmd/axonclaw/prompts/prompts.go` - Prompt building functions
- `cmd/axonclaw/prompts/bootstrap.go` - File loading/saving
- `cmd/axonclaw/prompts/templates.go` - Default templates
- `cmd/axonclaw/prompts/templates/SYSTEM.md` - Default system template
- `cmd/axonclaw/prompts/templates/SOUL.md` - Default personality template
- `cmd/axonclaw/prompts/templates/INSTRUCTION.md` - Built-in instructions
- `cmd/axonclaw/bootstrap/bootstrap.go` - Initialization logic
- `cmd/axonclaw/runner/runner.go` - Runtime assembly
