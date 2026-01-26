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

export type ProviderQuotaChannel = {
  id: string;
  name: string;
  type: string;
  quotaStatus?: {
    status: string;
    nextResetAt: string | null;
    ready: boolean;
    quotaData: any;
  };
};

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

  // Filter for OAuth channels (claudecode, codex) - check both lowercase and PascalCase
  const oauthChannels = channels.filter((c: any) => {
    const type = c.type?.toLowerCase();
    const match = ['claudecode', 'codex'].includes(type);
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
