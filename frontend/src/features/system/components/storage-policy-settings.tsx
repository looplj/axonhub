'use client';

import React, { useState, useRef, useEffect } from 'react';
import { Loader2, Save, Play } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger,
} from '@/components/ui/alert-dialog';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Switch } from '@/components/ui/switch';
import { useSystemContext } from '../context/system-context';
import {
  useStoragePolicy,
  useUpdateStoragePolicy,
  useTriggerGcCleanup,
  previewGcCleanup,
  CleanupOption,
  GcCleanupPreviewItem,
} from '../data/system';

const BODY_CLEANUP_TYPES = new Set(['request_bodies', 'response_bodies', 'response_chunks']);
const BODY_CLEANUP_TYPE_ORDER = ['request_bodies', 'response_bodies', 'response_chunks'];

type BodyPreviewGroup = {
  key: string;
  count: number;
  cutoffTime: string;
  types: string[];
};

function formatPreviewCount(count: number, locale?: string) {
  return count.toLocaleString(locale || undefined);
}

function formatPreviewDate(cutoffTime: string, locale?: string) {
  return new Date(cutoffTime).toLocaleDateString(locale || undefined);
}

function joinTypeLabels(labels: string[], locale?: string) {
  try {
    return new Intl.ListFormat(locale || undefined, { style: 'narrow', type: 'conjunction' }).format(labels);
  } catch {
    return labels.join(', ');
  }
}

function groupBodyPreviewItems(items: GcCleanupPreviewItem[]): BodyPreviewGroup[] {
  const groups = new Map<string, BodyPreviewGroup>();

  for (const item of items) {
    if (!BODY_CLEANUP_TYPES.has(item.resourceType)) {
      continue;
    }

    // 三种载荷各自算 cutoff，时间戳会差几毫秒；按保留天数合并。
    const key =
      item.retentionDays > 0
        ? `days:${item.retentionDays}`
        : `day:${new Date(item.cutoffTime).toISOString().slice(0, 10)}`;
    const existing = groups.get(key);
    if (existing) {
      if (!existing.types.includes(item.resourceType)) {
        existing.types.push(item.resourceType);
      }
      if (item.estimatedCount > existing.count) {
        existing.count = item.estimatedCount;
      }
      continue;
    }

    groups.set(key, {
      key,
      count: item.estimatedCount,
      cutoffTime: item.cutoffTime,
      types: [item.resourceType],
    });
  }

  return Array.from(groups.values()).map((group) => ({
    ...group,
    types: BODY_CLEANUP_TYPE_ORDER.filter((type) => group.types.includes(type)),
  }));
}

const DEFAULT_CLEANUP_OPTIONS: CleanupOption[] = [
  { resourceType: 'requests', enabled: false, cleanupDays: 3 },
  { resourceType: 'usage_logs', enabled: false, cleanupDays: 30 },
  { resourceType: 'request_bodies', enabled: false, cleanupDays: 7 },
  { resourceType: 'response_bodies', enabled: false, cleanupDays: 7 },
  { resourceType: 'response_chunks', enabled: false, cleanupDays: 3 },
];

function ensureCleanupOptions(options: CleanupOption[]): CleanupOption[] {
  const byType = new Map(options.map((option) => [option.resourceType, option]));
  const merged = options.map((option) => ({ ...option }));

  for (const def of DEFAULT_CLEANUP_OPTIONS) {
    if (!byType.has(def.resourceType)) {
      merged.push({ ...def });
    }
  }

  return merged;
}

function upsertCleanupOption(
  options: CleanupOption[],
  resourceType: string,
  patch: Partial<CleanupOption>
): CleanupOption[] {
  if (!options.some((option) => option.resourceType === resourceType)) {
    return [...options, { resourceType, enabled: false, cleanupDays: 7, ...patch }];
  }

  return options.map((option) => (option.resourceType === resourceType ? { ...option, ...patch } : option));
}

function cleanupOption(options: CleanupOption[], resourceType: string): CleanupOption | undefined {
  return options.find((option) => option.resourceType === resourceType);
}

