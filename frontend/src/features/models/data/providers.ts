import { useQuery } from '@tanstack/react-query';
import providersDataRaw from './providers.json';
import { providersDataSchema, type ProvidersData } from './providers.schema';

const PROVIDERS_URL = 'https://raw.githubusercontent.com/ThinkInAIXYZ/PublicProviderConf/refs/heads/dev/dist/all.json';
const DEVELOPERS_URL =
  'https://raw.githubusercontent.com/looplj/axonhub/refs/heads/unstable/frontend/src/features/models/data/providers.json';

export function useProvidersData() {
  return useQuery<ProvidersData>({
    queryKey: ['providers-data'],
    queryFn: async () => {
      try {
        const response = await fetch(PROVIDERS_URL);
        if (!response.ok) {
          throw new Error('Failed to fetch providers data');
        }
        const data = await response.json();
        return providersDataSchema.parse(data);
      } catch (error) {
        console.error('Failed to fetch remote providers data, falling back to local:', error);
        return providersDataSchema.parse(providersDataRaw);
      }
    },
    staleTime: 1000 * 60 * 60 * 24, // 1 day
    placeholderData: providersDataSchema.parse(providersDataRaw),
  });
}

export function useDevelopersData() {
  return useQuery<ProvidersData>({
    queryKey: ['providers-data'],
    queryFn: async () => {
      try {
        const response = await fetch(DEVELOPERS_URL);
        if (!response.ok) {
          throw new Error('Failed to fetch developers data');
        }
        const data = await response.json();
        return providersDataSchema.parse(data);
      } catch (error) {
        console.error('Failed to fetch remote developers data, falling back to local:', error);
        return providersDataSchema.parse(providersDataRaw);
      }
    },
    staleTime: 1000 * 60 * 60 * 24, // 1 day
    placeholderData: providersDataSchema.parse(providersDataRaw),
  });
}
