'use client';

import React, { useCallback } from 'react';
import { Loader2, Settings2, Activity, RotateCcw } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Alert, AlertDescription } from '@/components/ui/alert';
import { Switch } from '@/components/ui/switch';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Textarea } from '@/components/ui/textarea';
import { useChannelSetting, useUpdateChannelSetting, type AutoSyncFrequency, type ProbeFrequency } from '@/features/system/data/system';
import { usePermissions } from '@/hooks/usePermissions';
import { useChannels } from '../context/channels-context';

const MAX_PROMPT_CODE_POINTS = 4096;
const DEFAULT_TEST_SYSTEM_PROMPT = 'You are a helpful assistant.';
const DEFAULT_TEST_USER_PROMPT = "Hello world, I'm AxonHub.\nPlease tell me who you are?";

const countCodePoints = (value: string) => Array.from(value).length;

const PROBE_FREQUENCY_OPTIONS: { value: ProbeFrequency; label: string }[] = [
  { value: 'ONE_MINUTE', label: '1 minute' },
  { value: 'FIVE_MINUTES', label: '5 minutes' },
  { value: 'THIRTY_MINUTES', label: '30 minutes' },
  { value: 'ONE_HOUR', label: '1 hour' },
];

const AUTO_SYNC_FREQUENCY_OPTIONS: { value: AutoSyncFrequency; label: string }[] = [
  { value: 'ONE_HOUR', label: '1 hour' },
  { value: 'SIX_HOURS', label: '6 hours' },
  { value: 'ONE_DAY', label: '1 day' },
];

