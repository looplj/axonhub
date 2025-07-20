import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { graphqlRequest } from '@/global/graphql'
import { toast } from 'sonner'
import {
  Channel,
  ChannelConnection,
  CreateChannelInput,
  UpdateChannelInput,
  channelConnectionSchema,
  channelSchema,
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

const DELETE_CHANNEL_MUTATION = `
  mutation DeleteChannel($id: ID!) {
    deleteChannel(id: $id)
  }
`

// Query hooks
export function useChannels(variables?: {
  first?: number
  after?: string
  orderBy?: { field: 'CREATED_AT'; direction: 'ASC' | 'DESC' }
  where?: Record<string, any>
}) {
  return useQuery({
    queryKey: ['channels', variables],
    queryFn: async () => {
      const data = await graphqlRequest<{ channels: ChannelConnection }>(
        CHANNELS_QUERY,
        variables
      )
      return channelConnectionSchema.parse(data?.channels)
    },
  })
}

export function useChannel(id: string) {
  return useQuery({
    queryKey: ['channel', id],
    queryFn: async () => {
      const data = await graphqlRequest<{ channels: ChannelConnection }>(
        CHANNELS_QUERY,
        { where: { id } }
      )
      const channel = data.channels.edges[0]?.node
      if (!channel) {
        throw new Error('Channel not found')
      }
      return channelSchema.parse(channel)
    },
    enabled: !!id,
  })
}

// Mutation hooks
export function useCreateChannel() {
  const queryClient = useQueryClient()

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
      toast.success('Channel 创建成功')
    },
    onError: (error) => {
      toast.error(`创建失败: ${error.message}`)
    },
  })
}

export function useUpdateChannel() {
  const queryClient = useQueryClient()

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
      toast.success('Channel 更新成功')
    },
    onError: (error) => {
      toast.error(`更新失败: ${error.message}`)
    },
  })
}

export function useDeleteChannel() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: async (id: string) => {
      const data = await graphqlRequest<{ deleteChannel: boolean }>(
        DELETE_CHANNEL_MUTATION,
        { id }
      )
      return data.deleteChannel
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['channels'] })
      toast.success('Channel 删除成功')
    },
    onError: (error) => {
      toast.error(`删除失败: ${error.message}`)
    },
  })
}