export function StoragePolicySettings() {
  const { t, i18n } = useTranslation();
  const { isLoading, setIsLoading } = useSystemContext();

  const { data: storagePolicy, isLoading: isLoadingStoragePolicy } = useStoragePolicy();
  const updateStoragePolicy = useUpdateStoragePolicy();
  const triggerGcCleanup = useTriggerGcCleanup();

  const [storagePolicyState, setStoragePolicyState] = useState({
    storeChunks: storagePolicy?.storeChunks ?? false,
    livePreview: storagePolicy?.livePreview ?? false,
    storeRequestBody: storagePolicy?.storeRequestBody ?? true,
    storeResponseBody: storagePolicy?.storeResponseBody ?? true,
    cleanupOptions: ensureCleanupOptions(storagePolicy?.cleanupOptions ?? []),
  });

  const [manualRequestsDays, setManualRequestsDays] = useState(30);
  const [manualUsageLogsDays, setManualUsageLogsDays] = useState(7);
  const [manualRequestBodiesDays, setManualRequestBodiesDays] = useState(7);
  const [manualResponseBodiesDays, setManualResponseBodiesDays] = useState(7);
  const [manualResponseChunksDays, setManualResponseChunksDays] = useState(3);
  const [previewItems, setPreviewItems] = useState<GcCleanupPreviewItem[]>([]);
  const [isPreviewLoading, setIsPreviewLoading] = useState(false);
  const [previewFailed, setPreviewFailed] = useState(false);
  const previewTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const previewAbortRef = useRef<AbortController | null>(null);

  const dialogOpenRef = useRef(false);

  useEffect(() => {
    if (storagePolicy) {
      setStoragePolicyState({
        storeChunks: storagePolicy.storeChunks,
        livePreview: storagePolicy.livePreview,
        storeRequestBody: storagePolicy.storeRequestBody,
        storeResponseBody: storagePolicy.storeResponseBody,
        cleanupOptions: ensureCleanupOptions(storagePolicy.cleanupOptions),
      });
    }
  }, [storagePolicy]);

  const fetchPreview = React.useCallback(async (
    reqDays: number,
    usageDays: number,
    requestBodyDays: number,
    responseBodyDays: number,
    chunkDays: number,
  ) => {
    if (reqDays <= 0 && usageDays <= 0 && requestBodyDays <= 0 && responseBodyDays <= 0 && chunkDays <= 0) {
      setPreviewItems([]);
      setPreviewFailed(false);
      return;
    }
    previewAbortRef.current?.abort();
    const controller = new AbortController();
    previewAbortRef.current = controller;
    let timedOut = false;
    const timeoutId = window.setTimeout(() => {
      timedOut = true;
      controller.abort();
    }, 12_000);
    setIsPreviewLoading(true);
    setPreviewFailed(false);
    try {
      const items = await previewGcCleanup(
        {
          requestsCleanupDays: reqDays,
          usageLogsCleanupDays: usageDays,
          requestBodiesCleanupDays: requestBodyDays,
          responseBodiesCleanupDays: responseBodyDays,
          responseChunksCleanupDays: chunkDays,
        },
        controller.signal
      );
      if (timedOut) {
        setPreviewItems([]);
        setPreviewFailed(true);
        return;
      }
      if (controller.signal.aborted) {
        return;
      }
      setPreviewItems(items);
    } catch (error) {
      const aborted =
        controller.signal.aborted || (error instanceof DOMException && error.name === 'AbortError');
      if (aborted && !timedOut) {
        return;
      }
      setPreviewItems([]);
      setPreviewFailed(true);
    } finally {
      window.clearTimeout(timeoutId);
      if (previewAbortRef.current === controller) {
        setIsPreviewLoading(false);
      }
    }
  }, []);

  const schedulePreview = (
    reqDays: number,
    usageDays: number,
    requestBodyDays: number,
    responseBodyDays: number,
    chunkDays: number,
  ) => {
    if (previewTimerRef.current) {
      clearTimeout(previewTimerRef.current);
    }
    previewTimerRef.current = setTimeout(() => {
      fetchPreview(reqDays, usageDays, requestBodyDays, responseBodyDays, chunkDays);
    }, 500);
  };

  const handleDialogOpenChange = (open: boolean) => {
    dialogOpenRef.current = open;
    if (!open) {
      if (previewTimerRef.current) {
        clearTimeout(previewTimerRef.current);
      }
      previewAbortRef.current?.abort();
      setIsPreviewLoading(false);
      return;
    }

    {
      const reqDays = cleanupOption(storagePolicyState.cleanupOptions, 'requests')?.cleanupDays || 30;
      const usageDays = cleanupOption(storagePolicyState.cleanupOptions, 'usage_logs')?.cleanupDays || 7;
      const requestBodyDays = cleanupOption(storagePolicyState.cleanupOptions, 'request_bodies')?.cleanupDays || 7;
      const responseBodyDays = cleanupOption(storagePolicyState.cleanupOptions, 'response_bodies')?.cleanupDays || 7;
      const chunkDays = cleanupOption(storagePolicyState.cleanupOptions, 'response_chunks')?.cleanupDays || 3;
      setManualRequestsDays(reqDays);
      setManualUsageLogsDays(usageDays);
      setManualRequestBodiesDays(requestBodyDays);
      setManualResponseBodiesDays(responseBodyDays);
      setManualResponseChunksDays(chunkDays);
      setPreviewItems([]);
      setPreviewFailed(false);
      schedulePreview(reqDays, usageDays, requestBodyDays, responseBodyDays, chunkDays);
    }
  };

  const clampDays = (value: number) => {
    if (!Number.isFinite(value) || value <= 0) {
      return 0;
    }
    return Math.min(365, Math.floor(value));
  };

  const handleManualRequestsDaysChange = (value: number) => {
    const days = clampDays(value);
    setManualRequestsDays(days);
    schedulePreview(days, manualUsageLogsDays, manualRequestBodiesDays, manualResponseBodiesDays, manualResponseChunksDays);
  };

  const handleManualUsageLogsDaysChange = (value: number) => {
    const days = clampDays(value);
    setManualUsageLogsDays(days);
    schedulePreview(manualRequestsDays, days, manualRequestBodiesDays, manualResponseBodiesDays, manualResponseChunksDays);
  };

  const handleManualRequestBodiesDaysChange = (value: number) => {
    const days = clampDays(value);
    setManualRequestBodiesDays(days);
    schedulePreview(manualRequestsDays, manualUsageLogsDays, days, manualResponseBodiesDays, manualResponseChunksDays);
  };

  const handleManualResponseBodiesDaysChange = (value: number) => {
    const days = clampDays(value);
    setManualResponseBodiesDays(days);
    schedulePreview(manualRequestsDays, manualUsageLogsDays, manualRequestBodiesDays, days, manualResponseChunksDays);
  };

  const handleManualResponseChunksDaysChange = (value: number) => {
    const days = clampDays(value);
    setManualResponseChunksDays(days);
    schedulePreview(manualRequestsDays, manualUsageLogsDays, manualRequestBodiesDays, manualResponseBodiesDays, days);
  };

  const handleManualCleanup = () => {
    triggerGcCleanup.mutate({
      requestsCleanupDays: manualRequestsDays,
      usageLogsCleanupDays: manualUsageLogsDays,
      requestBodiesCleanupDays: manualRequestBodiesDays,
      responseBodiesCleanupDays: manualResponseBodiesDays,
      responseChunksCleanupDays: manualResponseChunksDays,
    });
  };

  const handleSave = async () => {
    setIsLoading(true);
    try {
      await updateStoragePolicy.mutateAsync({
        storeChunks: storagePolicyState.storeChunks,
        livePreview: storagePolicyState.livePreview,
        storeRequestBody: storagePolicyState.storeRequestBody,
        storeResponseBody: storagePolicyState.storeResponseBody,
        cleanupOptions: storagePolicyState.cleanupOptions.map((option) => ({
          resourceType: option.resourceType,
          enabled: option.enabled,
          cleanupDays: option.cleanupDays,
        })),
      });
    } finally {
      setIsLoading(false);
    }
  };

  const handleCleanupOptionChange = (resourceType: string, field: keyof CleanupOption, value: CleanupOption[keyof CleanupOption]) => {
    setStoragePolicyState({
      ...storagePolicyState,
      cleanupOptions: upsertCleanupOption(storagePolicyState.cleanupOptions, resourceType, {
        [field]: value,
      }),
    });
  };

  const hasChanges =
    storagePolicy &&
    (storagePolicy.storeChunks !== storagePolicyState.storeChunks ||
      storagePolicy.livePreview !== storagePolicyState.livePreview ||
      storagePolicy.storeRequestBody !== storagePolicyState.storeRequestBody ||
      storagePolicy.storeResponseBody !== storagePolicyState.storeResponseBody ||
      JSON.stringify(ensureCleanupOptions(storagePolicy.cleanupOptions)) !==
        JSON.stringify(storagePolicyState.cleanupOptions));

  if (isLoadingStoragePolicy) {
    return (
      <div className='flex h-32 items-center justify-center'>
        <Loader2 className='h-6 w-6 animate-spin' />
        <span className='text-muted-foreground ml-2'>{t('common.loading')}</span>
      </div>
    );
  }

  const resourceTypeLabel = (rt: string) => {
    const key = `system.storage.policy.resourceTypes.${rt}`;
    const label = t(key);
    return label === key ? rt : label;
  };

  const renderManualDaysRow = (
    resourceType: string,
    value: number,
    onChange: (value: number) => void,
  ) => (
    <div className='flex items-center gap-4'>
      <Label className='min-w-36 w-40 shrink-0'>{resourceTypeLabel(resourceType)}</Label>
      <div className='flex items-center gap-2'>
        <Input
          type='number'
          min='0'
          max='365'
          value={value}
          onChange={(e) => onChange(parseInt(e.target.value, 10) || 0)}
          className='w-20'
        />
        <span className='text-muted-foreground text-sm'>{t('system.storage.policy.days')}</span>
      </div>
    </div>
  );

  const cleanupOptionDescription = (resourceType: string) => {
    const key = `system.storage.policy.resourceTypeDescriptions.${resourceType}`;
    const description = t(key);
    return description === key ? '' : description;
  };

  return (
    <>
      <Card>
        <CardHeader className='flex flex-row items-center justify-between space-y-0 pb-2'>
          <div className='space-y-1.5'>
            <CardTitle>{t('system.storage.policy.title')}</CardTitle>
            <CardDescription>{t('system.storage.policy.description')}</CardDescription>
          </div>
          <AlertDialog onOpenChange={handleDialogOpenChange}>
            <AlertDialogTrigger asChild>
              <Button
                variant='outline'
                size='sm'
                disabled={triggerGcCleanup.isPending || isLoading}
              >
                {triggerGcCleanup.isPending ? (
                  <Loader2 className='mr-2 h-4 w-4 animate-spin' />
                ) : (
                  <Play className='mr-2 h-4 w-4' />
                )}
                {t('system.storage.policy.runCleanupNow')}
              </Button>
            </AlertDialogTrigger>
            <AlertDialogContent className='max-h-[90vh] overflow-y-auto'>
              <AlertDialogHeader>
                <AlertDialogTitle>{t('system.storage.policy.runCleanupManualTitle')}</AlertDialogTitle>
                <AlertDialogDescription>
                  {t('system.storage.policy.runCleanupManualDescription')}
                </AlertDialogDescription>
              </AlertDialogHeader>
              <div className='space-y-5 py-4'>
                <div className='space-y-3'>
                  <div className='text-sm font-medium'>{t('system.storage.policy.runCleanupGroupDelete')}</div>
                  {renderManualDaysRow('requests', manualRequestsDays, handleManualRequestsDaysChange)}
                  {renderManualDaysRow('usage_logs', manualUsageLogsDays, handleManualUsageLogsDaysChange)}
                </div>
                <div className='space-y-3'>
                  <div className='text-sm font-medium'>{t('system.storage.policy.runCleanupGroupStrip')}</div>
                  {renderManualDaysRow('request_bodies', manualRequestBodiesDays, handleManualRequestBodiesDaysChange)}
                  {renderManualDaysRow('response_bodies', manualResponseBodiesDays, handleManualResponseBodiesDaysChange)}
                  {renderManualDaysRow('response_chunks', manualResponseChunksDays, handleManualResponseChunksDaysChange)}
                </div>
                <div className='text-muted-foreground text-xs'>{t('system.storage.policy.runCleanupSkipHint')}</div>
                <div className='rounded-lg border p-3'>
                  <div className='text-sm font-medium mb-2'>{t('system.storage.policy.runCleanupPreviewLabel')}</div>
                  {isPreviewLoading ? (
                    <div className='flex items-center gap-2 text-muted-foreground text-sm'>
                      <Loader2 className='h-3 w-3 animate-spin' />
                      {t('system.storage.policy.runCleanupPreviewLoading')}
                    </div>
                  ) : previewFailed ? (
                    <div className='text-muted-foreground text-sm'>
                      {t('system.storage.policy.runCleanupPreviewFailed')}
                    </div>
                  ) : previewItems.length === 0 ? (
                    <div className='text-muted-foreground text-sm'>
                      {t('system.storage.policy.runCleanupPreviewEmpty')}
                    </div>
                  ) : (
                    <div className='space-y-3'>
                      {previewItems.some((item) => !BODY_CLEANUP_TYPES.has(item.resourceType)) ? (
                        <ul className='space-y-1'>
                          {previewItems
                            .filter((item) => !BODY_CLEANUP_TYPES.has(item.resourceType))
                            .map((item) => (
                              <li key={item.resourceType} className='text-sm'>
                                {t('system.storage.policy.runCleanupPreviewItem', {
                                  count: formatPreviewCount(item.estimatedCount, i18n.language),
                                  resourceType: resourceTypeLabel(item.resourceType),
                                  date: formatPreviewDate(item.cutoffTime, i18n.language),
                                })}
                              </li>
                            ))}
                        </ul>
                      ) : null}
                      {(() => {
                        const bodyGroups = groupBodyPreviewItems(previewItems);
                        if (bodyGroups.length === 0) {
                          return null;
                        }
                        return (
                          <div className='space-y-2'>
                            <div className='text-sm font-medium'>
                              {t('system.storage.policy.runCleanupPreviewScanLabel')}
                            </div>
                            {bodyGroups.map((group) => (
                              <div key={group.key} className='space-y-1'>
                                <div className='text-sm'>
                                  {t('system.storage.policy.runCleanupPreviewBodyRange', {
                                    count: formatPreviewCount(group.count, i18n.language),
                                    date: formatPreviewDate(group.cutoffTime, i18n.language),
                                  })}
                                </div>
                                <div className='text-sm'>
                                  {t('system.storage.policy.runCleanupPreviewBodyTypes', {
                                    types: joinTypeLabels(
                                      group.types.map((type) => resourceTypeLabel(type)),
                                      i18n.language
                                    ),
                                  })}
                                </div>
                              </div>
                            ))}
                            <div className='text-muted-foreground text-xs'>
                              {t('system.storage.policy.runCleanupPreviewBodyHint')}
                            </div>
                          </div>
                        );
                      })()}
                    </div>
                  )}
                </div>
              </div>
              <AlertDialogFooter>
                <AlertDialogCancel>{t('system.storage.policy.runCleanupCancel')}</AlertDialogCancel>
                <AlertDialogAction
                  onClick={handleManualCleanup}
                  disabled={triggerGcCleanup.isPending}
                >
                  {triggerGcCleanup.isPending ? (
                    <>
                      <Loader2 className='mr-2 h-4 w-4 animate-spin' />
                      {t('common.loading')}
                    </>
                  ) : (
                    t('system.storage.policy.runCleanupConfirm')
                  )}
                </AlertDialogAction>
              </AlertDialogFooter>
            </AlertDialogContent>
          </AlertDialog>
        </CardHeader>
        <CardContent className='space-y-6'>
          <div className='flex items-center justify-between' id='storage-enabled-switch'>
            <div className='space-y-0.5'>
              <Label htmlFor='storage-policy-store-chunks'>{t('system.storage.policy.storeChunks.label')}</Label>
              <div className='text-muted-foreground text-sm'>{t('system.storage.policy.storeChunks.description')}</div>
            </div>
            <Switch
              id='storage-policy-store-chunks'
              checked={storagePolicyState.storeChunks}
              onCheckedChange={(checked) =>
                setStoragePolicyState({
                  ...storagePolicyState,
                  storeChunks: checked,
                })
              }
              disabled={isLoading}
            />
          </div>

          <div className='flex items-center justify-between'>
            <div className='space-y-0.5'>
              <Label htmlFor='storage-policy-live-preview'>{t('system.storage.policy.livePreview.label')}</Label>
              <div className='text-muted-foreground text-sm'>{t('system.storage.policy.livePreview.description')}</div>
            </div>
            <Switch
              id='storage-policy-live-preview'
              checked={storagePolicyState.livePreview}
              onCheckedChange={(checked) =>
                setStoragePolicyState({
                  ...storagePolicyState,
                  livePreview: checked,
                })
              }
              disabled={isLoading}
            />
          </div>

          <div className='flex items-center justify-between'>
            <div className='space-y-0.5'>
              <Label htmlFor='storage-policy-store-request-body'>{t('system.storage.policy.storeRequestBody.label')}</Label>
              <div className='text-muted-foreground text-sm'>{t('system.storage.policy.storeRequestBody.description')}</div>
            </div>
            <Switch
              id='storage-policy-store-request-body'
              checked={storagePolicyState.storeRequestBody}
              onCheckedChange={(checked) =>
                setStoragePolicyState({
                  ...storagePolicyState,
                  storeRequestBody: checked,
                })
              }
              disabled={isLoading}
            />
          </div>

          <div className='flex items-center justify-between'>
            <div className='space-y-0.5'>
              <Label htmlFor='storage-policy-store-response-body'>{t('system.storage.policy.storeResponseBody.label')}</Label>
              <div className='text-muted-foreground text-sm'>{t('system.storage.policy.storeResponseBody.description')}</div>
            </div>
            <Switch
              id='storage-policy-store-response-body'
              checked={storagePolicyState.storeResponseBody}
              onCheckedChange={(checked) =>
                setStoragePolicyState({
                  ...storagePolicyState,
                  storeResponseBody: checked,
                })
              }
              disabled={isLoading}
            />
          </div>

          <div className='space-y-4'>
            <div className='space-y-2'>
              <div className='text-lg font-medium'>{t('system.storage.policy.cleanupOptions')}</div>
              <div className='text-muted-foreground text-sm'>{t('system.storage.policy.cleanupDescription')}</div>
            </div>
            {storagePolicyState.cleanupOptions.map((option) => (
              <div
                key={option.resourceType}
                className='flex flex-col gap-4 rounded-lg border p-4'
                id={'storage-cleanup-option-' + option.resourceType}
              >
                <div className='flex items-center justify-between'>
                  <div className='space-y-0.5'>
                    <div className='font-medium'>{t(`system.storage.policy.resourceTypes.${option.resourceType}`)}</div>
                    {cleanupOptionDescription(option.resourceType) ? (
                      <div className='text-muted-foreground text-sm'>
                        {cleanupOptionDescription(option.resourceType)}
                      </div>
                    ) : null}
                  </div>
                  <Switch
                    checked={option.enabled}
                    onCheckedChange={(checked) => handleCleanupOptionChange(option.resourceType, 'enabled', checked)}
                    disabled={isLoading}
                  />
                </div>
                {option.enabled && (
                  <div className='flex items-center gap-2'>
                    <Label htmlFor={`cleanup-days-${option.resourceType}`}>{t('system.storage.policy.cleanupDays')}</Label>
                    <Input
                      id={`cleanup-days-${option.resourceType}`}
                      type='number'
                      min='1'
                      max='365'
                      value={option.cleanupDays}
                      onChange={(e) => handleCleanupOptionChange(option.resourceType, 'cleanupDays', parseInt(e.target.value) || 1)}
                      className='w-24'
                      disabled={isLoading}
                    />
                    <span>{t('system.storage.policy.days')}</span>
                  </div>
                )}
              </div>
            ))}
          </div>

          <div className='flex justify-end'>
            <Button onClick={handleSave} disabled={isLoading || updateStoragePolicy.isPending || !hasChanges} size='sm'>
              {isLoading || updateStoragePolicy.isPending ? (
                <>
                  <Loader2 className='mr-2 h-4 w-4 animate-spin' />
                  {t('system.buttons.saving')}
                </>
              ) : (
                <>
                  <Save className='mr-2 h-4 w-4' />
                  {t('system.buttons.save')}
                </>
              )}
            </Button>
          </div>
        </CardContent>
      </Card>
    </>
  );
}
