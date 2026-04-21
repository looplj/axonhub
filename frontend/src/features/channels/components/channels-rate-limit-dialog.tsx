'use client';

import { useEffect, useCallback, useRef } from 'react';
import { z } from 'zod';
import { useForm, useFieldArray } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { Plus, X, Info, RotateCcw } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';
import { Button } from '@/components/ui/button';
import { Card, CardAction, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Form, FormField, FormItem, FormLabel, FormMessage, FormControl, FormDescription } from '@/components/ui/form';
import { Input } from '@/components/ui/input';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Tooltip, TooltipTrigger, TooltipContent } from '@/components/ui/tooltip';
import { useGeneralSettings } from '@/features/system/data/system';
import { useUpdateChannel } from '../data/channels';
import { Channel } from '../data/schema';
import { mergeChannelSettingsForUpdate } from '../utils/merge';
import { utcToTzDatetime, tzDatetimeToUtc, getTzDateParts } from '../utils/timezone';

interface Props {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  currentRow: Channel;
}

const RATE_LIMIT_DURATIONS = ['ONE_MIN', 'ONE_HOUR', 'FIVE_HOUR', 'ONE_WEEK', 'ONE_MONTH'] as const;
type RateLimitDuration = (typeof RATE_LIMIT_DURATIONS)[number];

const DURATION_I18N_KEYS: Record<RateLimitDuration, string> = {
  ONE_MIN: 'channels.dialogs.rateLimit.durations.1min',
  ONE_HOUR: 'channels.dialogs.rateLimit.durations.1hr',
  FIVE_HOUR: 'channels.dialogs.rateLimit.durations.5hr',
  ONE_WEEK: 'channels.dialogs.rateLimit.durations.1wk',
  ONE_MONTH: 'channels.dialogs.rateLimit.durations.1mo',
};

const localDatetimeSchema = z
  .string()
  .regex(/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}$/, 'Invalid datetime format');

const rateLimitFormSchema = z
  .object({
    rpm: z
      .union([z.number().int().positive(), z.literal('')])
      .optional()
      .nullable(),
    rpmDuration: z.enum(RATE_LIMIT_DURATIONS).optional().nullable(),
    rpmWindowAnchor: localDatetimeSchema.optional().nullable(),
    tpm: z
      .union([z.number().int().positive(), z.literal('')])
      .optional()
      .nullable(),
    tpmDuration: z.enum(RATE_LIMIT_DURATIONS).optional().nullable(),
    tpmWindowAnchor: localDatetimeSchema.optional().nullable(),
    cost: z
      .union([z.number().positive(), z.literal('')])
      .optional()
      .nullable(),
    costDuration: z.enum(RATE_LIMIT_DURATIONS).optional().nullable(),
    costWindowAnchor: localDatetimeSchema.optional().nullable(),
    maxConcurrent: z
      .union([z.number().int().positive(), z.literal('')])
      .optional()
      .nullable(),
    modelConcurrent: z
      .array(
        z.object({
          model: z.string(),
          limit: z.union([z.number().int().positive(), z.literal('')]),
        })
      )
      .optional(),
  })
  .refine(
    (data) => {
      const models = data.modelConcurrent?.map((entry) => entry.model.trim().toLowerCase()) ?? [];
      const uniqueModels = new Set(models);
      return models.length === uniqueModels.size;
    },
    {
      message: 'channels.dialogs.rateLimit.fields.modelConcurrent.duplicateError',
      path: ['modelConcurrent'],
    }
  );

type RateLimitFormValues = z.infer<typeof rateLimitFormSchema>;

function modelConcurrentToArray(
  rateLimit: { modelConcurrent?: { model: string; limit: number | null }[] | null } | null | undefined,
  supportedModels: string[]
): { model: string; limit: number | '' }[] {
  if (!rateLimit?.modelConcurrent) return [];
  return rateLimit.modelConcurrent.map((entry) => {
    const serverModel = entry.model ?? '';
    const originalCasedModel = supportedModels.find((m) => m.toLowerCase() === serverModel.toLowerCase()) ?? serverModel;
    return {
      model: originalCasedModel,
      limit: entry.limit ?? '',
    };
  });
}

