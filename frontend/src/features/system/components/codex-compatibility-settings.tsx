'use client';

import { useState } from 'react';
import { AlertCircle, Info, Loader2, Save } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { usePermissions } from '@/hooks/usePermissions';
import { Alert, AlertDescription } from '@/components/ui/alert';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Label } from '@/components/ui/label';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Switch } from '@/components/ui/switch';
import {
  NO_CODEX_CHANNEL_VALUE,
  channelSelectValue,
  codexAccess,
  codexFormControls,
  codexSettingsViewState,
  parseChannelSelectValue,
  resolveSelection,
  toUpdateInput,
  type CodexChannelStatus,
  type CodexCompatibilitySettings as CodexSettingsValue,
} from '../data/codex-compatibility';
import {
  useCodexCatalogChannels,
  useCodexCatalogTest,
  useCodexCompatibilitySettings,
  useUpdateCodexCompatibilitySettings,
} from '../data/codex-compatibility-api';
import { CodexCatalogTestPanel } from './codex-catalog-test-panel';

const STATUS_BADGE_CLASS: Record<CodexChannelStatus, string> = {
  enabled: 'bg-emerald-50 text-emerald-700 border-emerald-200 dark:bg-emerald-950 dark:text-emerald-400 dark:border-emerald-800',
  disabled: 'bg-gray-50 text-gray-600 border-gray-200 dark:bg-gray-900 dark:text-gray-400 dark:border-gray-700',
  archived: 'bg-amber-50 text-amber-700 border-amber-200 dark:bg-amber-950 dark:text-amber-400 dark:border-amber-800',
};

export function CodexCompatibilitySettings() {
  const { t } = useTranslation();
  const { hasSystemScope } = usePermissions();
  const canReadSettings = hasSystemScope('read_settings');
  const access = codexAccess(hasSystemScope);
  const settingsQuery = useCodexCompatibilitySettings();
  const channelsQuery = useCodexCatalogChannels();
  const update = useUpdateCodexCompatibilitySettings();
  // Unsaved edits only; everything else is derived from the saved settings so a refetch never needs syncing.
  const [draft, setDraft] = useState<Partial<CodexSettingsValue>>({});

  const saved = settingsQuery.data;
  const channels = channelsQuery.data;
  const resolved = saved ? resolveSelection(saved, channels) : null;
  const form: CodexSettingsValue | null =
    saved && resolved
      ? { enabled: draft.enabled ?? saved.enabled, channelID: draft.channelID !== undefined ? draft.channelID : resolved.channelID }
      : null;
  const test = useCodexCatalogTest(form?.channelID ?? null);

  const viewState = codexSettingsViewState({
    canReadSettings,
    isLoading: settingsQuery.isLoading || channelsQuery.isLoading,
    hasData: Boolean(saved && resolved && form),
  });

  if (viewState === 'permission') {
    return (
      <Alert data-testid='codex-compat-read-permission'>
        <Info />
        <AlertDescription>{t('common.routeGuard.noPermission')}</AlertDescription>
      </Alert>
    );
  }

  if (viewState === 'loading') {
    return (
      <div className='flex h-32 items-center justify-center'>
        <Loader2 className='h-6 w-6 animate-spin' />
        <span className='text-muted-foreground ml-2'>{t('common.loading')}</span>
      </div>
    );
  }

  if (viewState === 'loadFailed') {
    return (
      <Alert variant='destructive'>
        <AlertCircle />
        <AlertDescription>{t('system.codex.loadFailed')}</AlertDescription>
      </Alert>
    );
  }

  const controls = codexFormControls({ access, form, saved, saving: update.isPending, test: test.state });
  const missingWrite = !hasSystemScope('write_settings');
  const missingChannels = !hasSystemScope('read_channels');
  const listedSelection = form.channelID !== null && channels?.some((channel) => channel.id === form.channelID);

  return (
    <Card data-testid='codex-compat-card'>
      <CardHeader>
        <CardTitle>{t('system.codex.title')}</CardTitle>
        <CardDescription>{t('system.codex.description')}</CardDescription>
      </CardHeader>
      <CardContent className='space-y-6'>
        {!access.canEdit && (
          <Alert data-testid='codex-compat-permission-notice'>
            <Info />
            <AlertDescription>
              {missingWrite && <p>{t('system.codex.permissions.write', { scope: t('scopes.write_settings') })}</p>}
              {missingChannels && <p>{t('system.codex.permissions.channels', { scope: t('scopes.read_channels') })}</p>}
            </AlertDescription>
          </Alert>
        )}

        <div className='flex items-center justify-between gap-4 rounded-lg border p-4'>
          <div className='space-y-1'>
            <Label htmlFor='codex-compat-enabled'>{t('system.codex.enabled.label')}</Label>
            <div className='text-muted-foreground text-sm'>{t('system.codex.enabled.description')}</div>
          </div>
          <Switch
            id='codex-compat-enabled'
            data-testid='codex-compat-enabled-switch'
            checked={form.enabled}
            disabled={!access.canEdit || update.isPending}
            onCheckedChange={(enabled) => setDraft((prev) => ({ ...prev, enabled }))}
          />
        </div>

        <div className='space-y-2'>
          <Label htmlFor='codex-compat-channel'>{t('system.codex.channel.label')}</Label>
          <Select
            value={channelSelectValue(form.channelID)}
            disabled={!access.canEdit || update.isPending}
            onValueChange={(value) => setDraft((prev) => ({ ...prev, channelID: parseChannelSelectValue(value) }))}
          >
            <SelectTrigger id='codex-compat-channel' className='w-full sm:max-w-md' data-testid='codex-compat-channel-select'>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value={NO_CODEX_CHANNEL_VALUE} data-testid='codex-compat-channel-option-none'>
                {t('system.codex.channel.none')}
              </SelectItem>
              {form.channelID !== null && !listedSelection && (
                <SelectItem value={channelSelectValue(form.channelID)}>
                  {t('system.codex.channel.unknown', { id: form.channelID })}
                </SelectItem>
              )}
              {channels?.map((channel) => (
                <SelectItem
                  key={channel.id}
                  value={channelSelectValue(channel.id)}
                  data-testid={`codex-compat-channel-option-${channel.id}`}
                >
                  <span className='truncate'>{channel.name}</span>
                  <Badge variant='outline' className={`h-5 px-1.5 text-[10px] font-normal ${STATUS_BADGE_CLASS[channel.status]}`}>
                    {t(`channels.status.${channel.status}`)}
                  </Badge>
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
          <div className='text-muted-foreground text-sm'>{t('system.codex.channel.description')}</div>
          {channels?.length === 0 && <div className='text-muted-foreground text-sm'>{t('system.codex.channel.empty')}</div>}
          {resolved.referenceMissing && (
            <p className='text-destructive text-sm' role='alert' data-testid='codex-compat-reference-missing'>
              {t('system.codex.channel.referenceMissing')}
            </p>
          )}
          {controls.issue === 'channel_required' && !resolved.referenceMissing && (
            <p className='text-destructive text-sm' role='alert' data-testid='codex-compat-issue'>
              {t('system.codex.validation.channelRequired')}
            </p>
          )}
        </div>

        <CodexCatalogTestPanel state={test.state} canTest={controls.canTest} onTest={test.run} onCancel={test.cancel} />

        <div className='flex justify-end'>
          <Button
            type='button'
            className='min-w-[100px]'
            disabled={!controls.canSave}
            data-testid='codex-compat-save-button'
            onClick={() => update.mutate(toUpdateInput(form), { onSuccess: () => setDraft({}) })}
          >
            {update.isPending ? (
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
