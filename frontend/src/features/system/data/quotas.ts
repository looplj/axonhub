import { useQuery } from '@tanstack/react-query';
import { graphqlRequest } from '@/gql/graphql';

const CHECK_PROVIDER_QUOTAS_QUERY = `
  mutation CheckProviderQuotas {
    checkProviderQuotas
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
        }
      }
    }
  }
`;

export async function checkProviderQuotas() {
  return graphqlRequest(CHECK_PROVIDER_QUOTAS_QUERY);
}

type ProviderQuotaDataCommon = {
  plan_type?: string;
  error?: string;
}

type ProviderClaudeQuotaData = ProviderQuotaDataCommon & {
  windows?: {
    '5h'?: { utilization?: number; reset?: number; status?: string };
    '7d'?: { utilization?: number; reset?: number; status?: string };
    overage?: { utilization?: number; reset?: number; status?: string };
  };
  representative_claim?: string;
}

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
}


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
}

export type NanoGPTQuotaWindow = {
  used?: number;
  remaining?: number;
  percentUsed?: number;
  resetAt?: number;
}

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
}

export type ProviderWaferQuotaData = ProviderQuotaDataCommon & {
  current_period_used_percent?: number | null;
  remaining_included_requests?: number | null;
  included_request_limit?: number | null;
  overage_request_count?: number | null;
  window_start?: string | null;
  window_end?: string | null;
  plan_tier?: string | null;
}

export type ProviderSyntheticQuotaData = ProviderQuotaDataCommon & {
  weeklyTokenLimit?: { percentRemaining?: number | null; remainingCredits?: string | null; maxCredits?: string | null; nextRegenAt?: string | null } | null;
  rollingFiveHourLimit?: { limited?: boolean | null; remaining?: number | null; max?: number | null; nextTickAt?: string | null; tickPercent?: number | null } | null;
}

export type ProviderNeuralWattQuotaData = ProviderQuotaDataCommon & {
  balance?: { credits_remaining_usd?: number | null; total_credits_usd?: number | null } | null;
  subscription?: { kwh_included?: number | null; kwh_used?: number | null; kwh_remaining?: number | null; in_overage?: boolean | null; status?: string | null; plan?: string | null } | null;
}

export type ProviderQuotaChannel = {
  id: string;
  name: string;
  quotaStatus?: {
    status: 'available' | 'warning' | 'exhausted' | 'unknown';
    nextResetAt: string | null;
    ready: boolean;
  };
} & (
    | {
      type: 'claudecode'
      quotaStatus?: {
        quotaData: ProviderClaudeQuotaData
      }
    }
    | {
      type: 'codex'
      quotaStatus?: {
        quotaData: ProviderCodexQuotaData
      }
    }
    | {
      type: 'github_copilot'
      quotaStatus?: {
        quotaData: ProviderGitHubCopilotQuotaData
      }
    }
    | {
      type: 'nanogpt'
      quotaStatus?: {
        quotaData: ProviderNanoGPTQuotaData
      }
    }
    | {
      type: 'nanogpt_responses'
      quotaStatus?: {
        quotaData: ProviderNanoGPTQuotaData
      }
    }
    | {
      type: 'openai'
      providerType: 'wafer'
      quotaStatus?: {
        quotaData: ProviderWaferQuotaData
      }
    }
    | {
      type: 'openai'
      providerType: 'synthetic'
      quotaStatus?: {
        quotaData: ProviderSyntheticQuotaData
      }
    }
    | {
      type: 'openai'
      providerType: 'neuralwatt'
      quotaStatus?: {
        quotaData: ProviderNeuralWattQuotaData
      }
    }
    | {
      type: 'openai'
      providerType?: undefined
      quotaStatus?: {
        quotaData: ProviderQuotaDataCommon
      }
    }
  )

export function useProviderQuotaStatuses() {
  const { data } = useQuery({
    queryKey: ['provider-quotas'],
    queryFn: async () => {
      const input = {
        where: {
          statusIn: ['enabled']
        }
      };
      return graphqlRequest<any>(PROVIDER_QUOTA_STATUSES_QUERY, { input });
    },
    refetchInterval: 60000,
    refetchIntervalInBackground: true,
  });

  const channels = data?.queryChannels?.edges?.map((e: any) => e.node) || [];

  // Filter for quota-enabled channels (any channel with providerQuotaStatus)
  const oauthChannels = channels.filter((c: any) => c.providerQuotaStatus != null);

  // Map to standard format - providerQuotaStatus is a single object, not an edge/node structure
  return oauthChannels.map((channel: any): ProviderQuotaChannel => {
    const quotaStatus = channel.providerQuotaStatus;
    const providerType = quotaStatus?.providerType;
    return {
      id: channel.id,
      name: channel.name,
      type: channel.type,
      ...(channel.type === 'openai' ? { providerType: providerType || undefined } : {}),
      quotaStatus,
    };
  });
}
