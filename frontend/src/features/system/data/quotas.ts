import { useQuery } from '@tanstack/react-query';
import { graphqlRequest } from '@/gql/graphql';

const CHECK_PROVIDER_QUOTAS_QUERY = `
  mutation CheckProviderQuotas {
    checkProviderQuotas
  }
`;

const RESET_CHANNEL_QUOTA_NOW_MUTATION = `
  mutation ResetChannelQuotaNow($channelID: ID!) {
    resetChannelQuotaNow(channelID: $channelID)
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
          providerQuotaStatus {
            status
            nextResetAt
            ready
            quotaData
            providerType
          }
          settings {
            providerQuota {
              opencodeGo {
                workspaceId
              }
            }
          }
        }
      }
    }
  }
`;

export async function checkProviderQuotas() {
  return graphqlRequest(CHECK_PROVIDER_QUOTAS_QUERY);
}

export async function resetChannelQuotaNow(channelID: string) {
  return graphqlRequest(RESET_CHANNEL_QUOTA_NOW_MUTATION, { channelID });
}

type ProviderQuotaDataCommon = {
  plan_type?: string;
  error?: string;
};

type ProviderClaudeQuotaData = ProviderQuotaDataCommon & {
  windows?: {
    '5h'?: { utilization?: number; reset?: number; status?: string };
    '7d'?: { utilization?: number; reset?: number; status?: string };
    overage?: { utilization?: number; reset?: number; status?: string };
  };
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

export type ProviderWaferQuotaData = ProviderQuotaDataCommon & {
  current_period_used_percent?: number | null;
  remaining_included_requests?: number | null;
  included_request_limit?: number | null;
  overage_request_count?: number | null;
  window_start?: string | null;
  window_end?: string | null;
  plan_tier?: string | null;
};

export type ProviderSyntheticQuotaData = ProviderQuotaDataCommon & {
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
};

export type ProviderZhipuQuotaData = ProviderQuotaDataCommon & {
  rows?: ZhipuWindowRow[];
  level?: string;
};

export type ClineQuotaWindow = {
  items_count: number;
  used_cost_units: number;
  limit_cost_units: number;
  remaining_cost_units: number;
  credits_used: number;
  usage_ratio?: number;
  usage_percent?: number;
  next_reset_at?: string | null;
};

type ClineBalance = {
  raw_balance?: number | null;
  unit_note?: string;
};

type ClineUsageFetch = {
  pages: number;
  items_seen: number;
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

export type ProviderClineQuotaData = ProviderClinePassQuotaData | ProviderClineDirectQuotaData | ProviderClineErrorQuotaData;

export function isClinePassPoolQuotaData(qd: ProviderClineQuotaData): qd is ProviderClinePassQuotaData {
  return qd.pool === 'cline_pass';
}

export type ProviderQuotaChannel = {
  id: string;
  name: string;
  quotaStatus: {
    status: 'available' | 'warning' | 'exhausted' | 'unknown';
    nextResetAt: string | null;
    ready: boolean;
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
      workspaceId?: string | null;
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
      type: 'openai' | 'openai_responses';
      providerType: 'wafer';
      quotaStatus: {
        quotaData: ProviderWaferQuotaData;
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
      providerType?: undefined;
      quotaStatus: {
        quotaData: ProviderQuotaDataCommon;
      };
    }
);

type ProviderQuotaStatusNode = {
  status: 'available' | 'warning' | 'exhausted' | 'unknown';
  nextResetAt: string | null;
  ready: boolean;
  quotaData: unknown;
  providerType: string;
};

type QueryChannelNode = {
  id: string;
  name: string;
  type: string;
  providerQuotaStatus: ProviderQuotaStatusNode | null;
  settings?: {
    providerQuota?: {
      opencodeGo?: {
        workspaceId?: string | null;
      } | null;
    } | null;
  } | null;
};

type QueryChannelsResponse = {
  queryChannels: {
    edges: Array<{
      node: QueryChannelNode | null;
    } | null>;
  };
};

type QueryChannelNodeWithQuota = QueryChannelNode & {
  providerQuotaStatus: ProviderQuotaStatusNode;
};

function hasProviderQuotaStatus(node: QueryChannelNode | null | undefined): node is QueryChannelNodeWithQuota {
  return node?.providerQuotaStatus != null;
}

function parseChannelNode(node: QueryChannelNodeWithQuota): ProviderQuotaChannel {
  const quotaStatus = node.providerQuotaStatus;
  const providerType = quotaStatus.providerType;

  const base = {
    id: node.id,
    name: node.name,
    quotaStatus: {
      status: quotaStatus.status,
      nextResetAt: quotaStatus.nextResetAt,
      ready: quotaStatus.ready,
    },
  };

  if (node.type === 'claudecode') {
    return {
      ...base,
      type: 'claudecode' as const,
      quotaStatus: { ...base.quotaStatus, quotaData: node.providerQuotaStatus.quotaData as ProviderClaudeQuotaData },
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
      workspaceId: node.settings?.providerQuota?.opencodeGo?.workspaceId ?? null,
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
  if (node.type === 'openai' || node.type === 'openai_responses') {
    const typeVal = node.type as 'openai' | 'openai_responses';
    if (providerType === 'wafer') {
      return {
        ...base,
        type: typeVal,
        providerType: 'wafer' as const,
        quotaStatus: { ...base.quotaStatus, quotaData: node.providerQuotaStatus.quotaData as ProviderWaferQuotaData },
      };
    }
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
    return {
      ...base,
      type: typeVal,
      providerType: undefined,
      quotaStatus: { ...base.quotaStatus, quotaData: node.providerQuotaStatus.quotaData as ProviderQuotaDataCommon },
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
          statusIn: ['enabled'],
        },
      };
      return graphqlRequest<QueryChannelsResponse>(PROVIDER_QUOTA_STATUSES_QUERY, { input });
    },
    refetchInterval: 60000,
    refetchIntervalInBackground: true,
  });

  const channels = (query.data?.queryChannels?.edges ?? [])
    .map((edge) => edge?.node ?? null)
    .filter(hasProviderQuotaStatus)
    .filter((c) => {
      // Skip channels that have no credentials configured, since they cannot be
      // checked and only add noise to the quota popover. Other errors are still
      // shown so admins can spot credential/permission issues.
      const quotaData = c.providerQuotaStatus.quotaData as { error?: string } | undefined;
      return quotaData?.error !== 'channel has no credentials';
    })
    .map(parseChannelNode);

  return {
    channels,
    isLoading: query.isLoading,
    isError: query.isError,
    error: query.error,
    isFetching: query.isFetching,
  };
}