const TIME_ONLY_DURATIONS: RateLimitDuration[] = ['ONE_HOUR'];
const DATE_DURATIONS: RateLimitDuration[] = ['FIVE_HOUR', 'ONE_WEEK', 'ONE_MONTH'];

function durationUsesTimeOnlyAnchor(duration: RateLimitDuration | null | undefined): boolean {
  return !!duration && TIME_ONLY_DURATIONS.includes(duration);
}

function durationUsesDatetimeAnchor(duration: RateLimitDuration | null | undefined): boolean {
  return !!duration && DATE_DURATIONS.includes(duration);
}

function durationUsesAnchor(duration: RateLimitDuration | null | undefined): boolean {
  return durationUsesTimeOnlyAnchor(duration) || durationUsesDatetimeAnchor(duration);
}

function getLocalDateValue(localDatetime: string | null | undefined): string {
  if (!localDatetime) return '';
  const datePart = localDatetime.split('T')[0] ?? '';
  return /^\d{4}-\d{2}-\d{2}$/.test(datePart) ? datePart : '';
}

function getLocalTimeValue(localDatetime: string | null | undefined): string {
  if (!localDatetime) return '';
  const timePart = localDatetime.split('T')[1] ?? '';
  return /^\d{2}:\d{2}$/.test(timePart) ? timePart : '';
}

function mergeLocalDateAndTime(localDatetime: string | null | undefined, timeValue: string, timezone: string): string | null {
  if (!timeValue) return null;

  let datePart = localDatetime?.split('T')[0];
  if (!datePart || !/^\d{4}-\d{2}-\d{2}$/.test(datePart)) {
    const { year, month, day } = getTzDateParts(timezone);
    datePart = `${year}-${String(month).padStart(2, '0')}-${String(day).padStart(2, '0')}`;
  }

  return `${datePart}T${timeValue}`;
}

// For date-based durations: when date is selected but no time exists yet, default to 00:00
function mergeLocalDateAndTimeForDateInput(
  localDatetime: string | null | undefined,
  dateValue: string,
  timeValue?: string
): string | null {
  if (!dateValue) return null;

  const timePart = timeValue && /^\d{2}:\d{2}$/.test(timeValue) ? timeValue : '00:00';
  return `${dateValue}T${timePart}`;
}

function getRateLimitDefaults(currentRow: Channel, timezone: string): RateLimitFormValues {
  return {
    rpm: currentRow.settings?.rateLimit?.rpm ?? '',
    rpmDuration: currentRow.settings?.rateLimit?.rpmDuration ?? 'ONE_MIN',
    rpmWindowAnchor: utcToTzDatetime(currentRow.settings?.rateLimit?.rpmWindowAnchor, timezone) || null,
    tpm: currentRow.settings?.rateLimit?.tpm ?? '',
    tpmDuration: currentRow.settings?.rateLimit?.tpmDuration ?? 'ONE_MIN',
    tpmWindowAnchor: utcToTzDatetime(currentRow.settings?.rateLimit?.tpmWindowAnchor, timezone) || null,
    cost: currentRow.settings?.rateLimit?.cost ?? '',
    costDuration: currentRow.settings?.rateLimit?.costDuration ?? 'ONE_WEEK',
    costWindowAnchor: utcToTzDatetime(currentRow.settings?.rateLimit?.costWindowAnchor, timezone) || null,
    maxConcurrent: currentRow.settings?.rateLimit?.maxConcurrent ?? '',
    modelConcurrent: modelConcurrentToArray(currentRow.settings?.rateLimit, currentRow.supportedModels),
  };
}

