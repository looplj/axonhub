import { useMemo } from 'react';
import { useQuery } from '@tanstack/react-query';
import { graphqlRequest } from '@/gql/graphql';
import type { ChannelQuotaRoutingMode } from '@/features/channels/data/schema';

const CHECK_PROVIDER_QUOTAS_QUERY = `
  mutation CheckProviderQuotas {
    checkProviderQuotas
  }
`;

const RESET_CHANNEL_QUOTA_NOW_MUTATION = `
  mutation ResetChannelQuotaNow($channelID: ID!, $resetID: String) {
    resetChannelQuotaNow(channelID: $channelID, resetID: $resetID)
  }
`;

const PROVIDER_QUOTA_STATUSES_QUERY = `
  query ProviderQuotaStatuses($input: QueryChannelInput!) {
    queryChannels(input: $input) {
      edges {
        node {
          id
          name
          type
          orderingWeight
          settings {
            quotaRoutingMode
          }
          # The persisted per-key view is only recomputed during collection, so
          # the current disable records carry the ownership the popover shows:
          # a key an operator took over manually is never quota-owned again.
          disabledAPIKeys {
            key
            expiresAt
            quotaDisabled
          }
          providerQuotaStatus {
            status
            nextResetAt
            ready
            quotaData
            providerType
            accountKey
          }
        }
      }
    }
  }
`;

export async function checkProviderQuotas() {
  return graphqlRequest(CHECK_PROVIDER_QUOTAS_QUERY);
}

export async function resetChannelQuotaNow(channelID: string, resetID?: string) {
  return graphqlRequest(RESET_CHANNEL_QUOTA_NOW_MUTATION, { channelID, resetID });
}

export type ProviderQuotaReset = {
  clears?: string[];
  recommended?: boolean;
  remaining?: number;
  redeemable?: boolean;
  unavailableReason?: string;
  upstreamReason?: string;
  id: string;
  status: string;
  type?: string;
  grantedAt?: string;
  expiresAt?: string;
  title?: string;
};

export type ProviderQuotaResetList = {
  ineligibleReason?: string;
  eligible?: boolean;
  cooldownUntil?: string;
  windowModels?: Record<string, string>;
  supported: boolean;
  resets: ProviderQuotaReset[];
  error?: string;
};

/** A normalized non-window account amount, such as a prepaid credit balance. */
export type ProviderQuotaBalance = {
  account?: string;
  currency?: string;
  total: number;
  granted?: number;
  topped_up?: number;
};

type ProviderQuotaDataCommon = {
  plan_type?: string;
  error?: string;
  error_code?: string;
  _resets?: ProviderQuotaResetList;
  _balances?: ProviderQuotaBalance[];
};

type ProviderClaudeQuotaWindow = {
  utilization?: number;
  reset?: number;
  status?: string;
};

type ProviderClaudeQuotaData = ProviderQuotaDataCommon & {
  windows?: {
    '5h'?: ProviderClaudeQuotaWindow;
    '7d'?: ProviderClaudeQuotaWindow;
    overage?: ProviderClaudeQuotaWindow;
  };
  model_windows?: (ProviderClaudeQuotaWindow & { model: string })[];
  representative_claim?: string;
};

type ProviderCodexQuotaData = ProviderQuotaDataCommon & {
  rate_limit?: {
    primary_window?: {
      used_percent?: number;
      reset_at?: number;
      reset_after_seconds?: number;
      limit_window_seconds?: number;
    };
    secondary_window?: {
      used_percent?: number;
      reset_at?: number;
      reset_after_seconds?: number;
      limit_window_seconds?: number;
    };
  };
};

export type XAISubscriptionBillingWindow = {
  readonly usage_percent?: number;
  readonly reset_at?: string;
  readonly limit_usd?: number;
  readonly used_usd?: number;
};

export type ProviderXAISubscriptionQuotaData = ProviderQuotaDataCommon & {
  readonly billing?: {
    readonly weekly?: XAISubscriptionBillingWindow;
    readonly monthly?: XAISubscriptionBillingWindow;
  };
};

type CopilotQuotaSnapshot = {
  entitlement: number;
  has_quota: boolean;
  overage_count: number;
  overage_permitted: boolean;
  percent_remaining: number;
  quota_id: string;
  quota_remaining: number;
  quota_reset_at: number;
  remaining: number;
  timestamp_utc: string;
  unlimited: boolean;
};

type ProviderGitHubCopilotQuotaData = ProviderQuotaDataCommon & {
  limited_user_quotas?: {
    chat?: number;
    completions?: number;
    [key: string]: number | undefined;
  };
  quota_snapshots?: {
    chat?: CopilotQuotaSnapshot;
    completions?: CopilotQuotaSnapshot;
    premium_interactions?: CopilotQuotaSnapshot;
    premium_models?: CopilotQuotaSnapshot;
    [key: string]: CopilotQuotaSnapshot | undefined;
  };
  total_quotas?: {
    chat?: number;
    completions?: number;
    [key: string]: number | undefined;
  };
};

export type NanoGPTQuotaWindow = {
  used?: number;
  remaining?: number;
  percentUsed?: number;
  resetAt?: number;
};

export type ProviderNanoGPTQuotaData = ProviderQuotaDataCommon & {
  state?: string;
  active?: boolean;
  allowOverage?: boolean;
  limits?: {
    weeklyInputTokens?: number;
    dailyImages?: number;
    dailyInputTokens?: number;
  };
  windows?: {
    weeklyInputTokens?: NanoGPTQuotaWindow | null;
    dailyImages?: NanoGPTQuotaWindow | null;
    dailyInputTokens?: NanoGPTQuotaWindow | null;
  };
  period?: { currentPeriodEnd?: string };
};

