import { useMemo } from 'react';
import { useQuery } from '@tanstack/react-query';
import { graphqlRequest } from '@/gql/graphql';
import { useDebounce } from '@/hooks/use-debounce';
import { channelModelEntrySchema, type ChannelModelEntry, type ChannelSettings } from './schema';

export type ModelPreviewSettings = Pick<
  ChannelSettings,
  'extraModelPrefix' | 'modelMappings' | 'autoTrimedModelPrefixes' | 'hideOriginalModels' | 'hideMappedModels' | 'lowercaseModelId'
>;

const CHANNEL_MODEL_PREVIEW_QUERY = `
  query PreviewChannelModelEntries($input: ChannelModelPreviewInput!) {
    previewChannelModelEntries(input: $input) {
      requestModel
      actualModel
      source
    }
  }
`;

export function useChannelModelPreview(models: string[], settings: ModelPreviewSettings | null | undefined, enabled: boolean) {
  // Send only model-name settings, never channel credentials or unrelated options.
  const serialized = JSON.stringify({
    supportedModels: models,
    settings: {
      extraModelPrefix: settings?.extraModelPrefix ?? '',
      modelMappings: settings?.modelMappings ?? [],
      autoTrimedModelPrefixes: settings?.autoTrimedModelPrefixes ?? [],
      hideOriginalModels: settings?.hideOriginalModels ?? false,
      hideMappedModels: settings?.hideMappedModels ?? false,
      lowercaseModelId: settings?.lowercaseModelId ?? false,
    },
  });
  const debounced = useDebounce(serialized, 300);
  const input = useMemo(() => JSON.parse(debounced), [debounced]);
  const query = useQuery({
    queryKey: ['channel-model-preview', input],
    queryFn: async () => {
      const data = await graphqlRequest<{ previewChannelModelEntries: ChannelModelEntry[] }>(CHANNEL_MODEL_PREVIEW_QUERY, { input });
      return channelModelEntrySchema.array().parse(data.previewChannelModelEntries);
    },
    enabled,
  });
  return { ...query, isUpdating: query.isFetching || serialized !== debounced };
}