export function ChannelsSystemSettingsDialog() {
  const { t } = useTranslation();
  const { open, setOpen } = useChannels();
  const { hasSystemScope } = usePermissions();
  const isOpen = open === 'channelSettings';
  const canReadSettings = hasSystemScope('read_settings');
  const canWriteSettings = hasSystemScope('write_settings');
  const { data: settings, isLoading } = useChannelSetting({ enabled: isOpen && canReadSettings });
  const updateSettings = useUpdateChannelSetting();

  const [probeEnabled, setProbeEnabled] = React.useState(false);
  const [probeFrequency, setProbeFrequency] = React.useState<ProbeFrequency>('ONE_MINUTE');
  const [autoSyncFrequency, setAutoSyncFrequency] = React.useState<AutoSyncFrequency>('ONE_HOUR');
  const [testSystemPrompt, setTestSystemPrompt] = React.useState(DEFAULT_TEST_SYSTEM_PROMPT);
  const [testUserPrompt, setTestUserPrompt] = React.useState(DEFAULT_TEST_USER_PROMPT);

  React.useEffect(() => {
    if (!isOpen || !settings) return;
    if (settings?.probe) {
      setProbeEnabled(settings.probe.enabled);
      setProbeFrequency(settings.probe.frequency);
    }
    if (settings?.autoSync?.frequency) {
      setAutoSyncFrequency(settings.autoSync.frequency);
    }
    setTestSystemPrompt(settings.testSystemPrompt ?? DEFAULT_TEST_SYSTEM_PROMPT);
    setTestUserPrompt(settings.testUserPrompt ?? DEFAULT_TEST_USER_PROMPT);
  }, [isOpen, settings]);

  const systemPromptLength = countCodePoints(testSystemPrompt);
  const userPromptLength = countCodePoints(testUserPrompt);
  const promptTooLong = systemPromptLength > MAX_PROMPT_CODE_POINTS || userPromptLength > MAX_PROMPT_CODE_POINTS;

  const handleSave = useCallback(async () => {
    if (!canWriteSettings || promptTooLong) return;
    await updateSettings.mutateAsync({
      probe: {
        enabled: probeEnabled,
        frequency: probeFrequency,
      },
      autoSync: {
        frequency: autoSyncFrequency,
      },
      testSystemPrompt,
      testUserPrompt,
    });
    setOpen(null);
  }, [updateSettings, probeEnabled, probeFrequency, autoSyncFrequency, testSystemPrompt, testUserPrompt, canWriteSettings, promptTooLong, setOpen]);

  const handleClose = useCallback(() => {
    setOpen(null);
  }, [setOpen]);

  if (!canReadSettings) return null;

  return (
    <Dialog open={isOpen} onOpenChange={handleClose}>
      <DialogContent className='max-h-[90vh] overflow-y-auto sm:max-w-[720px]'>
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
                    disabled={updateSettings.isPending || !canWriteSettings}
                  />
                </div>

                {probeEnabled && (
                  <div className='space-y-2'>
                    <label htmlFor='probe-frequency' className='text-sm font-medium'>
                      {t('channels.dialogs.systemSettings.channelProbe.frequencyLabel')}
                    </label>
                    <Select value={probeFrequency} onValueChange={(value) => setProbeFrequency(value as ProbeFrequency)}>
                      <SelectTrigger id='probe-frequency' disabled={updateSettings.isPending || !canWriteSettings}>
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
            <Card>
              <CardHeader className='pb-0'>
                <CardTitle className='flex items-center gap-2 text-sm'>
                  <Activity className='text-muted-foreground h-4 w-4' />
                  {t('channels.dialogs.systemSettings.autoSync.label')}
                </CardTitle>
              </CardHeader>
              <CardContent className='space-y-4 pt-4'>
                <div className='space-y-2'>
                  <label htmlFor='auto-sync-frequency' className='text-sm font-medium'>
                    {t('channels.dialogs.systemSettings.autoSync.frequencyLabel')}
                  </label>
                  <Select value={autoSyncFrequency} onValueChange={(value) => setAutoSyncFrequency(value as AutoSyncFrequency)}>
                    <SelectTrigger id='auto-sync-frequency' disabled={updateSettings.isPending || !canWriteSettings}>
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      {AUTO_SYNC_FREQUENCY_OPTIONS.map((option) => (
                        <SelectItem key={option.value} value={option.value}>
                          {option.label}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                  <p className='text-muted-foreground text-xs'>{t('channels.dialogs.systemSettings.autoSync.frequencyDescription')}</p>
                </div>
              </CardContent>
            </Card>

            <Card>
              <CardHeader className='pb-0'>
                <CardTitle className='flex items-center gap-2 text-sm'>
                  <Settings2 className='text-muted-foreground h-4 w-4' />
                  {t('channels.dialogs.systemSettings.testPrompt.label')}
                </CardTitle>
              </CardHeader>
              <CardContent className='space-y-4 pt-4'>
                <Alert>
                  <AlertDescription>
                    <p>{t('channels.dialogs.systemSettings.testPrompt.warning')}</p>
                    <p className='mt-1'>{t('channels.dialogs.systemSettings.testPrompt.sensitiveWarning')}</p>
                  </AlertDescription>
                </Alert>

                <div className='space-y-2'>
                  <div className='flex items-center justify-between gap-2'>
                    <label htmlFor='channel-test-system-prompt' className='text-sm font-medium'>
                      {t('channels.dialogs.systemSettings.testPrompt.system.label')}
                    </label>
                    <Button
                      type='button'
                      variant='ghost'
                      size='sm'
                      onClick={() => setTestSystemPrompt(DEFAULT_TEST_SYSTEM_PROMPT)}
                      disabled={updateSettings.isPending || !canWriteSettings}
                      data-testid='channel-test-system-prompt-reset'
                    >
                      <RotateCcw className='mr-1 h-3.5 w-3.5' />
                      {t('channels.dialogs.systemSettings.testPrompt.reset')}
                    </Button>
                  </div>
                  <Textarea
                    id='channel-test-system-prompt'
                    data-testid='channel-test-system-prompt'
                    rows={4}
                    value={testSystemPrompt}
                    onChange={(event) => setTestSystemPrompt(event.target.value)}
                    disabled={updateSettings.isPending || !canWriteSettings}
                    aria-describedby='channel-test-system-prompt-count'
                  />
                  <p
                    id='channel-test-system-prompt-count'
                    className={`text-right text-xs ${systemPromptLength > MAX_PROMPT_CODE_POINTS ? 'text-destructive' : 'text-muted-foreground'}`}
                  >
                    {t('channels.dialogs.systemSettings.testPrompt.characterCount', {
                      count: systemPromptLength,
                      max: MAX_PROMPT_CODE_POINTS,
                    })}
                  </p>
                  {systemPromptLength > MAX_PROMPT_CODE_POINTS && (
                    <p className='text-destructive text-xs'>
                      {t('channels.dialogs.systemSettings.testPrompt.tooLong', { max: MAX_PROMPT_CODE_POINTS })}
                    </p>
                  )}
                </div>

                <div className='space-y-2'>
                  <div className='flex items-center justify-between gap-2'>
                    <label htmlFor='channel-test-user-prompt' className='text-sm font-medium'>
                      {t('channels.dialogs.systemSettings.testPrompt.user.label')}
                    </label>
                    <Button
                      type='button'
                      variant='ghost'
                      size='sm'
                      onClick={() => setTestUserPrompt(DEFAULT_TEST_USER_PROMPT)}
                      disabled={updateSettings.isPending || !canWriteSettings}
                      data-testid='channel-test-user-prompt-reset'
                    >
                      <RotateCcw className='mr-1 h-3.5 w-3.5' />
                      {t('channels.dialogs.systemSettings.testPrompt.reset')}
                    </Button>
                  </div>
                  <Textarea
                    id='channel-test-user-prompt'
                    data-testid='channel-test-user-prompt'
                    rows={4}
                    value={testUserPrompt}
                    onChange={(event) => setTestUserPrompt(event.target.value)}
                    disabled={updateSettings.isPending || !canWriteSettings}
                    aria-describedby='channel-test-user-prompt-count'
                  />
                  <p
                    id='channel-test-user-prompt-count'
                    className={`text-right text-xs ${userPromptLength > MAX_PROMPT_CODE_POINTS ? 'text-destructive' : 'text-muted-foreground'}`}
                  >
                    {t('channels.dialogs.systemSettings.testPrompt.characterCount', {
                      count: userPromptLength,
                      max: MAX_PROMPT_CODE_POINTS,
                    })}
                  </p>
                  {userPromptLength > MAX_PROMPT_CODE_POINTS && (
                    <p className='text-destructive text-xs'>
                      {t('channels.dialogs.systemSettings.testPrompt.tooLong', { max: MAX_PROMPT_CODE_POINTS })}
                    </p>
                  )}
                </div>
              </CardContent>
            </Card>
          </div>
        )}

        <DialogFooter>
          <Button variant='outline' onClick={handleClose} disabled={updateSettings.isPending}>
            {t('common.buttons.cancel')}
          </Button>
          <Button onClick={handleSave} disabled={updateSettings.isPending || isLoading || !canWriteSettings || promptTooLong}>
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
