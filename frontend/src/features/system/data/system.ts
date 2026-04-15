import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { graphqlRequest } from '@/gql/graphql';
import { toast } from 'sonner';
import { getTokenFromStorage } from '@/stores/authStore';
import i18n from '@/lib/i18n';
import { useErrorHandler } from '@/hooks/use-error-handler';
import type { ProxyConfig } from '@/features/channels/data/schema';

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

const GET_CACHE_DIAGNOSTICS_QUERY = `
  query GetCacheDiagnostics($input: GetCacheDiagnosticsInput) {
    getCacheDiagnostics(input: $input) {
      fileName
      content
      targets
    }
  }
`;

const CLEAR_CACHE_MUTATION = `
  mutation ClearCache($input: ClearCacheInput!) {
    clearCache(input: $input) {
      success
      message
      targets
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
      livePreview
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
      emptyResponseDetection
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

const WEBHOOK_NOTIFIER_CONFIG_QUERY = `
  query WebhookNotifierConfig {
    webhookNotifierConfig {
      targets {
        name
        enabled
        url
        proxy {
          type
          url
          username
          password
        }
        timeoutMs
        headers {
          key
          value
        }
        body
      }
      subscriptions {
        event
        targetNames
      }
    }
  }
`;

const UPDATE_WEBHOOK_NOTIFIER_CONFIG_MUTATION = `
  mutation UpdateWebhookNotifierConfig($input: WebhookNotifierConfigInput!) {
    updateWebhookNotifierConfig(input: $input)
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
      completedAt
      systemModelSetting {
        onboarded
        completedAt
      }
      autoDisableChannel {
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

const COMPLETE_AUTO_DISABLE_CHANNEL_ONBOARDING_MUTATION = `
  mutation CompleteAutoDisableChannelOnboarding($input: CompleteAutoDisableChannelOnboardingInput!) {
    completeAutoDisableChannelOnboarding(input: $input)
  }
`;

const TRIGGER_GC_CLEANUP_MUTATION = `
  mutation triggerGcCleanup {
    triggerGcCleanup
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

export interface VideoStorageSettings {
  enabled: boolean;
  dataStorageID: number;
  scanIntervalMinutes: number;
  scanLimit: number;
}

export interface UpdateVideoStorageSettingsInput {
  enabled?: boolean;
  dataStorageID?: number;
  scanIntervalMinutes?: number;
  scanLimit?: number;
}

export interface StoragePolicy {
  storeChunks: boolean;
  livePreview: boolean;
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
  livePreview?: boolean;
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

export interface WebhookHeader {
  key: string;
  value: string;
}

export interface WebhookTarget {
  name: string;
  enabled: boolean;
  url: string;
  proxy?: ProxyConfig | null;
  timeoutMs: number;
  headers: WebhookHeader[];
  body: string;
}

export interface WebhookSubscription {
  event: string;
  targetNames: string[];
}

export interface WebhookNotifierConfig {
  targets: WebhookTarget[];
  subscriptions: WebhookSubscription[];
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
  emptyResponseDetection: boolean;
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
  emptyResponseDetection?: boolean;
}

export interface UpdateDefaultDataStorageInput {
  dataStorageID: string;
}

export interface SystemModelSettingOnboarding {
  onboarded: boolean;
  completedAt?: string;
}

export interface AutoDisableChannelOnboarding {
  onboarded: boolean;
  completedAt?: string;
}

export interface OnboardingInfo {
  onboarded: boolean;
  completedAt?: string;
  systemModelSetting?: SystemModelSettingOnboarding;
  autoDisableChannel?: AutoDisableChannelOnboarding;
}

export interface CompleteOnboardingInput {
  dummy?: string;
}

export interface CompleteSystemModelSettingOnboardingInput {
  dummy?: string;
}

export interface CompleteAutoDisableChannelOnboardingInput {
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

export type DiagnosticsTarget = 'CHANNEL_CACHE';

export interface GetCacheDiagnosticsPayload {
  fileName: string;
  content: string;
  targets: DiagnosticsTarget[];
}

export interface ClearCachePayload {
  success: boolean;
  message: string;
  targets: DiagnosticsTarget[];
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

export function useTriggerGcCleanup() {
  return useMutation({
    mutationFn: async () => {
      const data = await graphqlRequest<{ triggerGcCleanup: boolean }>(TRIGGER_GC_CLEANUP_MUTATION);
      return data.triggerGcCleanup;
    },
    onSuccess: () => {
      toast.success(i18n.t('system.storage.policy.runCleanupSuccess'));
    },
    onError: () => {
      toast.error(i18n.t('system.storage.policy.runCleanupError'));
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

export function useWebhookNotifierConfig() {
  const { handleError } = useErrorHandler();

  return useQuery({
    queryKey: ['webhookNotifierConfig'],
    queryFn: async () => {
      try {
        const data = await graphqlRequest<{ webhookNotifierConfig: WebhookNotifierConfig }>(WEBHOOK_NOTIFIER_CONFIG_QUERY);
        return data.webhookNotifierConfig;
      } catch (error) {
        handleError(error, i18n.t('common.errors.internalServerError'));
        throw error;
      }
    },
  });
}

export function useUpdateWebhookNotifierConfig() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (input: WebhookNotifierConfig) => {
      const data = await graphqlRequest<{ updateWebhookNotifierConfig: boolean }>(UPDATE_WEBHOOK_NOTIFIER_CONFIG_MUTATION, { input });
      return data.updateWebhookNotifierConfig;
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['webhookNotifierConfig'] });
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
      } catch (_error) {
        return {
          onboarded: true,
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

export function useCompleteAutoDisableChannelOnboarding() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (input?: CompleteAutoDisableChannelOnboardingInput) => {
      const data = await graphqlRequest<{ completeAutoDisableChannelOnboarding: boolean }>(
        COMPLETE_AUTO_DISABLE_CHANNEL_ONBOARDING_MUTATION,
        { input: input || {} }
      );
      return data.completeAutoDisableChannelOnboarding;
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

export function useExportCacheDiagnostics() {
  const { handleError } = useErrorHandler();

  return useMutation({
    mutationFn: async () => {
      const data = await graphqlRequest<{ getCacheDiagnostics: GetCacheDiagnosticsPayload }>(
        GET_CACHE_DIAGNOSTICS_QUERY,
        { input: { targets: ['CHANNEL_CACHE'] } }
      );
      return data.getCacheDiagnostics;
    },
    onSuccess: (data) => {
      const blob = new Blob([data.content], { type: 'application/json' });
      const url = URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = data.fileName;
      document.body.appendChild(a);
      a.click();
      document.body.removeChild(a);
      URL.revokeObjectURL(url);
      toast.success(i18n.t('system.diagnostics.cache.exportSuccess'));
    },
    onError: (error) => {
      handleError(error, i18n.t('system.diagnostics.cache.exportFailed'));
    },
  });
}

export function useClearCache() {
  const { handleError } = useErrorHandler();

  return useMutation({
    mutationFn: async () => {
      const data = await graphqlRequest<{ clearCache: ClearCachePayload }>(CLEAR_CACHE_MUTATION, {
        input: { targets: ['CHANNEL_CACHE'] },
      });
      return data.clearCache;
    },
    onSuccess: (data) => {
      if (data.success) {
        toast.success(i18n.t('system.diagnostics.cache.clearSuccess'));
        return;
      }

      toast.error(data.message || i18n.t('system.diagnostics.cache.clearFailed'));
    },
    onError: (error) => {
      handleError(error, i18n.t('system.diagnostics.cache.clearFailed'));
    },
  });
}

// Model Settings
const MODEL_SETTINGS_QUERY = `
  query ModelSettings {
    systemModelSettings {
      fallbackToChannelsOnModelNotFound
      queryAllChannelModels
      defaultModelAPIIncludeAll
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
      autoSync {
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

const VIDEO_STORAGE_SETTINGS_QUERY = `
  query VideoStorageSettings {
    videoStorageSettings {
      enabled
      dataStorageID
      scanIntervalMinutes
      scanLimit
    }
  }
`;

const UPDATE_VIDEO_STORAGE_SETTINGS_MUTATION = `
  mutation UpdateVideoStorageSettings($input: UpdateVideoStorageSettingsInput!) {
    updateVideoStorageSettings(input: $input)
  }
`;

export interface ModelSettings {
  fallbackToChannelsOnModelNotFound: boolean;
  queryAllChannelModels: boolean;
  defaultModelAPIIncludeAll: boolean;
}

export interface UpdateModelSettingsInput {
  fallbackToChannelsOnModelNotFound?: boolean;
  queryAllChannelModels?: boolean;
  defaultModelAPIIncludeAll?: boolean;
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

export type AutoSyncFrequency = 'ONE_HOUR' | 'SIX_HOURS' | 'ONE_DAY';

export interface ChannelProbeSetting {
  enabled: boolean;
  frequency: ProbeFrequency;
}

export interface ChannelModelAutoSyncSetting {
  frequency: AutoSyncFrequency;
}

export interface ChannelSetting {
  probe: ChannelProbeSetting;
  autoSync: ChannelModelAutoSyncSetting;
}

export interface UpdateChannelProbeSettingInput {
  enabled?: boolean;
  frequency?: ProbeFrequency;
}

export interface UpdateChannelModelAutoSyncSettingInput {
  frequency?: AutoSyncFrequency;
}

export interface UpdateSystemChannelSettingsInput {
  probe?: UpdateChannelProbeSettingInput;
  autoSync?: UpdateChannelModelAutoSyncSettingInput;
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
    placeholderData: (previousData) => previousData,
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

export function useVideoStorageSettings() {
  const { handleError } = useErrorHandler();

  return useQuery({
    queryKey: ['videoStorageSettings'],
    queryFn: async () => {
      try {
        const data = await graphqlRequest<{ videoStorageSettings: VideoStorageSettings }>(VIDEO_STORAGE_SETTINGS_QUERY);
        return data.videoStorageSettings;
      } catch (error) {
        handleError(error, i18n.t('common.errors.internalServerError'));
        throw error;
      }
    },
  });
}

export function useUpdateVideoStorageSettings() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (input: UpdateVideoStorageSettingsInput) => {
      const data = await graphqlRequest<{ updateVideoStorageSettings: boolean }>(UPDATE_VIDEO_STORAGE_SETTINGS_MUTATION, { input });
      return data.updateVideoStorageSettings;
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['videoStorageSettings'] });
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
      formData.append(
        'operations',
        JSON.stringify({
          query: RESTORE_MUTATION,
          variables: { file: null, input },
        })
      );
      formData.append('map', JSON.stringify({ '0': ['variables.file'] }));
      formData.append('0', file);

      const token = getTokenFromStorage();
      const response = await fetch('/admin/graphql', {
        method: 'POST',
        headers: {
          Authorization: token ? `Bearer ${token}` : '',
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
      const data = await graphqlRequest<{ triggerAutoBackup: { success: boolean; message?: string } }>(TRIGGER_AUTO_BACKUP_MUTATION);
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

// Proxy Presets
const PROXY_PRESETS_QUERY = `
  query ProxyPresets {
    proxyPresets {
      name
      url
      username
      password
    }
  }
`;

const SAVE_PROXY_PRESET_MUTATION = `
  mutation SaveProxyPreset($input: SaveProxyPresetInput!) {
    saveProxyPreset(input: $input)
  }
`;

const DELETE_PROXY_PRESET_MUTATION = `
  mutation DeleteProxyPreset($url: String!) {
    deleteProxyPreset(url: $url)
  }
`;

export interface ProxyPreset {
  name?: string;
  url: string;
  username?: string;
  password?: string;
}

export interface SaveProxyPresetInput {
  name?: string;
  url: string;
  username?: string;
  password?: string;
}

export function useProxyPresets() {
  const { handleError } = useErrorHandler();

  return useQuery({
    queryKey: ['proxyPresets'],
    queryFn: async () => {
      try {
        const data = await graphqlRequest<{ proxyPresets: ProxyPreset[] }>(PROXY_PRESETS_QUERY);
        return data.proxyPresets;
      } catch (error) {
        handleError(error, i18n.t('common.errors.internalServerError'));
        throw error;
      }
    },
  });
}

export function useSaveProxyPreset() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (input: SaveProxyPresetInput) => {
      const data = await graphqlRequest<{ saveProxyPreset: boolean }>(SAVE_PROXY_PRESET_MUTATION, { input });
      return data.saveProxyPreset;
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['proxyPresets'] });
    },
    onError: () => {
      toast.error(i18n.t('common.errors.systemUpdateFailed'));
    },
  });
}

export function useDeleteProxyPreset() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (url: string) => {
      const data = await graphqlRequest<{ deleteProxyPreset: boolean }>(DELETE_PROXY_PRESET_MUTATION, { url });
      return data.deleteProxyPreset;
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['proxyPresets'] });
      toast.success(i18n.t('common.success.systemUpdated'));
    },
    onError: () => {
      toast.error(i18n.t('common.errors.systemUpdateFailed'));
    },
  });
}


