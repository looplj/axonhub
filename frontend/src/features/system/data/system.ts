import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { graphqlRequest } from '@/gql/graphql'
import { toast } from 'sonner'
import { useErrorHandler } from '@/hooks/use-error-handler'
import i18n from '@/lib/i18n'

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

const BRAND_SETTINGS_QUERY = `
  query BrandSettings {
    brandSettings {
      name
      logo
    }
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

export interface BrandSettings {
  name?: string
  logo?: string
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

export function useBrandSettings() {
  const { handleError } = useErrorHandler()
  
  return useQuery({
    queryKey: ['brandSettings'],
    queryFn: async () => {
      try {
        const data = await graphqlRequest<{ brandSettings: BrandSettings }>(
          BRAND_SETTINGS_QUERY
        )
        return data.brandSettings
      } catch (error) {
        handleError(error, '获取品牌设置')
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
      queryClient.invalidateQueries({ queryKey: ['brandSettings'] })
      toast.success(i18n.t('common.success.systemUpdated'))
    },
    onError: (error: any) => {
      toast.error(i18n.t('common.errors.systemUpdateFailed'))
    },
  })
}