import { useMutation, useQuery, useQueryClient, keepPreviousData } from '@tanstack/react-query';
import { graphqlRequest } from '@/gql/graphql';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';
import { useSelectedProjectId } from '@/stores/projectStore';
import { useErrorHandler } from '@/hooks/use-error-handler';
import { useRequestPermissions } from '../../../hooks/useRequestPermissions';
import type {
  ApiKey,
  ApiKeyConnection,
  ApiKeyProfileQuotaUsage,
  ApiKeyTokenUsageStats,
  CreateApiKeyInput,
  UpdateApiKeyInput,
  UpdateApiKeyProfilesInput,
} from './schema';
import { apiKeyConnectionSchema, apiKeyProfileQuotaUsageSchema, apiKeySchema, apiKeyTokenUsageStatsSchema } from './schema';

const NOAUTH_API_KEY_TYPE = 'noauth';

// Dynamic GraphQL query builders
function buildApiKeysQuery(permissions: { canViewUsers: boolean }) {
  const userFields = permissions.canViewUsers
    ? `
          user {
            id
            firstName
            lastName
          }`
    : '';

  return `
    query GetApiKeys($first: Int, $after: Cursor, $orderBy: APIKeyOrder, $where: APIKeyWhereInput) {
      apiKeys(first: $first, after: $after, orderBy: $orderBy, where: $where) {
        edges {
          node {
            id
            createdAt
            updatedAt${userFields}
            key
            name
            type
            status
            scopes
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
}

function buildApiKeyQuery(permissions: { canViewUsers: boolean }) {
  const userFields = permissions.canViewUsers
    ? `
      user {
        id
        firstName
        lastName
      }`
    : '';

  return `
    query GetApiKey($id: ID!) {
      node(id: $id) {
        ... on APIKey {
        id
        createdAt
        updatedAt${userFields}
        key
        name
        type
        status
        scopes
        profiles {
          activeProfile
          profiles {
            name
            modelMappings { from to }
            channelIDs
            channelTags
            channelTagsMatchMode
            modelIDs
            loadBalanceStrategy
            quota {
              requests
              totalTokens
              cost
              period {
                type
                pastDuration { value unit }
                calendarDuration { unit }
              }
            }
          }
        }
      }
    }
}
  `;
}

function buildCreateApiKeyMutation(permissions: { canViewUsers: boolean }) {
  const userFields = permissions.canViewUsers
    ? `
      user {
        id
        firstName
        lastName
      }`
    : '';

  return `
    mutation CreateAPIKey($input: CreateAPIKeyInput!) {
      createAPIKey(input: $input) {
        id
        createdAt
        updatedAt${userFields}
        key
        name
        type
        status
        scopes
      }
    }
  `;
}

function buildUpdateApiKeyMutation(permissions: { canViewUsers: boolean }) {
  const userFields = permissions.canViewUsers
    ? `
      user {
        id
        firstName
        lastName
      }`
    : '';

  return `
    mutation UpdateAPIKey($id: ID!, $input: UpdateAPIKeyInput!) {
      updateAPIKey(id: $id, input: $input) {
        id
        createdAt
        updatedAt${userFields}
        key
        name
        type
        status
        scopes
      }
    }
  `;
}

const UPDATE_APIKEY_STATUS_MUTATION = `
  mutation UpdateAPIKeyStatus($id: ID!, $status: APIKeyStatus!) {
    updateAPIKeyStatus(id: $id, status: $status) {
      id
      status
    }
  }
`;

const UPDATE_APIKEY_PROFILES_MUTATION = `
  mutation UpdateAPIKeyProfiles($id: ID!, $input: UpdateAPIKeyProfilesInput!) {
    updateAPIKeyProfiles(id: $id, input: $input) {
      id
      name
      status
      profiles {
        activeProfile
        profiles {
          name
          modelMappings {
            from
            to
          }
          channelIDs
          channelTags
          channelTagsMatchMode
          modelIDs
          loadBalanceStrategy
          quota {
            requests
            totalTokens
            cost
            period {
              type
              pastDuration { value unit }
              calendarDuration { unit }
            }
          }
        }
      }
    }
  }
`;

const BULK_DISABLE_APIKEYS_MUTATION = `
  mutation BulkDisableAPIKeys($ids: [ID!]!) {
    bulkDisableAPIKeys(ids: $ids)
  }
`;

const BULK_ENABLE_APIKEYS_MUTATION = `
  mutation BulkEnableAPIKeys($ids: [ID!]!) {
    bulkEnableAPIKeys(ids: $ids)
  }
`;

const BULK_ARCHIVE_APIKEYS_MUTATION = `
  mutation BulkArchiveAPIKeys($ids: [ID!]!) {
    bulkArchiveAPIKeys(ids: $ids)
  }
`;

const APIKEY_QUOTA_USAGES_QUERY = `
  query APIKeyQuotaUsages($apiKeyId: ID!) {
    apiKeyQuotaUsages(apiKeyId: $apiKeyId) {
      profileName
      quota {
        requests
        totalTokens
        cost
        period {
          type
          pastDuration { value unit }
          calendarDuration { unit }
        }
      }
      window { start end }
      usage { requestCount totalTokens totalCost }
    }
  }
`;

const APIKEY_TOKEN_USAGE_STATS_QUERY = `
  query APIKeyTokenUsageStats($input: APIKeyTokenUsageStatsInput) {
    apiKeyTokenUsageStats(input: $input) {
      apiKeyId
      inputTokens
      outputTokens
      cachedTokens
      reasoningTokens
      topModels {
        modelId
        inputTokens
        outputTokens
        cachedTokens
        reasoningTokens
      }
    }
  }
`;

// React Query hooks
export function useApiKeys(
  variables?: {
    first?: number;
    after?: string;
    orderBy?: { field: 'CREATED_AT'; direction: 'ASC' | 'DESC' };
    where?: {
      nameContainsFold?: string;
      status?: string;
      userID?: string;
      projectID?: string;
      [key: string]: unknown;
    };
  },
  options?: {
    disableAutoFetch?: boolean;
  }
) {
  const { t } = useTranslation();
  const { handleError } = useErrorHandler();
  const permissions = useRequestPermissions();
  const selectedProjectId = useSelectedProjectId();

  return useQuery({
    queryKey: ['apiKeys', variables, permissions, selectedProjectId],
    queryFn: async () => {
      try {
        const query = buildApiKeysQuery(permissions);
        const headers = selectedProjectId ? { 'X-Project-ID': selectedProjectId } : undefined;
        const mergedVariables = {
          ...variables,
          where: {
            ...variables?.where,
            typeNotIn: [NOAUTH_API_KEY_TYPE],
          },
        };
        const data = await graphqlRequest<{ apiKeys: ApiKeyConnection }>(query, mergedVariables, headers);
        return apiKeyConnectionSchema.parse(data?.apiKeys);
      } catch (error) {
        handleError(error, t('common.errors.internalServerError'));
        throw error;
      }
    },
    enabled: !options?.disableAutoFetch && !!selectedProjectId, // Only query when a project is selected
  });
}

export function useApiKey(id: string) {
  const { t } = useTranslation();
  const { handleError } = useErrorHandler();
  const permissions = useRequestPermissions();
  const selectedProjectId = useSelectedProjectId();

  return useQuery({
    queryKey: ['apiKey', id, permissions, selectedProjectId],
    queryFn: async () => {
      try {
        const query = buildApiKeyQuery(permissions);
        const headers = selectedProjectId ? { 'X-Project-ID': selectedProjectId } : undefined;
        const data = await graphqlRequest<{ node: ApiKey }>(query, { id }, headers);
        return apiKeySchema.parse(data.node);
      } catch (error) {
        handleError(error, t('common.errors.internalServerError'));
        throw error;
      }
    },
    enabled: !!id,
  });
}

export function useApiKeyQuotaUsages(
  apiKeyId: string,
  options?: {
    enabled?: boolean;
    refetchInterval?: number;
  }
) {
  const { t } = useTranslation();
  const { handleError } = useErrorHandler();
  const selectedProjectId = useSelectedProjectId();

  return useQuery({
    queryKey: ['apiKeyQuotaUsages', apiKeyId, selectedProjectId],
    queryFn: async () => {
      try {
        const headers = selectedProjectId ? { 'X-Project-ID': selectedProjectId } : undefined;
        const data = await graphqlRequest<{ apiKeyQuotaUsages: ApiKeyProfileQuotaUsage[] }>(
          APIKEY_QUOTA_USAGES_QUERY,
          { apiKeyId },
          headers
        );
        return apiKeyProfileQuotaUsageSchema.array().parse(data.apiKeyQuotaUsages);
      } catch (error) {
        handleError(error, t('common.errors.internalServerError'));
        throw error;
      }
    },
    enabled: !!apiKeyId && (options?.enabled ?? true),
    refetchInterval: options?.refetchInterval,
  });
}

export function useApiKeyTokenUsageStats(
  variables?: {
    apiKeyIds?: string[];
    createdAtGTE?: string;
    createdAtLTE?: string;
  },
  options?: {
    enabled?: boolean;
  }
) {
  const { t } = useTranslation();
  const { handleError } = useErrorHandler();
  const selectedProjectId = useSelectedProjectId();

  return useQuery({
    queryKey: ['apiKeyTokenUsageStats', variables, selectedProjectId],
    queryFn: async () => {
      try {
        const headers = selectedProjectId ? { 'X-Project-ID': selectedProjectId } : undefined;
        const data = await graphqlRequest<{ apiKeyTokenUsageStats: ApiKeyTokenUsageStats[] }>(
          APIKEY_TOKEN_USAGE_STATS_QUERY,
          { input: variables && Object.keys(variables).length > 0 ? variables : undefined },
          headers
        );
        return apiKeyTokenUsageStatsSchema.array().parse(data.apiKeyTokenUsageStats);
      } catch (error) {
        handleError(error, t('common.errors.internalServerError'));
        throw error;
      }
    },
    enabled: !!selectedProjectId && (options?.enabled ?? true),
    placeholderData: keepPreviousData,
    staleTime: 30000, // Consider data fresh for 30 seconds
  });
}

export function useCreateApiKey() {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const permissions = useRequestPermissions();
  const selectedProjectId = useSelectedProjectId();
  const { handleError } = useErrorHandler();

  return useMutation({
    mutationFn: (input: CreateApiKeyInput) => {
      const mutation = buildCreateApiKeyMutation(permissions);
      // Automatically add projectID if not provided and a project is selected
      const inputWithProject = {
        ...input,
        projectID: input.projectID ?? (selectedProjectId ? selectedProjectId : undefined),
      };
      const headers = selectedProjectId ? { 'X-Project-ID': selectedProjectId } : undefined;
      return graphqlRequest<{ createAPIKey: ApiKey }>(mutation, { input: inputWithProject }, headers);
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['apiKeys'] });
      toast.success(t('apikeys.messages.createSuccess'));
    },
    onError: (error) => {
      handleError(error, { context: t('apikeys.dialogs.create.title') });
    },
  });
}

export function useUpdateApiKey() {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const permissions = useRequestPermissions();
  const selectedProjectId = useSelectedProjectId();
  const { handleError } = useErrorHandler();

  return useMutation({
    mutationFn: ({ id, input }: { id: string; input: UpdateApiKeyInput }) => {
      const mutation = buildUpdateApiKeyMutation(permissions);
      const headers = selectedProjectId ? { 'X-Project-ID': selectedProjectId } : undefined;
      return graphqlRequest<{ updateAPIKey: ApiKey }>(mutation, { id, input }, headers);
    },
    onSuccess: (_, variables) => {
      queryClient.invalidateQueries({ queryKey: ['apiKeys'] });
      queryClient.invalidateQueries({ queryKey: ['apiKey', variables.id] });
      toast.success(t('apikeys.messages.updateSuccess'));
    },
    onError: (error) => {
      handleError(error, { context: t('apikeys.dialogs.edit.title') });
    },
  });
}

export function useUpdateApiKeyStatus() {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const selectedProjectId = useSelectedProjectId();

  return useMutation({
    mutationFn: ({ id, status }: { id: string; status: 'enabled' | 'disabled' | 'archived' }) => {
      const headers = selectedProjectId ? { 'X-Project-ID': selectedProjectId } : undefined;
      return graphqlRequest<{ updateAPIKeyStatus: ApiKey }>(UPDATE_APIKEY_STATUS_MUTATION, { id, status }, headers);
    },
    onSuccess: (data, variables) => {
      queryClient.invalidateQueries({ queryKey: ['apiKeys'] });
      queryClient.invalidateQueries({ queryKey: ['apiKey', variables.id] });
      const statusText =
        data.updateAPIKeyStatus.status === 'enabled'
          ? t('apikeys.status.enabled')
          : data.updateAPIKeyStatus.status === 'disabled'
            ? t('apikeys.status.disabled')
            : t('apikeys.status.archived');
      toast.success(t('apikeys.messages.statusUpdateSuccess', { status: statusText }));
    },
    onError: () => {
      toast.error(t('common.errors.internalServerError'));
    },
  });
}

export function useUpdateApiKeyProfiles() {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const selectedProjectId = useSelectedProjectId();

  return useMutation({
    mutationFn: ({ id, input }: { id: string; input: UpdateApiKeyProfilesInput }) => {
      const headers = selectedProjectId ? { 'X-Project-ID': selectedProjectId } : undefined;
      return graphqlRequest<{ updateAPIKeyProfiles: ApiKey }>(UPDATE_APIKEY_PROFILES_MUTATION, { id, input }, headers);
    },
    onSuccess: (_, variables) => {
      queryClient.invalidateQueries({ queryKey: ['apiKeys'] });
      queryClient.invalidateQueries({ queryKey: ['apiKey', variables.id] });
      toast.success(t('apikeys.messages.profilesUpdateSuccess'));
    },
    onError: () => {
      toast.error(t('common.errors.internalServerError'));
    },
  });
}

export function useBulkDisableApiKeys() {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const selectedProjectId = useSelectedProjectId();

  return useMutation({
    mutationFn: async (ids: string[]) => {
      const headers = selectedProjectId ? { 'X-Project-ID': selectedProjectId } : undefined;
      const data = await graphqlRequest<{ bulkDisableAPIKeys: boolean }>(BULK_DISABLE_APIKEYS_MUTATION, { ids }, headers);
      return data.bulkDisableAPIKeys;
    },
    onSuccess: (_, variables) => {
      queryClient.invalidateQueries({ queryKey: ['apiKeys'] });
      toast.success(t('apikeys.messages.bulkDisableSuccess', { count: variables.length }));
    },
    onError: () => {
      toast.error(t('common.errors.internalServerError'));
    },
  });
}

export function useBulkEnableApiKeys() {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const selectedProjectId = useSelectedProjectId();

  return useMutation({
    mutationFn: async (ids: string[]) => {
      const headers = selectedProjectId ? { 'X-Project-ID': selectedProjectId } : undefined;
      const data = await graphqlRequest<{ bulkEnableAPIKeys: boolean }>(BULK_ENABLE_APIKEYS_MUTATION, { ids }, headers);
      return data.bulkEnableAPIKeys;
    },
    onSuccess: (_, variables) => {
      queryClient.invalidateQueries({ queryKey: ['apiKeys'] });
      toast.success(t('apikeys.messages.bulkEnableSuccess', { count: variables.length }));
    },
    onError: () => {
      toast.error(t('common.errors.internalServerError'));
    },
  });
}

export function useBulkArchiveApiKeys() {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const selectedProjectId = useSelectedProjectId();

  return useMutation({
    mutationFn: async (ids: string[]) => {
      const headers = selectedProjectId ? { 'X-Project-ID': selectedProjectId } : undefined;
      const data = await graphqlRequest<{ bulkArchiveAPIKeys: boolean }>(BULK_ARCHIVE_APIKEYS_MUTATION, { ids }, headers);
      return data.bulkArchiveAPIKeys;
    },
    onSuccess: (_, variables) => {
      queryClient.invalidateQueries({ queryKey: ['apiKeys'] });
      toast.success(t('apikeys.messages.bulkArchiveSuccess', { count: variables.length }));
    },
    onError: () => {
      toast.error(t('common.errors.internalServerError'));
    },
  });
}
