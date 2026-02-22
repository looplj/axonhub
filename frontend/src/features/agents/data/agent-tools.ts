export const builtinToolOptions = ['Bash', 'Read', 'Write', 'Edit', 'Glob', 'Grep', 'Skill', 'MemoryAdd', 'MemoryGet', 'MemorySearch', 'MemoryList', 'MemoryDelete'] as const;
export type BuiltinToolName = (typeof builtinToolOptions)[number];
