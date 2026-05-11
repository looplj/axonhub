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
  usePreviewGcCleanup,
  CleanupOption,
  GcCleanupPreviewItem,
} from '../data/system';

export function StoragePolicySettings() {
  const { t } = useTranslation();
  const { isLoading, setIsLoading } = useSystemContext();

  const { data: storagePolicy, isLoading: isLoadingStoragePolicy } = useStoragePolicy();
  const updateStoragePolicy = useUpdateStoragePolicy();
  const triggerGcCleanup = useTriggerGcCleanup();
  const previewGcCleanup = usePreviewGcCleanup();

  const [storagePolicyState, setStoragePolicyState] = useState({
    storeChunks: storagePolicy?.storeChunks ?? false,
    livePreview: storagePolicy?.livePreview ?? false,
    storeRequestBody: storagePolicy?.storeRequestBody ?? true,
    storeResponseBody: storagePolicy?.storeResponseBody ?? true,
    cleanupOptions: storagePolicy?.cleanupOptions ?? [],
  });

  const [manualRequestsDays, setManualRequestsDays] = useState(30);
  const [manualUsageLogsDays, setManualUsageLogsDays] = useState(7);
  const [previewItems, setPreviewItems] = useState<GcCleanupPreviewItem[]>([]);
  const [isPreviewLoading, setIsPreviewLoading] = useState(false);
  const previewTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  const dialogOpenRef = useRef(false);

  useEffect(() => {
    if (storagePolicy) {
      setStoragePolicyState({
        storeChunks: storagePolicy.storeChunks,
        livePreview: storagePolicy.livePreview,
        storeRequestBody: storagePolicy.storeRequestBody,
        storeResponseBody: storagePolicy.storeResponseBody,
        cleanupOptions: storagePolicy.cleanupOptions,
      });
    }
  }, [storagePolicy]);

  const fetchPreview = React.useCallback(async (reqDays: number, usageDays: number) => {
    if (reqDays <= 0 && usageDays <= 0) {
      setPreviewItems([]);
      return;
    }
    setIsPreviewLoading(true);
    try {
      const items = await previewGcCleanup.mutateAsync({
        requestsCleanupDays: reqDays,
        usageLogsCleanupDays: usageDays,
      });
      setPreviewItems(items);
    } catch {
      setPreviewItems([]);
    } finally {
      setIsPreviewLoading(false);
    }
  }, [previewGcCleanup]);

  const schedulePreview = (reqDays: number, usageDays: number) => {
    if (previewTimerRef.current) {
      clearTimeout(previewTimerRef.current);
    }
    previewTimerRef.current = setTimeout(() => {
      fetchPreview(reqDays, usageDays);
    }, 500);
  };

  const handleDialogOpenChange = (open: boolean) => {
    dialogOpenRef.current = open;
    if (open) {
      const requestsOption = storagePolicyState.cleanupOptions.find(o => o.resourceType === 'requests');
      const usageLogsOption = storagePolicyState.cleanupOptions.find(o => o.resourceType === 'usage_logs');
      const reqDays = requestsOption?.cleanupDays || 30;
      const usageDays = usageLogsOption?.cleanupDays || 7;
      setManualRequestsDays(reqDays);
      setManualUsageLogsDays(usageDays);
      setPreviewItems([]);
      schedulePreview(reqDays, usageDays);
    }
  };

  const handleManualRequestsDaysChange = (value: number) => {
    const days = Math.max(1, Math.min(365, value || 1));
    setManualRequestsDays(days);
    schedulePreview(days, manualUsageLogsDays);
  };

  const handleManualUsageLogsDaysChange = (value: number) => {
    const days = Math.max(1, Math.min(365, value || 1));
    setManualUsageLogsDays(days);
    schedulePreview(manualRequestsDays, days);
  };

  const handleManualCleanup = () => {
    triggerGcCleanup.mutate({
      requestsCleanupDays: manualRequestsDays,
      usageLogsCleanupDays: manualUsageLogsDays,
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

  const handleCleanupOptionChange = (index: number, field: keyof CleanupOption, value: any) => {
    const newOptions = [...storagePolicyState.cleanupOptions];
    newOptions[index] = {
      ...newOptions[index],
      [field]: value,
    };
    setStoragePolicyState({
      ...storagePolicyState,
      cleanupOptions: newOptions,
    });
  };

  const hasChanges =
    storagePolicy &&
    (storagePolicy.storeChunks !== storagePolicyState.storeChunks ||
      storagePolicy.livePreview !== storagePolicyState.livePreview ||
      storagePolicy.storeRequestBody !== storagePolicyState.storeRequestBody ||
      storagePolicy.storeResponseBody !== storagePolicyState.storeResponseBody ||
      JSON.stringify(storagePolicy.cleanupOptions) !== JSON.stringify(storagePolicyState.cleanupOptions));

  if (isLoadingStoragePolicy) {
    return (
      <div className='flex h-32 items-center justify-center'>
        <Loader2 className='h-6 w-6 animate-spin' />
        <span className='text-muted-foreground ml-2'>{t('common.loading')}</span>
      </div>
    );
  }

  const resourceTypeLabel = (rt: string) => {
    if (rt === 'requests') return t('system.storage.policy.resourceTypes.requests');
    if (rt === 'usage_logs') return t('system.storage.policy.resourceTypes.usage_logs');
    return rt;
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
            <AlertDialogContent>
              <AlertDialogHeader>
                <AlertDialogTitle>{t('system.storage.policy.runCleanupManualTitle')}</AlertDialogTitle>
                <AlertDialogDescription>
                  {t('system.storage.policy.runCleanupManualDescription')}
                </AlertDialogDescription>
              </AlertDialogHeader>
              <div className='space-y-4 py-4'>
                <div className='flex items-center gap-4'>
                  <Label className='w-32 shrink-0'>{t('system.storage.policy.resourceTypes.requests')}</Label>
                  <div className='flex items-center gap-2'>
                    <Input
                      type='number'
                      min='1'
                      max='365'
                      value={manualRequestsDays}
                      onChange={(e) => handleManualRequestsDaysChange(parseInt(e.target.value) || 1)}
                      className='w-20'
                    />
                    <span className='text-muted-foreground text-sm'>{t('system.storage.policy.days')}</span>
                  </div>
                </div>
                <div className='flex items-center gap-4'>
                  <Label className='w-32 shrink-0'>{t('system.storage.policy.resourceTypes.usage_logs')}</Label>
                  <div className='flex items-center gap-2'>
                    <Input
                      type='number'
                      min='1'
                      max='365'
                      value={manualUsageLogsDays}
                      onChange={(e) => handleManualUsageLogsDaysChange(parseInt(e.target.value) || 1)}
                      className='w-20'
                    />
                    <span className='text-muted-foreground text-sm'>{t('system.storage.policy.days')}</span>
                  </div>
                </div>
                <div className='rounded-lg border p-3'>
                  <div className='text-sm font-medium mb-2'>{t('system.storage.policy.runCleanupPreviewLabel')}</div>
                  {isPreviewLoading ? (
                    <div className='flex items-center gap-2 text-muted-foreground text-sm'>
                      <Loader2 className='h-3 w-3 animate-spin' />
                      {t('system.storage.policy.runCleanupPreviewLoading')}
                    </div>
                  ) : previewItems.length === 0 ? (
                    <div className='text-muted-foreground text-sm'>
                      {t('system.storage.policy.runCleanupPreviewEmpty')}
                    </div>
                  ) : (
                    <ul className='space-y-1'>
                      {previewItems.map((item) => (
                        <li key={item.resourceType} className='text-sm'>
                          {t('system.storage.policy.runCleanupPreviewItem', {
                            count: item.estimatedCount,
                            resourceType: resourceTypeLabel(item.resourceType),
                            date: new Date(item.cutoffTime).toLocaleDateString(),
                          })}
                        </li>
                      ))}
                    </ul>
                  )}
                </div>
              </div>
              <AlertDialogFooter>
                <AlertDialogCancel>{t('system.storage.policy.runCleanupCancel')}</AlertDialogCancel>
                <AlertDialogAction
                  onClick={handleManualCleanup}
                  disabled={isPreviewLoading || triggerGcCleanup.isPending}
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
            {storagePolicyState.cleanupOptions.map((option, index) => (
              <div
                key={option.resourceType}
                className='flex flex-col gap-4 rounded-lg border p-4'
                id={'storage-cleanup-option-' + option.resourceType}
              >
                <div className='flex items-center justify-between'>
                  <div className='font-medium'>{t(`system.storage.policy.resourceTypes.${option.resourceType}`)}</div>
                  <Switch
                    checked={option.enabled}
                    onCheckedChange={(checked) => handleCleanupOptionChange(index, 'enabled', checked)}
                    disabled={isLoading}
                  />
                </div>
                {option.enabled && (
                  <div className='flex items-center gap-2'>
                    <Label htmlFor={`cleanup-days-${index}`}>{t('system.storage.policy.cleanupDays')}</Label>
                    <Input
                      id={`cleanup-days-${index}`}
                      type='number'
                      min='1'
                      max='365'
                      value={option.cleanupDays}
                      onChange={(e) => handleCleanupOptionChange(index, 'cleanupDays', parseInt(e.target.value) || 1)}
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