function WindowAnchorField({
  control,
  name,
  duration,
  timezone,
}: {
  control: ReturnType<typeof useForm<RateLimitFormValues>>['control'];
  name: 'rpmWindowAnchor' | 'tpmWindowAnchor' | 'costWindowAnchor';
  duration: RateLimitDuration | null | undefined;
  timezone: string;
}) {
  const { t } = useTranslation();

  const isTimeOnly = durationUsesTimeOnlyAnchor(duration);
  const isDateBased = durationUsesDatetimeAnchor(duration);

  if (!isTimeOnly && !isDateBased) {
    return null;
  }

  return (
    <FormField
      control={control}
      name={name}
      render={({ field }) => {
        const anchorValue = field.value;

        if (isTimeOnly) {
          const timeValue = getLocalTimeValue(anchorValue);

          return (
            <FormItem className='w-[220px]'>
              <FormLabel className='flex items-center gap-1'>
                {t('channels.dialogs.rateLimit.fields.windowAnchor.startTime')}
                <Tooltip>
                  <TooltipTrigger asChild>
                    <Info className='text-muted-foreground h-3.5 w-3.5 shrink-0 cursor-help' />
                  </TooltipTrigger>
                  <TooltipContent side='top' className='max-w-[260px]'>
                    {t('channels.dialogs.rateLimit.fields.windowAnchor.hourTooltip')}
                  </TooltipContent>
                </Tooltip>
              </FormLabel>
              <FormControl>
                <Input
                  type='time'
                  value={timeValue}
                  onChange={(e) => {
                    field.onChange(mergeLocalDateAndTime(anchorValue, e.target.value, timezone));
                  }}
                />
              </FormControl>
              <FormMessage />
            </FormItem>
          );
        }

        const dateValue = getLocalDateValue(anchorValue);
        const timeValue = getLocalTimeValue(anchorValue);

        return (
          <FormItem className='flex gap-2 items-end'>
            <div>
              <FormLabel className='flex items-center gap-1'>
                {t('channels.dialogs.rateLimit.fields.windowAnchor.startDate')}
                <Tooltip>
                  <TooltipTrigger asChild>
                    <Info className='text-muted-foreground h-3.5 w-3.5 shrink-0 cursor-help' />
                  </TooltipTrigger>
                  <TooltipContent side='top' className='max-w-[260px]'>
                    {t('channels.dialogs.rateLimit.fields.windowAnchor.dateTooltip')}
                  </TooltipContent>
                </Tooltip>
              </FormLabel>
              <FormControl>
                <Input
                  type='date'
                  value={dateValue}
                  onChange={(e) => {
                    field.onChange(mergeLocalDateAndTimeForDateInput(anchorValue, e.target.value, timeValue || undefined));
                  }}
                />
              </FormControl>
            </div>
            <div>
              <FormLabel className='flex items-center gap-1'>
                {t('channels.dialogs.rateLimit.fields.windowAnchor.startTime')}
              </FormLabel>
              <FormControl>
                <Input
                  type='time'
                  disabled={!dateValue}
                  value={timeValue}
                  onChange={(e) => {
                    field.onChange(mergeLocalDateAndTimeForDateInput(anchorValue, dateValue, e.target.value));
                  }}
                />
              </FormControl>
            </div>
            <FormMessage />
          </FormItem>
        );
      }}
    />
  );
}

