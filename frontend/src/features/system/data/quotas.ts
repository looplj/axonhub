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
  )

export function useProviderQuotaStatuses() {
  const { data, error } = useQuery({
    queryKey: ['provider-quotas'],
    queryFn: async () => {
      const input = {
        where: {
          statusIn: ['enabled']
        }
      };
      return graphqlRequest<any>(PROVIDER_QUOTA_STATUSES_QUERY, { input });
    },
    refetchInterval: 60000, // Refetch every minute
  });

  const channels = data?.queryChannels?.edges?.map((e: any) => e.node) || [];

  // Filter for OAuth channels (claudecode, codex, github_copilot) - check both lowercase and PascalCase
  const oauthChannels = channels.filter((c: any) => {
    const type = c.type?.toLowerCase();
    const match = ['claudecode', 'codex', 'github_copilot'].includes(type);
    return match;
  });

  // Map to standard format - providerQuotaStatus is a single object, not an edge/node structure
  return oauthChannels.map((channel: any): ProviderQuotaChannel => {
    const quotaStatus = channel.providerQuotaStatus;
    return {
      id: channel.id,
      name: channel.name,
      type: channel.type,
      quotaStatus,
    };
  });
}
