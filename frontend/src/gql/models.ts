import { useMutation } from '@tanstack/react-query'
import { graphqlRequest } from './graphql'

export interface Model {
  id: string
  status: 'enabled' | 'disabled' | 'archived'
}

export interface ModelsResponse {
  models: Model[]
}

const MODELS_QUERY = `
  query Models($status: ChannelStatus) {
    models(status: $status) {
      id
      status
    }
  }
`

export function useQueryModels() {
  return useMutation({
    mutationFn: async (status?: 'enabled' | 'disabled' | 'archived') => {
      const data = await graphqlRequest<{
        models: Model[]
      }>(MODELS_QUERY, { status })
      return data.models
    },
  })
}