export function ChannelsRateLimitDialog({ open, onOpenChange, currentRow }: Props) {
  const { t } = useTranslation();
  const updateChannel = useUpdateChannel();
  const { data: generalSettings } = useGeneralSettings();
  const timezone = generalSettings?.timezone || 'UTC';

  const form = useForm<RateLimitFormValues>({
    resolver: zodResolver(rateLimitFormSchema),
    defaultValues: getRateLimitDefaults(currentRow, timezone),
  });

  const { fields, append, remove } = useFieldArray({
    control: form.control,
    name: 'modelConcurrent',
  });

  const addModelEntry = useCallback(() => {
    append({ model: '', limit: '' });
  }, [append]);

  const prevOpenRef = useRef(false);
  const prevTimezoneRef = useRef(timezone);

  useEffect(() => {
    const isOpening = open && !prevOpenRef.current;
    const timezoneChanged = open && prevTimezoneRef.current !== timezone && !form.formState.isDirty;

    if (isOpening || timezoneChanged) {
      form.reset(getRateLimitDefaults(currentRow, timezone));
    }

    prevOpenRef.current = open;
    prevTimezoneRef.current = timezone;
  }, [open, currentRow, timezone, form, form.formState.isDirty]);

  const onSubmit = async (values: RateLimitFormValues) => {
    try {
      const finalRpm = values.rpm === '' || values.rpm == null ? null : values.rpm;
      const finalTpm = values.tpm === '' || values.tpm == null ? null : values.tpm;
      const finalCost = values.cost === '' || values.cost == null ? null : values.cost;

      const rateLimit: Record<string, unknown> = {
        rpm: finalRpm,
        rpmDuration: finalRpm != null ? values.rpmDuration : null,
        rpmWindowAnchor: finalRpm != null && durationUsesAnchor(values.rpmDuration) ? tzDatetimeToUtc(values.rpmWindowAnchor, timezone) : null,
        tpm: finalTpm,
        tpmDuration: finalTpm != null ? values.tpmDuration : null,
        tpmWindowAnchor: finalTpm != null && durationUsesAnchor(values.tpmDuration) ? tzDatetimeToUtc(values.tpmWindowAnchor, timezone) : null,
        cost: finalCost,
        costDuration: finalCost != null ? values.costDuration : null,
        costWindowAnchor: finalCost != null && durationUsesAnchor(values.costDuration) ? tzDatetimeToUtc(values.costWindowAnchor, timezone) : null,
        maxConcurrent: values.maxConcurrent === '' || values.maxConcurrent == null ? null : values.maxConcurrent,
      };

      if (values.modelConcurrent && values.modelConcurrent.length > 0) {
        const mcArray: { model: string; limit: number | null }[] = [];
        for (const entry of values.modelConcurrent) {
          const model = entry.model.trim().toLowerCase();
          if (!model) continue;
          const limit = entry.limit != null && entry.limit !== '' ? entry.limit : null;
          mcArray.push({ model, limit });
        }
        if (mcArray.length > 0) {
          rateLimit.modelConcurrent = mcArray;
        }
      }

      const hasModelConcurrent =
        !!rateLimit.modelConcurrent && (rateLimit.modelConcurrent as { model: string; limit: number | null }[]).length > 0;
      const rateLimitValue =
        rateLimit.rpm == null &&
        rateLimit.rpmDuration == null &&
        rateLimit.rpmWindowAnchor == null &&
        rateLimit.tpm == null &&
        rateLimit.tpmDuration == null &&
        rateLimit.tpmWindowAnchor == null &&
        rateLimit.cost == null &&
        rateLimit.costDuration == null &&
        rateLimit.costWindowAnchor == null &&
        rateLimit.maxConcurrent == null &&
        !hasModelConcurrent
          ? null
          : rateLimit;

      const nextSettings = mergeChannelSettingsForUpdate(currentRow.settings, {
        rateLimit: rateLimitValue,
      });

      await updateChannel.mutateAsync({
        id: currentRow.id,
        input: {
          settings: nextSettings,
        },
      });
      onOpenChange(false);
    } catch (_error) {
      toast.error(t('common.errors.internalServerError'));
    }
  };

  return (
    <Dialog
      open={open}
      onOpenChange={(state) => {
        if (!state) {
          form.reset();
        }
        onOpenChange(state);
      }}
    >
      <DialogContent className='sm:max-w-2xl'>
        <DialogHeader className='text-left'>
          <DialogTitle>{t('channels.dialogs.rateLimit.title')}</DialogTitle>
          <DialogDescription>{t('channels.dialogs.rateLimit.description', { name: currentRow.name })}</DialogDescription>
        </DialogHeader>

        <div className='space-y-6'>
          <Card>
            <CardHeader>
              <CardTitle className='text-lg'>{t('channels.dialogs.rateLimit.config.title')}</CardTitle>
              <CardDescription>{t('channels.dialogs.rateLimit.config.description')}</CardDescription>
              <CardAction>
                <Tooltip>
                  <TooltipTrigger asChild>
                    <Button
                      type='button'
                      variant='ghost'
                      size='icon'
                      className='text-muted-foreground hover:text-foreground h-8 w-8 shrink-0'
                      aria-label={t('channels.dialogs.rateLimit.config.reset')}
                      onClick={() => {
                        form.reset({
                          ...form.getValues(),
                          rpm: '',
                          rpmDuration: 'ONE_MIN',
                          rpmWindowAnchor: null,
                          tpm: '',
                          tpmDuration: 'ONE_MIN',
                          tpmWindowAnchor: null,
                          cost: '',
                          costDuration: 'ONE_WEEK',
                          costWindowAnchor: null,
                          maxConcurrent: '',
                        });
                      }}
                    >
                      <RotateCcw className='h-4 w-4' />
                    </Button>
                  </TooltipTrigger>
                  <TooltipContent>{t('channels.dialogs.rateLimit.config.reset')}</TooltipContent>
                </Tooltip>
              </CardAction>
            </CardHeader>
            <CardContent className='space-y-4'>
              <Form {...form}>
                <form className='space-y-4'>
                  <FormField
                    control={form.control}
                    name='rpm'
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>{t('channels.dialogs.rateLimit.fields.rpm.label')}</FormLabel>
                        <div className='flex items-end gap-2'>
                          <FormControl>
                            <Input
                              type='number'
                              className='w-[130px]'
                              placeholder={t('channels.dialogs.rateLimit.fields.rpm.placeholder')}
                              value={field.value === '' || field.value == null ? '' : field.value}
                              onChange={(e) => {
                                const val = e.target.value;
                                field.onChange(val === '' ? '' : parseInt(val, 10));
                              }}
                            />
                          </FormControl>
                          <FormField
                            control={form.control}
                            name='rpmDuration'
                            render={({ field: durationField }) => (
                              <FormItem className='w-[150px]'>
                                <Select value={durationField.value ?? 'ONE_MIN'} onValueChange={durationField.onChange}>
                                  <SelectTrigger>
                                    <SelectValue />
                                  </SelectTrigger>
                                  <SelectContent>
                                    {RATE_LIMIT_DURATIONS.map((duration) => (
                                      <SelectItem key={duration} value={duration}>
                                        {t(DURATION_I18N_KEYS[duration])}
                                      </SelectItem>
                                    ))}
                                  </SelectContent>
                                </Select>
                                <FormMessage />
                              </FormItem>
                            )}
                          />
                          <WindowAnchorField
                            control={form.control}
                            name='rpmWindowAnchor'
                            duration={form.watch('rpmDuration')}
                            timezone={timezone}
                          />
                        </div>
                        <FormDescription>{t('channels.dialogs.rateLimit.fields.rpm.description')}</FormDescription>
                        <FormMessage />
                      </FormItem>
                    )}
                  />

                  <FormField
                    control={form.control}
                    name='tpm'
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>{t('channels.dialogs.rateLimit.fields.tpm.label')}</FormLabel>
                        <div className='flex items-end gap-2'>
                          <FormControl>
                            <Input
                              type='number'
                              className='w-[130px]'
                              placeholder={t('channels.dialogs.rateLimit.fields.tpm.placeholder')}
                              value={field.value === '' || field.value == null ? '' : field.value}
                              onChange={(e) => {
                                const val = e.target.value;
                                field.onChange(val === '' ? '' : parseInt(val, 10));
                              }}
                            />
                          </FormControl>
                          <FormField
                            control={form.control}
                            name='tpmDuration'
                            render={({ field: durationField }) => (
                              <FormItem className='w-[150px]'>
                                <Select value={durationField.value ?? 'ONE_MIN'} onValueChange={durationField.onChange}>
                                  <SelectTrigger>
                                    <SelectValue />
                                  </SelectTrigger>
                                  <SelectContent>
                                    {RATE_LIMIT_DURATIONS.map((duration) => (
                                      <SelectItem key={duration} value={duration}>
                                        {t(DURATION_I18N_KEYS[duration])}
                                      </SelectItem>
                                    ))}
                                  </SelectContent>
                                </Select>
                                <FormMessage />
                              </FormItem>
                            )}
                          />
                          <WindowAnchorField
                            control={form.control}
                            name='tpmWindowAnchor'
                            duration={form.watch('tpmDuration')}
                            timezone={timezone}
                          />
                        </div>
                        <FormDescription>{t('channels.dialogs.rateLimit.fields.tpm.description')}</FormDescription>
                        <FormMessage />
                      </FormItem>
                    )}
                  />

                  <FormField
                    control={form.control}
                    name='cost'
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>{t('channels.dialogs.rateLimit.fields.cost.label')}</FormLabel>
                        <div className='flex items-end gap-2'>
                          <FormControl>
                            <Input
                              type='number'
                              className='w-[130px]'
                              step='0.01'
                              placeholder={t('channels.dialogs.rateLimit.fields.cost.placeholder')}
                              value={field.value === '' || field.value == null ? '' : field.value}
                              onChange={(e) => {
                                const val = e.target.value;
                                field.onChange(val === '' ? '' : parseFloat(val));
                              }}
                            />
                          </FormControl>
                          <FormField
                            control={form.control}
                            name='costDuration'
                            render={({ field: durationField }) => (
                              <FormItem className='w-[150px]'>
                                <Select value={durationField.value ?? 'ONE_WEEK'} onValueChange={durationField.onChange}>
                                  <SelectTrigger>
                                    <SelectValue />
                                  </SelectTrigger>
                                  <SelectContent>
                                    {RATE_LIMIT_DURATIONS.map((duration) => (
                                      <SelectItem key={duration} value={duration}>
                                        {t(DURATION_I18N_KEYS[duration])}
                                      </SelectItem>
                                    ))}
                                  </SelectContent>
                                </Select>
                                <FormMessage />
                              </FormItem>
                            )}
                          />
                          <WindowAnchorField
                            control={form.control}
                            name='costWindowAnchor'
                            duration={form.watch('costDuration')}
                            timezone={timezone}
                          />
                        </div>
                        <FormDescription>{t('channels.dialogs.rateLimit.fields.cost.description')}</FormDescription>
                        <FormMessage />
                      </FormItem>
                    )}
                  />

                  <FormField
                    control={form.control}
                    name='maxConcurrent'
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>{t('channels.dialogs.rateLimit.fields.maxConcurrent.label')}</FormLabel>
                        <FormControl>
                          <Input
                            type='number'
                            placeholder={t('channels.dialogs.rateLimit.fields.maxConcurrent.placeholder')}
                            value={field.value === '' || field.value == null ? '' : field.value}
                            onChange={(e) => {
                              const val = e.target.value;
                              field.onChange(val === '' ? '' : parseInt(val, 10));
                            }}
                          />
                        </FormControl>
                        <FormDescription>{t('channels.dialogs.rateLimit.fields.maxConcurrent.description')}</FormDescription>
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                </form>
              </Form>
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <CardTitle className='text-lg'>{t('channels.dialogs.rateLimit.fields.modelConcurrent.label')}</CardTitle>
              <CardDescription>{t('channels.dialogs.rateLimit.fields.modelConcurrent.description')}</CardDescription>
              <CardAction>
                <Tooltip>
                  <TooltipTrigger asChild>
                    <Button
                      type='button'
                      variant='ghost'
                      size='icon'
                      className='text-muted-foreground hover:text-foreground h-8 w-8 shrink-0'
                      aria-label={t('channels.dialogs.rateLimit.fields.modelConcurrent.reset')}
                      onClick={() => {
                        form.reset({
                          ...form.getValues(),
                          modelConcurrent: [],
                        });
                      }}
                    >
                      <RotateCcw className='h-4 w-4' />
                    </Button>
                  </TooltipTrigger>
                  <TooltipContent>{t('channels.dialogs.rateLimit.fields.modelConcurrent.reset')}</TooltipContent>
                </Tooltip>
              </CardAction>
            </CardHeader>
            <CardContent className='space-y-4'>
              <Form {...form}>
                <form className='space-y-3'>
                  {fields.length === 0 ? (
                    <p className='text-muted-foreground py-2 text-center text-sm'>
                      {t('channels.dialogs.rateLimit.fields.modelConcurrent.noModels')}
                    </p>
                  ) : (
                    fields.map((fieldEntry, index) => (
                      <div key={fieldEntry.id} className='flex items-center gap-2'>
                        <FormField
                          control={form.control}
                          name={`modelConcurrent.${index}.model`}
                          render={({ field }) => (
                            <FormItem className='flex-1'>
                              <Select value={field.value ?? ''} onValueChange={field.onChange}>
                                <SelectTrigger>
                                  <SelectValue placeholder={t('channels.dialogs.rateLimit.fields.modelConcurrent.modelPlaceholder')} />
                                </SelectTrigger>
                                <SelectContent>
                                  {currentRow.supportedModels.map((model) => (
                                    <SelectItem key={model} value={model}>
                                      {model}
                                    </SelectItem>
                                  ))}
                                </SelectContent>
                              </Select>
                              <FormMessage />
                            </FormItem>
                          )}
                        />
                        <FormField
                          control={form.control}
                          name={`modelConcurrent.${index}.limit`}
                          render={({ field }) => (
                            <FormItem className='w-[140px]'>
                              <FormControl>
                                <Input
                                  type='number'
                                  placeholder={t('channels.dialogs.rateLimit.fields.modelConcurrent.limitPlaceholder')}
                                  value={field.value === '' || field.value == null ? '' : field.value}
                                  onChange={(e) => {
                                    const val = e.target.value;
                                    field.onChange(val === '' ? '' : parseInt(val, 10));
                                  }}
                                />
                              </FormControl>
                              <FormMessage />
                            </FormItem>
                          )}
                        />
                        <Button
                          type='button'
                          variant='ghost'
                          size='icon'
                          onClick={() => remove(index)}
                          className='text-muted-foreground hover:text-destructive shrink-0'
                          aria-label={t('channels.dialogs.rateLimit.fields.modelConcurrent.removeModel')}
                        >
                          <X className='h-4 w-4' />
                        </Button>
                      </div>
                    ))
                  )}
                  <FormField control={form.control} name='modelConcurrent' render={() => <FormMessage />} />
                  <Button type='button' variant='outline' size='sm' onClick={addModelEntry}>
                    <Plus className='mr-1 h-4 w-4' />
                    {t('channels.dialogs.rateLimit.fields.modelConcurrent.addModel')}
                  </Button>
                </form>
              </Form>
            </CardContent>
          </Card>
        </div>

        <DialogFooter>
          <Button type='button' variant='outline' onClick={() => onOpenChange(false)}>
            {t('common.buttons.cancel')}
          </Button>
          <Button type='button' onClick={form.handleSubmit(onSubmit)} disabled={updateChannel.isPending}>
            {updateChannel.isPending ? t('common.buttons.saving') : t('common.buttons.save')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
