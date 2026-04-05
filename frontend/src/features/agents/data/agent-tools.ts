export const builtinToolsConfig = [
  { name: 'Bash', defaultEnabled: true },
  { name: 'Read', defaultEnabled: true },
  { name: 'Write', defaultEnabled: true },
  { name: 'Edit', defaultEnabled: true },
  { name: 'Glob', defaultEnabled: true },
  { name: 'Grep', defaultEnabled: true },
  { name: 'Skill', defaultEnabled: true },
  { name: 'MemoryAdd', defaultEnabled: false },
  { name: 'MemoryGet', defaultEnabled: false },
  { name: 'MemorySearch', defaultEnabled: false },
  { name: 'MemoryList', defaultEnabled: false },
  { name: 'MemoryDelete', defaultEnabled: false },
  { name: 'WebFetch', defaultEnabled: true },
  { name: 'WebSearch', defaultEnabled: true },
] as const;

export const builtinToolOptions = builtinToolsConfig.map(t => t.name);
export type BuiltinToolName = (typeof builtinToolsConfig)[number]['name'];

export const builtinToolDefaultEnabled: Record<BuiltinToolName, boolean> = {
  ...Object.fromEntries(builtinToolsConfig.map(t => [t.name, t.defaultEnabled])),
} as Record<BuiltinToolName, boolean>;

export const builtinSkillsConfig = [{ name: 'memory-management', defaultEnabled: true }] as const;

export const builtinSkillOptions = builtinSkillsConfig.map(s => s.name);
export type BuiltinSkillName = (typeof builtinSkillsConfig)[number]['name'];

export const builtinSkillDefaultEnabled: Record<BuiltinSkillName, boolean> = {
  ...Object.fromEntries(builtinSkillsConfig.map(s => [s.name, s.defaultEnabled])),
} as Record<BuiltinSkillName, boolean>;
