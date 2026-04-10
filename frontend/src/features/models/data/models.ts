import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { graphqlRequest } from '@/gql/graphql';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';
import { useErrorHandler } from '@/hooks/use-error-handler';
import { Model, ModelConnection, CreateModelInput, UpdateModelInput, modelConnectionSchema, modelSchema } from './schema';

const MODELS_QUERY = `
  query GetModels(
    $first: Int
    $after: Cursor
    $last: Int
    $before: Cursor
    $where: ModelWhereInput
    $orderBy: ModelOrder
  ) {
    models(first: $first, after: $after, last: $last, before: $before, where: $where, orderBy: $orderBy) {
      edges {
        node {
          id
          createdAt
          updatedAt
          developer
          modelID
          icon
          type
          name
          group
          modelCard {
            reasoning {
              supported
              default
            }
            toolCall
            temperature
            modalities {
              input
              output
            }
            vision
            cost {
              input
              output
              cacheRead
              cacheWrite
            }
            limit {
              context
              output
            }
            knowledge
            releaseDate
            lastUpdated
          }
          settings {
            associations {
              type
              priority
              disabled
              channelModel {
                channelId
                modelId
              }
              channelRegex {
                channelId
                pattern
              }
              regex {
                pattern
                exclude {
                  channelNamePattern
                  channelIds
                  channelTags
                }
              }
              modelId {
                modelId
                exclude {
                  channelNamePattern
                  channelIds
                  channelTags
                }
              }
              channelTagsModel {
                channelTags
                modelId
              }
              channelTagsRegex {
                channelTags
                pattern
              }
            }
          }
          status
          remark
          associatedChannelCount
        }
        cursor
      }
      pageInfo {
        hasNextPage
        hasPreviousPage
        startCursor
        endCursor
      }
      totalCount
    }
  }
`;

const CREATE_MODEL_MUTATION = `
  mutation CreateModel($input: CreateModelInput!) {
    createModel(input: $input) {
      id
      createdAt
      updatedAt
      developer
      modelID
      icon
      type
      name
      group
      modelCard {
        reasoning {
          supported
          default
        }
        toolCall
        temperature
        modalities {
          input
          output
        }
        vision
        cost {
          input
          output
          cacheRead
          cacheWrite
        }
        limit {
          context
          output
        }
        knowledge
        releaseDate
        lastUpdated
      }
      settings {
        associations {
          type
          priority
          disabled
          channelModel {
            channelId
            modelId
          }
          channelRegex {
            channelId
            pattern
          }
          regex {
            pattern
            exclude {
              channelNamePattern
              channelIds
              channelTags
            }
          }
          modelId {
            modelId
            exclude {
              channelNamePattern
              channelIds
              channelTags
            }
          }
        }
      }
      status
      remark
      associatedChannelCount
    }
  }
`;

const BULK_CREATE_MODELS_MUTATION = `
  mutation BulkCreateModels($inputs: [CreateModelInput!]!) {
    bulkCreateModels(inputs: $inputs) {
      id
      createdAt
      updatedAt
      developer
      modelID
      icon
      type
      name
      group
      modelCard {
        reasoning {
          supported
          default
        }
        toolCall
        temperature
        modalities {
          input
          output
        }
        vision
        cost {
          input
          output
          cacheRead
          cacheWrite
        }
        limit {
          context
          output
        }
        knowledge
        releaseDate
        lastUpdated
      }
      settings {
        associations {
          type
          priority
          disabled
          channelModel {
            channelId
            modelId
          }
          channelRegex {
            channelId
            pattern
          }
          regex {
            pattern
            exclude {
              channelNamePattern
              channelIds
              channelTags
            }
          }
          modelId {
            modelId
            exclude {
              channelNamePattern
              channelIds
              channelTags
            }
          }
        }
      }
      status
      remark
      associatedChannelCount
    }
  }
`;