export type ProviderSyntheticQuotaData = ProviderQuotaDataCommon & {
  search?: {
    hourly?: {
      limit?: number | null;
      requests?: number | null;
      renewsAt?: string | null;
    } | null;
  } | null;
  planWeeklyLimit?: {
    nextRegenAt?: string | null;
    percentRemaining?: number | null;
    nextRegenPercent?: number | null;
  } | null;
  weeklyTokenLimit?: {
    percentRemaining?: number | null;
    remainingCredits?: string | null;
    maxCredits?: string | null;
    nextRegenAt?: string | null;
  } | null;
  rollingFiveHourLimit?: {
    limited?: boolean | null;
    remaining?: number | null;
    max?: number | null;
    nextTickAt?: string | null;
    tickPercent?: number | null;
  } | null;
};

export type ProviderNeuralWattQuotaData = ProviderQuotaDataCommon & {
  balance?: { credits_remaining_usd?: number | null; total_credits_usd?: number | null } | null;
  subscription?: {
    kwh_included?: number | null;
    kwh_used?: number | null;
    kwh_remaining?: number | null;
    in_overage?: boolean | null;
    status?: string | null;
    plan?: string | null;
    kwh_reset_date?: string | null;
  } | null;
  daily_free_energy?: {
    kwh_included?: number | null;
    kwh_used?: number | null;
    kwh_remaining?: number | null;
    reset_at?: string | null;
    model_labels?: string[] | null;
  } | null;
  usage_current_month?: {
    cost_usd?: number | null;
    requests?: number | null;
    tokens?: number | null;
    energy_kwh?: number | null;
  } | null;
};

export type ProviderCharmHyperQuotaData = ProviderQuotaDataCommon & {
  balance?: number | null;
};

export type ProviderDeepseekBalanceInfo = {
  currency?: string | null;
  total_balance?: number | string | null;
  granted_balance?: number | string | null;
  topped_up_balance?: number | string | null;
};

export type ProviderDeepseekQuotaData = ProviderQuotaDataCommon & {
  is_available?: boolean | null;
  balance_infos?: ProviderDeepseekBalanceInfo[] | null;
};

export type ProviderApertisQuotaData = ProviderQuotaDataCommon & {
  is_subscriber?: boolean;
  payg?: {
    account_credits?: number;
    token_used?: number;
    token_total?: number | string;
    token_remaining?: number | string;
    token_is_unlimited?: boolean;
    token_monthly_limit_usd?: number;
    token_monthly_used_usd?: number;
    monthly_reset_day?: number;
  };
  subscription?: {
    plan_type?: string;
    status?: string;
    cycle_quota_limit?: number;
    cycle_quota_used?: number;
    cycle_quota_remaining?: number;
    cycle_start?: string;
    cycle_end?: string;
    payg_fallback_enabled?: boolean;
    payg_spent_usd?: number;
    payg_limit_usd?: number;
  };
};

export type OpenCodeGoQuotaWindow = {
  usage_percent?: number;
  reset_in_seconds?: number;
  reset_time?: string;
  status?: string;
  percent_remaining?: number;
};

export type ProviderOpenCodeGoQuotaData = ProviderQuotaDataCommon & {
  windows?: {
    rolling?: OpenCodeGoQuotaWindow;
    weekly?: OpenCodeGoQuotaWindow;
    monthly?: OpenCodeGoQuotaWindow;
  };
};

export type KimiCodeUsageRow = {
  label: string;
  used: number;
  limit: number;
  resetAt?: string;
  resetAfterSeconds?: number;
};

export type ProviderKimiCodeQuotaData = ProviderQuotaDataCommon & {
  rows?: KimiCodeUsageRow[];
  boosterWallet?: {
    balanceCents: number;
    totalCents: number;
    monthlyChargeLimitEnabled: boolean;
    monthlyChargeLimitCents: number;
    monthlyUsedCents: number;
    currency: string;
  };
};

export type MinimaxModelRow = {
  modelName: string;
  intervalUsedPercent: number;
  intervalTotalPercent: number;
  intervalPercent: number;
  intervalStatus: string;
  intervalResetAt?: string;
  weeklyUsedPercent: number;
  weeklyTotalPercent: number;
  weeklyPercent: number;
  weeklyStatus: string;
  weeklyResetAt?: string;
  weeklyBoostPermille?: number;
};

export type ProviderMinimaxQuotaData = ProviderQuotaDataCommon & {
  rows?: MinimaxModelRow[];
};

export type ZhipuWindowRow = {
  window: string;
  usedPercent: number;
  status: string;
  resetAt?: string;
  usage?: number;
  used?: number;
  remaining?: number;
};

export type ProviderZhipuQuotaData = ProviderQuotaDataCommon & {
  rows?: ZhipuWindowRow[];
  level?: string;
};

export type ProviderZenmuxQuotaPlan = {
  tier?: string;
  amount_usd?: number;
  expires_at?: string;
};

export type ProviderZenmuxQuotaWindow = {
  usage_percentage?: number;
  resets_at?: string;
  max_flows?: number;
  used_flows?: number;
  remaining_flows?: number;
  used_value_usd?: number;
  max_value_usd?: number;
};

export type ProviderZenmuxQuotaData = ProviderQuotaDataCommon & {
  plan?: ProviderZenmuxQuotaPlan;
  account_status?: string;
  quota_5_hour?: ProviderZenmuxQuotaWindow;
  quota_7_day?: ProviderZenmuxQuotaWindow;
  quota_monthly?: {
    max_flows?: number;
    max_value_usd?: number;
  };
};

