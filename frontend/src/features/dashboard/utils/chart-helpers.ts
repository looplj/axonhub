export interface ChartData {
  name: string;
  throughput: number;
  requestCount: number;
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
    .filter((item) => Number.isFinite(item.throughput) || Number.isFinite(Number(item.throughput)))
    .map((item) => ({
      name: String(item.name ?? 'Unknown'),
      throughput: safeNumber(item.throughput),
      requestCount: safeNumber(item.requestCount),
    }));
}
