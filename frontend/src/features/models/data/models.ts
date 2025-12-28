import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { graphqlRequest } from '@/gql/graphql'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import {
  Model,
  ModelConnection,
  CreateModelInput,
  UpdateModelInput,
  modelConnectionSchema,
  modelSchema,
} from './schema'

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
`

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
`

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
`

const DELETE_MODEL_MUTATION = `
  mutation DeleteModel($id: ID!) {
    deleteModel(id: $id)
  }
`

const BULK_DISABLE_MODELS_MUTATION = `
  mutation BulkDisableModels($ids: [ID!]!) {
    bulkDisableModels(ids: $ids)
  }
`

const BULK_ENABLE_MODELS_MUTATION = `
  mutation BulkEnableModels($ids: [ID!]!) {
    bulkEnableModels(ids: $ids)
  }
`

interface QueryModelsArgs {
  first?: number
  after?: string
  last?: number
  before?: string
  where?: Record<string, any>
  orderBy?: {
    field: 'CREATED_AT' | 'UPDATED_AT' | 'NAME' | 'MODEL_ID'
    direction: 'ASC' | 'DESC'
  }
}

export function useQueryModels(args: QueryModelsArgs) {
  return useQuery({
    queryKey: ['models', args],
    queryFn: async () => {
      const data = await graphqlRequest<{ models: ModelConnection }>(MODELS_QUERY, args)
      return modelConnectionSchema.parse(data.models)
    },
  })
}

export function useCreateModel() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: async (input: CreateModelInput) => {
      const data = await graphqlRequest<{ createModel: Model }>(CREATE_MODEL_MUTATION, { input })
      return modelSchema.parse(data.createModel)
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['models'] })
      toast.success(t('models.messages.createSuccess'))
    },
    onError: (error: Error) => {
      toast.error(t('models.messages.createError', { error: error.message }))
    },
  })
}

export function useBulkCreateModels() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: async (inputs: CreateModelInput[]) => {
      const data = await graphqlRequest<{ bulkCreateModels: Model[] }>(BULK_CREATE_MODELS_MUTATION, { inputs })
      return data.bulkCreateModels.map((model) => modelSchema.parse(model))
    },
    onSuccess: (_data, variables) => {
      queryClient.invalidateQueries({ queryKey: ['models'] })
      toast.success(t('models.messages.bulkCreateSuccess', { count: variables.length }))
    },
    onError: (error: Error) => {
      toast.error(t('models.messages.bulkCreateError', { error: error.message }))
    },
  })
}

export function useUpdateModel() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: async ({ id, input }: { id: string; input: UpdateModelInput }) => {
      const data = await graphqlRequest<{ updateModel: Model }>(UPDATE_MODEL_MUTATION, { id, input })
      return modelSchema.parse(data.updateModel)
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['models'] })
      toast.success(t('models.messages.updateSuccess'))
    },
    onError: (error: Error) => {
      toast.error(t('models.messages.updateError', { error: error.message }))
    },
  })
}

export function useDeleteModel() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: async (id: string) => {
      await graphqlRequest(DELETE_MODEL_MUTATION, { id })
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['models'] })
      toast.success(t('models.messages.deleteSuccess'))
    },
    onError: (error: Error) => {
      toast.error(t('models.messages.deleteError', { error: error.message }))
    },
  })
}

export function useBulkDisableModels() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: async (ids: string[]) => {
      const data = await graphqlRequest<{ bulkDisableModels: boolean }>(BULK_DISABLE_MODELS_MUTATION, { ids })
      return data.bulkDisableModels
    },
    onSuccess: (_data, variables) => {
      queryClient.invalidateQueries({ queryKey: ['models'] })
      toast.success(t('models.messages.bulkDisableSuccess', { count: variables.length }))
    },
    onError: (error: Error) => {
      toast.error(t('models.messages.bulkDisableError', { error: error.message }))
    },
  })
}

export function useBulkEnableModels() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: async (ids: string[]) => {
      const data = await graphqlRequest<{ bulkEnableModels: boolean }>(BULK_ENABLE_MODELS_MUTATION, { ids })
      return data.bulkEnableModels
    },
    onSuccess: (_data, variables) => {
      queryClient.invalidateQueries({ queryKey: ['models'] })
      toast.success(t('models.messages.bulkEnableSuccess', { count: variables.length }))
    },
    onError: (error: Error) => {
      toast.error(t('models.messages.bulkEnableError', { error: error.message }))
    },
  })
}
