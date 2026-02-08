export interface ChartData {
  name: string;
  throughput: number;
  requestCount: number;
  confidenceLevel?: 'high' | 'medium' | 'low';
}

export function safeNumber(value: unknown): number {
  const num = Number(value);
  return Number.isFinite(num) ? num : 0;
}

export function safeToFixed(value: unknown, decimals: number = 1): string {
  return safeNumber(value).toFixed(decimals);
}

export function sanitizeChartData(items: ChartData[]): ChartData[] {
  return items
    .map((item) => ({
      name: String(item.name ?? 'Unknown'),
      throughput: safeNumber(item.throughput),
      requestCount: safeNumber(item.requestCount),
      confidenceLevel: item.confidenceLevel,
    }))
    .filter((item) => Number.isFinite(item.throughput));
}