import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { graphqlRequest } from '@/gql/graphql';
import { toast } from 'sonner';
import i18n from '@/lib/i18n';
import { useErrorHandler } from '@/hooks/use-error-handler';
import { getTokenFromStorage } from '@/stores/authStore';

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
`;

export const CHECK_FOR_UPDATE_QUERY = `
  query CheckForUpdate {
    checkForUpdate {
      currentVersion
      latestVersion
      hasUpdate
      releaseUrl
    }
  }
`;

const BRAND_SETTINGS_QUERY = `
  query BrandSettings {
    brandSettings {
      brandName
      brandLogo
    }
  }
`;

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
`;

const UPDATE_BRAND_SETTINGS_MUTATION = `
  mutation UpdateBrandSettings($input: UpdateBrandSettingsInput!) {
    updateBrandSettings(input: $input)
  }
`;

const UPDATE_STORAGE_POLICY_MUTATION = `
  mutation UpdateStoragePolicy($input: UpdateStoragePolicyInput!) {
    updateStoragePolicy(input: $input)
  }
`;

const RETRY_POLICY_QUERY = `
  query RetryPolicy {
    retryPolicy {
      maxChannelRetries
      maxSingleChannelRetries
      retryDelayMs
      loadBalancerStrategy
      enabled
      autoDisableChannel {
        enabled
        statuses {
          status
          times
        }
      }
    }
  }
`;

const UPDATE_RETRY_POLICY_MUTATION = `
  mutation UpdateRetryPolicy($input: UpdateRetryPolicyInput!) {
    updateRetryPolicy(input: $input)
  }
`;

const DEFAULT_DATA_STORAGE_QUERY = `
  query DefaultDataStorageID {
    defaultDataStorageID
  }
`;

const UPDATE_DEFAULT_DATA_STORAGE_MUTATION = `
  mutation UpdateDefaultDataStorage($input: UpdateDefaultDataStorageInput!) {
    updateDefaultDataStorage(input: $input)
  }
`;

const ONBOARDING_INFO_QUERY = `
  query OnboardingInfo {
    onboardingInfo {
      onboarded
      version
      completedAt
      systemModelSetting {
        onboarded
        completedAt
      }
    }
  }
`;

const COMPLETE_ONBOARDING_MUTATION = `
  mutation CompleteOnboarding($input: CompleteOnboardingInput!) {
    completeOnboarding(input: $input)
  }
`;

const COMPLETE_SYSTEM_MODEL_SETTING_ONBOARDING_MUTATION = `
  mutation CompleteSystemModelSettingOnboarding($input: CompleteSystemModelSettingOnboardingInput!) {
    completeSystemModelSettingOnboarding(input: $input)
  }
`;

// Types
export interface BrandSettings {
  brandName?: string;
  brandLogo?: string;
}

export interface SystemGeneralSettings {
  currencyCode: string;
  timezone: string;
}

export interface UpdateSystemGeneralSettingsInput {
  currencyCode?: string;
  timezone?: string;
}

export interface StoragePolicy {
  storeChunks: boolean;
  storeRequestBody: boolean;
  storeResponseBody: boolean;
  cleanupOptions: CleanupOption[];
}

export interface CleanupOption {
  resourceType: string;
  enabled: boolean;
  cleanupDays: number;
}

export interface UpdateBrandSettingsInput {
  brandName?: string;
  brandLogo?: string;
}

export interface UpdateStoragePolicyInput {
  storeChunks?: boolean;
  storeRequestBody?: boolean;
  storeResponseBody?: boolean;
  cleanupOptions?: CleanupOptionInput[];
}

export interface CleanupOptionInput {
  resourceType: string;
  enabled: boolean;
  cleanupDays: number;
}

export interface AutoDisableChannelStatus {
  status: number;
  times: number;
}

export interface AutoDisableChannel {
  enabled: boolean;
  statuses: AutoDisableChannelStatus[];
}

export interface RetryPolicy {
  maxChannelRetries: number;
  maxSingleChannelRetries: number;
  retryDelayMs: number;
  loadBalancerStrategy: string;
  enabled: boolean;
  autoDisableChannel: AutoDisableChannel;
}

export interface AutoDisableChannelStatusInput {
  status: number;
  times: number;
}

export interface AutoDisableChannelInput {
  enabled?: boolean;
  statuses?: AutoDisableChannelStatusInput[];
}

export interface RetryPolicyInput {
  maxChannelRetries?: number;
  maxSingleChannelRetries?: number;
  retryDelayMs?: number;
  loadBalancerStrategy?: string;
  enabled?: boolean;
  autoDisableChannel?: AutoDisableChannelInput;
}