export type ClineQuotaWindow = {
  window_state?: 'active' | 'inactive' | 'unavailable' | 'invalid';
  active_window?: boolean;
  window_start_at?: string;
  cost_start_at?: string;
  items_count?: number;
  used_cost_units?: number;
  limit_cost_units: number;
  remaining_cost_units?: number;
  credits_used?: number;
  usage_ratio?: number;
  usage_percent?: number;
  cost_usage_ratio?: number;
  cost_usage_percent?: number;
  usage_source?: string;
  reset_source?: string;
  cost_source?: string;
  next_reset_at?: string | null;
};

type ClineBalance = {
  raw_balance?: number | null;
  unit_note?: string;
};

type ClineUsageFetch = {
  pages: number;
  items_seen: number;
  cline_pass_items_seen?: number;
  direct_items_seen?: number;
  unclassified_items_seen?: number;
  invalid_timestamp_items?: number;
  truncated: boolean;
};

type ProviderClinePassQuotaData = ProviderQuotaDataCommon & {
  model_scope: 'cline_pass_only' | 'mixed' | 'unknown';
  status_basis: string;
  pool: 'cline_pass';
  pool_note?: string;
  cost_scale: number;
  balance: ClineBalance;
  windows: {
    last5h: ClineQuotaWindow;
    last7d: ClineQuotaWindow;
    last30d: ClineQuotaWindow;
  };
  usage_fetch: ClineUsageFetch;
};

type ProviderClineUnavailablePassQuotaData = ProviderQuotaDataCommon & {
  model_scope: 'cline_pass_only' | 'mixed' | 'unknown';
  status_basis: 'cline_pass_unavailable' | 'cline_pass_unavailable_mixed_pool';
  pool: 'cline_pass';
  pool_note?: string;
  pass_state: 'unavailable';
  balance: ClineBalance;
  cost_scale?: never;
  windows?: never;
  usage_fetch?: never;
};

type ProviderClineDirectQuotaData = ProviderQuotaDataCommon & {
  model_scope: 'direct_only';
  status_basis: string;
  pool: 'direct_credit' | string;
  pool_note?: string;
  balance: ClineBalance;
  cost_scale?: never;
  windows?: never;
  usage_fetch?: never;
};

type ProviderClineErrorQuotaData = ProviderQuotaDataCommon & {
  model_scope?: undefined;
  status_basis?: string;
  pool?: string;
  balance?: ClineBalance;
  cost_scale?: never;
  windows?: never;
  usage_fetch?: never;
};

export type ProviderClineQuotaData =
  ProviderClinePassQuotaData | ProviderClineUnavailablePassQuotaData | ProviderClineDirectQuotaData | ProviderClineErrorQuotaData;

export function isClineActivePassQuotaData(qd: ProviderClineQuotaData): qd is ProviderClinePassQuotaData {
  return qd.pool === 'cline_pass' && qd.windows != null;
}

export function isClineUnavailablePassQuotaData(qd: ProviderClineQuotaData): qd is ProviderClineUnavailablePassQuotaData {
  return 'pass_state' in qd && qd.pass_state === 'unavailable';
}

export type CommandCodeQuotaWindow = {
  used_usd?: number;
  cap_usd?: number;
  usage_percent?: number;
  reset_time?: string;
};

export type ProviderCommandCodeQuotaData = ProviderQuotaDataCommon & {
  plan_id?: string;
  plan_label?: string;
  subscription_status?: string;
  current_period_end?: string;
  credits?: {
    monthly_remaining_usd?: number;
    monthly_limit_usd?: number;
    purchased_credits_usd?: number;
  };
  windows?: {
    five_hour?: CommandCodeQuotaWindow;
    weekly?: CommandCodeQuotaWindow;
  };
};

export type OllamaQuotaWindow = {
  usage_percent?: number;
  status?: string;
  percent_remaining?: number;
  reset_time?: string;
};

export type ProviderOllamaQuotaData = ProviderQuotaDataCommon & {
  windows?: {
    monthly?: OllamaQuotaWindow;
    '5h'?: OllamaQuotaWindow;
    weekly?: OllamaQuotaWindow;
  };
  credits?: {
    purchased_usd?: number;
  };
};

/**
 * A single limit window as normalized by the backend and stashed under
 * `quotaData._limits`. `periodCost` is the money the window has spent, and
 * `periodQuota` is the money value the whole window is worth: the backend may
 * derive both from AxonHub usage logs, or the provider may report them itself
 * (Synthetic's weekly credits); both are absent when no figure exists.
 */
export type ProviderQuotaLimit = {
  type: string;
  status: string;
  usageRatio: number;
  ready: boolean;
  window?: string;
  account?: string;
  nextResetAt?: string;
  periodStart?: string;
  periodCost?: number;
  periodQuota?: number;
};

/**
 * One API key of a channel whose keys are independent quota accounts, as
 * serialized by the backend under `quotaData._api_key_quotas`. `ref` is an
 * opaque digest; `index` and `suffix` are display-only. None of them identifies
 * a key for a mutation.
 */
export type ProviderAPIKeyQuota = {
  ref: string;
  index: number;
  suffix: string;
  disabled: boolean;
  /** True when a provider quota observation owns the disable. */
  quotaDisabled: boolean;
  /** `check_failed` when this key could not be read. */
  errorCode?: string;
  status: 'available' | 'warning' | 'exhausted' | 'unknown';
  ready: boolean;
  nextResetAt?: string;
  /** This account's provider payload, including its own `_limits`/`_balances`. */
  quotaData: Record<string, unknown>;
  limits: ProviderQuotaLimit[];
  balances: ProviderQuotaBalance[];
};

function optionalString(value: unknown): string | undefined {
  return typeof value === 'string' && value !== '' ? value : undefined;
}

function optionalNumber(value: unknown): number | undefined {
  return typeof value === 'number' && Number.isFinite(value) ? value : undefined;
}

