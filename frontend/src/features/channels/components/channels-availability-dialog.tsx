import { useCallback, useEffect, useState } from 'react';
import { Link } from '@tanstack/react-router';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';
import { cn } from '@/lib/utils';
import { Button } from '@/components/ui/button';
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from '@/components/ui/collapsible';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
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

type CronPickerValue = '' | 'none' | 'opposite' | (typeof CRON_PRESETS)[number] | 'custom';

interface CronPickerProps {
  /** '' (unconfigured), a preset expression, 'custom', or a component-specific option value (e.g. 'none', 'opposite'). */
  selection: CronPickerValue;
  customValue: string;
  onSelectionChange: (value: CronPickerValue) => void;
  onCustomValueChange: (value: string) => void;
  placeholder: string;
  ariaLabel: string;
  testId?: string;
  /** Extra options rendered before the presets, e.g. the "opposite" re-enable. */
  extraOptions?: { value: string; label: string }[];
}

function CronPicker({
  selection,
  customValue,
  onSelectionChange,
  onCustomValueChange,
  placeholder,
  ariaLabel,
  testId,
  extraOptions = [],
}: CronPickerProps) {
  const { t } = useTranslation();
  const isCustom = selection === 'custom';

  return (
    <div className='space-y-2'>
      <div className='flex gap-2'>
        {/* The dropdown stays visible in custom mode (pinned to the custom
            item) so a preset, "none" or "opposite" can be picked without
            clearing the custom cron first. */}
        <Select value={isCustom ? 'custom' : selection} onValueChange={(value) => onSelectionChange(value as CronPickerValue)}>
          <SelectTrigger className={cn('shrink-0', isCustom && 'w-28')}>
            <SelectValue placeholder={placeholder} />
          </SelectTrigger>
          <SelectContent>
            {extraOptions.map(({ value, label }) => (
              <SelectItem key={value} value={value}>
                {label}
              </SelectItem>
            ))}
            {CRON_PRESETS.map((expr) => (
              <SelectItem key={expr} value={expr}>
                {t(CRON_PRESET_LABEL_KEYS[expr])}
              </SelectItem>
            ))}
            <SelectItem value='custom'>{t('channels.dialogs.apiKeyRules.fields.cronCustom')}</SelectItem>
          </SelectContent>
        </Select>
        {isCustom && (
          <Input
            data-testid={testId}
            className='min-w-0 flex-1 font-mono'
            aria-label={ariaLabel}
            value={customValue}
            placeholder={placeholder}
            onChange={(event) => onCustomValueChange(event.target.value)}
          />
        )}
      </div>
    </div>
  );
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
    () => schedule.disableCron !== '' && !CRON_PRESETS.includes(schedule.disableCron as (typeof CRON_PRESETS)[number])
  );
  const [customReenableCron, setCustomReenableCron] = useState(
    () =>
      schedule.reenableMode === 'cron' &&
      schedule.reenableCron !== '' &&
      !CRON_PRESETS.includes(schedule.reenableCron as (typeof CRON_PRESETS)[number])
  );

  useEffect(() => {
    if (open) {
      setMode(effectiveAutoDisableMode(currentRow.policies));
      setRules(toApiKeyAutoDisableRuleFormValues(currentRow.policies?.apiKeyAutoDisableRules));
      const next = toScheduledDisableFormValue(currentRow.policies?.scheduledDisable);
      setSchedule(next);
      setCustomDisableCron(next.disableCron !== '' && !CRON_PRESETS.includes(next.disableCron as (typeof CRON_PRESETS)[number]));
      setCustomReenableCron(
        next.reenableMode === 'cron' &&
          next.reenableCron !== '' &&
          !CRON_PRESETS.includes(next.reenableCron as (typeof CRON_PRESETS)[number])
      );
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
      // An empty custom re-enable cron is not a schedule: normalize it to
      // "none" so the payload never sends cron mode without a cron.
      scheduledDisable: {
        ...schedule,
        reenableMode: schedule.reenableMode === 'cron' && schedule.reenableCron.trim() === '' ? 'none' : schedule.reenableMode,
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
  }, [currentPolicies?.stream, currentRow.id, mode, onOpenChange, rules, schedule, t, updateChannel]);

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
            <CronPicker
              selection={
                customDisableCron
                  ? 'custom'
                  : CRON_PRESETS.includes(schedule.disableCron as (typeof CRON_PRESETS)[number])
                    ? (schedule.disableCron as CronPickerValue)
                    : ''
              }
              customValue={schedule.disableCron}
              onSelectionChange={(value) => {
                if (value === 'custom') {
                  setCustomDisableCron(true);
                } else if (value === '') {
                  setCustomDisableCron(false);
                  setSchedule((prev) => ({ ...prev, disableCron: '' }));
                } else {
                  setCustomDisableCron(false);
                  setSchedule((prev) => ({ ...prev, disableCron: value }));
                }
              }}
              onCustomValueChange={(value) => setSchedule((prev) => ({ ...prev, disableCron: value }))}
              placeholder={t('channels.dialogs.availability.scheduled.disableCron.placeholder')}
              ariaLabel={t('channels.dialogs.availability.scheduled.disableCron.label')}
              testId='availability-scheduled-disable-cron'
            />
            <Select
              value={schedule.disableTimezone || 'UTC'}
              onValueChange={(value) => setSchedule((prev) => ({ ...prev, disableTimezone: value }))}
            >
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

          <div className='space-y-2'>
            <Label className='text-sm font-normal'>{t('channels.dialogs.availability.scheduled.reenable.label')}</Label>
            <CronPicker
              selection={
                schedule.reenableMode === 'opposite'
                  ? 'opposite'
                  : schedule.reenableMode === 'cron' && customReenableCron
                    ? 'custom'
                    : schedule.reenableMode === 'cron' && CRON_PRESETS.includes(schedule.reenableCron as (typeof CRON_PRESETS)[number])
                      ? (schedule.reenableCron as CronPickerValue)
                      : 'none'
              }
              customValue={schedule.reenableCron}
              onSelectionChange={(value) => {
                if (value === 'opposite') {
                  setCustomReenableCron(false);
                  setSchedule((prev) => ({ ...prev, reenableMode: 'opposite', reenableCron: '', reenableTimezone: 'UTC' }));
                } else if (value === 'custom') {
                  setCustomReenableCron(true);
                  setSchedule((prev) => ({ ...prev, reenableMode: 'cron' }));
                } else if (value === 'none') {
                  setCustomReenableCron(false);
                  setSchedule((prev) => ({ ...prev, reenableMode: 'none', reenableCron: '', reenableTimezone: 'UTC' }));
                } else {
                  setCustomReenableCron(false);
                  setSchedule((prev) => ({ ...prev, reenableMode: 'cron', reenableCron: value }));
                }
              }}
              onCustomValueChange={(value) => setSchedule((prev) => ({ ...prev, reenableMode: 'cron', reenableCron: value }))}
              placeholder={t('channels.dialogs.availability.scheduled.reenable.placeholder')}
              ariaLabel={t('channels.dialogs.availability.scheduled.reenable.label')}
              testId='availability-scheduled-reenable-cron'
              extraOptions={[
                { value: 'none', label: t('channels.dialogs.availability.scheduled.reenableNone') },
                { value: 'opposite', label: t('channels.dialogs.availability.scheduled.reenableOpposite') },
              ]}
            />
            {schedule.reenableMode === 'cron' && (
              <Select
                value={schedule.reenableTimezone || 'UTC'}
                onValueChange={(value) => setSchedule((prev) => ({ ...prev, reenableTimezone: value }))}
              >
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
            )}
            <p className='text-muted-foreground text-xs'>
              {t(
                schedule.reenableMode === 'opposite'
                  ? 'channels.dialogs.availability.scheduled.reenableOppositeHint'
                  : 'channels.dialogs.availability.scheduled.reenable.hint'
              )}
            </p>
          </div>
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
