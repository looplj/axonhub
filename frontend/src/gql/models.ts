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

const MODELS_QUERY = `
  query Models($input: QueryModelsInput!) {
    queryModels(input: $input) {
      id
      status
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
