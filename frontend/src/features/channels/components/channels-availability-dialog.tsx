import { useCallback, useEffect, useState } from 'react';
import { Link } from '@tanstack/react-router';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';
import { Button } from '@/components/ui/button';
import { Checkbox } from '@/components/ui/checkbox';
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from '@/components/ui/collapsible';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { useRetryPolicy } from '@/features/system/data/system';
import {
  type ApiKeyAutoDisableRuleFormValue,
  type ScheduledDisableFormValue,
  availabilityPoliciesPayload,
  effectiveAutoDisableMode,
  serializeApiKeyAutoDisableRules,
  toApiKeyAutoDisableRuleFormValues,
  toScheduledDisableFormValue,
} from '../data/auto-disable';
import { useUpdateChannel } from '../data/channels';
import { APIKeyAutoDisableMode, Channel } from '../data/schema';
import {
  ApiKeyAutoDisableRulesEditor,
  ApiKeyAutoDisableRulesPreview,
  CRON_PRESET_LABEL_KEYS,
  CRON_PRESETS,
  TIMEZONE_OPTIONS,
  zoneOffsetLabel,
} from './api-key-auto-disable-rules-editor';

interface Props {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  currentRow: Channel;
}

export function ChannelsAvailabilityDialog({ open, onOpenChange, currentRow }: Props) {
  const { t } = useTranslation();
  const updateChannel = useUpdateChannel();
  const { data: retryPolicy } = useRetryPolicy();
  const currentPolicies = currentRow.policies ?? null;
  const [mode, setMode] = useState<APIKeyAutoDisableMode>(() => effectiveAutoDisableMode(currentRow.policies));
  const [rules, setRules] = useState<ApiKeyAutoDisableRuleFormValue[]>(() =>
    toApiKeyAutoDisableRuleFormValues(currentPolicies?.apiKeyAutoDisableRules)
  );
  const [schedule, setSchedule] = useState<ScheduledDisableFormValue>(() =>
    toScheduledDisableFormValue(currentRow.policies?.scheduledDisable)
  );
  const [customDisableCron, setCustomDisableCron] = useState(
    () => !CRON_PRESETS.includes(schedule.disableCron as (typeof CRON_PRESETS)[number])
  );
  const [reenableEnabled, setReenableEnabled] = useState(() => schedule.reenableCron !== '');
  const [customReenableCron, setCustomReenableCron] = useState(
    () => schedule.reenableCron !== '' && !CRON_PRESETS.includes(schedule.reenableCron as (typeof CRON_PRESETS)[number])
  );

  useEffect(() => {
    if (open) {
      setMode(effectiveAutoDisableMode(currentRow.policies));
      setRules(toApiKeyAutoDisableRuleFormValues(currentRow.policies?.apiKeyAutoDisableRules));
      const next = toScheduledDisableFormValue(currentRow.policies?.scheduledDisable);
      setSchedule(next);
      setCustomDisableCron(!CRON_PRESETS.includes(next.disableCron as (typeof CRON_PRESETS)[number]));
      setReenableEnabled(next.reenableCron !== '');
      setCustomReenableCron(next.reenableCron !== '' && !CRON_PRESETS.includes(next.reenableCron as (typeof CRON_PRESETS)[number]));
    }
  }, [currentRow, open]);

  const handleRulesChange = useCallback((next: ApiKeyAutoDisableRuleFormValue[]) => {
    setRules((prev) => (JSON.stringify(prev) === JSON.stringify(next) ? prev : next));
  }, []);

  const globalRules = retryPolicy?.autoDisableChannel?.rules ?? [];
  const globalEnabled = retryPolicy?.autoDisableChannel?.enabled ?? false;

  const onSubmit = useCallback(async () => {
    const payload = availabilityPoliciesPayload({
      stream: currentPolicies?.stream,
      mode,
      rules,
      scheduledDisable: {
        ...schedule,
        reenableCron: reenableEnabled ? schedule.reenableCron : '',
        reenableTimezone: reenableEnabled ? schedule.reenableTimezone : '',
      },
    });
    if (payload.emptiedCustom) {
      toast.error(t('channels.dialogs.availability.custom.emptyPrompt'));
    }

    try {
      await updateChannel.mutateAsync({
        id: currentRow.id,
        input: {
          policies: {
            stream: payload.stream,
            apiKeyAutoDisableMode: payload.apiKeyAutoDisableMode,
            apiKeyAutoDisableRules: payload.apiKeyAutoDisableRules,
            scheduledDisable: payload.scheduledDisable,
          },
        },
      });
      toast.success(t('channels.messages.updateSuccess'));
      onOpenChange(false);
    } catch {
      // useUpdateChannel reports the request error.
    }
  }, [currentPolicies?.stream, currentRow.id, mode, onOpenChange, reenableEnabled, rules, schedule, t, updateChannel]);

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className='max-h-[85vh] overflow-y-auto sm:max-w-3xl'>
        <DialogHeader className='text-left'>
          <DialogTitle>{t('channels.dialogs.availability.title')}</DialogTitle>
          <DialogDescription>{t('channels.dialogs.availability.description', { name: currentRow.name })}</DialogDescription>
        </DialogHeader>

        <div className='bg-muted/50 text-muted-foreground rounded-md border p-3 text-sm'>{t('channels.dialogs.availability.scope')}</div>

        <RadioGroup value={mode} onValueChange={(value) => setMode(value as APIKeyAutoDisableMode)} className='gap-2'>
          {(['inherit', 'custom', 'off'] as const).map((value) => (
            <div key={value} className='flex items-start gap-2 rounded-md border p-3'>
              <RadioGroupItem value={value} id={`auto-disable-mode-${value}`} className='mt-1' />
              <Label htmlFor={`auto-disable-mode-${value}`} className='flex flex-1 cursor-pointer flex-col gap-0.5 font-normal'>
                <span className='font-medium'>{t(`channels.dialogs.availability.modes.${value}`)}</span>
                <span className='text-muted-foreground text-sm'>{t(`channels.dialogs.availability.modes.${value}Description`)}</span>
              </Label>
            </div>
          ))}
        </RadioGroup>

        <div className='space-y-4 rounded-md border p-4'>
          <Label className='text-sm font-medium'>{t('channels.dialogs.availability.scheduled.title')}</Label>

          <div className='space-y-2'>
            <Label className='text-sm font-normal'>{t('channels.dialogs.availability.scheduled.disableCron.label')}</Label>
            <div className='flex gap-2'>
              <Select
                value={customDisableCron ? 'custom' : (CRON_PRESETS.includes(schedule.disableCron as (typeof CRON_PRESETS)[number]) ? schedule.disableCron : 'custom')}
                onValueChange={(value) => {
                  if (value === 'custom') {
                    setCustomDisableCron(true);
                  } else {
                    setCustomDisableCron(false);
                    setSchedule((prev) => ({ ...prev, disableCron: value }));
                  }
                }}
              >
                <SelectTrigger className={customDisableCron ? 'w-1/2' : undefined}>
                  <SelectValue placeholder={t('channels.dialogs.availability.scheduled.disableCron.placeholder')} />
                </SelectTrigger>
                <SelectContent>
                  {CRON_PRESETS.map((expr) => (
                    <SelectItem key={expr} value={expr}>
                      {t(CRON_PRESET_LABEL_KEYS[expr])}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
              {customDisableCron && (
                <Input
                  data-testid='availability-scheduled-disable-cron'
                  className='w-1/2 font-mono'
                  aria-label={t('channels.dialogs.availability.scheduled.disableCron.label')}
                  value={schedule.disableCron}
                  placeholder={t('channels.dialogs.availability.scheduled.disableCron.placeholder')}
                  onChange={(event) => setSchedule((prev) => ({ ...prev, disableCron: event.target.value }))}
                />
              )}
            </div>
            <Select value={schedule.disableTimezone || 'UTC'} onValueChange={(value) => setSchedule((prev) => ({ ...prev, disableTimezone: value }))}>
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {TIMEZONE_OPTIONS.map((zone) => {
                  const offset = zoneOffsetLabel(zone);
                  return (
                    <SelectItem key={zone} value={zone}>
                      {offset ? `${zone} (${offset})` : zone}
                    </SelectItem>
                  );
                })}
              </SelectContent>
            </Select>
            <p className='text-muted-foreground text-xs'>{t('channels.dialogs.availability.scheduled.disableCron.hint')}</p>
          </div>

          <Label className='flex items-center gap-2 text-sm font-normal'>
            <Checkbox checked={reenableEnabled} onCheckedChange={(checked) => setReenableEnabled(checked === true)} />
            {t('channels.dialogs.availability.scheduled.reenableToggle')}
          </Label>

          {reenableEnabled && (
            <div className='space-y-2'>
              <Label className='text-sm font-normal'>{t('channels.dialogs.availability.scheduled.reenable.label')}</Label>
              <div className='flex gap-2'>
                <Select
                  value={customReenableCron ? 'custom' : (CRON_PRESETS.includes(schedule.reenableCron as (typeof CRON_PRESETS)[number]) ? schedule.reenableCron : 'custom')}
                  onValueChange={(value) => {
                    if (value === 'custom') {
                      setCustomReenableCron(true);
                    } else {
                      setCustomReenableCron(false);
                      setSchedule((prev) => ({ ...prev, reenableCron: value }));
                    }
                  }}
                >
                  <SelectTrigger className={customReenableCron ? 'w-1/2' : undefined}>
                    <SelectValue placeholder={t('channels.dialogs.availability.scheduled.reenable.label')} />
                  </SelectTrigger>
                  <SelectContent>
                    {CRON_PRESETS.map((expr) => (
                      <SelectItem key={expr} value={expr}>
                        {t(CRON_PRESET_LABEL_KEYS[expr])}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
                {customReenableCron && (
                  <Input
                    className='w-1/2 font-mono'
                    aria-label={t('channels.dialogs.availability.scheduled.reenable.label')}
                    value={schedule.reenableCron}
                    placeholder='0 0 1 * *'
                    onChange={(event) => setSchedule((prev) => ({ ...prev, reenableCron: event.target.value }))}
                  />
                )}
              </div>
              <Select value={schedule.reenableTimezone || 'UTC'} onValueChange={(value) => setSchedule((prev) => ({ ...prev, reenableTimezone: value }))}>
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {TIMEZONE_OPTIONS.map((zone) => {
                    const offset = zoneOffsetLabel(zone);
                    return (
                      <SelectItem key={zone} value={zone}>
                        {offset ? `${zone} (${offset})` : zone}
                      </SelectItem>
                    );
                  })}
                </SelectContent>
              </Select>
              <p className='text-muted-foreground text-xs'>{t('channels.dialogs.availability.scheduled.reenable.hint')}</p>
            </div>
          )}
        </div>

        {mode === 'inherit' && (
          <div className='space-y-3'>
            {!globalEnabled && <p className='text-muted-foreground text-sm'>{t('channels.dialogs.availability.inherit.globalDisabled')}</p>}
            <ApiKeyAutoDisableRulesPreview rules={globalRules} emptyLabel={t('channels.dialogs.availability.inherit.emptyGlobal')} />
            <Button variant='link' className='h-auto px-0' asChild>
              <Link to='/system/$section' params={{ section: 'requests' }} search={{ tab: 'retry' }}>
                {t('channels.dialogs.availability.inherit.goToSettings')}
              </Link>
            </Button>
          </div>
        )}

        {mode === 'custom' && (
          <div className='space-y-4'>
            <ApiKeyAutoDisableRulesEditor rules={rules} onChange={handleRulesChange} allowDelete />
            <Collapsible>
              <CollapsibleTrigger className='text-sm underline-offset-4 hover:underline'>
                {t('channels.dialogs.availability.custom.fallbackTitle')}
              </CollapsibleTrigger>
              <CollapsibleContent className='mt-3'>
                <ApiKeyAutoDisableRulesPreview rules={globalRules} emptyLabel={t('channels.dialogs.availability.inherit.emptyGlobal')} />
              </CollapsibleContent>
            </Collapsible>
          </div>
        )}

        {mode === 'off' && (
          <div className='space-y-3'>
            <p className='text-muted-foreground text-sm'>{t('channels.dialogs.availability.off.description')}</p>
            <ApiKeyAutoDisableRulesPreview
              rules={serializeApiKeyAutoDisableRules(rules)}
              emptyLabel={t('channels.dialogs.availability.off.emptySaved')}
              inactiveHint={t('channels.dialogs.availability.off.savedRulesHint')}
            />
          </div>
        )}

        <DialogFooter>
          <Button type='button' variant='outline' onClick={() => onOpenChange(false)}>
            {t('common.buttons.cancel')}
          </Button>
          <Button type='button' disabled={updateChannel.isPending} onClick={onSubmit}>
            {updateChannel.isPending ? t('common.buttons.saving') : t('common.buttons.save')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