export interface UpdateDefaultDataStorageInput {
  dataStorageID: string;
}

export interface SystemModelSettingOnboarding {
  onboarded: boolean;
  completedAt?: string;
}

export interface OnboardingInfo {
  onboarded: boolean;
  version: string;
  completedAt?: string;
  systemModelSetting?: SystemModelSettingOnboarding;
}

export interface CompleteOnboardingInput {
  dummy?: string;
}

export interface CompleteSystemModelSettingOnboardingInput {
  dummy?: string;
}

export interface SystemVersion {
  version: string;
  commit: string;
  buildTime: string;
  goVersion: string;
  platform: string;
  uptime: string;
}

export interface VersionCheck {
  currentVersion: string;
  latestVersion: string;
  hasUpdate: boolean;
  releaseUrl: string;
}

// Hooks
export function useBrandSettings() {
  const { handleError } = useErrorHandler();

  return useQuery({
    queryKey: ['brandSettings'],
    queryFn: async () => {
      try {
        const data = await graphqlRequest<{ brandSettings: BrandSettings }>(BRAND_SETTINGS_QUERY);
        return data.brandSettings;
      } catch (error) {
        handleError(error, i18n.t('common.errors.internalServerError'));
        throw error;
      }
    },
  });
}

export function useStoragePolicy() {
  const { handleError } = useErrorHandler();

  return useQuery({
    queryKey: ['storagePolicy'],
    queryFn: async () => {
      try {
        const data = await graphqlRequest<{ storagePolicy: StoragePolicy }>(STORAGE_POLICY_QUERY);
        return data.storagePolicy;
      } catch (error) {
        handleError(error, i18n.t('common.errors.internalServerError'));
        throw error;
      }
    },
  });
}

export function useUpdateBrandSettings() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (input: UpdateBrandSettingsInput) => {
      const data = await graphqlRequest<{ updateBrandSettings: boolean }>(UPDATE_BRAND_SETTINGS_MUTATION, { input });
      return data.updateBrandSettings;
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['brandSettings'] });
      toast.success(i18n.t('common.success.systemUpdated'));
    },
    onError: () => {
      toast.error(i18n.t('common.errors.systemUpdateFailed'));
    },
  });
}

export function useUpdateStoragePolicy() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (input: UpdateStoragePolicyInput) => {
      const data = await graphqlRequest<{ updateStoragePolicy: boolean }>(UPDATE_STORAGE_POLICY_MUTATION, { input });
      return data.updateStoragePolicy;
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['storagePolicy'] });
      toast.success(i18n.t('common.success.systemUpdated'));
    },
    onError: () => {
      toast.error(i18n.t('common.errors.systemUpdateFailed'));
    },
  });
}

export function useRetryPolicy() {
  const { handleError } = useErrorHandler();

  return useQuery({
    queryKey: ['retryPolicy'],
    queryFn: async () => {
      try {
        const data = await graphqlRequest<{ retryPolicy: RetryPolicy }>(RETRY_POLICY_QUERY);
        return data.retryPolicy;
      } catch (error) {
        handleError(error, i18n.t('common.errors.internalServerError'));
        throw error;
      }
    },
  });
}

export function useUpdateRetryPolicy() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (input: RetryPolicyInput) => {
      const data = await graphqlRequest<{ updateRetryPolicy: boolean }>(UPDATE_RETRY_POLICY_MUTATION, { input });
      return data.updateRetryPolicy;
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['retryPolicy'] });
      toast.success(i18n.t('common.success.systemUpdated'));
    },
    onError: () => {
      toast.error(i18n.t('common.errors.systemUpdateFailed'));
    },
  });
}

export function useDefaultDataStorageID() {
  const { handleError } = useErrorHandler();

  return useQuery({
    queryKey: ['defaultDataStorageID'],
    queryFn: async () => {
      try {
        const data = await graphqlRequest<{ defaultDataStorageID: string | null }>(DEFAULT_DATA_STORAGE_QUERY);
        return data.defaultDataStorageID;
      } catch (error) {
        handleError(error, i18n.t('common.errors.internalServerError'));
        throw error;
      }
    },
  });
}

export function useUpdateDefaultDataStorage() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (input: UpdateDefaultDataStorageInput) => {
      const data = await graphqlRequest<{ updateDefaultDataStorage: boolean }>(UPDATE_DEFAULT_DATA_STORAGE_MUTATION, { input });
      return data.updateDefaultDataStorage;
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['defaultDataStorageID'] });
      toast.success(i18n.t('common.success.systemUpdated'));
    },
    onError: () => {
      toast.error(i18n.t('common.errors.systemUpdateFailed'));
    },
  });
}