const UPDATE_MODEL_MUTATION = `
  mutation UpdateModel($id: ID!, $input: UpdateModelInput!) {
    updateModel(id: $id, input: $input) {
      id
      createdAt
      updatedAt
      developer
      modelID
      icon
      type
      name
      group
      modelCard {
        reasoning {
          supported
          default
        }
        toolCall
        temperature
        modalities {
          input
          output
        }
        vision
        cost {
          input
          output
          cacheRead
          cacheWrite
        }
        limit {
          context
          output
        }
        knowledge
        releaseDate
        lastUpdated
      }
      settings {
        associations {
          type
          priority
          disabled
          channelModel {
            channelId
            modelId
          }
          channelRegex {
            channelId
            pattern
          }
          regex {
            pattern
            exclude {
              channelNamePattern
              channelIds
              channelTags
            }
          }
          modelId {
            modelId
            exclude {
              channelNamePattern
              channelIds
              channelTags
            }
          }
        }
      }
      status
      remark
      associatedChannelCount
    }
  }
`;

const DELETE_MODEL_MUTATION = `
  mutation DeleteModel($id: ID!) {
    deleteModel(id: $id)
  }
`;

const BULK_DISABLE_MODELS_MUTATION = `
  mutation BulkDisableModels($ids: [ID!]!) {
    bulkDisableModels(ids: $ids)
  }
`;

const BULK_ENABLE_MODELS_MUTATION = `
  mutation BulkEnableModels($ids: [ID!]!) {
    bulkEnableModels(ids: $ids)
  }
`;

const QUERY_UNASSOCIATED_CHANNELS = `
  query QueryUnassociatedChannels {
    queryUnassociatedChannels {
      channel {
        id
        name
        type
        status
      }
      models
    }
  }
`;

interface QueryModelsArgs {
  first?: number;
  after?: string;
  last?: number;
  before?: string;
  where?: Record<string, any>;
  orderBy?: {
    field: 'CREATED_AT' | 'UPDATED_AT' | 'NAME' | 'MODEL_ID';
    direction: 'ASC' | 'DESC';
  };
}

export function useQueryModels(args: QueryModelsArgs) {
  return useQuery({
    queryKey: ['models', args],
    queryFn: async () => {
      const data = await graphqlRequest<{ models: ModelConnection }>(MODELS_QUERY, args);
      return modelConnectionSchema.parse(data.models);
    },
  });
}

interface QueryAllModelsArgs {
  where?: Record<string, any>;
}

export function useQueryAllModels(args: QueryAllModelsArgs) {
  return useQuery({
    queryKey: ['models', 'all', args],
    queryFn: async () => {
      const data = await graphqlRequest<{ models: ModelConnection }>(MODELS_QUERY, {
        first: 10000,
        ...args,
      });
      return modelConnectionSchema.parse(data.models);
    },
  });
}

export function useCreateModel() {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const { handleError } = useErrorHandler();

  return useMutation({
    mutationFn: async (input: CreateModelInput) => {
      const data = await graphqlRequest<{ createModel: Model }>(CREATE_MODEL_MUTATION, { input });
      return modelSchema.parse(data.createModel);
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['models'] });
      toast.success(t('models.messages.createSuccess'));
    },
    onError: (error) => {
      handleError(error, { context: t('models.dialogs.create.title') });
    },
  });
}

export function useBulkCreateModels() {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const { handleError } = useErrorHandler();

  return useMutation({
    mutationFn: async (inputs: CreateModelInput[]) => {
      const data = await graphqlRequest<{ bulkCreateModels: Model[] }>(BULK_CREATE_MODELS_MUTATION, { inputs });
      return data.bulkCreateModels.map((model) => modelSchema.parse(model));
    },
    onSuccess: (_data, variables) => {
      queryClient.invalidateQueries({ queryKey: ['models'] });
      toast.success(t('models.messages.bulkCreateSuccess', { count: variables.length }));
    },
    onError: (error) => {
      handleError(error, { context: 'Bulk Create Models' });
    },
  });
}

export function useUpdateModel() {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const { handleError } = useErrorHandler();

  return useMutation({
    mutationFn: async ({ id, input }: { id: string; input: UpdateModelInput }) => {
      const data = await graphqlRequest<{ updateModel: Model }>(UPDATE_MODEL_MUTATION, { id, input });
      return modelSchema.parse(data.updateModel);
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['models'] });
      toast.success(t('models.messages.updateSuccess'));
    },
    onError: (error) => {
      handleError(error, { context: t('models.dialogs.edit.title') });
    },
  });
}

