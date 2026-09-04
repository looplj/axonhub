export type ZenmuxQuotaDisplayLimit = {
  readonly window?: string;
  readonly usageRatio: number;
};

export type ZenmuxQuotaDisplayData = {
  readonly plan?: {
    readonly tier?: string;
  };
  readonly quota_5_hour?: {
    readonly usage_percentage?: number;
  };
  readonly quota_7_day?: {
    readonly usage_percentage?: number;
  };
  readonly quota_monthly?: {
    readonly max_flows?: number;
    readonly max_value_usd?: number;
  };
};

export function getZenmuxUsagePercentage(limits: readonly ZenmuxQuotaDisplayLimit[]): number {
  return Math.max(
    0,
    ...limits.filter((limit) => limit.window === '5h' || limit.window === '7d').map((limit) => limit.usageRatio * 100)
  );
}

export function getZenmuxMonthlyQuotaUSD(data: ZenmuxQuotaDisplayData): number | undefined {
  return data.quota_monthly?.max_value_usd;
}

export function capitalizeZenmuxTier(tier: string): string {
  return tier.charAt(0).toUpperCase() + tier.slice(1);
}