const NORMALIZED_QUOTA_STATUSES = ['available', 'warning', 'exhausted', 'unknown'] as const;
type NormalizedQuotaStatus = (typeof NORMALIZED_QUOTA_STATUSES)[number];

function isNormalizedQuotaStatus(value: unknown): value is NormalizedQuotaStatus {
  return NORMALIZED_QUOTA_STATUSES.some((status) => status === value);
}

function requiredString(value: unknown): string | undefined {
  return typeof value === 'string' && value.trim() !== '' ? value : undefined;
}

function parseQuotaLimit(entry: unknown): ProviderQuotaLimit | undefined {
  if (typeof entry !== 'object' || entry === null || Array.isArray(entry)) return undefined;

  const limit = entry as Record<string, unknown>;
  const type = requiredString(limit.type);
  const window = requiredString(limit.window);
  const account = optionalString(limit.account);
  if (!type || !window || !isNormalizedQuotaStatus(limit.status)) return undefined;

  const usageRatio = optionalNumber(limit.usageRatio);
  if (usageRatio === undefined || usageRatio < 0 || usageRatio > 1) return undefined;

  if (limit.ready !== undefined && typeof limit.ready !== 'boolean') return undefined;

  const nextResetAt = optionalString(limit.nextResetAt);
  if (limit.nextResetAt !== undefined && nextResetAt === undefined) return undefined;
  if (nextResetAt !== undefined && Number.isNaN(Date.parse(nextResetAt))) return undefined;

  const periodStart = optionalString(limit.periodStart);
  if (limit.periodStart !== undefined && periodStart === undefined) return undefined;

  const periodCost = optionalNumber(limit.periodCost);
  if (limit.periodCost !== undefined && periodCost === undefined) return undefined;

  const periodQuota = optionalNumber(limit.periodQuota);
  if (limit.periodQuota !== undefined && periodQuota === undefined) return undefined;

  return {
    type,
    status: limit.status,
    usageRatio,
    ready: limit.ready === true,
    window,
    account,
    nextResetAt,
    periodStart,
    periodCost,
    periodQuota,
  };
}

export function parseQuotaLimits(quotaData: unknown): ProviderQuotaLimit[] {
  if (typeof quotaData !== 'object' || quotaData === null) return [];

  const raw = (quotaData as { _limits?: unknown })._limits;
  if (!Array.isArray(raw)) return [];

  return raw.flatMap((entry) => {
    const parsed = parseQuotaLimit(entry);
    return parsed ? [parsed] : [];
  });
}

/**
 * Non-window account amounts normalized by the backend and stashed under
 * `quotaData._balances` (for example DeepSeek's prepaid balance). The table
 * renders these as money rows instead of percentage bars.
 */
export function parseQuotaBalances(quotaData: unknown): ProviderQuotaBalance[] {
  if (typeof quotaData !== 'object' || quotaData === null) return [];

  const raw = (quotaData as { _balances?: unknown })._balances;
  if (!Array.isArray(raw)) return [];

  return raw.flatMap((entry) => {
    if (typeof entry !== 'object' || entry === null || Array.isArray(entry)) return [];

    const balance = entry as Record<string, unknown>;
    const total = optionalNumber(balance.total);
    if (total === undefined) return [];

    return [
      {
        account: optionalString(balance.account),
        currency: optionalString(balance.currency),
        total,
        granted: optionalNumber(balance.granted),
        topped_up: optionalNumber(balance.topped_up),
      },
    ];
  });
}

/**
 * Parses the channel's per-key quota rows. Malformed entries are omitted, and an
 * absent `_api_key_quotas` yields an empty list so a stored snapshot taken
 * before per-key collection still renders through the legacy provider branch.
 */
export function parseAPIKeyQuotas(quotaData: unknown): ProviderAPIKeyQuota[] {
  if (typeof quotaData !== 'object' || quotaData === null) return [];

  const raw = (quotaData as { _api_key_quotas?: unknown })._api_key_quotas;
  if (!Array.isArray(raw)) return [];

  return raw.flatMap((entry) => {
    if (typeof entry !== 'object' || entry === null || Array.isArray(entry)) return [];

    const row = entry as Record<string, unknown>;
    const ref = requiredString(row.ref);
    if (!ref) return [];

    const index = optionalNumber(row.index);
    if (index === undefined || !Number.isInteger(index) || index < 1) return [];

    if (!isNormalizedQuotaStatus(row.status)) return [];
    if (typeof row.ready !== 'boolean') return [];
    if (typeof row.disabled !== 'boolean' || typeof row.quotaDisabled !== 'boolean') return [];

    const nextResetAt = optionalString(row.nextResetAt);
    if (row.nextResetAt !== undefined && nextResetAt === undefined) return [];
    if (nextResetAt !== undefined && Number.isNaN(Date.parse(nextResetAt))) return [];

    const nested = typeof row.quotaData === 'object' && row.quotaData !== null && !Array.isArray(row.quotaData)
      ? (row.quotaData as Record<string, unknown>)
      : {};

    return [
      {
        ref,
        index,
        suffix: typeof row.suffix === 'string' ? row.suffix : '',
        disabled: row.disabled,
        quotaDisabled: row.quotaDisabled,
        errorCode: optionalString(row.errorCode),
        status: row.status,
        ready: row.ready,
        nextResetAt,
        quotaData: nested,
        limits: parseQuotaLimits(nested),
        balances: parseQuotaBalances(nested),
      },
    ];
  });
}

/** Formats a normalized balance amount for display. */
export function formatQuotaBalance(value?: number | null): string | undefined {
  if (value == null || !Number.isFinite(value)) return undefined;
  return value.toFixed(2);
}

