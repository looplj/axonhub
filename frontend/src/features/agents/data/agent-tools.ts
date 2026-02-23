export const builtinToolOptions = [
  'Bash',
  'Read',
  'Write',
  'Edit',
  'Glob',
  'Grep',
  'Skill',
  'MemoryAdd',
  'MemoryGet',
  'MemorySearch',
  'MemoryList',
  'MemoryDelete',
  'WebFetch',
  'WebSearch',
] as const;
export type BuiltinToolName = (typeof builtinToolOptions)[number];
