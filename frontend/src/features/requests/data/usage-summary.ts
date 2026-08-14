import type { UsageLog } from './usage-logs-schema';

type UsageLogLike = Partial<UsageLog> | null | undefined;

export interface EmbeddedUsageLogConnection {
  edges?: Array<{
    node?: UsageLogLike;
  }>;
}

type UsageLogCollection = EmbeddedUsageLogConnection | UsageLogLike[];

export interface UsageSummary {
  source?: UsageLog['source'];
  promptTokens: number;
  completionTokens: number;
  totalTokens: number;
  promptCachedTokens: number;
  promptWriteCachedTokens: number;
  completionReasoningTokens: number;
  completionAudioTokens: number;
  totalCost: number | null;
  costItems: Array<{
    itemCode: string;
    quantity: number;
    subtotal: number;
  }>;
}

export interface UsageByPurposeSummary {
  primary: UsageSummary | null;
  visionDelegation: UsageSummary | null;
}

export function usageLogsFromConnection(connection?: UsageLogCollection | null): UsageLogLike[] {
  if (Array.isArray(connection)) {
    return connection.filter((node): node is Partial<UsageLog> => Boolean(node));
  }

  return connection?.edges?.map((edge) => edge.node).filter((node): node is Partial<UsageLog> => Boolean(node)) ?? [];
}

export function aggregateUsageLogs(logs: UsageLogLike[]): UsageSummary | null {
  const validLogs = logs.filter((log): log is Partial<UsageLog> => Boolean(log));
  if (validLogs.length === 0) return null;

  const costItems = new Map<string, { itemCode: string; quantity: number; subtotal: number }>();
  let hasCost = false;
  let totalCost = 0;

  const summary: UsageSummary = {
    source: validLogs.find((log) => log.source)?.source,
    promptTokens: 0,
    completionTokens: 0,
    totalTokens: 0,
    promptCachedTokens: 0,
    promptWriteCachedTokens: 0,
    completionReasoningTokens: 0,
    completionAudioTokens: 0,
    totalCost: null,
    costItems: [],
  };

  for (const log of validLogs) {
    const promptTokens = log.promptTokens ?? 0;
    const completionTokens = log.completionTokens ?? 0;
    summary.promptTokens += promptTokens;
    summary.completionTokens += completionTokens;
    summary.totalTokens += log.totalTokens && log.totalTokens > 0 ? log.totalTokens : promptTokens + completionTokens;
    summary.promptCachedTokens += log.promptCachedTokens ?? 0;
    summary.promptWriteCachedTokens += log.promptWriteCachedTokens ?? 0;
    summary.completionReasoningTokens += log.completionReasoningTokens ?? 0;
    summary.completionAudioTokens += log.completionAudioTokens ?? 0;

    if (log.totalCost != null) {
      hasCost = true;
      totalCost += log.totalCost;
    }

    for (const item of log.costItems ?? []) {
      const current = costItems.get(item.itemCode) ?? {
        itemCode: item.itemCode,
        quantity: 0,
        subtotal: 0,
      };
      current.quantity += item.quantity;
      current.subtotal += item.subtotal;
      costItems.set(item.itemCode, current);
    }
  }

  summary.totalCost = hasCost ? totalCost : null;
  summary.costItems = Array.from(costItems.values());
  return summary;
}

export function aggregateUsageConnection(connection?: UsageLogCollection | null): UsageSummary | null {
  return aggregateUsageLogs(usageLogsFromConnection(connection));
}

export function aggregateUsageByPurposeConnection(connection?: UsageLogCollection | null): UsageByPurposeSummary {
  const logs = usageLogsFromConnection(connection);

  return {
    primary: aggregateUsageLogs(logs.filter((log) => log?.requestExecution?.purpose !== 'vision_delegation')),
    visionDelegation: aggregateUsageLogs(logs.filter((log) => log?.requestExecution?.purpose === 'vision_delegation')),
  };
}

// Cache hit rates describe the client-facing primary model. Vision delegation
// is a separate internal request whose image tokens are not comparable to it.
export function aggregatePrimaryUsageConnection(connection?: UsageLogCollection | null): UsageSummary | null {
  const logs = usageLogsFromConnection(connection);
  const { primary } = aggregateUsageByPurposeConnection(connection);
  if (primary) return primary;

  // Older usage logs have no execution relation. Keep them usable as primary
  // usage, but never relabel a vision-only connection as a primary request.
  return logs.some((log) => log?.requestExecution) ? null : aggregateUsageConnection(connection);
}
