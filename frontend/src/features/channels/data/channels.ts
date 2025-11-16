import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { graphqlRequest } from '@/gql/graphql'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { useErrorHandler } from '@/hooks/use-error-handler'
import {
  Channel,
  ChannelConnection,
  CreateChannelInput,
  UpdateChannelInput,
  channelConnectionSchema,
  channelSchema,
  BulkImportChannelsInput,
  BulkImportChannelsResult,
  bulkImportChannelsResultSchema,
  BulkUpdateChannelOrderingInput,
  BulkUpdateChannelOrderingResult,
  bulkUpdateChannelOrderingResultSchema,
  channelOrderingConnectionSchema,
} from './schema'

// GraphQL queries and mutations
const CHANNELS_QUERY = `
  query GetChannels(
    $first: Int
    $after: Cursor
    $orderBy: ChannelOrder
    $where: ChannelWhereInput
  ) {
    channels(first: $first, after: $after, orderBy: $orderBy, where: $where) {
      edges {
        node {
          id
          createdAt
          updatedAt
          type
          baseURL
          name
          status
          supportedModels
          defaultTestModel
          settings {
            extraModelPrefix
            modelMappings {
              from
              to
            }
            overrideParameters
            proxy {
              type
              url
              username
              password
            }
          }
          orderingWeight
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
`

const CREATE_CHANNEL_MUTATION = `
  mutation CreateChannel($input: CreateChannelInput!) {
    createChannel(input: $input) {
      id
      type
      createdAt
      updatedAt
      type
      baseURL
      name
      status
      supportedModels
      defaultTestModel
      settings {
        extraModelPrefix
        modelMappings {
          from
          to
        }
        overrideParameters
        proxy {
          type
          url
          username
          password
        }
      }
      orderingWeight
    }
  }
`

const UPDATE_CHANNEL_MUTATION = `
  mutation UpdateChannel($id: ID!, $input: UpdateChannelInput!) {
    updateChannel(id: $id, input: $input) {
      id
      type
      createdAt
      updatedAt
      baseURL
      name
      status
      supportedModels
      defaultTestModel
      settings {
        extraModelPrefix
        modelMappings {
          from
          to
        }
        overrideParameters
        proxy {
          type
          url
          username
          password
        }
      }
      orderingWeight
    }
  }
`

const UPDATE_CHANNEL_STATUS_MUTATION = `
  mutation UpdateChannelStatus($id: ID!, $status: ChannelStatus!) {
    updateChannelStatus(id: $id, status: $status) {
      id
      status
    }
  }
`

const BULK_ARCHIVE_CHANNELS_MUTATION = `
  mutation BulkArchiveChannels($ids: [ID!]!) {
    bulkArchiveChannels(ids: $ids)
  }
`

const BULK_DISABLE_CHANNELS_MUTATION = `
  mutation BulkDisableChannels($ids: [ID!]!) {
    bulkDisableChannels(ids: $ids)
  }
`

const BULK_ENABLE_CHANNELS_MUTATION = `
  mutation BulkEnableChannels($ids: [ID!]!) {
    bulkEnableChannels(ids: $ids)
  }
`

const DELETE_CHANNEL_MUTATION = `
  mutation DeleteChannel($id: ID!) {
    deleteChannel(id: $id)
  }
`

const BULK_DELETE_CHANNELS_MUTATION = `
  mutation BulkDeleteChannels($ids: [ID!]!) {
    bulkDeleteChannels(ids: $ids)
  }
`

const TEST_CHANNEL_MUTATION = `
  mutation TestChannel($input: TestChannelInput!) {
    testChannel(input: $input) {
      latency
      success
      error
      message
    }
  }
`

const BULK_IMPORT_CHANNELS_MUTATION = `
  mutation BulkImportChannels($input: BulkImportChannelsInput!) {
    bulkImportChannels(input: $input) {
      success
      created
      failed
      errors
      channels {
        id
        createdAt
        updatedAt
        type
        baseURL
        name
        status
        supportedModels
        defaultTestModel
        settings {
          extraModelPrefix
          modelMappings {
            from
            to
          }
          overrideParameters
        }
      }
    }
  }
`

const BULK_UPDATE_CHANNEL_ORDERING_MUTATION = `
  mutation BulkUpdateChannelOrdering($input: BulkUpdateChannelOrderingInput!) {
    bulkUpdateChannelOrdering(input: $input) {
      success
      updated
      channels {
        id
        createdAt
        updatedAt
        type
        baseURL
        name
        status
        supportedModels
        defaultTestModel
        orderingWeight
        settings {
          extraModelPrefix
          modelMappings {
            from
            to
          }
        }
      }
    }
  }
`

