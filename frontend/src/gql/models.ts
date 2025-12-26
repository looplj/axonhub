import { useMutation } from '@tanstack/react-query'
import { graphqlRequest } from './graphql'

export interface Model {
  id: string
  status: 'enabled' | 'disabled' | 'archived'
}

export interface ModelsResponse {
  queryModels: Model[]
}

export interface QueryModelsInput {
  statusIn?: ('enabled' | 'disabled' | 'archived')[]
  includeMapping?: boolean
  includePrefix?: boolean
}

export interface ModelAssociationInput {
  type: 'channel_model' | 'channel_regex' | 'regex' | 'model'
  priority?: number
  channelModel?: {
    channelId: number
    modelId: string
  }
  channelRegex?: {
    channelId: number
    pattern: string
  }
  regex?: {
    pattern: string
    exclude?: ExcludeAssociationInput[]
  }
  modelId?: {
    modelId: string
    exclude?: ExcludeAssociationInput[]
  }
}

export interface ExcludeAssociationInput {
  channelNamePattern?: string
  channelIds?: number[]
}

export interface ChannelModelEntry {
  requestModel: string
  actualModel: string
  source: string
}

export interface ModelChannelConnection {
  channel: {
    id: string
    name: string
    type: string
    status: string
  }
  models: ChannelModelEntry[]
}

const MODELS_QUERY = `
  query Models($input: QueryModelsInput!) {
    queryModels(input: $input) {
      id
      status
    }
  }
`

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
`

export function useQueryModels() {
  return useMutation({
    mutationFn: async (input: QueryModelsInput = {}) => {
      const data = await graphqlRequest<{
        queryModels: Model[]
      }>(MODELS_QUERY, { input })
      return data.queryModels
    },
  })
}

export function useQueryModelChannelConnections() {
  return useMutation({
    mutationFn: async (associations: ModelAssociationInput[]) => {
      const data = await graphqlRequest<{
        queryModelChannelConnections: ModelChannelConnection[]
      }>(MODEL_CHANNEL_CONNECTIONS_QUERY, { associations })
      return data.queryModelChannelConnections
    },
  })
}
