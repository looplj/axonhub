import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { graphqlRequest } from '@/gql/graphql'
import { toast } from 'sonner'
import { useErrorHandler } from '@/hooks/use-error-handler'
import { useTranslation } from 'react-i18next'
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
            modelMappings {
              from
              to
            }
          }

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
        modelMappings {
          from
          to
        }
      }

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
        modelMappings {
          from
          to
        }
      }

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
          modelMappings {
            from
            to
          }
        }
      }
    }
  }
`

// Query hooks
export function useChannels(variables?: {
  first?: number
  after?: string
  orderBy?: { field: 'CREATED_AT'; direction: 'ASC' | 'DESC' }
  where?: Record<string, unknown>
}) {
  const { handleError } = useErrorHandler()

  return useQuery({
    queryKey: ['channels', variables],
    queryFn: async () => {
      try {
        const data = await graphqlRequest<{ channels: ChannelConnection }>(
          CHANNELS_QUERY,
          variables
        )
        return channelConnectionSchema.parse(data?.channels)
      } catch (error) {
        handleError(error, '获取渠道数据')
        throw error
      }
    },
  })
}

export function useChannel(id: string) {
  const { handleError } = useErrorHandler()

  return useQuery({
    queryKey: ['channel', id],
    queryFn: async () => {
      try {
        const data = await graphqlRequest<{ channels: ChannelConnection }>(
          CHANNELS_QUERY,
          { where: { id } }
        )
        const channel = data.channels.edges[0]?.node
        if (!channel) {
          throw new Error('Channel not found')
        }
        return channelSchema.parse(channel)
      } catch (error) {
        handleError(error, '获取渠道详情')
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
      const data = await graphqlRequest<{ createChannel: Channel }>(
        CREATE_CHANNEL_MUTATION,
        { input }
      )
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
    mutationFn: async ({
      id,
      input,
    }: {
      id: string
      input: UpdateChannelInput
    }) => {
      const data = await graphqlRequest<{ updateChannel: Channel }>(
        UPDATE_CHANNEL_MUTATION,
        { id, input }
      )
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
    mutationFn: async ({
      id,
      status,
    }: {
      id: string
      status: 'enabled' | 'disabled' | 'archived'
    }) => {
      const data = await graphqlRequest<{ updateChannelStatus: boolean }>(
        UPDATE_CHANNEL_STATUS_MUTATION,
        { id, status }
      )
      return data.updateChannelStatus
    },
    onSuccess: (_data, variables) => {
      queryClient.invalidateQueries({ queryKey: ['channels'] })
      const statusText = variables.status === 'enabled' 
        ? t('channels.status.enabled') 
        : variables.status === 'archived'
          ? t('channels.status.archived')
          : t('channels.status.disabled')
      
      const messageKey = variables.status === 'archived' 
        ? 'channels.messages.archiveSuccess' 
        : 'channels.messages.statusUpdateSuccess'
      
      toast.success(variables.status === 'archived' 
        ? t(messageKey)
        : t(messageKey, { status: statusText })
      )
    },
    onError: (error, variables) => {
      const errorKey = variables.status === 'archived' 
        ? 'channels.messages.archiveError' 
        : 'channels.messages.statusUpdateError'
      toast.error(t(errorKey, { error: error.message }))
    },
  })
}

export function useTestChannel() {
  const { t } = useTranslation()

  return useMutation({
    mutationFn: async ({
      channelID,
      modelID,
    }: {
      channelID: string
      modelID?: string
    }) => {
      const data = await graphqlRequest<{ 
        testChannel: { 
          latency: number
          success: boolean
          message?: string | null
          error?: string | null
        } 
      }>(
        TEST_CHANNEL_MUTATION,
        { input: { channelID, modelID } }
      )
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

  return useQuery({
    queryKey: ['channelNames'],
    queryFn: async () => {
      try {
        const data = await graphqlRequest<{ channels: ChannelConnection }>(
          CHANNELS_QUERY,
          {
            first: 1000, // Get a large number to capture all existing channels
            orderBy: {
              field: 'CREATED_AT',
              direction: 'ASC'
            }
          }
        )
        const channelConnection = channelConnectionSchema.parse(data?.channels)
        return channelConnection.edges?.map(edge => edge.node.name) || []
      } catch (error) {
        handleError(error, 'Failed to load existing channel names')
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
      const data = await graphqlRequest<{ bulkImportChannels: BulkImportChannelsResult }>(
        BULK_IMPORT_CHANNELS_MUTATION,
        { input }
      )
      return bulkImportChannelsResultSchema.parse(data.bulkImportChannels)
    },
    onSuccess: (data) => {
      queryClient.invalidateQueries({ queryKey: ['channels'] })
      
      if (data.success) {
        toast.success(t('channels.messages.bulkImportSuccess', { 
          created: data.created 
        }))
      } else {
        toast.error(t('channels.messages.bulkImportPartialError', {
          created: data.created,
          failed: data.failed
        }))
      }
    },
    onError: (error) => {
      toast.error(t('channels.messages.bulkImportError', { error: error.message }))
    },
  })
}
