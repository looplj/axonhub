import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { graphqlRequest } from '@/gql/graphql'
import { toast } from 'sonner'
import i18n from '@/lib/i18n'
import { useErrorHandler } from '@/hooks/use-error-handler'

// GraphQL queries and mutations
const SYSTEM_VERSION_QUERY = `
  query SystemVersion {
    systemVersion {
      version
      commit
      buildTime
      goVersion
      platform
      uptime
    }
  }
`

export const CHECK_FOR_UPDATE_QUERY = `
  query CheckForUpdate {
    checkForUpdate {
      currentVersion
      latestVersion
      hasUpdate
      releaseUrl
    }
  }
`

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
      storeRequestBody
      storeResponseBody
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

const RETRY_POLICY_QUERY = `
  query RetryPolicy {
    retryPolicy {
      maxChannelRetries
      maxSingleChannelRetries
      retryDelayMs
      enabled
    }
  }
`

const UPDATE_RETRY_POLICY_MUTATION = `
  mutation UpdateRetryPolicy($input: UpdateRetryPolicyInput!) {
    updateRetryPolicy(input: $input)
  }
`

const DEFAULT_DATA_STORAGE_QUERY = `
  query DefaultDataStorageID {
    defaultDataStorageID
  }
`

const UPDATE_DEFAULT_DATA_STORAGE_MUTATION = `
  mutation UpdateDefaultDataStorage($input: UpdateDefaultDataStorageInput!) {
    updateDefaultDataStorage(input: $input)
  }
`

const ONBOARDING_INFO_QUERY = `
  query OnboardingInfo {
    onboardingInfo {
      onboarded
      version
      completedAt
    }
  }
`

const COMPLETE_ONBOARDING_MUTATION = `
  mutation CompleteOnboarding($input: CompleteOnboardingInput!) {
    completeOnboarding(input: $input)
  }
`

// Types
export interface BrandSettings {
  brandName?: string
  brandLogo?: string
}

export interface StoragePolicy {
  storeChunks: boolean
  storeRequestBody: boolean
  storeResponseBody: boolean
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
  storeRequestBody?: boolean
  storeResponseBody?: boolean
  cleanupOptions?: CleanupOptionInput[]
}

export interface CleanupOptionInput {
  resourceType: string
  enabled: boolean
  cleanupDays: number
}

export interface RetryPolicy {
  maxChannelRetries: number
  maxSingleChannelRetries: number
  retryDelayMs: number
  enabled: boolean
}

export interface RetryPolicyInput {
  maxChannelRetries?: number
  maxSingleChannelRetries?: number
  retryDelayMs?: number
  enabled?: boolean
}

export interface UpdateDefaultDataStorageInput {
  dataStorageID: string
}

export interface OnboardingInfo {
  onboarded: boolean
  version: string
  completedAt?: string
}

export interface CompleteOnboardingInput {
  dummy?: string
}

export interface SystemVersion {
  version: string
  commit: string
  buildTime: string
  goVersion: string
  platform: string
  uptime: string
}

export interface VersionCheck {
  currentVersion: string
  latestVersion: string
  hasUpdate: boolean
  releaseUrl: string
}

// Hooks
export function useBrandSettings() {
  const { handleError } = useErrorHandler()

  return useQuery({
    queryKey: ['brandSettings'],
    queryFn: async () => {
      try {
        const data = await graphqlRequest<{ brandSettings: BrandSettings }>(BRAND_SETTINGS_QUERY)
        return data.brandSettings
      } catch (error) {
        handleError(error, i18n.t('common.errors.internalServerError'))
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
        const data = await graphqlRequest<{ storagePolicy: StoragePolicy }>(STORAGE_POLICY_QUERY)
        return data.storagePolicy
      } catch (error) {
        handleError(error, i18n.t('common.errors.internalServerError'))
        throw error
      }
    },
  })
}

export function useUpdateBrandSettings() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: async (input: UpdateBrandSettingsInput) => {
      const data = await graphqlRequest<{ updateBrandSettings: boolean }>(UPDATE_BRAND_SETTINGS_MUTATION, { input })
      return data.updateBrandSettings
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['brandSettings'] })
      toast.success(i18n.t('common.success.systemUpdated'))
    },
    onError: () => {
      toast.error(i18n.t('common.errors.systemUpdateFailed'))
    },
  })
}