export function useOnboardingInfo() {
  return useQuery({
    queryKey: ['onboardingInfo'],
    queryFn: async () => {
      try {
        const data = await graphqlRequest<{ onboardingInfo: OnboardingInfo | null }>(ONBOARDING_INFO_QUERY);
        return data.onboardingInfo;
      } catch (error) {
        return {
          onboarded: true,
          version: '',
          completedAt: new Date().toISOString(),
        };
      }
    },
  });
}

export function useCompleteOnboarding() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (input?: CompleteOnboardingInput) => {
      const data = await graphqlRequest<{ completeOnboarding: boolean }>(COMPLETE_ONBOARDING_MUTATION, { input: input || {} });
      return data.completeOnboarding;
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['onboardingInfo'] });
    },
    onError: () => {
      toast.error(i18n.t('common.errors.onboardingFailed'));
    },
  });
}

export function useCompleteSystemModelSettingOnboarding() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (input?: CompleteSystemModelSettingOnboardingInput) => {
      const data = await graphqlRequest<{ completeSystemModelSettingOnboarding: boolean }>(
        COMPLETE_SYSTEM_MODEL_SETTING_ONBOARDING_MUTATION,
        { input: input || {} }
      );
      return data.completeSystemModelSettingOnboarding;
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['onboardingInfo'] });
    },
    onError: () => {
      toast.error(i18n.t('common.errors.onboardingFailed'));
    },
  });
}

export function useSystemVersion() {
  return useQuery({
    queryKey: ['systemVersion'],
    queryFn: async () => {
      const data = await graphqlRequest<{ systemVersion: SystemVersion }>(SYSTEM_VERSION_QUERY);
      return data.systemVersion;
    },
  });
}

export function useCheckForUpdate() {
  return useQuery({
    queryKey: ['checkForUpdate'],
    queryFn: async () => {
      const data = await graphqlRequest<{ checkForUpdate: VersionCheck }>(CHECK_FOR_UPDATE_QUERY);
      return data.checkForUpdate;
    },
    retry: false,
    staleTime: 60 * 60 * 1000, // 1 hour
  });
}

// Model Settings
const MODEL_SETTINGS_QUERY = `
  query ModelSettings {
    systemModelSettings {
      fallbackToChannelsOnModelNotFound
      queryAllChannelModels
    }
  }
`;

const UPDATE_MODEL_SETTINGS_MUTATION = `
  mutation UpdateModelSettings($input: UpdateSystemModelSettingsInput!) {
    updateSystemModelSettings(input: $input)
  }
`;

const CHANNEL_SETTINGS_QUERY = `
  query SystemChannelSettings {
    systemChannelSettings {
      probe {
        enabled
        frequency
      }
    }
  }
`;

const UPDATE_CHANNEL_SETTINGS_MUTATION = `
  mutation UpdateChannelSettings($input: UpdateSystemChannelSettingsInput!) {
    updateSystemChannelSettings(input: $input)
  }
`;

const SYSTEM_GENERAL_SETTINGS_QUERY = `
  query SystemGeneralSettings {
    systemGeneralSettings {
      currencyCode
      timezone
    }
  }
`;

const UPDATE_SYSTEM_GENERAL_SETTINGS_MUTATION = `
  mutation UpdateSystemGeneralSettings($input: UpdateSystemGeneralSettingsInput!) {
    updateSystemGeneralSettings(input: $input)
  }
`;

export interface ModelSettings {
  fallbackToChannelsOnModelNotFound: boolean;
  queryAllChannelModels: boolean;
}

export interface UpdateModelSettingsInput {
  fallbackToChannelsOnModelNotFound?: boolean;
  queryAllChannelModels?: boolean;
}

export function useModelSettings() {
  const { handleError } = useErrorHandler();

  return useQuery({
    queryKey: ['modelSettings'],
    queryFn: async () => {
      try {
        const data = await graphqlRequest<{ systemModelSettings: ModelSettings }>(MODEL_SETTINGS_QUERY);
        return data.systemModelSettings;
      } catch (error) {
        handleError(error, i18n.t('common.errors.internalServerError'));
        throw error;
      }
    },
  });
}

export function useUpdateModelSettings() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (input: UpdateModelSettingsInput) => {
      const data = await graphqlRequest<{ updateSystemModelSettings: boolean }>(UPDATE_MODEL_SETTINGS_MUTATION, { input });
      return data.updateSystemModelSettings;
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['modelSettings'] });
      toast.success(i18n.t('common.success.systemUpdated'));
    },
    onError: () => {
      toast.error(i18n.t('common.errors.systemUpdateFailed'));
    },
  });
}