const ALL_CHANNELS_QUERY = `
  query GetAllChannels {
    channels(
      first: 1000, 
      orderBy: { field: ORDERING_WEIGHT, direction: DESC }
      where: { statusIn: [enabled, disabled] }
    ) {
      totalCount
      edges {
        node {
          id
          name
          type
          status
          baseURL
          orderingWeight
        }
      }
    }
  }
`

const FETCH_MODELS_QUERY = `
  query FetchModels($input: FetchModelsInput!) {
    fetchModels(input: $input) {
      models {
        id
      }
      error
    }
  }
`

const CHANNEL_TYPES_QUERY = `
  query CountChannelsByType {
    countChannelsByType {
      type
      count
    }
  }
`

// Query hooks
export function useChannels(
  variables?: {
    first?: number
    after?: string
    orderBy?: { field: 'CREATED_AT' | 'ORDERING_WEIGHT'; direction: 'ASC' | 'DESC' }
    where?: Record<string, unknown>
  },
  options?: {
    disableAutoFetch?: boolean
  }
) {
  const { handleError } = useErrorHandler()
  const { t } = useTranslation()

  return useQuery({
    enabled: !options?.disableAutoFetch,
    queryKey: ['channels', variables],
    queryFn: async () => {
      try {
        const data = await graphqlRequest<{ channels: ChannelConnection }>(CHANNELS_QUERY, variables)
        return channelConnectionSchema.parse(data?.channels)
      } catch (error) {
        handleError(error, t('channels.errors.fetchList'))
        throw error
      }
    },
  })
}

export function useChannel(id: string) {
  const { handleError } = useErrorHandler()
  const { t } = useTranslation()

  return useQuery({
    queryKey: ['channel', id],
    queryFn: async () => {
      try {
        const data = await graphqlRequest<{ channels: ChannelConnection }>(CHANNELS_QUERY, { where: { id } })
        const channel = data.channels.edges[0]?.node
        if (!channel) {
          throw new Error(t('channels.errors.notFound'))
        }
        return channelSchema.parse(channel)
      } catch (error) {
        handleError(error, t('channels.errors.fetchDetail'))
        throw error
      }
    },
    enabled: !!id,
  })
}

// Mutation hooks
export function useCreateChannel() {
  const queryClient = useQueryClient()
  const { t } = useTranslation()

  return useMutation({
    mutationFn: async (input: CreateChannelInput) => {
      const data = await graphqlRequest<{ createChannel: Channel }>(CREATE_CHANNEL_MUTATION, { input })
      return channelSchema.parse(data.createChannel)
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['channels'] })
      toast.success(t('channels.messages.createSuccess'))
    },
    onError: (error) => {
      toast.error(t('channels.messages.createError', { error: error.message }))
    },
  })
}

export function useUpdateChannel() {
  const queryClient = useQueryClient()
  const { t } = useTranslation()

  return useMutation({
    mutationFn: async ({ id, input }: { id: string; input: UpdateChannelInput }) => {
      const data = await graphqlRequest<{ updateChannel: Channel }>(UPDATE_CHANNEL_MUTATION, { id, input })
      return channelSchema.parse(data.updateChannel)
    },
    onSuccess: (data) => {
      queryClient.invalidateQueries({ queryKey: ['channels'] })
      queryClient.invalidateQueries({ queryKey: ['channel', data.id] })
      toast.success(t('channels.messages.updateSuccess'))
    },
    onError: (error) => {
      toast.error(t('channels.messages.updateError', { error: error.message }))
    },
  })
}

export function useUpdateChannelStatus() {
  const queryClient = useQueryClient()
  const { t } = useTranslation()

  return useMutation({
    mutationFn: async ({ id, status }: { id: string; status: 'enabled' | 'disabled' | 'archived' }) => {
      const data = await graphqlRequest<{ updateChannelStatus: boolean }>(UPDATE_CHANNEL_STATUS_MUTATION, {
        id,
        status,
      })
      return data.updateChannelStatus
    },
    onSuccess: (_data, variables) => {
      queryClient.invalidateQueries({ queryKey: ['channels'] })
      const statusText =
        variables.status === 'enabled'
          ? t('channels.status.enabled')
          : variables.status === 'archived'
            ? t('channels.status.archived')
            : t('channels.status.disabled')

      const messageKey = variables.status === 'archived' ? 'channels.messages.archiveSuccess' : 'channels.messages.statusUpdateSuccess'

      toast.success(variables.status === 'archived' ? t(messageKey) : t(messageKey, { status: statusText }))
    },
    onError: (error, variables) => {
      const errorKey = variables.status === 'archived' ? 'channels.messages.archiveError' : 'channels.messages.statusUpdateError'
      toast.error(t(errorKey, { error: error.message }))
    },
  })
}

