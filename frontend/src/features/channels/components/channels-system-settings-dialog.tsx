'use client';

import React, { useCallback } from 'react';
import { Activity, Loader2, MessageSquareText, RefreshCw, RotateCcw } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { Button } from '@/components/ui/button';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Alert, AlertDescription } from '@/components/ui/alert';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Textarea } from '@/components/ui/textarea';
import {
  SETTINGS_DIALOG_CONTENT_CLASS,
  SettingsDialogBody,
  SettingsSection,
  SettingsSubOptions,
  SettingsSwitchRow,
} from '@/components/settings-dialog-layout';
import { useChannelSetting, useUpdateChannelSetting, type AutoSyncFrequency, type ProbeFrequency } from '@/features/system/data/system';
import { usePermissions } from '@/hooks/usePermissions';
import { useChannels } from '../context/channels-context';

const MAX_PROMPT_CODE_POINTS = 4096;
const DEFAULT_TEST_SYSTEM_PROMPT = 'You are a helpful assistant.';
const DEFAULT_TEST_USER_PROMPT = "Hello world, I'm AxonHub.\nPlease tell me who you are?";

const countCodePoints = (value: string) => Array.from(value).length;

const FREQUENCY_LABEL_KEYS: Record<ProbeFrequency | AutoSyncFrequency, string> = {
  ONE_MINUTE: 'channels.dialogs.systemSettings.frequency.oneMinute',
  FIVE_MINUTES: 'channels.dialogs.systemSettings.frequency.fiveMinutes',
  THIRTY_MINUTES: 'channels.dialogs.systemSettings.frequency.thirtyMinutes',
  ONE_HOUR: 'channels.dialogs.systemSettings.frequency.oneHour',
  SIX_HOURS: 'channels.dialogs.systemSettings.frequency.sixHours',
  ONE_DAY: 'channels.dialogs.systemSettings.frequency.oneDay',
};

const PROBE_FREQUENCY_OPTIONS: ProbeFrequency[] = ['ONE_MINUTE', 'FIVE_MINUTES', 'THIRTY_MINUTES', 'ONE_HOUR'];

const AUTO_SYNC_FREQUENCY_OPTIONS: AutoSyncFrequency[] = ['ONE_HOUR', 'SIX_HOURS', 'ONE_DAY'];

type SettingsSectionKey = 'health' | 'autoSync' | 'testPrompt';

export function ChannelsSystemSettingsDialog() {
  const { t } = useTranslation();
  const { open, setOpen } = useChannels();
  const { hasSystemScope } = usePermissions();
  const isOpen = open === 'channelSettings';
  const [activeSection, setActiveSection] = React.useState<SettingsSectionKey>('health');
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
    if (isOpen) {
      setActiveSection('health');
    }
  }, [isOpen]);

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

  const fieldsDisabled = updateSettings.isPending || !canWriteSettings;
  const sections = [
    { key: 'health' as const, title: t('channels.dialogs.systemSettings.channelProbe.label'), icon: Activity },
    { key: 'autoSync' as const, title: t('channels.dialogs.systemSettings.autoSync.label'), icon: RefreshCw },
    { key: 'testPrompt' as const, title: t('channels.dialogs.systemSettings.testPrompt.label'), icon: MessageSquareText },
  ];

  return (
    <Dialog open={isOpen} onOpenChange={handleClose}>
      <DialogContent className={SETTINGS_DIALOG_CONTENT_CLASS}>
        <DialogHeader className='shrink-0'>
          <DialogTitle>{t('channels.dialogs.systemSettings.title')}</DialogTitle>
          <DialogDescription>{t('channels.dialogs.systemSettings.description')}</DialogDescription>
        </DialogHeader>

        {isLoading ? (
          <div className='flex flex-1 items-center justify-center'>
            <Loader2 className='h-8 w-8 animate-spin' />
          </div>
        ) : (
          <SettingsDialogBody items={sections} active={activeSection} onSelect={setActiveSection}>
            {activeSection === 'health' && (
              <SettingsSection title={t('channels.dialogs.systemSettings.channelProbe.label')}>
                <SettingsSwitchRow
                  id='probe-enabled'
                  label={t('channels.dialogs.systemSettings.channelProbe.enabledLabel')}
                  description={
                    <>
                      <p>{t('channels.dialogs.systemSettings.channelProbe.enabledDescription')}</p>
                      <p>{t('channels.dialogs.systemSettings.channelProbe.probeDescription')}</p>
                    </>
                  }
                  checked={probeEnabled}
                  onCheckedChange={setProbeEnabled}
                  disabled={fieldsDisabled}
                >
                  {probeEnabled && (
                    <SettingsSubOptions>
                      <label htmlFor='probe-frequency' className='text-sm font-medium'>
                        {t('channels.dialogs.systemSettings.channelProbe.frequencyLabel')}
                      </label>
                      <Select value={probeFrequency} onValueChange={(value) => setProbeFrequency(value as ProbeFrequency)}>
                        <SelectTrigger id='probe-frequency' className='w-40' disabled={fieldsDisabled}>
                          <SelectValue />
                        </SelectTrigger>
                        <SelectContent>
                          {PROBE_FREQUENCY_OPTIONS.map((value) => (
                            <SelectItem key={value} value={value}>
                              {t(FREQUENCY_LABEL_KEYS[value])}
                            </SelectItem>
                          ))}
                        </SelectContent>
                      </Select>
                      <p className='text-muted-foreground text-xs'>
                        {t('channels.dialogs.systemSettings.channelProbe.frequencyDescription')}
                      </p>
                      <p className='text-xs text-amber-600 dark:text-amber-500'>
                        {t('channels.dialogs.systemSettings.channelProbe.frequencyWarning')}
                      </p>
                    </SettingsSubOptions>
                  )}
                </SettingsSwitchRow>
              </SettingsSection>
            )}

            {activeSection === 'autoSync' && (
              <SettingsSection title={t('channels.dialogs.systemSettings.autoSync.label')}>
                <div className='flex items-start justify-between gap-4 p-4'>
                  <div className='space-y-1'>
                    <label htmlFor='auto-sync-frequency' className='text-sm font-medium'>
                      {t('channels.dialogs.systemSettings.autoSync.frequencyLabel')}
                    </label>
                    <p className='text-muted-foreground text-xs leading-relaxed'>
                      {t('channels.dialogs.systemSettings.autoSync.frequencyDescription')}
                    </p>
                  </div>
                  <Select value={autoSyncFrequency} onValueChange={(value) => setAutoSyncFrequency(value as AutoSyncFrequency)}>
                    <SelectTrigger id='auto-sync-frequency' className='w-32 shrink-0' disabled={fieldsDisabled}>
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      {AUTO_SYNC_FREQUENCY_OPTIONS.map((value) => (
                        <SelectItem key={value} value={value}>
                          {t(FREQUENCY_LABEL_KEYS[value])}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>
              </SettingsSection>
            )}

            {activeSection === 'testPrompt' && (
              <SettingsSection title={t('channels.dialogs.systemSettings.testPrompt.label')}>
                <div className='space-y-4 p-4'>
                  <Alert>
                    <AlertDescription className='text-xs'>
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
                        disabled={fieldsDisabled}
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
                      disabled={fieldsDisabled}
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
                        disabled={fieldsDisabled}
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
                      disabled={fieldsDisabled}
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
                </div>
              </SettingsSection>
            )}
          </SettingsDialogBody>
        )}

        <DialogFooter className='shrink-0'>
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
