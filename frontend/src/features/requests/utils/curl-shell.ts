export function escapeShellValue(value: string): string {
  return value.replace(/'/g, "'\\''");
}
