import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { graphqlRequest } from '@/gql/graphql'
import { toast } from 'sonner'
import { useErrorHandler } from '@/hooks/use-error-handler'

// GraphQL queries and mutations
const SYSTEM_SETTINGS_QUERY = `
  query SystemSettings {
    systemSettings {
      storeChunks
      brandName
      brandLogo
    }
  }
`

const UPDATE_SYSTEM_SETTINGS_MUTATION = `
  mutation UpdateSystemSettings($input: UpdateSystemSettingsInput!) {
    updateSystemSettings(input: $input)
  }
`

// Types
export interface SystemSettings {
  storeChunks: boolean
  brandName?: string
  brandLogo?: string
}

export interface UpdateSystemSettingsInput {
  storeChunks?: boolean
  brandName?: string
  brandLogo?: string
}

// Hooks
export function useSystemSettings() {
  const { handleError } = useErrorHandler()
  
  return useQuery({
    queryKey: ['systemSettings'],
    queryFn: async () => {
      try {
        const data = await graphqlRequest<{ systemSettings: SystemSettings }>(
          SYSTEM_SETTINGS_QUERY
        )
        return data.systemSettings
      } catch (error) {
        handleError(error, '获取系统设置')
        throw error
      }
    },
  })
}

export function useUpdateSystemSettings() {
  const queryClient = useQueryClient()
  
  return useMutation({
    mutationFn: async (input: UpdateSystemSettingsInput) => {
      const data = await graphqlRequest<{ updateSystemSettings: boolean }>(
        UPDATE_SYSTEM_SETTINGS_MUTATION,
        { input }
      )
      return data.updateSystemSettings
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['systemSettings'] })
      toast.success('系统设置更新成功')
    },
    onError: (error: any) => {
      toast.error(`更新系统设置失败: ${error.message}`)
    },
  })
}