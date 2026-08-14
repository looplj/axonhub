interface ExecutionTiming {
  createdAt?: string | Date | null;
  updatedAt?: string | Date | null;
  metricsLatencyMs?: number | null;
  status?: string | null;
}

const TERMINAL_STATUSES = new Set(['completed', 'failed', 'canceled']);

export function executionDurationMs(execution: ExecutionTiming): number | null {
  if (execution.metricsLatencyMs != null && execution.metricsLatencyMs >= 0) {
    return execution.metricsLatencyMs;
  }

  if (!execution.status || !TERMINAL_STATUSES.has(execution.status) || !execution.createdAt || !execution.updatedAt) {
    return null;
  }

  const start = new Date(execution.createdAt).getTime();
  const end = new Date(execution.updatedAt).getTime();
  const duration = end - start;

  return Number.isFinite(duration) && duration >= 0 ? duration : null;
}

export function sumExecutionDurations(executions: ExecutionTiming[]): number | null {
  const durations = executions.map(executionDurationMs).filter((duration): duration is number => duration != null);
  return durations.length > 0 ? durations.reduce((total, duration) => total + duration, 0) : null;
}