const CLAUDE_WINDOW_KEYS = {
  '5h': '5h',
  '7d': '7d',
  overage: 'overage',
  primary: '5h',
  secondary: '7d',
} as const;

type ClaudeWindowKey = (typeof CLAUDE_WINDOW_KEYS)[keyof typeof CLAUDE_WINDOW_KEYS];

function getClaudeWindowKey(value: unknown): ClaudeWindowKey | undefined {
  return typeof value === 'string' ? CLAUDE_WINDOW_KEYS[value as keyof typeof CLAUDE_WINDOW_KEYS] : undefined;
}

function parseClaudeReset(value: string | undefined): number | undefined {
  if (!value) return undefined;
  const timestamp = Date.parse(value);
  return Number.isFinite(timestamp) ? timestamp / 1000 : undefined;
}

/**
 * Claude Code has used both `5h`/`7d` and primary/secondary names for its
 * two quota periods. Keep the UI-facing shape stable and fill missing windows
 * from the normalized `_limits` data when the raw provider payload omits one.
 */
function parseClaudeQuotaData(quotaData: unknown, limits: ProviderQuotaLimit[]): ProviderClaudeQuotaData {
  if (typeof quotaData !== 'object' || quotaData === null) return {};

  const source = quotaData as Record<string, unknown>;
  const windows: Record<string, ProviderClaudeQuotaWindow> = {};
  const addWindow = (name: string, value: unknown) => {
    if (typeof value !== 'object' || value === null) return;
    const key = getClaudeWindowKey(name);
    if (key && (name === key || !windows[key])) {
      windows[key] = value as ProviderClaudeQuotaWindow;
    }
  };

  if (typeof source.windows === 'object' && source.windows !== null) {
    for (const [name, value] of Object.entries(source.windows)) {
      addWindow(name, value);
    }
  }

  // Accept providers that expose the two periods directly instead of nesting
  // them under `windows`.
  addWindow('primary', source.primary);
  addWindow('secondary', source.secondary);

  for (const limit of limits) {
    const key = getClaudeWindowKey(limit.window);
    if (!key || windows[key]) continue;

    windows[key] = {
      utilization: limit.usageRatio,
      reset: parseClaudeReset(limit.nextResetAt),
      status: limit.status,
    };
  }

  return {
    ...source,
    windows: windows as ProviderClaudeQuotaData['windows'],
  } as ProviderClaudeQuotaData;
}

export type ProviderQuotaChannel = {
  id: string;
  name: string;
  // Display-sort rank set via the channels page "Order channels" dialog; the
  // load balancer treats higher weight as higher priority, so the quota
  // popover mirrors that order in its 'preferred' sort mode.
  orderingWeight: number;
  // Account identity shared by channels drawing from the same provider account
  // (e.g. the same ZenMux management key). Undefined means the channel has its
  // own quota account.
  accountKey?: string;
  // Names of the channels sharing this account, only set on the representative
  // entry built by the quota popover grouping.
  sharedAccountNames?: string[];
  // Quota routing mode declared on the channel settings; INHERIT defers to the global default.
  quotaRoutingMode: ChannelQuotaRoutingMode;
  quotaStatus: {
    status: 'available' | 'warning' | 'exhausted' | 'unknown';
    nextResetAt: string | null;
    ready: boolean;
    limits: ProviderQuotaLimit[];
    /** Per-key results of a channel whose keys are independent quota accounts. */
    apiKeyQuotas: ProviderAPIKeyQuota[];
  };
} & (
  | {
      type: 'claudecode';
      quotaStatus: {
        quotaData: ProviderClaudeQuotaData;
      };
    }
  | {
      type: 'codex';
      quotaStatus: {
        quotaData: ProviderCodexQuotaData;
      };
    }
  | {
      type: 'xai_subscription';
      quotaStatus: {
        quotaData: ProviderXAISubscriptionQuotaData;
      };
    }
  | {
      type: 'cline';
      quotaStatus: {
        quotaData: ProviderClineQuotaData;
      };
    }
  | {
      type: 'github_copilot';
      quotaStatus: {
        quotaData: ProviderGitHubCopilotQuotaData;
      };
    }
  | {
      type: 'deepseek' | 'deepseek_anthropic';
      quotaStatus: {
        quotaData: ProviderDeepseekQuotaData;
      };
    }
  | {
      type: 'nanogpt';
      quotaStatus: {
        quotaData: ProviderNanoGPTQuotaData;
      };
    }
  | {
      type: 'nanogpt_responses';
      quotaStatus: {
        quotaData: ProviderNanoGPTQuotaData;
      };
    }
  | {
      type: 'opencode_go' | 'opencode_go_anthropic';
      quotaStatus: {
        quotaData: ProviderOpenCodeGoQuotaData;
      };
    }
  | {
      type: 'moonshot_coding';
      quotaStatus: {
        quotaData: ProviderKimiCodeQuotaData;
      };
    }
  | {
      type: 'minimax' | 'minimax_anthropic';
      quotaStatus: {
        quotaData: ProviderMinimaxQuotaData;
      };
    }
  | {
      type: 'zhipu' | 'zhipu_anthropic';
      quotaStatus: {
        quotaData: ProviderZhipuQuotaData;
      };
    }
  | {
      type: 'zai' | 'zai_anthropic';
      quotaStatus: {
        quotaData: ProviderZhipuQuotaData;
      };
    }
  | {
      type: 'zenmux' | 'zenmux_responses' | 'zenmux_anthropic' | 'zenmux_gemini' | 'zenmux_video';
      quotaStatus: {
        quotaData: ProviderZenmuxQuotaData;
      };
    }
  | {
      type: 'openai' | 'openai_responses';
      providerType: 'synthetic';
      quotaStatus: {
        quotaData: ProviderSyntheticQuotaData;
      };
    }
  | {
      type: 'openai' | 'openai_responses';
      providerType: 'neuralwatt';
      quotaStatus: {
        quotaData: ProviderNeuralWattQuotaData;
      };
    }
  | {
      type: 'openai' | 'openai_responses';
      providerType: 'apertis';
      quotaStatus: {
        quotaData: ProviderApertisQuotaData;
      };
    }
  | {
      type: 'openai' | 'openai_responses';
      providerType: 'charm_hyper';
      quotaStatus: {
        quotaData: ProviderCharmHyperQuotaData;
      };
    }
  | {
      type: 'openai' | 'openai_responses';
      providerType: 'deepseek';
      quotaStatus: {
        quotaData: ProviderDeepseekQuotaData;
      };
    }
  | {
      type: 'openai' | 'openai_responses';
      providerType?: undefined;
      quotaStatus: {
        quotaData: ProviderQuotaDataCommon;
      };
    }
  | {
      type: 'commandcode' | 'commandcode_anthropic';
      quotaStatus: {
        quotaData: ProviderCommandCodeQuotaData;
      };
    }
  | {
      type: 'ollama' | 'ollama_anthropic';
      quotaStatus: {
        quotaData: ProviderOllamaQuotaData;
      };
    }
);

