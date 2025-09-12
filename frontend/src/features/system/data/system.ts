import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { graphqlRequest } from '@/gql/graphql'
import { toast } from 'sonner'
import { useErrorHandler } from '@/hooks/use-error-handler'
import i18n from '@/lib/i18n'

// GraphQL queries and mutations
const BRAND_SETTINGS_QUERY = `
  query BrandSettings {
    brandSettings {
      brandName
      brandLogo
    }
  }
`

const STORAGE_POLICY_QUERY = `
  query StoragePolicy {
    storagePolicy {
      storeChunks
      cleanupOptions {
        resourceType
        enabled
        cleanupDays
      }
    }
  }
`

const UPDATE_BRAND_SETTINGS_MUTATION = `
  mutation UpdateBrandSettings($input: UpdateBrandSettingsInput!) {
    updateBrandSettings(input: $input)
  }
`

const UPDATE_STORAGE_POLICY_MUTATION = `
  mutation UpdateStoragePolicy($input: UpdateStoragePolicyInput!) {
    updateStoragePolicy(input: $input)
  }
`

// Types
export interface BrandSettings {
  brandName?: string
  brandLogo?: string
}

export interface StoragePolicy {
  storeChunks: boolean
  cleanupOptions: CleanupOption[]
}

export interface CleanupOption {
  resourceType: string
  enabled: boolean
  cleanupDays: number
}

export interface UpdateBrandSettingsInput {
  brandName?: string
  brandLogo?: string
}

export interface UpdateStoragePolicyInput {
  storeChunks?: boolean
  cleanupOptions?: CleanupOptionInput[]
}

export interface CleanupOptionInput {
  resourceType: string
  enabled: boolean
  cleanupDays: number
}

// Hooks
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

export function useStoragePolicy() {
  const { handleError } = useErrorHandler()
  
  return useQuery({
    queryKey: ['storagePolicy'],
    queryFn: async () => {
      try {
        const data = await graphqlRequest<{ storagePolicy: StoragePolicy }>(
          STORAGE_POLICY_QUERY
        )
        return data.storagePolicy
      } catch (error) {
        handleError(error, '获取存储策略')
        throw error
      }
    },
  })
}

export function useUpdateBrandSettings() {
  const queryClient = useQueryClient()
  
  return useMutation({
    mutationFn: async (input: UpdateBrandSettingsInput) => {
      const data = await graphqlRequest<{ updateBrandSettings: boolean }>(
        UPDATE_BRAND_SETTINGS_MUTATION,
        { input }
      )
      return data.updateBrandSettings
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['brandSettings'] })
      toast.success(i18n.t('common.success.systemUpdated'))
    },
    onError: (error: any) => {
      toast.error(i18n.t('common.errors.systemUpdateFailed'))
    },
  })
}

export function useUpdateStoragePolicy() {
  const queryClient = useQueryClient()
  
  return useMutation({
    mutationFn: async (input: UpdateStoragePolicyInput) => {
      const data = await graphqlRequest<{ updateStoragePolicy: boolean }>(
        UPDATE_STORAGE_POLICY_MUTATION,
        { input }
      )
      return data.updateStoragePolicy
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['storagePolicy'] })
      toast.success(i18n.t('common.success.systemUpdated'))
    },
    onError: (error: any) => {
      toast.error(i18n.t('common.errors.systemUpdateFailed'))
    },
  })
}