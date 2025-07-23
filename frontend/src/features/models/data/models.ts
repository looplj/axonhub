import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { graphqlRequest } from '@/gql/graphql'
import type { Model, ModelConnection, CreateModelInput, UpdateModelInput } from './schema'
import { toast } from 'sonner'

// GraphQL queries and mutations
const MODELS_QUERY = `
  query Models($first: Int, $after: String, $orderBy: ModelOrder) {
    models(first: $first, after: $after, orderBy: $orderBy) {
      edges {
        node {
          id
          createdAt
          updatedAt
          name
          description
          provider
          modelID
          config
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

const MODEL_QUERY = `
  query Model($id: ID!) {
    model(id: $id) {
      id
      createdAt
      updatedAt
      name
      description
      provider
      modelID
      config
    }
  }
`

const CREATE_MODEL_MUTATION = `
  mutation CreateModel($input: CreateModelInput!) {
    createModel(input: $input) {
      id
      createdAt
      updatedAt
      name
      description
      provider
      modelID
      config
    }
  }
`

const UPDATE_MODEL_MUTATION = `
  mutation UpdateModel($id: ID!, $input: UpdateModelInput!) {
    updateModel(id: $id, input: $input) {
      id
      createdAt
      updatedAt
      name
      description
      provider
      modelID
      config
    }
  }
`

const DELETE_MODEL_MUTATION = `
  mutation DeleteModel($id: ID!) {
    deleteModel(id: $id) {
      id
    }
  }
`

// React Query hooks
export function useModels(first = 10, after?: string) {
  return useQuery({
    queryKey: ['models', first, after],
    queryFn: () => graphqlRequest<{ models: ModelConnection }>(MODELS_QUERY, { first, after }),
    select: (data) => data.models,
  })
}

export function useModel(id: string) {
  return useQuery({
    queryKey: ['model', id],
    queryFn: () => graphqlRequest<{ model: Model }>(MODEL_QUERY, { id }),
    select: (data) => data.model,
    enabled: !!id,
  })
}

export function useCreateModel() {
  const queryClient = useQueryClient()
  
  return useMutation({
    mutationFn: (input: CreateModelInput) => 
      graphqlRequest<{ createModel: Model }>(CREATE_MODEL_MUTATION, { input }),
    onSuccess: (data) => {
      queryClient.invalidateQueries({ queryKey: ['models'] })
      toast.success('Model created successfully')
    },
    onError: (error) => {
      toast.error('Failed to create model')
      console.error('Create model error:', error)
    },
  })
}

export function useUpdateModel() {
  const queryClient = useQueryClient()
  
  return useMutation({
    mutationFn: ({ id, input }: { id: string; input: UpdateModelInput }) => 
      graphqlRequest<{ updateModel: Model }>(UPDATE_MODEL_MUTATION, { id, input }),
    onSuccess: (data, variables) => {
      queryClient.invalidateQueries({ queryKey: ['models'] })
      queryClient.invalidateQueries({ queryKey: ['model', variables.id] })
      toast.success('Model updated successfully')
    },
    onError: (error) => {
      toast.error('Failed to update model')
      console.error('Update model error:', error)
    },
  })
}

export function useDeleteModel() {
  const queryClient = useQueryClient()
  
  return useMutation({
    mutationFn: (id: string) => 
      graphqlRequest<{ deleteModel: { id: string } }>(DELETE_MODEL_MUTATION, { id }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['models'] })
      toast.success('Model deleted successfully')
    },
    onError: (error) => {
      toast.error('Failed to delete model')
      console.error('Delete model error:', error)
    },
  })
}