type ProviderQuotaStatusNode = {
  status: 'available' | 'warning' | 'exhausted' | 'unknown';
  nextResetAt: string | null;
  ready: boolean;
  quotaData: unknown;
  providerType: string;
  accountKey?: string | null;
};

type QueryChannelNode = {
  id: string;
  name: string;
  type: string;
  orderingWeight: number;
  settings: { quotaRoutingMode: ChannelQuotaRoutingMode } | null;
  disabledAPIKeys: Array<{ key: string; expiresAt: string | null; quotaDisabled: boolean }> | null;
  providerQuotaStatus: ProviderQuotaStatusNode | null;
};

type QueryChannelsResponse = {
  queryChannels: {
    edges: Array<{
      node: QueryChannelNode | null;
    } | null>;
  };
  // Hex SHA-256 of every disabled key across the fetched channels, keyed by
  // the key itself: the same value the backend stamps as a row's ref.
  disabledKeyDigests: Record<string, string>;
};

type QueryChannelNodeWithQuota = QueryChannelNode & {
  providerQuotaStatus: ProviderQuotaStatusNode;
};

function hasProviderQuotaStatus(node: QueryChannelNode | null | undefined): node is QueryChannelNodeWithQuota {
  return node?.providerQuotaStatus != null;
}

type DisabledKeyRecord = { key: string; expiresAt: string | null; quotaDisabled: boolean };

// digestAPIKeys hashes the disabled keys the query carries, using the same hex
// SHA-256 the backend stamps as a per-key quota row's ref, so a row can be
// matched to the disable record it names without exposing the key. Keys that
// cannot be hashed are reported as unresolved so the ownership sync can keep
// their persisted rows instead of clearing them.
async function digestAPIKeys(keys: string[]): Promise<{ digests: Record<string, string>; unresolved: Set<string> }> {
  const digests: Record<string, string> = {};
  const unresolved = new Set<string>();
  const subtle = globalThis.crypto?.subtle;
  if (!subtle) {
    for (const key of keys) {
      unresolved.add(key);
    }
    return { digests, unresolved };
  }

  for (const key of keys) {
    try {
      const bytes = new TextEncoder().encode(key);
      const digest = new Uint8Array(await subtle.digest('SHA-256', bytes));
      digests[key] = Array.from(digest, (byte) => byte.toString(16).padStart(2, '0')).join('');
    } catch {
      // An unavailable digest cannot prove the key's identity.
      unresolved.add(key);
    }
  }
  return { digests, unresolved };
}

// applyDisabledKeyOwnershipSync derives each per-key row's usability and
// ownership from the channel's current disable records as well as from
// collection. A key an operator kept disabled manually is no longer quota-owned
// and the row must stop claiming automatic recovery from the moment the
// mutation lands; a key the operator re-enabled is usable again even when the
// stored row still says parked. The query supplies each key's digest so the row
// it names can be matched without exposing the key.
//
// A null/undefined list means the records are unavailable, not that the channel
// has none; the persisted rows then stay exactly as stored. An empty list is a
// real answer and clears non-matching rows. Unresolved digests are scoped to
// this channel's own keys: a digest that failed to compute for an unrelated
// channel must not protect this channel's rows.
export function applyDisabledKeyOwnershipSync(
  apiKeyQuotas: ProviderAPIKeyQuota[],
  disabledKeys: DisabledKeyRecord[] | null,
  digests: Record<string, string>,
  unresolvedKeys?: Set<string>
): ProviderAPIKeyQuota[] {
  if (apiKeyQuotas.length === 0) return apiKeyQuotas;
  if (disabledKeys == null) return apiKeyQuotas;

  const now = Date.now();
  const refOwnership = new Map<string, boolean>();
  for (const record of disabledKeys) {
    if (record.expiresAt && Date.parse(record.expiresAt) <= now) continue;
    const ref = digests[record.key];
    if (ref) refOwnership.set(ref, record.quotaDisabled === true);
  }

  // A key whose digest could not be computed may still name a stale row; the
  // persisted ownership can only be cleared once every reference resolved.
  // Only this channel's own keys count: unresolved digests from other channels
  // say nothing about this one.
  const unresolved = unresolvedKeys != null && disabledKeys.some((record) => unresolvedKeys.has(record.key));

  return apiKeyQuotas.map((account) => {
    const quotaOwned = refOwnership.get(account.ref);
    if (quotaOwned === undefined) {
      // No active record: the key is usable, and nothing quota-owned.
      if (account.disabled || account.quotaDisabled) {
        if (unresolved) {
          // The key's identity cannot be proven absent from the disable list;
          // keep the persisted ownership and usability as stored.
          return account;
        }
        return { ...account, disabled: false, quotaDisabled: false };
      }
      return account;
    }

    // The key is in the channel's disabled list, so it is not usable; the
    // record's ownership flag decides whether a quota recovery may clear it.
    return { ...account, disabled: true, quotaDisabled: quotaOwned };
  });
}

