import { useCallback, useEffect, useState } from 'react';
import { z } from 'zod';
import { useFieldArray, useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { IconPlus, IconTrash } from '@tabler/icons-react';
import { Info } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Form, FormControl, FormDescription, FormField, FormItem, FormLabel, FormMessage } from '@/components/ui/form';
import { Input } from '@/components/ui/input';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Textarea } from '@/components/ui/textarea';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import { useUpdateChannel } from '../data/channels';
import { apiKeyAutoDisableRuleFormSchema, Channel } from '../data/schema';

interface Props {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  currentRow: Channel;
}

const formSchema = z.object({
  rules: z.array(apiKeyAutoDisableRuleFormSchema),
});

type FormValues = z.infer<typeof formSchema>;

const PRESET_DISABLE_DURATIONS = [5, 15, 30, 60, 120, 360, 720, 1440];

// Presets cover the recovery points operators actually reach for; each maps to
// one standard 5-field crontab expression.
const CRON_PRESETS = ['0 * * * *', '0 */6 * * *', '0 0 * * *', '0 0 * * 1', '0 0 1 * *'] as const;

const CRON_PRESET_LABEL_KEYS: Record<string, string> = {
  '0 * * * *': 'channels.dialogs.availability.cronPresets.hourly',
  '0 */6 * * *': 'channels.dialogs.availability.cronPresets.every6h',
  '0 0 * * *': 'channels.dialogs.availability.cronPresets.daily',
  '0 0 * * 1': 'channels.dialogs.availability.cronPresets.weekly',
  '0 0 1 * *': 'channels.dialogs.availability.cronPresets.monthly',
};

// One representative zone per region an operator is likely to bill against.
// Kept short on purpose: the value only shifts when a crontab expression is
// interpreted, so the exact city matters less than the offset it implies.
const TIMEZONE_OPTIONS = [
  'UTC',
  'Asia/Shanghai',
  'Asia/Hong_Kong',
  'Asia/Tokyo',
  'Asia/Seoul',
  'Asia/Singapore',
  'Asia/Kolkata',
  'Asia/Dubai',
  'Europe/London',
  'Europe/Paris',
  'Europe/Moscow',
  'America/Sao_Paulo',
  'America/New_York',
  'America/Chicago',
  'America/Denver',
  'America/Los_Angeles',
  'Australia/Sydney',
  'Pacific/Auckland',
] as const;

// Resolved at render rather than hardcoded so the label stays correct across
// daylight saving transitions.
function zoneOffsetLabel(timeZone: string): string {
  try {
    const parts = new Intl.DateTimeFormat('en-US', { timeZone, timeZoneName: 'longOffset' }).formatToParts(new Date());
    return parts.find((part) => part.type === 'timeZoneName')?.value ?? '';
  } catch {
    return '';
  }
}

function HelpTooltip({ children }: { children: React.ReactNode }) {
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <button type='button' className='text-muted-foreground hover:text-foreground inline-flex' aria-hidden='false'>
          <Info className='h-3.5 w-3.5' />
        </button>
      </TooltipTrigger>
      <TooltipContent className='max-w-sm'>{children}</TooltipContent>
    </Tooltip>
  );
}