export function useBulkArchiveChannels() {
  const queryClient = useQueryClient()
  const { t } = useTranslation()

  return useMutation({
    mutationFn: async (ids: string[]) => {
      const data = await graphqlRequest<{ bulkArchiveChannels: boolean }>(BULK_ARCHIVE_CHANNELS_MUTATION, { ids })
      return data.bulkArchiveChannels
    },
    onSuccess: (_data, variables) => {
      queryClient.invalidateQueries({ queryKey: ['channels'] })
      toast.success(t('channels.messages.bulkArchiveSuccess', { count: variables.length }))
    },
    onError: (error) => {
      toast.error(t('channels.messages.bulkArchiveError', { error: error.message }))
    },
  })
}

export function useBulkDisableChannels() {
  const queryClient = useQueryClient()
  const { t } = useTranslation()

  return useMutation({
    mutationFn: async (ids: string[]) => {
      const data = await graphqlRequest<{ bulkDisableChannels: boolean }>(BULK_DISABLE_CHANNELS_MUTATION, { ids })
      return data.bulkDisableChannels
    },
    onSuccess: (_data, variables) => {
      queryClient.invalidateQueries({ queryKey: ['channels'] })
      toast.success(t('channels.messages.bulkDisableSuccess', { count: variables.length }))
    },
    onError: (error) => {
      toast.error(t('channels.messages.bulkDisableError', { error: error.message }))
    },
  })
}

export function useBulkEnableChannels() {
  const queryClient = useQueryClient()
  const { t } = useTranslation()

  return useMutation({
    mutationFn: async (ids: string[]) => {
      const data = await graphqlRequest<{ bulkEnableChannels: boolean }>(BULK_ENABLE_CHANNELS_MUTATION, { ids })
      return data.bulkEnableChannels
    },
    onSuccess: (_data, variables) => {
      queryClient.invalidateQueries({ queryKey: ['channels'] })
      toast.success(t('channels.messages.bulkEnableSuccess', { count: variables.length }))
    },
    onError: (error) => {
      toast.error(t('channels.messages.bulkEnableError', { error: error.message }))
    },
  })
}

export function useDeleteChannel() {
  const queryClient = useQueryClient()
  const { t } = useTranslation()

  return useMutation({
    mutationFn: async (id: string) => {
      const data = await graphqlRequest<{ deleteChannel: boolean }>(DELETE_CHANNEL_MUTATION, { id })
      return data.deleteChannel
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['channels'] })
      toast.success(t('channels.messages.deleteSuccess'))
    },
    onError: (error) => {
      toast.error(t('channels.messages.deleteError', { error: error.message }))
    },
  })
}

export function useBulkDeleteChannels() {
  const queryClient = useQueryClient()
  const { t } = useTranslation()

  return useMutation({
    mutationFn: async (ids: string[]) => {
      const data = await graphqlRequest<{ bulkDeleteChannels: boolean }>(BULK_DELETE_CHANNELS_MUTATION, { ids })
      return data.bulkDeleteChannels
    },
    onSuccess: (_data, variables) => {
      queryClient.invalidateQueries({ queryKey: ['channels'] })
      toast.success(t('channels.messages.bulkDeleteSuccess', { count: variables.length }))
    },
    onError: (error) => {
      toast.error(t('channels.messages.bulkDeleteError', { error: error.message }))
    },
  })
}

export function useTestChannel() {
  const { t } = useTranslation()

  return useMutation({
    mutationFn: async ({
      channelID,
      modelID,
      proxy,
    }: {
      channelID: string
      modelID?: string
      proxy?: { type: string; url?: string; username?: string; password?: string }
    }) => {
      const data = await graphqlRequest<{
        testChannel: {
          latency: number
          success: boolean
          message?: string | null
          error?: string | null
        }
      }>(TEST_CHANNEL_MUTATION, { input: { channelID, modelID, proxy } })
      return data.testChannel
    },
    onSuccess: (data) => {
      if (data.success) {
        toast.success(t('channels.messages.testSuccess', { latency: data.latency.toFixed(2) }))
      } else {
        // Handle case where GraphQL request succeeds but test fails
        const errorMsg = data.error || t('channels.messages.testUnknownError')
        toast.error(t('channels.messages.testError', { error: errorMsg }))
      }
    },
    onError: (error) => {
      // Handle GraphQL/network errors
      toast.error(t('channels.messages.testError', { error: error.message }))
    },
  })
}