export function useDeleteModel() {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const { handleError } = useErrorHandler();

  return useMutation({
    mutationFn: async (id: string) => {
      await graphqlRequest(DELETE_MODEL_MUTATION, { id });
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['models'] });
      toast.success(t('models.messages.deleteSuccess'));
    },
    onError: (error) => {
      handleError(error, { context: 'Delete Model' });
    },
  });
}

export function useBulkDisableModels() {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const { handleError } = useErrorHandler();

  return useMutation({
    mutationFn: async (ids: string[]) => {
      try {
        const data = await graphqlRequest<{ bulkDisableModels: boolean }>(BULK_DISABLE_MODELS_MUTATION, { ids });
        return data.bulkDisableModels;
      } catch (error) {
        handleError(error, { context: 'Bulk Disable Models' });
        throw error;
      }
    },
    onSuccess: (_data, variables) => {
      queryClient.invalidateQueries({ queryKey: ['models'] });
      toast.success(t('models.messages.bulkDisableSuccess', { count: variables.length }));
    },
  });
}

export function useBulkEnableModels() {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const { handleError } = useErrorHandler();

  return useMutation({
    mutationFn: async (ids: string[]) => {
      try {
        const data = await graphqlRequest<{ bulkEnableModels: boolean }>(BULK_ENABLE_MODELS_MUTATION, { ids });
        return data.bulkEnableModels;
      } catch (error) {
        handleError(error, { context: 'Bulk Enable Models' });
        throw error;
      }
    },
    onSuccess: (_data, variables) => {
      queryClient.invalidateQueries({ queryKey: ['models'] });
      toast.success(t('models.messages.bulkEnableSuccess', { count: variables.length }));
    },
  });
}

export interface UnassociatedChannel {
  channel: {
    id: string;
    name: string;
    type: string;
    status: string;
  };
  models: string[];
}

export interface ModelAssociationInput {
  type: 'channel_model' | 'channel_regex' | 'regex' | 'model' | 'channel_tags_model' | 'channel_tags_regex';
  priority?: number;
  disabled?: boolean;
  channelModel?: {
    channelId: number;
    modelId: string;
  };
  channelRegex?: {
    channelId: number;
    pattern: string;
  };
  regex?: {
    pattern: string;
    exclude?: ExcludeAssociationInput[];
  };
  modelId?: {
    modelId: string;
    exclude?: ExcludeAssociationInput[];
  };
  channelTagsModel?: {
    channelTags: string[];
    modelId: string;
  };
  channelTagsRegex?: {
    channelTags: string[];
    pattern: string;
  };
}

export interface ExcludeAssociationInput {
  channelNamePattern?: string;
  channelIds?: number[];
  channelTags?: string[];
}

export interface ChannelModelEntry {
  requestModel: string;
  actualModel: string;
  source: string;
}

export interface ModelChannelConnection {
  channel: {
    id: string;
    name: string;
    type: string;
    status: string;
  };
  models: ChannelModelEntry[];
}

export function useQueryUnassociatedChannels() {
  return useQuery({
    queryKey: ['unassociatedChannels'],
    queryFn: async () => {
      const data = await graphqlRequest<{ queryUnassociatedChannels: UnassociatedChannel[] }>(QUERY_UNASSOCIATED_CHANNELS);
      return data.queryUnassociatedChannels;
    },
    enabled: false,
  });
}

const MODEL_CHANNEL_CONNECTIONS_QUERY = `
  query QueryModelChannelConnections($associations: [ModelAssociationInput!]!) {
    queryModelChannelConnections(associations: $associations) {
      channel {
        id
        name
        type
        status
      }
      models {
        requestModel
        actualModel
        source
      }
    }
  }
`;

export function useQueryModelChannelConnections() {
  return useMutation({
    mutationFn: async (associations: ModelAssociationInput[]) => {
      const data = await graphqlRequest<{
        queryModelChannelConnections: ModelChannelConnection[];
      }>(MODEL_CHANNEL_CONNECTIONS_QUERY, { associations });
      return data.queryModelChannelConnections;
    },
  });
}