// parseChannelNode converts a channel with quota status into the discriminated
// provider quota view used by the quota popover.
function parseChannelNode(node: QueryChannelNodeWithQuota): ProviderQuotaChannel {
  const quotaStatus = node.providerQuotaStatus;
  const providerType = quotaStatus.providerType;

  const base = {
    id: node.id,
    name: node.name,
    orderingWeight: node.orderingWeight,
    quotaRoutingMode: node.settings?.quotaRoutingMode ?? 'INHERIT',
    accountKey: optionalString(quotaStatus.accountKey),
    quotaStatus: {
      status: quotaStatus.status,
      nextResetAt: quotaStatus.nextResetAt,
      ready: quotaStatus.ready,
      limits: parseQuotaLimits(quotaStatus.quotaData),
      apiKeyQuotas: parseAPIKeyQuotas(quotaStatus.quotaData),
    },
  };

  if (
    node.type === 'zenmux' ||
    node.type === 'zenmux_responses' ||
    node.type === 'zenmux_anthropic' ||
    node.type === 'zenmux_gemini' ||
    node.type === 'zenmux_video'
  ) {
    return {
      ...base,
      type: node.type as 'zenmux' | 'zenmux_responses' | 'zenmux_anthropic' | 'zenmux_gemini' | 'zenmux_video',
      quotaStatus: { ...base.quotaStatus, quotaData: node.providerQuotaStatus.quotaData as ProviderZenmuxQuotaData },
    };
  }

  if (node.type === 'claudecode') {
    return {
      ...base,
      type: 'claudecode' as const,
      quotaStatus: {
        ...base.quotaStatus,
        quotaData: parseClaudeQuotaData(node.providerQuotaStatus.quotaData, base.quotaStatus.limits),
      },
    };
  }
  if (node.type === 'codex') {
    return {
      ...base,
      type: 'codex' as const,
      quotaStatus: { ...base.quotaStatus, quotaData: node.providerQuotaStatus.quotaData as ProviderCodexQuotaData },
    };
  }
  if (node.type === 'cline') {
    return {
      ...base,
      type: 'cline' as const,
      quotaStatus: { ...base.quotaStatus, quotaData: node.providerQuotaStatus.quotaData as ProviderClineQuotaData },
    };
  }
  if (node.type === 'github_copilot') {
    return {
      ...base,
      type: 'github_copilot' as const,
      quotaStatus: { ...base.quotaStatus, quotaData: node.providerQuotaStatus.quotaData as ProviderGitHubCopilotQuotaData },
    };
  }
  if (node.type === 'deepseek' || node.type === 'deepseek_anthropic') {
    return {
      ...base,
      type: node.type as 'deepseek' | 'deepseek_anthropic',
      quotaStatus: { ...base.quotaStatus, quotaData: node.providerQuotaStatus.quotaData as ProviderDeepseekQuotaData },
    };
  }
  if (node.type === 'nanogpt') {
    return {
      ...base,
      type: 'nanogpt' as const,
      quotaStatus: { ...base.quotaStatus, quotaData: node.providerQuotaStatus.quotaData as ProviderNanoGPTQuotaData },
    };
  }
  if (node.type === 'nanogpt_responses') {
    return {
      ...base,
      type: 'nanogpt_responses' as const,
      quotaStatus: { ...base.quotaStatus, quotaData: node.providerQuotaStatus.quotaData as ProviderNanoGPTQuotaData },
    };
  }
  if (node.type === 'opencode_go' || node.type === 'opencode_go_anthropic') {
    return {
      ...base,
      type: node.type as 'opencode_go' | 'opencode_go_anthropic',
      quotaStatus: { ...base.quotaStatus, quotaData: node.providerQuotaStatus.quotaData as ProviderOpenCodeGoQuotaData },
    };
  }
  if (node.type === 'moonshot_coding') {
    return {
      ...base,
      type: 'moonshot_coding' as const,
      quotaStatus: { ...base.quotaStatus, quotaData: node.providerQuotaStatus.quotaData as ProviderKimiCodeQuotaData },
    };
  }
  if (node.type === 'minimax' || node.type === 'minimax_anthropic') {
    return {
      ...base,
      type: node.type as 'minimax' | 'minimax_anthropic',
      quotaStatus: { ...base.quotaStatus, quotaData: node.providerQuotaStatus.quotaData as ProviderMinimaxQuotaData },
    };
  }
  if (node.type === 'zhipu' || node.type === 'zhipu_anthropic') {
    return {
      ...base,
      type: node.type as 'zhipu' | 'zhipu_anthropic',
      quotaStatus: { ...base.quotaStatus, quotaData: node.providerQuotaStatus.quotaData as ProviderZhipuQuotaData },
    };
  }
  if (node.type === 'zai' || node.type === 'zai_anthropic') {
    return {
      ...base,
      type: node.type as 'zai' | 'zai_anthropic',
      quotaStatus: { ...base.quotaStatus, quotaData: node.providerQuotaStatus.quotaData as ProviderZhipuQuotaData },
    };
  }
  if (node.type === 'openai' || node.type === 'openai_responses') {
    const typeVal = node.type as 'openai' | 'openai_responses';
    if (providerType === 'synthetic') {
      return {
        ...base,
        type: typeVal,
        providerType: 'synthetic' as const,
        quotaStatus: { ...base.quotaStatus, quotaData: node.providerQuotaStatus.quotaData as ProviderSyntheticQuotaData },
      };
    }
    if (providerType === 'neuralwatt') {
      return {
        ...base,
        type: typeVal,
        providerType: 'neuralwatt' as const,
        quotaStatus: { ...base.quotaStatus, quotaData: node.providerQuotaStatus.quotaData as ProviderNeuralWattQuotaData },
      };
    }
    if (providerType === 'apertis') {
      return {
        ...base,
        type: typeVal,
        providerType: 'apertis' as const,
        quotaStatus: { ...base.quotaStatus, quotaData: node.providerQuotaStatus.quotaData as ProviderApertisQuotaData },
      };
    }
    if (providerType === 'charm_hyper') {
      return {
        ...base,
        type: typeVal,
        providerType: 'charm_hyper' as const,
        quotaStatus: { ...base.quotaStatus, quotaData: node.providerQuotaStatus.quotaData as ProviderCharmHyperQuotaData },
      };
    }
    if (providerType === 'deepseek') {
      return {
        ...base,
        type: typeVal,
        providerType: 'deepseek' as const,
        quotaStatus: { ...base.quotaStatus, quotaData: node.providerQuotaStatus.quotaData as ProviderDeepseekQuotaData },
      };
    }
    return {
      ...base,
      type: typeVal,
      providerType: undefined,
      quotaStatus: { ...base.quotaStatus, quotaData: node.providerQuotaStatus.quotaData as ProviderQuotaDataCommon },
    };
  }

  if (node.type === 'commandcode' || node.type === 'commandcode_anthropic') {
    return {
      ...base,
      type: node.type as 'commandcode' | 'commandcode_anthropic',
      quotaStatus: { ...base.quotaStatus, quotaData: node.providerQuotaStatus.quotaData as ProviderCommandCodeQuotaData },
    };
  }
  if (node.type === 'ollama' || node.type === 'ollama_anthropic') {
    return {
      ...base,
      type: node.type as 'ollama' | 'ollama_anthropic',
      quotaStatus: { ...base.quotaStatus, quotaData: node.providerQuotaStatus.quotaData as ProviderOllamaQuotaData },
    };
  }
  return {
    ...base,
    type: node.type as ProviderQuotaChannel['type'],
    quotaStatus: { ...base.quotaStatus, quotaData: node.providerQuotaStatus.quotaData as ProviderQuotaDataCommon },
  };
}