export function useUpdateStoragePolicy() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: async (input: UpdateStoragePolicyInput) => {
      const data = await graphqlRequest<{ updateStoragePolicy: boolean }>(UPDATE_STORAGE_POLICY_MUTATION, { input })
      return data.updateStoragePolicy
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['storagePolicy'] })
      toast.success(i18n.t('common.success.systemUpdated'))
    },
    onError: () => {
      toast.error(i18n.t('common.errors.systemUpdateFailed'))
    },
  })
}

export function useRetryPolicy() {
  const { handleError } = useErrorHandler()

  return useQuery({
    queryKey: ['retryPolicy'],
    queryFn: async () => {
      try {
        const data = await graphqlRequest<{ retryPolicy: RetryPolicy }>(RETRY_POLICY_QUERY)
        return data.retryPolicy
      } catch (error) {
        handleError(error, i18n.t('common.errors.internalServerError'))
        throw error
      }
    },
  })
}

export function useUpdateRetryPolicy() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: async (input: RetryPolicyInput) => {
      const data = await graphqlRequest<{ updateRetryPolicy: boolean }>(UPDATE_RETRY_POLICY_MUTATION, { input })
      return data.updateRetryPolicy
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['retryPolicy'] })
      toast.success(i18n.t('common.success.systemUpdated'))
    },
    onError: () => {
      toast.error(i18n.t('common.errors.systemUpdateFailed'))
    },
  })
}

export function useDefaultDataStorageID() {
  const { handleError } = useErrorHandler()

  return useQuery({
    queryKey: ['defaultDataStorageID'],
    queryFn: async () => {
      try {
        const data = await graphqlRequest<{ defaultDataStorageID: string | null }>(DEFAULT_DATA_STORAGE_QUERY)
        return data.defaultDataStorageID
      } catch (error) {
        handleError(error, i18n.t('common.errors.internalServerError'))
        throw error
      }
    },
  })
}

export function useUpdateDefaultDataStorage() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: async (input: UpdateDefaultDataStorageInput) => {
      const data = await graphqlRequest<{ updateDefaultDataStorage: boolean }>(UPDATE_DEFAULT_DATA_STORAGE_MUTATION, { input })
      return data.updateDefaultDataStorage
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['defaultDataStorageID'] })
      toast.success(i18n.t('common.success.systemUpdated'))
    },
    onError: () => {
      toast.error(i18n.t('common.errors.systemUpdateFailed'))
    },
  })
}

export function useOnboardingInfo() {
  return useQuery({
    queryKey: ['onboardingInfo'],
    queryFn: async () => {
      try {
        const data = await graphqlRequest<{ onboardingInfo: OnboardingInfo | null }>(ONBOARDING_INFO_QUERY)
        return data.onboardingInfo
      } catch (error) {
        return {
          onboarded: true,
          version: '',
          completedAt: new Date().toISOString(),
        }
      }
    },
  })
}

export function useCompleteOnboarding() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: async (input?: CompleteOnboardingInput) => {
      const data = await graphqlRequest<{ completeOnboarding: boolean }>(COMPLETE_ONBOARDING_MUTATION, { input: input || {} })
      return data.completeOnboarding
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['onboardingInfo'] })
    },
    onError: () => {
      toast.error(i18n.t('common.errors.onboardingFailed'))
    },
  })
}

export function useSystemVersion() {
  return useQuery({
    queryKey: ['systemVersion'],
    queryFn: async () => {
      const data = await graphqlRequest<{ systemVersion: SystemVersion }>(SYSTEM_VERSION_QUERY)
      return data.systemVersion
    },
  })
}

export function useCheckForUpdate() {
  return useQuery({
    queryKey: ['checkForUpdate'],
    queryFn: async () => {
      const data = await graphqlRequest<{ checkForUpdate: VersionCheck }>(CHECK_FOR_UPDATE_QUERY)
      return data.checkForUpdate
    },
    retry: false,
    staleTime: 5 * 60 * 1000, // 5 minutes
  })
}