export type ProbeFrequency = 'ONE_MINUTE' | 'FIVE_MINUTES' | 'THIRTY_MINUTES' | 'ONE_HOUR';

export interface ChannelProbeSetting {
  enabled: boolean;
  frequency: ProbeFrequency;
}

export interface ChannelSetting {
  probe: ChannelProbeSetting;
}

export interface UpdateChannelProbeSettingInput {
  enabled?: boolean;
  frequency?: ProbeFrequency;
}

export interface UpdateSystemChannelSettingsInput {
  probe?: UpdateChannelProbeSettingInput;
}

export function useChannelSetting() {
  const { handleError } = useErrorHandler();

  return useQuery({
    queryKey: ['channelSetting'],
    queryFn: async () => {
      try {
        const data = await graphqlRequest<{ systemChannelSettings: ChannelSetting }>(CHANNEL_SETTINGS_QUERY);
        return data.systemChannelSettings;
      } catch (error) {
        handleError(error, i18n.t('common.errors.internalServerError'));
        throw error;
      }
    },
  });
}

export function useUpdateChannelSetting() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (input: UpdateSystemChannelSettingsInput) => {
      const data = await graphqlRequest<{ updateSystemChannelSettings: boolean }>(UPDATE_CHANNEL_SETTINGS_MUTATION, { input });
      return data.updateSystemChannelSettings;
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['channelSetting'] });
      queryClient.invalidateQueries({ queryKey: ['channelProbeData'] });
      toast.success(i18n.t('common.success.systemUpdated'));
    },
    onError: () => {
      toast.error(i18n.t('common.errors.systemUpdateFailed'));
    },
  });
}

export function useGeneralSettings() {
  const { handleError } = useErrorHandler();

  return useQuery({
    queryKey: ['generalSettings'],
    queryFn: async () => {
      try {
        const data = await graphqlRequest<{ systemGeneralSettings: SystemGeneralSettings }>(SYSTEM_GENERAL_SETTINGS_QUERY);
        return data.systemGeneralSettings;
      } catch (error) {
        handleError(error, i18n.t('common.errors.internalServerError'));
        throw error;
      }
    },
  });
}

export function useUpdateGeneralSettings() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (input: UpdateSystemGeneralSettingsInput) => {
      const data = await graphqlRequest<{ updateSystemGeneralSettings: boolean }>(UPDATE_SYSTEM_GENERAL_SETTINGS_MUTATION, { input });
      return data.updateSystemGeneralSettings;
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['generalSettings'] });
      toast.success(i18n.t('common.success.systemUpdated'));
    },
    onError: () => {
      toast.error(i18n.t('common.errors.systemUpdateFailed'));
    },
  });
}

// Backup and Restore
const BACKUP_MUTATION = `
  mutation Backup($input: BackupOptionsInput!) {
    backup(input: $input) {
      success
      data
      message
    }
  }
`;

const RESTORE_MUTATION = `
  mutation Restore($file: Upload!, $input: RestoreOptionsInput!) {
    restore(file: $file, input: $input) {
      success
      message
    }
  }
`;

export interface BackupOptionsInput {
  includeChannels: boolean;
  includeModelPrices: boolean;
  includeModels: boolean;
  includeAPIKeys: boolean;
}

export interface BackupPayload {
  success: boolean;
  data?: string;
  message?: string;
}

export interface RestoreOptionsInput {
  includeChannels: boolean;
  includeModelPrices: boolean;
  includeModels: boolean;
  includeAPIKeys: boolean;
  channelConflictStrategy: 'skip' | 'overwrite' | 'error';
  modelConflictStrategy: 'skip' | 'overwrite' | 'error';
  modelPriceConflictStrategy: 'skip' | 'overwrite' | 'error';
  apiKeyConflictStrategy: 'skip' | 'overwrite' | 'error';
}

export interface RestorePayload {
  success: boolean;
  message?: string;
}

export function useBackup() {
  return useMutation({
    mutationFn: async (input: BackupOptionsInput) => {
      const data = await graphqlRequest<{ backup: BackupPayload }>(BACKUP_MUTATION, { input });
      return data.backup;
    },
    onSuccess: (data) => {
      if (data.success && data.data) {
        const blob = new Blob([data.data], { type: 'application/json' });
        const url = URL.createObjectURL(blob);
        const a = document.createElement('a');
        a.href = url;
        const timestamp = new Date().toISOString().replace(/[:.]/g, '-');
        a.download = `axonhub-backup-${timestamp}.json`;
        document.body.appendChild(a);
        a.click();
        document.body.removeChild(a);
        URL.revokeObjectURL(url);
        toast.success(data.message || i18n.t('system.backup.success'));
      } else {
        toast.error(data.message || i18n.t('system.backup.failed'));
      }
    },
    onError: () => {
      toast.error(i18n.t('system.backup.failed'));
    },
  });
}

