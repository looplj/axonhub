import { useCallback, useEffect, useState, type Dispatch, type ReactNode, type SetStateAction } from 'react';
import { z } from 'zod';
import { useFieldArray, useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { IconPlus, IconTrash } from '@tabler/icons-react';
import { Info } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Form, FormControl, FormDescription, FormField, FormItem, FormLabel, FormMessage } from '@/components/ui/form';
import { Input } from '@/components/ui/input';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Textarea } from '@/components/ui/textarea';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import { type ApiKeyAutoDisableRuleFormValue } from '../data/auto-disable';
import { type APIKeyAutoDisableRule, apiKeyAutoDisableRuleFormSchema } from '../data/schema';

const formSchema = z.object({
  rules: z.array(apiKeyAutoDisableRuleFormSchema),
});

const PRESET_DISABLE_DURATIONS = [5, 15, 30, 60, 120, 360, 720, 1440];

const CRON_PRESETS = ['0 * * * *', '0 */6 * * *', '0 0 * * *', '0 0 * * 1', '0 0 1 * *'] as const;

const CRON_PRESET_LABEL_KEYS: Record<string, string> = {
  '0 * * * *': 'channels.dialogs.availability.cronPresets.hourly',
  '0 */6 * * *': 'channels.dialogs.availability.cronPresets.every6h',
  '0 0 * * *': 'channels.dialogs.availability.cronPresets.daily',
  '0 0 * * 1': 'channels.dialogs.availability.cronPresets.weekly',
  '0 0 1 * *': 'channels.dialogs.availability.cronPresets.monthly',
};

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

function zoneOffsetLabel(timeZone: string): string {
  try {
    const parts = new Intl.DateTimeFormat('en-US', { timeZone, timeZoneName: 'longOffset' }).formatToParts(new Date());
    return parts.find((part) => part.type === 'timeZoneName')?.value ?? '';
  } catch {
    return '';
  }
}

function HelpTooltip({ children }: { children: ReactNode }) {
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

function actionLabelKey(action: string): string {
  switch (action) {
    case 'temporary_disable':
      return 'channels.dialogs.apiKeyRules.actions.temporaryDisable';
    case 'disable_until_cron':
      return 'channels.dialogs.apiKeyRules.actions.disableUntilCron';
    case 'permanent_disable':
      return 'channels.dialogs.apiKeyRules.actions.permanentDisable';
    case 'permanent_disable_delete':
      return 'channels.dialogs.apiKeyRules.actions.permanentDelete';
    default:
      return 'channels.dialogs.apiKeyRules.fields.action';
  }
}

interface EditorProps {
  rules: ApiKeyAutoDisableRuleFormValue[];
  onChange: (rules: ApiKeyAutoDisableRuleFormValue[]) => void;
  allowDelete?: boolean;
}

export function ApiKeyAutoDisableRulesEditor({ rules, onChange, allowDelete = true }: EditorProps) {
  const { t } = useTranslation();
  const [customDurationModes, setCustomDurationModes] = useState<Record<string, true>>({});
  const [customCronModes, setCustomCronModes] = useState<Record<string, true>>({});

  const form = useForm<z.infer<typeof formSchema>>({
    resolver: zodResolver(formSchema),
    values: { rules },
  });

  const {
    fields: ruleFields,
    append: appendRule,
    remove: removeRule,
  } = useFieldArray({
    control: form.control,
    name: 'rules',
  });

  useEffect(() => {
    const subscription = form.watch((value) => {
      onChange((value.rules ?? []) as ApiKeyAutoDisableRuleFormValue[]);
    });
    return () => subscription.unsubscribe();
  }, [form, onChange]);

  const setModeFlag = useCallback((setter: Dispatch<SetStateAction<Record<string, true>>>, fieldID: string, enabled: boolean) => {
    setter((previous) => {
      if (enabled) return previous[fieldID] ? previous : { ...previous, [fieldID]: true };
      if (!previous[fieldID]) return previous;
      const next = { ...previous };
      delete next[fieldID];
      return next;
    });
  }, []);

  return (
    <Form {...form}>
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
          const timezoneChoices = TIMEZONE_OPTIONS.includes(timezone as (typeof TIMEZONE_OPTIONS)[number])
            ? [...TIMEZONE_OPTIONS]
            : [timezone, ...TIMEZONE_OPTIONS];
          const cron = form.watch(`rules.${index}.disableUntilCron`) ?? '';
          const customCron = !!customCronModes[field.id] || (cron !== '' && !CRON_PRESETS.includes(cron as (typeof CRON_PRESETS)[number]));
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
                          <SelectItem value='temporary_disable'>{t('channels.dialogs.apiKeyRules.actions.temporaryDisable')}</SelectItem>
                          <SelectItem value='disable_until_cron'>{t('channels.dialogs.apiKeyRules.actions.disableUntilCron')}</SelectItem>
                          <SelectItem value='permanent_disable'>{t('channels.dialogs.apiKeyRules.actions.permanentDisable')}</SelectItem>
                          {allowDelete && (
                            <SelectItem value='permanent_disable_delete'>
                              {t('channels.dialogs.apiKeyRules.actions.permanentDelete')}
                            </SelectItem>
                          )}
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
                              <SelectItem value='custom'>{t('channels.dialogs.apiKeyRules.fields.disableDurationCustom')}</SelectItem>
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
    </Form>
  );
}

interface PreviewProps {
  rules?: APIKeyAutoDisableRule[] | null;
  emptyLabel?: string;
  inactiveHint?: string;
}

export function ApiKeyAutoDisableRulesPreview({ rules, emptyLabel, inactiveHint }: PreviewProps) {
  const { t } = useTranslation();
  if (!rules || rules.length === 0) {
    return <div className='text-muted-foreground rounded-md border border-dashed p-4 text-sm'>{emptyLabel}</div>;
  }

  return (
    <div className='space-y-2'>
      {inactiveHint && <p className='text-muted-foreground text-sm'>{inactiveHint}</p>}
      {rules.map((rule, index) => (
        <div key={index} className='rounded-md border p-3 text-sm'>
          <div className='mb-1 font-medium'>{t('channels.dialogs.apiKeyRules.ruleLabel', { index: index + 1 })}</div>
          <div className='text-muted-foreground space-y-0.5'>
            <div>
              {t('channels.dialogs.apiKeyRules.fields.statusCodes')}: {rule.statusCodes?.length ? rule.statusCodes.join(', ') : '—'}
            </div>
            <div>
              {t('channels.dialogs.apiKeyRules.fields.times')}: {rule.times}
            </div>
            <div>
              {t('channels.dialogs.apiKeyRules.fields.action')}: {t(actionLabelKey(rule.action))}
            </div>
            {rule.keywordPatterns && rule.keywordPatterns.length > 0 && (
              <div>
                {t('channels.dialogs.apiKeyRules.fields.keywordPatterns')}: {rule.keywordPatterns.join(', ')}
              </div>
            )}
          </div>
        </div>
      ))}
    </div>
  );
}
