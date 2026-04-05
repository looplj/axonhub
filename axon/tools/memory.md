Tools for saving and retrieving lightweight memories.

Path conventions (recommended):
- daily/{YYYY-MM-DD}.md: Daily logs and running context
- MEMORY.md: Curated durable decisions/preferences/facts
- project/{name}.md: Project-specific knowledge and patterns
- thread/{id}.md: Thread-scoped context

Available tools:
- MemoryAdd: Add an entry at a path ({path, content, source?})
- MemoryGet: Get aggregated content for a path ({path})
- MemorySearch: Search by keyword ({query, limit?}, default limit: 10)
- MemoryList: List stored entries ({limit?}, default limit: 20)
- MemoryDelete: Delete all entries at a path ({path}, irreversible)

Tips:
- Keep content short and specific.
- Do not store secrets (tokens, passwords, private keys).
