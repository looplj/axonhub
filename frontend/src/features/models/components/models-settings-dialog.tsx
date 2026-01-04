'use client';

import React, { useCallback } from 'react';
import { Loader2, Settings2, RefreshCcw, Layers } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Switch } from '@/components/ui/switch';
import { useModelSettings, useUpdateModelSettings, type UpdateModelSettingsInput } from '@/features/system/data/system';
import { useModels } from '../context/models-context';

export function ModelSettingsDialog() {
  const { t } = useTranslation();
  const { open, setOpen } = useModels();
  const { data: settings, isLoading } = useModelSettings();
  const updateModelSettings = useUpdateModelSettings();

  const isOpen = open === 'settings';

  const [fallbackEnabled, setFallbackEnabled] = React.useState(false);
  const [queryAllChannelModels, setQueryAllChannelModels] = React.useState(false);

  React.useEffect(() => {
    if (settings) {
      setFallbackEnabled(settings.fallbackToChannelsOnModelNotFound);
      setQueryAllChannelModels(settings.queryAllChannelModels);
    }
  }, [settings]);

  const handleSave = useCallback(async () => {
    const input: UpdateModelSettingsInput = {
      fallbackToChannelsOnModelNotFound: fallbackEnabled,
      queryAllChannelModels: queryAllChannelModels,
    };
    await updateModelSettings.mutateAsync(input);
    setOpen(null);
  }, [updateModelSettings, fallbackEnabled, queryAllChannelModels, setOpen]);

  const handleClose = useCallback(() => {
    setOpen(null);
  }, [setOpen]);

  return (
    <Dialog open={isOpen} onOpenChange={handleClose}>
      <DialogContent className='sm:max-w-[720px]'>
        <DialogHeader>
          <DialogTitle className='flex items-center gap-2'>
            <Settings2 className='h-5 w-5' />
            {t('models.dialogs.settings.title')}
          </DialogTitle>
          <DialogDescription>{t('models.dialogs.settings.description')}</DialogDescription>
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
                  <RefreshCcw className='text-muted-foreground h-4 w-4' />
                  {t('models.dialogs.settings.fallbackToChannels.label')}
                </CardTitle>
              </CardHeader>
              <CardContent className='pt-1'>
                <div className='flex items-center justify-between'>
                  <p className='text-muted-foreground pr-4 text-sm'>{t('models.dialogs.settings.fallbackToChannels.description')}</p>
                  <Switch
                    id='fallback-enabled'
                    checked={fallbackEnabled}
                    onCheckedChange={setFallbackEnabled}
                    disabled={updateModelSettings.isPending}
                  />
                </div>
              </CardContent>
            </Card>

            <Card>
              <CardHeader className='pb-0'>
                <CardTitle className='flex items-center gap-2 text-sm'>
                  <Layers className='text-muted-foreground h-4 w-4' />
                  {t('models.dialogs.settings.queryAllChannelModels.label')}
                </CardTitle>
              </CardHeader>
              <CardContent className='pt-1'>
                <div className='flex items-center justify-between'>
                  <p className='text-muted-foreground pr-4 text-sm'>{t('models.dialogs.settings.queryAllChannelModels.description')}</p>
                  <Switch
                    id='query-all-channel-models'
                    checked={queryAllChannelModels}
                    onCheckedChange={setQueryAllChannelModels}
                    disabled={updateModelSettings.isPending}
                  />
                </div>
              </CardContent>
            </Card>
          </div>
        )}

        <DialogFooter>
          <Button variant='outline' onClick={handleClose} disabled={updateModelSettings.isPending}>
            {t('common.buttons.cancel')}
          </Button>
          <Button onClick={handleSave} disabled={updateModelSettings.isPending || isLoading}>
            {updateModelSettings.isPending ? (
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