export function useAllChannelNames() {
  const { handleError } = useErrorHandler()
  const { t } = useTranslation()

  return useQuery({
    queryKey: ['channelNames'],
    queryFn: async () => {
      try {
        const data = await graphqlRequest<{ channels: ChannelConnection }>(CHANNELS_QUERY, {
          first: 1000, // Get a large number to capture all existing channels
          orderBy: {
            field: 'CREATED_AT',
            direction: 'ASC',
          },
        })
        const channelConnection = channelConnectionSchema.parse(data?.channels)
        return channelConnection.edges?.map((edge) => edge.node.name) || []
      } catch (error) {
        handleError(error, t('channels.errors.fetchNames'))
        throw error
      }
    },
    staleTime: 5 * 60 * 1000, // 5 minutes
  })
}

export function useBulkImportChannels() {
  const queryClient = useQueryClient()
  const { t } = useTranslation()

  return useMutation({
    mutationFn: async (input: BulkImportChannelsInput) => {
      const data = await graphqlRequest<{ bulkImportChannels: BulkImportChannelsResult }>(BULK_IMPORT_CHANNELS_MUTATION, { input })
      return bulkImportChannelsResultSchema.parse(data.bulkImportChannels)
    },
    onSuccess: (data) => {
      queryClient.invalidateQueries({ queryKey: ['channels'] })

      if (data.success) {
        toast.success(
          t('channels.messages.bulkImportSuccess', {
            created: data.created,
          })
        )
      } else {
        toast.error(
          t('channels.messages.bulkImportPartialError', {
            created: data.created,
            failed: data.failed,
          })
        )
      }
    },
    onError: (error) => {
      toast.error(t('channels.messages.bulkImportError', { error: error.message }))
    },
  })
}

export function useAllChannelsForOrdering(options?: { enabled?: boolean }) {
  const { handleError } = useErrorHandler()
  const { t } = useTranslation()

  return useQuery({
    queryKey: ['allChannelsForOrdering'],
    queryFn: async () => {
      try {
        const data = await graphqlRequest<{ channels: ChannelConnection }>(ALL_CHANNELS_QUERY)
        return channelOrderingConnectionSchema.parse(data?.channels)
      } catch (error) {
        handleError(error, t('channels.errors.fetchOrdering'))
        throw error
      }
    },
    enabled: options?.enabled !== false, // Default to true, only disable if explicitly set to false
  })
}

export function useBulkUpdateChannelOrdering() {
  const queryClient = useQueryClient()
  const { t } = useTranslation()

  return useMutation({
    mutationFn: async (input: BulkUpdateChannelOrderingInput) => {
      const data = await graphqlRequest<{ bulkUpdateChannelOrdering: BulkUpdateChannelOrderingResult }>(
        BULK_UPDATE_CHANNEL_ORDERING_MUTATION,
        { input }
      )
      return bulkUpdateChannelOrderingResultSchema.parse(data.bulkUpdateChannelOrdering)
    },
    onSuccess: (data) => {
      queryClient.invalidateQueries({ queryKey: ['channels'] })
      queryClient.invalidateQueries({ queryKey: ['allChannelsForOrdering'] })
      toast.success(
        t('channels.messages.orderingUpdateSuccess', {
          updated: data.updated,
        })
      )
    },
    onError: (error) => {
      toast.error(t('channels.messages.orderingUpdateError', { error: error.message }))
    },
  })
}

export function useFetchModels() {
  const { t } = useTranslation()

  return useMutation({
    mutationFn: async (input: { channelType: string; baseURL: string; apiKey?: string; channelID?: string }) => {
      const data = await graphqlRequest<{
        fetchModels: {
          models: Array<{ id: string }>
          error?: string | null
        }
      }>(FETCH_MODELS_QUERY, { input })
      return data.fetchModels
    },
    onSuccess: (data) => {
      if (data.error) {
        toast.error(t('channels.messages.fetchModelsError', { error: data.error }))
      } else {
        const count = data.models.length
        if (count > 100) {
          toast.success(t('channels.messages.fetchModelsSuccessLarge', { count }))
        } else {
          toast.success(t('channels.messages.fetchModelsSuccess', { count }))
        }
      }
    },
    onError: (error) => {
      toast.error(t('channels.messages.fetchModelsError', { error: error.message }))
    },
  })
}

export interface ChannelTypeCount {
  type: string
  count: number
}

export function useChannelTypes() {
  const { handleError } = useErrorHandler()
  const { t } = useTranslation()

  return useQuery({
    queryKey: ['channelTypes'],
    queryFn: async () => {
      try {
        const data = await graphqlRequest<{ countChannelsByType: ChannelTypeCount[] }>(CHANNEL_TYPES_QUERY)
        return data.countChannelsByType || []
      } catch (error) {
        handleError(error, t('channels.errors.fetchTypes'))
        throw error
      }
    },
    staleTime: 5 * 60 * 1000, // 5 minutes
  })
}
