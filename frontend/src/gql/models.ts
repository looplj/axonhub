import { useMutation } from '@tanstack/react-query'
import { graphqlRequest } from './graphql'

export interface Model {
  id: string
  status: 'enabled' | 'disabled' | 'archived'
}

export interface ModelsResponse {
  models: Model[]
}

export interface ModelsInput {
  statusIn?: ('enabled' | 'disabled' | 'archived')[]
  includeMapping?: boolean
  includePrefix?: boolean
}

const MODELS_QUERY = `
  query Models($input: ModelsInput!) {
    models(input: $input) {
      id
      status
    }
  }
`

export function useQueryModels() {
  return useMutation({
    mutationFn: async (input: ModelsInput = {}) => {
      const data = await graphqlRequest<{
        models: Model[]
      }>(MODELS_QUERY, { input })
      return data.models
    },
  })
}
