'use client';

import React, { useCallback } from 'react';
import { Loader2, Settings2, Activity } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Switch } from '@/components/ui/switch';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { useChannelSetting, useUpdateChannelSetting, type ProbeFrequency } from '@/features/system/data/system';
import { useChannels } from '../context/channels-context';

const PROBE_FREQUENCY_OPTIONS: { value: ProbeFrequency; label: string }[] = [
  { value: 'ONE_MINUTE', label: '1 minute' },
  { value: 'FIVE_MINUTES', label: '5 minutes' },
  { value: 'THIRTY_MINUTES', label: '30 minutes' },
  { value: 'ONE_HOUR', label: '1 hour' },
];

export function ChannelsSystemSettingsDialog() {
  const { t } = useTranslation();
  const { open, setOpen } = useChannels();
  const { data: settings, isLoading } = useChannelSetting();
  const updateSettings = useUpdateChannelSetting();

  const isOpen = open === 'channelSettings';

  const [probeEnabled, setProbeEnabled] = React.useState(false);
  const [probeFrequency, setProbeFrequency] = React.useState<ProbeFrequency>('ONE_MINUTE');

  React.useEffect(() => {
    if (settings?.probe) {
      setProbeEnabled(settings.probe.enabled);
      setProbeFrequency(settings.probe.frequency);
    }
  }, [settings]);

  const handleSave = useCallback(async () => {
    await updateSettings.mutateAsync({
      probe: {
        enabled: probeEnabled,
        frequency: probeFrequency,
      },
    });
    setOpen(null);
  }, [updateSettings, probeEnabled, probeFrequency, setOpen]);

  const handleClose = useCallback(() => {
    setOpen(null);
  }, [setOpen]);

  return (
    <Dialog open={isOpen} onOpenChange={handleClose}>
      <DialogContent className='sm:max-w-[720px]'>
        <DialogHeader>
          <DialogTitle className='flex items-center gap-2'>
            <Settings2 className='h-5 w-5' />
            {t('channels.dialogs.systemSettings.title')}
          </DialogTitle>
          <DialogDescription>{t('channels.dialogs.systemSettings.description')}</DialogDescription>
        </DialogHeader>

        {isLoading ? (
          <div className='flex items-center justify-center py-12'>
            <Loader2 className='h-8 w-8 animate-spin' />
          </div>
        ) : (
          <div className='space-y-4'>
            <Card>
              <CardHeader className='pb-0'>
                <CardTitle className='flex items-center gap-2 text-sm'>
                  <Activity className='text-muted-foreground h-4 w-4' />
                  {t('channels.dialogs.systemSettings.channelProbe.label')}
                </CardTitle>
              </CardHeader>
              <CardContent className='space-y-4 pt-4'>
                <div className='flex items-center justify-between'>
                  <div className='flex-1 pr-4'>
                    <p className='text-sm font-medium'>{t('channels.dialogs.systemSettings.channelProbe.enabledLabel')}</p>
                    <p className='text-muted-foreground text-sm'>{t('channels.dialogs.systemSettings.channelProbe.enabledDescription')}</p>
                    <p className='text-muted-foreground text-xs mt-1'>{t('channels.dialogs.systemSettings.channelProbe.probeDescription')}</p>
                  </div>
                  <Switch
                    id='probe-enabled'
                    checked={probeEnabled}
                    onCheckedChange={setProbeEnabled}
                    disabled={updateSettings.isPending}
                  />
                </div>

                {probeEnabled && (
                  <div className='space-y-2'>
                    <label htmlFor='probe-frequency' className='text-sm font-medium'>
                      {t('channels.dialogs.systemSettings.channelProbe.frequencyLabel')}
                    </label>
                    <Select value={probeFrequency} onValueChange={(value) => setProbeFrequency(value as ProbeFrequency)}>
                      <SelectTrigger id='probe-frequency' disabled={updateSettings.isPending}>
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        {PROBE_FREQUENCY_OPTIONS.map((option) => (
                          <SelectItem key={option.value} value={option.value}>
                            {option.label}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                    <p className='text-muted-foreground text-xs'>{t('channels.dialogs.systemSettings.channelProbe.frequencyDescription')}</p>
                    <p className='text-muted-foreground text-xs mt-1'>{t('channels.dialogs.systemSettings.channelProbe.frequencyWarning')}</p>
                  </div>
                )}
              </CardContent>
            </Card>
          </div>
        )}

        <DialogFooter>
          <Button variant='outline' onClick={handleClose} disabled={updateSettings.isPending}>
            {t('common.buttons.cancel')}
          </Button>
          <Button onClick={handleSave} disabled={updateSettings.isPending || isLoading}>
            {updateSettings.isPending ? (
              <>
                <Loader2 className='mr-2 h-4 w-4 animate-spin' />
                {t('common.buttons.saving')}
              </>
            ) : (
              t('common.buttons.save')
            )}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