export function useRestore() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async ({ file, input }: { file: File; input: RestoreOptionsInput }) => {
      const formData = new FormData();
      formData.append('operations', JSON.stringify({
        query: RESTORE_MUTATION,
        variables: { file: null, input }
      }));
      formData.append('map', JSON.stringify({ '0': ['variables.file'] }));
      formData.append('0', file);

      const token = getTokenFromStorage();
      const response = await fetch('/admin/graphql', {
        method: 'POST',
        headers: {
          'Authorization': token ? `Bearer ${token}` : '',
        },
        body: formData,
      });

      const result = await response.json();
      if (result.errors) {
        throw new Error(result.errors[0].message);
      }
      return result.data.restore as RestorePayload;
    },
    onSuccess: (data) => {
      if (data.success) {
        queryClient.invalidateQueries();
        toast.success(data.message || i18n.t('system.restore.success'));
      } else {
        toast.error(data.message || i18n.t('system.restore.failed'));
      }
    },
    onError: (error: Error) => {
      toast.error(error.message || i18n.t('system.restore.failed'));
    },
  });
}

// Auto Backup Settings
const AUTO_BACKUP_SETTINGS_QUERY = `
  query AutoBackupSettings {
    autoBackupSettings {
      enabled
      frequency
      dataStorageID
      includeChannels
      includeModels
      includeAPIKeys
      includeModelPrices
      retentionDays
      lastBackupAt
      lastBackupError
    }
  }
`;

const UPDATE_AUTO_BACKUP_SETTINGS_MUTATION = `
  mutation UpdateAutoBackupSettings($input: UpdateAutoBackupSettingsInput!) {
    updateAutoBackupSettings(input: $input)
  }
`;

const TRIGGER_AUTO_BACKUP_MUTATION = `
  mutation TriggerAutoBackup {
    triggerAutoBackup {
      success
      message
    }
  }
`;

export type BackupFrequency = 'daily' | 'weekly' | 'monthly';

export interface AutoBackupSettings {
  enabled: boolean;
  frequency: BackupFrequency;
  dataStorageID: number;
  includeChannels: boolean;
  includeModels: boolean;
  includeAPIKeys: boolean;
  includeModelPrices: boolean;
  retentionDays: number;
  lastBackupAt?: string;
  lastBackupError?: string;
}

export interface UpdateAutoBackupSettingsInput {
  enabled?: boolean;
  frequency?: BackupFrequency;
  dataStorageID?: number;
  includeChannels?: boolean;
  includeModels?: boolean;
  includeAPIKeys?: boolean;
  includeModelPrices?: boolean;
  retentionDays?: number;
}

export function useAutoBackupSettings() {
  const { handleError } = useErrorHandler();

  return useQuery({
    queryKey: ['autoBackupSettings'],
    queryFn: async () => {
      try {
        const data = await graphqlRequest<{ autoBackupSettings: AutoBackupSettings }>(AUTO_BACKUP_SETTINGS_QUERY);
        return data.autoBackupSettings;
      } catch (error) {
        handleError(error, i18n.t('common.errors.internalServerError'));
        throw error;
      }
    },
  });
}

export function useUpdateAutoBackupSettings() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (input: UpdateAutoBackupSettingsInput) => {
      const data = await graphqlRequest<{ updateAutoBackupSettings: boolean }>(UPDATE_AUTO_BACKUP_SETTINGS_MUTATION, { input });
      return data.updateAutoBackupSettings;
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['autoBackupSettings'] });
      toast.success(i18n.t('common.success.systemUpdated'));
    },
    onError: () => {
      toast.error(i18n.t('common.errors.systemUpdateFailed'));
    },
  });
}

export function useTriggerAutoBackup() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async () => {
      const data = await graphqlRequest<{ triggerAutoBackup: { success: boolean; message?: string } }>(
        TRIGGER_AUTO_BACKUP_MUTATION
      );
      return data.triggerAutoBackup;
    },
    onSuccess: (data) => {
      queryClient.invalidateQueries({ queryKey: ['autoBackupSettings'] });
      if (data.success) {
        toast.success(i18n.t('system.autoBackup.triggerSuccess'));
      } else {
        toast.error(data.message || i18n.t('system.autoBackup.triggerFailed'));
      }
    },
    onError: () => {
      toast.error(i18n.t('system.autoBackup.triggerFailed'));
    },
  });
}