// User-Agent Pass-Through Settings
const USER_AGENT_PASS_THROUGH_SETTINGS_QUERY = `
  query UserAgentPassThroughSettings {
    userAgentPassThroughSettings {
      enabled
    }
  }
`;

const UPDATE_USER_AGENT_PASS_THROUGH_SETTINGS_MUTATION = `
  mutation UpdateUserAgentPassThroughSettings($input: UpdateUserAgentPassThroughSettingsInput!) {
    updateUserAgentPassThroughSettings(input: $input)
  }
`;

export interface UserAgentPassThroughSettings {
  enabled: boolean;
}

export interface UpdateUserAgentPassThroughSettingsInput {
  enabled: boolean;
}

export function useUserAgentPassThroughSettings() {
  const { handleError } = useErrorHandler();

  return useQuery({
    queryKey: ['userAgentPassThroughSettings'],
    queryFn: async () => {
      try {
        const data = await graphqlRequest<{ userAgentPassThroughSettings: UserAgentPassThroughSettings }>(USER_AGENT_PASS_THROUGH_SETTINGS_QUERY);
        return data.userAgentPassThroughSettings;
      } catch (error) {
        handleError(error, i18n.t('common.errors.internalServerError'));
        throw error;
      }
    },
  });
}

export function useUpdateUserAgentPassThroughSettings() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (input: UpdateUserAgentPassThroughSettingsInput) => {
      const data = await graphqlRequest<{ updateUserAgentPassThroughSettings: boolean }>(UPDATE_USER_AGENT_PASS_THROUGH_SETTINGS_MUTATION, { input });
      return data.updateUserAgentPassThroughSettings;
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['userAgentPassThroughSettings'] });
      toast.success(i18n.t('common.success.systemUpdated'));
    },
    onError: () => {
      toast.error(i18n.t('common.errors.systemUpdateFailed'));
    },
  });
}
