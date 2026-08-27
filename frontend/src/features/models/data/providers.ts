import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import { graphqlRequest } from '@/gql/graphql';
import i18n from '@/lib/i18n';
import providersDataRaw from './providers.json';
import { providersDataSchema, type ProvidersData } from './providers.schema';

const fallbackProvidersData = providersDataSchema.parse(providersDataRaw);

const PROVIDERS_CATALOG_QUERY = `
  query ProvidersCatalog($filtered: Boolean) {
    providersCatalog(filtered: $filtered) {
      data
      fetchedAt
      source
      filtered
    }
  }
`;

const REFRESH_PROVIDERS_CATALOG_MUTATION = `
  mutation RefreshProvidersCatalog {
    refreshProvidersCatalog {
      data
      fetchedAt
      source
      filtered
    }
  }
`;

export type ProvidersCatalogInfo = {
  data: ProvidersData;
  source: string;
  fetchedAt?: string | null;
  filtered: boolean;
};

async function loadProvidersCatalog(filtered: boolean): Promise<ProvidersCatalogInfo> {
  try {
    const result = await graphqlRequest<{
      providersCatalog: {
        data: unknown;
        fetchedAt?: string | null;
        source: string;
        filtered: boolean;
      };
    }>(PROVIDERS_CATALOG_QUERY, { filtered });

    return {
      data: providersDataSchema.parse(result.providersCatalog.data),
      source: result.providersCatalog.source,
      fetchedAt: result.providersCatalog.fetchedAt,
      filtered: result.providersCatalog.filtered,
    };
  } catch (error) {
    console.error('Failed to load providers catalog, falling back to bundled data:', error);
    return {
      data: fallbackProvidersData,
      source: 'fallback',
      fetchedAt: null,
      filtered,
    };
  }
}

export function useProvidersCatalog(filtered: boolean) {
  return useQuery({
    queryKey: ['providers-catalog', filtered],
    queryFn: () => loadProvidersCatalog(filtered),
    staleTime: 5 * 60 * 1000,
    placeholderData: {
      data: fallbackProvidersData,
      source: 'fallback',
      fetchedAt: null,
      filtered,
    },
  });
}

export function useProvidersData() {
  const query = useProvidersCatalog(false);
  return {
    ...query,
    data: query.data?.data ?? fallbackProvidersData,
    source: query.data?.source ?? 'fallback',
    fetchedAt: query.data?.fetchedAt ?? null,
  };
}

export function useDevelopersData() {
  const query = useProvidersCatalog(true);
  return {
    ...query,
    data: query.data?.data ?? fallbackProvidersData,
    source: query.data?.source ?? 'fallback',
    fetchedAt: query.data?.fetchedAt ?? null,
  };
}

export function useRefreshProvidersCatalog() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async () => {
      const result = await graphqlRequest<{
        refreshProvidersCatalog: {
          data: unknown;
          fetchedAt?: string | null;
          source: string;
          filtered: boolean;
        };
      }>(REFRESH_PROVIDERS_CATALOG_MUTATION);

      return {
        data: providersDataSchema.parse(result.refreshProvidersCatalog.data),
        source: result.refreshProvidersCatalog.source,
        fetchedAt: result.refreshProvidersCatalog.fetchedAt,
        filtered: result.refreshProvidersCatalog.filtered,
      } satisfies ProvidersCatalogInfo;
    },
    onSuccess: (catalog) => {
      queryClient.setQueryData(['providers-catalog', true], catalog);
      queryClient.invalidateQueries({ queryKey: ['providers-catalog'] });
      toast.success(i18n.t('models.catalog.refreshSuccess'));
    },
    onError: () => {
      toast.error(i18n.t('models.catalog.refreshFailed'));
    },
  });
}
