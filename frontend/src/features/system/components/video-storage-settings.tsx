'use client';

import React from 'react';
import { Loader2, Save } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Switch } from '@/components/ui/switch';
import { useDataStorages } from '@/features/data-storages/data/data-storages';
import { extractNumberID } from '@/lib/utils';
import { useSystemContext } from '../context/system-context';
import { useUpdateVideoStorageSettings, useVideoStorageSettings } from '../data/system';

export function VideoStorageSettings() {
  const { t } = useTranslation();
  const { isLoading, setIsLoading } = useSystemContext();

  const { data: settings, isLoading: isLoadingSettings } = useVideoStorageSettings();
  const updateSettings = useUpdateVideoStorageSettings();

  const { data: dataStorages, isLoading: isLoadingStorages } = useDataStorages({ first: 100 });

  const externalStorages = React.useMemo(() => {
    return (
      dataStorages?.edges
        ?.map((e) => e.node)
        ?.filter((s) => s.status === 'active' && s.type !== 'database') ?? []
    );
  }, [dataStorages]);

  const [form, setForm] = React.useState({
    enabled: settings?.enabled ?? false,
    dataStorageID: settings?.dataStorageID ?? 0,
    scanIntervalMinutes: settings?.scanIntervalMinutes ?? 1,
    scanLimit: settings?.scanLimit ?? 50,
  });

  React.useEffect(() => {
    if (!settings) return;
    setForm({
      enabled: settings.enabled,
      dataStorageID: settings.dataStorageID,
      scanIntervalMinutes: settings.scanIntervalMinutes,
      scanLimit: settings.scanLimit,
    });
  }, [settings]);

  const canSave = React.useMemo(() => {
    if (!settings) return false;
    if (form.scanIntervalMinutes <= 0) return false;
    if (form.scanLimit <= 0) return false;
    if (form.enabled && form.dataStorageID <= 0) return false;
    return (
      form.enabled !== settings.enabled ||
      form.dataStorageID !== settings.dataStorageID ||
      form.scanIntervalMinutes !== settings.scanIntervalMinutes ||
      form.scanLimit !== settings.scanLimit
    );
  }, [form, settings]);

  const handleSave = async () => {
    setIsLoading(true);
    try {
      await updateSettings.mutateAsync({
        enabled: form.enabled,
        dataStorageID: form.dataStorageID,
        scanIntervalMinutes: form.scanIntervalMinutes,
        scanLimit: form.scanLimit,
      });
    } finally {
      setIsLoading(false);
    }
  };

  if (isLoadingSettings || isLoadingStorages) {
    return (
      <div className='flex h-32 items-center justify-center'>
        <Loader2 className='h-6 w-6 animate-spin' />
        <span className='text-muted-foreground ml-2'>{t('common.loading')}</span>
      </div>
    );
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t('system.videoStorage.title')}</CardTitle>
        <CardDescription>{t('system.videoStorage.description')}</CardDescription>
      </CardHeader>
      <CardContent className='space-y-6'>
        <div className='flex items-center justify-between'>
          <div className='space-y-0.5'>
            <Label htmlFor='video-storage-enabled'>{t('system.videoStorage.enabled.label')}</Label>
            <div className='text-muted-foreground text-sm'>{t('system.videoStorage.enabled.description')}</div>
          </div>
          <Switch
            id='video-storage-enabled'
            checked={form.enabled}
            onCheckedChange={(checked) => setForm((prev) => ({ ...prev, enabled: checked }))}
            disabled={isLoading}
          />
        </div>

        <div className='grid gap-2'>
          <Label htmlFor='video-storage-data-storage'>{t('system.videoStorage.dataStorage.label')}</Label>
          <Select
            value={form.dataStorageID > 0 ? String(form.dataStorageID) : ''}
            onValueChange={(value) => setForm((prev) => ({ ...prev, dataStorageID: parseInt(value) || 0 }))}
            disabled={isLoading || !form.enabled}
          >
            <SelectTrigger id='video-storage-data-storage'>
              <SelectValue placeholder={t('system.videoStorage.dataStorage.placeholder')} />
            </SelectTrigger>
            <SelectContent>
              {externalStorages.map((s) => (
                <SelectItem key={s.id} value={extractNumberID(s.id)}>
                  {s.name} ({s.type})
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
          <p className='text-muted-foreground text-sm'>{t('system.videoStorage.dataStorage.hint')}</p>
        </div>

        <div className='grid gap-2'>
          <Label htmlFor='video-storage-interval'>{t('system.videoStorage.scanInterval.label')}</Label>
          <Input
            id='video-storage-interval'
            type='number'
            min={1}
            value={form.scanIntervalMinutes}
            onChange={(e) => setForm((prev) => ({ ...prev, scanIntervalMinutes: parseInt(e.target.value) || 0 }))}
            disabled={isLoading}
          />
          <p className='text-muted-foreground text-sm'>{t('system.videoStorage.scanInterval.hint')}</p>
        </div>

        <div className='grid gap-2'>
          <Label htmlFor='video-storage-limit'>{t('system.videoStorage.scanLimit.label')}</Label>
          <Input
            id='video-storage-limit'
            type='number'
            min={1}
            value={form.scanLimit}
            onChange={(e) => setForm((prev) => ({ ...prev, scanLimit: parseInt(e.target.value) || 0 }))}
            disabled={isLoading}
          />
        </div>

        <div className='flex justify-end'>
          <Button onClick={handleSave} disabled={isLoading || updateSettings.isPending || !canSave} size='sm'>
            {isLoading || updateSettings.isPending ? (
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
  );
}
