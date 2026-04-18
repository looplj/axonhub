export const LONG_DURATIONS: ReadonlySet<string> = new Set(['ONE_WEEK', 'ONE_MONTH']);

export type FormatTimeMode = 'detailed' | 'compact';

export function formatTimeRemaining(
  resetAt: string | null | undefined,
  mode: FormatTimeMode = 'detailed',
  windowDuration?: string | null
): string | null {
  if (!resetAt) return mode === 'detailed' ? '' : null;
  const reset = new Date(resetAt).getTime();
  const now = Date.now();
  const diffMs = reset - now;
  if (diffMs <= 0) return mode === 'detailed' ? '' : null;

  const totalSeconds = Math.floor(diffMs / 1000);
  const totalMinutes = Math.floor(totalSeconds / 60);
  const totalHours = Math.floor(totalMinutes / 60);
  const totalDays = Math.floor(totalHours / 24);

  const isLongWindow = !!windowDuration && LONG_DURATIONS.has(windowDuration);

  if (isLongWindow) {
    if (totalDays >= 7) {
      const weeks = Math.floor(totalDays / 7);
      const remDays = totalDays % 7;
      return remDays > 0 ? `${weeks}w ${remDays}d` : `${weeks}w`;
    }
    if (totalDays >= 1) {
      const remHours = totalHours % 24;
      return remHours > 0 ? `${totalDays}d ${remHours}h` : `${totalDays}d`;
    }
    if (totalHours > 0) {
      const remMinutes = totalMinutes % 60;
      return remMinutes > 0 ? `${totalHours}h ${remMinutes}m` : `${totalHours}h`;
    }
    if (totalMinutes > 0) {
      const seconds = totalSeconds % 60;
      return seconds > 0 ? `${totalMinutes}m ${seconds}s` : `${totalMinutes}m`;
    }
    return `${totalSeconds}s`;
  }

  if (totalDays >= 1) {
    const remHours = totalHours % 24;
    return remHours > 0 ? `${totalDays}d ${remHours}h` : `${totalDays}d`;
  }
  if (totalHours > 0) {
    const minutes = totalMinutes % 60;
    return minutes > 0 ? `${totalHours}h ${minutes}m` : `${totalHours}h`;
  }
  if (totalMinutes > 0) {
    const seconds = totalSeconds % 60;
    return seconds > 0 ? `${totalMinutes}m ${seconds}s` : `${totalMinutes}m`;
  }

  if (mode === 'detailed') {
    return `${totalSeconds}s`;
  }
  return null;
}