export function ChannelsAvailabilityDialog({ open, onOpenChange, currentRow }: Props) {
  const { t } = useTranslation();
  const updateChannel = useUpdateChannel();
  const currentPolicies = currentRow.policies ?? null;
  const [customDurationModes, setCustomDurationModes] = useState<Record<string, true>>({});
  const [customCronModes, setCustomCronModes] = useState<Record<string, true>>({});

  const toFormValues = useCallback((): FormValues => {
    return {
      rules:
        currentPolicies?.apiKeyAutoDisableRules?.map((rule) => ({
          statusCodes: rule.statusCodes ?? [],
          keywordPatterns: rule.keywordPatterns ?? [],
          times: rule.times,
          action: rule.action,
          disableDurationMinutes: rule.action === 'temporary_disable' ? (rule.disableDurationMinutes ?? 30) : null,
          disableUntilCron: rule.action === 'disable_until_cron' ? (rule.disableUntilCron ?? '0 0 * * *') : null,
          disableUntilTimezone: rule.action === 'disable_until_cron' ? (rule.disableUntilTimezone || 'UTC') : null,
        })) ?? [],
    };
  }, [currentPolicies?.apiKeyAutoDisableRules]);

  const form = useForm<FormValues>({
    resolver: zodResolver(formSchema),
    defaultValues: toFormValues(),
  });

  const { fields: ruleFields, append: appendRule, remove: removeRule } = useFieldArray({
    control: form.control,
    name: 'rules',
  });

  useEffect(() => {
    if (open) {
      setCustomDurationModes({});
      setCustomCronModes({});
      form.reset(toFormValues());
    }
  }, [form, open, toFormValues]);

  const setModeFlag = useCallback(
    (setter: React.Dispatch<React.SetStateAction<Record<string, true>>>, fieldID: string, enabled: boolean) => {
      setter((previous) => {
        if (enabled) return previous[fieldID] ? previous : { ...previous, [fieldID]: true };
        if (!previous[fieldID]) return previous;
        const next = { ...previous };
        delete next[fieldID];
        return next;
      });
    },
    []
  );

  const onSubmit = useCallback(
    async (values: FormValues) => {
      // Each action owns one schedule field; the rest are sent as null so a rule
      // switched between actions cannot carry stale settings to the backend.
      const rules = values.rules.map((rule) => ({
        statusCodes: rule.statusCodes?.filter((code): code is number => code != null) ?? [],
        keywordPatterns: rule.keywordPatterns?.map((pattern) => pattern.trim()).filter(Boolean) ?? [],
        times: rule.times,
        action: rule.action,
        disableDurationMinutes: rule.action === 'temporary_disable' ? (rule.disableDurationMinutes ?? null) : null,
        disableUntilCron: rule.action === 'disable_until_cron' ? (rule.disableUntilCron?.trim() || null) : null,
        disableUntilTimezone: rule.action === 'disable_until_cron' ? (rule.disableUntilTimezone?.trim() || null) : null,
      }));

      try {
        await updateChannel.mutateAsync({
          id: currentRow.id,
          input: {
            // policies is replaced as a whole, so unrelated sections must be echoed back.
            policies: {
              stream: currentPolicies?.stream,
              apiKeyAutoDisableRules: rules.length > 0 ? rules : null,
            },
          },
        });
        toast.success(t('channels.messages.updateSuccess'));
        onOpenChange(false);
      } catch {
        // useUpdateChannel reports the request error.
      }
    },
    [currentPolicies?.stream, currentRow.id, onOpenChange, t, updateChannel]
  );

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className='max-h-[85vh] overflow-y-auto sm:max-w-3xl'>
        <DialogHeader className='text-left'>
          <DialogTitle>{t('channels.dialogs.availability.title')}</DialogTitle>
          <DialogDescription>{t('channels.dialogs.availability.description', { name: currentRow.name })}</DialogDescription>
        </DialogHeader>

        <div className='bg-muted/50 text-muted-foreground rounded-md border p-3 text-sm'>
          {t('channels.dialogs.availability.scope')}
        </div>

        <Form {...form}>
          <form className='space-y-4' onSubmit={form.handleSubmit(onSubmit)}>
            <div className='space-y-4'>
                {ruleFields.length === 0 && (
                  <div className='text-muted-foreground rounded-md border border-dashed p-6 text-center text-sm'>
                    {t('channels.dialogs.apiKeyRules.empty')}
                  </div>
                )}

                {ruleFields.map((field, index) => {
                  const action = form.watch(`rules.${index}.action`);

                  const duration = form.watch(`rules.${index}.disableDurationMinutes`);
                  const customDuration = !!customDurationModes[field.id] || (duration != null && !PRESET_DISABLE_DURATIONS.includes(duration));
                  const durationValue = customDuration ? 'custom' : String(duration ?? 30);

                  const timezone = form.watch(`rules.${index}.disableUntilTimezone`) || 'UTC';
                  // A value set through the API may not be in the shortlist; keep it
                  // selectable so opening the dialog cannot silently rewrite it.
                  const timezoneChoices = TIMEZONE_OPTIONS.includes(timezone as (typeof TIMEZONE_OPTIONS)[number])
                    ? [...TIMEZONE_OPTIONS]
                    : [timezone, ...TIMEZONE_OPTIONS];

                  const cron = form.watch(`rules.${index}.disableUntilCron`) ?? '';
                  const customCron =
                    !!customCronModes[field.id] || (cron !== '' && !CRON_PRESETS.includes(cron as (typeof CRON_PRESETS)[number]));
                  const cronValue = customCron ? 'custom' : cron;

                  return (
                    <div key={field.id} className='space-y-3 rounded-md border p-4'>
                      <div className='flex items-center justify-between'>
                        <Badge variant='outline'>{t('channels.dialogs.apiKeyRules.ruleLabel', { index: index + 1 })}</Badge>
                        <Button
                          type='button'
                          variant='ghost'
                          size='icon'
                          aria-label={t('common.buttons.delete')}
                          onClick={() => {
                            removeRule(index);
                            setModeFlag(setCustomDurationModes, field.id, false);
                            setModeFlag(setCustomCronModes, field.id, false);
                          }}
                        >
                          <IconTrash className='h-4 w-4 text-red-500' />
                        </Button>
                      </div>

                      <div className='grid grid-cols-1 gap-3 sm:grid-cols-2'>
                        <FormField
                          control={form.control}
                          name={`rules.${index}.statusCodes`}
                          render={({ field: input }) => (
                            <FormItem>
                              <FormLabel>{t('channels.dialogs.apiKeyRules.fields.statusCodes')}</FormLabel>
                              <FormControl>
                                <Input
                                  key={`status-${field.id}`}
                                  defaultValue={input.value?.join(', ') ?? ''}
                                  placeholder={t('channels.dialogs.apiKeyRules.fields.statusCodesPlaceholder')}
                                  onBlur={(event) => {
                                    const codes = event.target.value
                                      .split(/[,\s]+/)
                                      .map((value) => Number.parseInt(value, 10))
                                      .filter((value) => Number.isInteger(value) && value >= 100 && value <= 599);
                                    input.onChange(codes);
                                  }}
                                />
                              </FormControl>
                              <FormMessage />
                            </FormItem>
                          )}
                        />

                        <FormField
                          control={form.control}
                          name={`rules.${index}.times`}
                          render={({ field: input }) => (
                            <FormItem>
                              <FormLabel>{t('channels.dialogs.apiKeyRules.fields.times')}</FormLabel>
                              <FormControl>
                                <Input
                                  type='number'
                                  min={1}
                                  value={input.value ?? ''}
                                  onChange={(event) => {
                                    const raw = event.target.value;
                                    input.onChange(raw === '' ? undefined : Number.parseInt(raw, 10));
                                  }}
                                />
                              </FormControl>
                              <FormMessage />
                            </FormItem>
                          )}
                        />
                      </div>

                      <FormField
                        control={form.control}
                        name={`rules.${index}.keywordPatterns`}
                        render={({ field: input }) => (
                          <FormItem>
                            <FormLabel>{t('channels.dialogs.apiKeyRules.fields.keywordPatterns')}</FormLabel>
                            <FormControl>
                              <Textarea
                                key={`patterns-${field.id}`}
                                className='min-h-24 font-mono text-sm'
                                defaultValue={input.value?.join('\n') ?? ''}
                                placeholder={t('channels.dialogs.apiKeyRules.fields.keywordPatternsPlaceholder')}
                                onBlur={(event) =>
                                  input.onChange(
                                    event.target.value
                                      .split(/\r?\n/)
                                      .map((value) => value.trim())
                                      .filter(Boolean)
                                  )
                                }
                              />
                            </FormControl>
                            <FormMessage />
                          </FormItem>
                        )}
                      />

                      <div className='grid grid-cols-1 gap-3 sm:grid-cols-2'>
                        <FormField
                          control={form.control}
                          name={`rules.${index}.action`}
                          render={({ field: input }) => (
                            <FormItem>
                              <FormLabel>{t('channels.dialogs.apiKeyRules.fields.action')}</FormLabel>
                              <Select
                                value={input.value}
                                onValueChange={(value) => {
                                  input.onChange(value);

                                  if (value === 'temporary_disable') {
                                    if (!form.getValues(`rules.${index}.disableDurationMinutes`)) {
                                      form.setValue(`rules.${index}.disableDurationMinutes`, 30);
                                    }
                                  } else {
                                    setModeFlag(setCustomDurationModes, field.id, false);
                                    form.setValue(`rules.${index}.disableDurationMinutes`, null);
                                  }

                                  if (value === 'disable_until_cron') {
                                    if (!form.getValues(`rules.${index}.disableUntilCron`)) {
                                      form.setValue(`rules.${index}.disableUntilCron`, '0 0 * * *');
                                    }
                                    if (!form.getValues(`rules.${index}.disableUntilTimezone`)) {
                                      form.setValue(`rules.${index}.disableUntilTimezone`, 'UTC');
                                    }
                                  } else {
                                    setModeFlag(setCustomCronModes, field.id, false);
                                    form.setValue(`rules.${index}.disableUntilCron`, null);
                                    form.setValue(`rules.${index}.disableUntilTimezone`, null);
                                  }
                                }}
                              >
                                <FormControl>
                                  <SelectTrigger>
                                    <SelectValue />
                                  </SelectTrigger>
                                </FormControl>
                                <SelectContent>
                                  <SelectItem value='temporary_disable'>
                                    {t('channels.dialogs.apiKeyRules.actions.temporaryDisable')}
                                  </SelectItem>
                                  <SelectItem value='disable_until_cron'>
                                    {t('channels.dialogs.apiKeyRules.actions.disableUntilCron')}
                                  </SelectItem>
                                  <SelectItem value='permanent_disable'>
                                    {t('channels.dialogs.apiKeyRules.actions.permanentDisable')}
                                  </SelectItem>
                                  <SelectItem value='permanent_disable_delete'>
                                    {t('channels.dialogs.apiKeyRules.actions.permanentDelete')}
                                  </SelectItem>
                                </SelectContent>
                              </Select>
                            </FormItem>
                          )}
                        />

                        {action === 'temporary_disable' && (
                          <FormField
                            control={form.control}
                            name={`rules.${index}.disableDurationMinutes`}
                            render={({ field: input }) => (
                              <FormItem>
                                <FormLabel>{t('channels.dialogs.apiKeyRules.fields.disableDuration')}</FormLabel>
                                <div className='flex gap-2'>
                                  <Select
                                    value={durationValue}
                                    onValueChange={(value) => {
                                      if (value === 'custom') {
                                        setModeFlag(setCustomDurationModes, field.id, true);
                                        if (!input.value) input.onChange(30);
                                      } else {
                                        setModeFlag(setCustomDurationModes, field.id, false);
                                        input.onChange(Number.parseInt(value, 10));
                                      }
                                    }}
                                  >
                                    <SelectTrigger className={customDuration ? 'w-1/2' : undefined}>
                                      <SelectValue />
                                    </SelectTrigger>
                                    <SelectContent>
                                      {PRESET_DISABLE_DURATIONS.map((minutes) => (
                                        <SelectItem key={minutes} value={String(minutes)}>
                                          {t(`channels.dialogs.apiKeyRules.durations.${minutes}`)}
                                        </SelectItem>
                                      ))}
                                      <SelectItem value='custom'>
                                        {t('channels.dialogs.apiKeyRules.fields.disableDurationCustom')}
                                      </SelectItem>
                                    </SelectContent>
                                  </Select>
                                  {customDuration && (
                                    <Input
                                      type='number'
                                      min={1}
                                      className='w-1/2'
                                      value={input.value ?? ''}
                                      placeholder={t('channels.dialogs.apiKeyRules.fields.disableDurationCustomPlaceholder')}
                                      onChange={(event) => {
                                        const value = Number.parseInt(event.target.value, 10);
                                        input.onChange(Number.isInteger(value) && value > 0 ? value : null);
                                      }}
                                    />
                                  )}
                                </div>
                                <FormMessage />
                              </FormItem>
                            )}
                          />
                        )}

                        {action === 'disable_until_cron' && (
                          <FormField
                            control={form.control}
                            name={`rules.${index}.disableUntilCron`}
                            render={({ field: input }) => (
                              <FormItem>
                                <FormLabel className='flex items-center gap-2'>
                                  {t('channels.dialogs.apiKeyRules.fields.disableUntilCron')}
                                  <HelpTooltip>
                                    <div className='space-y-1 text-xs'>
                                      <p>{t('channels.dialogs.apiKeyRules.fields.disableUntilCronHelp')}</p>
                                      <p className='font-mono'>{t('channels.dialogs.apiKeyRules.fields.cronFormat')}</p>
                                    </div>
                                  </HelpTooltip>
                                </FormLabel>
                                <div className='flex gap-2'>
                                  <Select
                                    value={cronValue}
                                    onValueChange={(value) => {
                                      if (value === 'custom') {
                                        setModeFlag(setCustomCronModes, field.id, true);
                                      } else {
                                        setModeFlag(setCustomCronModes, field.id, false);
                                        input.onChange(value);
                                      }
                                    }}
                                  >
                                    <SelectTrigger className={customCron ? 'w-1/2' : undefined}>
                                      <SelectValue placeholder={t('channels.dialogs.apiKeyRules.fields.disableUntilCronPlaceholder')} />
                                    </SelectTrigger>
                                    <SelectContent>
                                      {CRON_PRESETS.map((expr) => (
                                        <SelectItem key={expr} value={expr}>
                                          {t(CRON_PRESET_LABEL_KEYS[expr])}
                                        </SelectItem>
                                      ))}
                                      <SelectItem value='custom'>{t('channels.dialogs.apiKeyRules.fields.cronCustom')}</SelectItem>
                                    </SelectContent>
                                  </Select>
                                  {customCron && (
                                    <Input
                                      className='w-1/2 font-mono'
                                      value={input.value ?? ''}
                                      placeholder='0 0 * * *'
                                      onChange={(event) => input.onChange(event.target.value)}
                                    />
                                  )}
                                </div>
                                <FormMessage />
                              </FormItem>
                            )}
                          />
                        )}
                      </div>

                      {action === 'disable_until_cron' && (
                        <FormField
                          control={form.control}
                          name={`rules.${index}.disableUntilTimezone`}
                          render={({ field: input }) => (
                            <FormItem>
                              <FormLabel>{t('channels.dialogs.apiKeyRules.fields.timezone')}</FormLabel>
                              <Select value={input.value || 'UTC'} onValueChange={input.onChange}>
                                <FormControl>
                                  <SelectTrigger>
                                    <SelectValue />
                                  </SelectTrigger>
                                </FormControl>
                                <SelectContent>
                                  {timezoneChoices.map((zone) => {
                                    const offset = zoneOffsetLabel(zone);
                                    return (
                                      <SelectItem key={zone} value={zone}>
                                        {offset ? `${zone} (${offset})` : zone}
                                      </SelectItem>
                                    );
                                  })}
                                </SelectContent>
                              </Select>
                              <FormDescription>{t('channels.dialogs.apiKeyRules.fields.timezoneHint')}</FormDescription>
                              <FormMessage />
                            </FormItem>
                          )}
                        />
                      )}
                    </div>
                  );
                })}

                <Button
                  type='button'
                  variant='outline'
                  className='w-full'
                  onClick={() =>
                    appendRule({
                      statusCodes: [],
                      keywordPatterns: [],
                      times: 3,
                      action: 'temporary_disable',
                      disableDurationMinutes: 30,
                      disableUntilCron: null,
                      disableUntilTimezone: null,
                    })
                  }
                >
                  <IconPlus className='mr-2 h-4 w-4' />
                  {t('channels.dialogs.apiKeyRules.addRule')}
                </Button>
            </div>

            <DialogFooter>
              <Button type='button' variant='outline' onClick={() => onOpenChange(false)}>
                {t('common.buttons.cancel')}
              </Button>
              <Button type='submit' disabled={updateChannel.isPending}>
                {updateChannel.isPending ? t('common.buttons.saving') : t('common.buttons.save')}
              </Button>
            </DialogFooter>
          </form>
        </Form>
      </DialogContent>
    </Dialog>
  );
}
