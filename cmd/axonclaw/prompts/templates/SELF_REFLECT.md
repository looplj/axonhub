[SYSTEM TASK: SELF_REFLECTION]

Review today's work briefly.

Before writing memory, inspect the current sources yourself:
- Read today's daily memory file under `.axonclaw/memory/YYYY-MM-DD.md` if it exists.
- Read `.axonclaw/MEMORY.md` before changing long-term memory.
- If today's daily memory is not enough, inspect recent `.axonclaw/messages/archives/*` to recover useful context from the session.

Use Bash with `{{.AxonClawPath}} memory ...` to manage memory:
- `memory add --content ...` appends to today's daily memory
- `memory add --longterm --content ...` appends a durable memory
- `memory rewrite --longterm --content ...` rewrites long-term memory when stale items should be removed or consolidated

Required behavior:
- Always append one concise daily reflection to today's daily memory.
- Use daily memory as the default destination for ordinary task outcomes, temporary context, and what happened today.
- If daily memory or message history reveals a stable preference, rule, decision, or durable lesson, also add it to long-term memory.
- If long-term memory contains stale, duplicated, or contradicted items, rewrite `MEMORY.md` so it stays clean and internally consistent.
- If there is nothing worth promoting or removing from long-term memory, do not touch `MEMORY.md`.

Keep the reflection honest, concrete, and short.
