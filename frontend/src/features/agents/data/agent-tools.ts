export const builtinToolOptions = ['bash', 'edit', 'glob', 'grep', 'memory', 'read', 'skill', 'write'] as const;
export type BuiltinToolName = (typeof builtinToolOptions)[number];