export function useProviderQuotaStatuses() {
  const query = useQuery({
    queryKey: ['provider-quotas'],
    queryFn: async () => {
      const input = {
        where: {
          // A channel whose last key this service disabled on quota grounds
          // stays visible (and pollable) until a key recovers; a later manual
          // channel disable clears autoDisabledAt and drops out here.
          or: [
            { statusIn: ['enabled'] },
            {
              statusIn: ['disabled'],
              autoDisabledAtNotNil: true,
              errorMessageHasPrefix: 'All API keys disabled',
            },
          ],
        },
      };
      const data = await graphqlRequest<QueryChannelsResponse>(PROVIDER_QUOTA_STATUSES_QUERY, { input });
      const keys = [
        ...new Set((data.queryChannels.edges ?? []).flatMap((edge) => (edge?.node?.disabledAPIKeys ?? []).map((key) => key.key))),
      ];
      const { digests, unresolved } = await digestAPIKeys(keys);
      return { ...data, disabledKeyDigests: digests, disabledKeyUnresolved: unresolved };
    },
    refetchInterval: 60000,
    refetchIntervalInBackground: true,
  });

  const nodes = useMemo(
    () =>
      (query.data?.queryChannels?.edges ?? [])
        .map((edge) => edge?.node ?? null)
        .filter(hasProviderQuotaStatus)
        .filter((c) => {
          // Skip channels that have no credentials configured, since they cannot be
          // checked and only add noise to the quota popover. Other failures remain
          // available with their generic status for administrators to inspect.
          const quotaData = c.providerQuotaStatus.quotaData as { error?: string; error_code?: string } | undefined;
          return quotaData?.error_code !== 'missing_credentials' && quotaData?.error !== 'channel has no credentials';
        }),
    [query.data]
  );

  // The per-key view is persisted during collection, so its ownership only
  // reflects the last collection pass. Derive it from the channel's current
  // disable records on every render: a key an operator took over manually must
  // stop claiming quota ownership and automatic recovery immediately. The
  // query precomputes the digest of every disabled key, the same value the
  // backend stamps as a row's ref.
  const channels = useMemo(
    () =>
      // The spread below would widen the discriminated union, so name the
      // element type explicitly: the runtime shape is parseChannelNode's,
      // with only the derived per-key rows substituted, and the cast restores
      // the type-level correlation between type and quotaData the spread
      // destroys (apiKeyQuotas has one declared type on every arm).
      nodes.map((node): ProviderQuotaChannel => {
        const channel = parseChannelNode(node);
        const digests = query.data?.disabledKeyDigests ?? {};
        const unresolved = query.data?.disabledKeyUnresolved;
        const derived = applyDisabledKeyOwnershipSync(channel.quotaStatus.apiKeyQuotas, node.disabledAPIKeys, digests, unresolved);
        return { ...channel, quotaStatus: { ...channel.quotaStatus, apiKeyQuotas: derived } } as ProviderQuotaChannel;
      }),
    [nodes, query.data]
  );

  return {
    channels,
    isLoading: query.isLoading,
    isError: query.isError,
    error: query.error,
    isFetching: query.isFetching,
  };
}
