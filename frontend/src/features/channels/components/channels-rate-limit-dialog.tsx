'use client';

import { useEffect, useCallback } from 'react';
import { z } from 'zod';
import { useForm, useFieldArray } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';
import { Plus, X, Info } from 'lucide-react';
import { format, parseISO } from 'date-fns';
import { Button } from '@/components/ui/button';
import { Tooltip, TooltipTrigger, TooltipContent } from '@/components/ui/tooltip';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Form, FormField, FormItem, FormLabel, FormMessage, FormControl, FormDescription } from '@/components/ui/form';
import { Input } from '@/components/ui/input';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { useUpdateChannel } from '../data/channels';
import { Channel } from '../data/schema';
import { mergeChannelSettingsForUpdate } from '../utils/merge';

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

function utcToLocalDatetime(utcIso: string | null | undefined): string {
  if (!utcIso) return '';
  try {
    // Ensure the string is treated as UTC: append 'Z' if no timezone info is present
    let normalized = utcIso;
    if (!normalized.endsWith('Z') && !/[+-]\d{2}:?\d{2}$/.test(normalized)) {
      normalized = normalized + 'Z';
    }
    const d = parseISO(normalized);
    return format(d, "yyyy-MM-dd'T'HH:mm");
  } catch {
    return '';
  }
}

function localDatetimeToUtc(localValue: string): string | null {
  if (!localValue) return null;
  const d = new Date(localValue);
  if (isNaN(d.getTime())) return null;
  return d.toISOString();
}

const rateLimitFormSchema = z.object({
  rpm: z.union([z.number().int().positive(), z.literal('')]).optional().nullable(),
  rpmDuration: z.enum(RATE_LIMIT_DURATIONS).optional().nullable(),
  rpmWindowAnchor: z.string().datetime({ message: 'Invalid ISO datetime format' }).optional().nullable(),
  tpm: z.union([z.number().int().positive(), z.literal('')]).optional().nullable(),
  tpmDuration: z.enum(RATE_LIMIT_DURATIONS).optional().nullable(),
  tpmWindowAnchor: z.string().datetime({ message: 'Invalid ISO datetime format' }).optional().nullable(),
  cost: z.union([z.number().positive(), z.literal('')]).optional().nullable(),
  costDuration: z.enum(RATE_LIMIT_DURATIONS).optional().nullable(),
  costWindowAnchor: z.string().datetime({ message: 'Invalid ISO datetime format' }).optional().nullable(),
  maxConcurrent: z.union([z.number().int().positive(), z.literal('')]).optional().nullable(),
  modelConcurrent: z.array(z.object({
    model: z.string(),
    limit: z.union([z.number().int().positive(), z.literal('')]),
  })).optional(),
}).refine(
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
    const originalCasedModel =
      supportedModels.find((m) => m.toLowerCase() === serverModel.toLowerCase()) ?? serverModel;
    return {
      model: originalCasedModel,
      limit: entry.limit ?? '',
    };
  });
}

const HOUR_DURATIONS: RateLimitDuration[] = ['ONE_HOUR', 'FIVE_HOUR'];
const DATE_DURATIONS: RateLimitDuration[] = ['ONE_WEEK', 'ONE_MONTH'];

function isHourBasedDuration(d: RateLimitDuration | null | undefined): boolean {
  return !!d && HOUR_DURATIONS.includes(d);
}

function WindowAnchorField({ control, name, duration }: { control: ReturnType<typeof useForm<RateLimitFormValues>>['control']; name: 'rpmWindowAnchor' | 'tpmWindowAnchor' | 'costWindowAnchor'; duration: RateLimitDuration | null | undefined }) {
  const { t } = useTranslation();

  const isHourBased = isHourBasedDuration(duration);
  const isDateBased = !!duration && DATE_DURATIONS.includes(duration);

  if (!isHourBased && !isDateBased) {
    return null;
  }

  return (
    <FormField
      control={control}
      name={name}
      render={({ field }) => {
        const anchorValue = field.value;

        if (isHourBased) {
          // Hour-based durations: show a number input for the hour (0–23)
          const hourValue = (() => {
            if (!anchorValue) return '';
            try {
              const d = parseISO(anchorValue.endsWith('Z') || /[+-]\d{2}:?\d{2}$/.test(anchorValue) ? anchorValue : anchorValue + 'Z');
              return String(d.getUTCHours());
            } catch {
              return '';
            }
          })();

          return (
            <FormItem className='w-[240px]'>
              <div className='flex items-center gap-1'>
                <FormLabel className='sr-only'>{t('channels.dialogs.rateLimit.fields.windowAnchor.label')}</FormLabel>
                <span className='text-sm text-muted-foreground whitespace-nowrap'>{t('channels.dialogs.rateLimit.fields.windowAnchor.startTime')}</span>
                <Tooltip>
                  <TooltipTrigger asChild>
                    <Info className='h-3.5 w-3.5 text-muted-foreground cursor-help shrink-0' />
                  </TooltipTrigger>
                  <TooltipContent side='top' className='max-w-[260px]'>
                    {t('channels.dialogs.rateLimit.fields.windowAnchor.hourTooltip')}
                  </TooltipContent>
                </Tooltip>
              </div>
              <FormControl>
                <Input
                  type='number'
                  min={0}
                  max={23}
                  placeholder='0–23'
                  value={hourValue}
                  onChange={(e) => {
                    const val = e.target.value;
                    if (val === '') {
                      field.onChange(null);
                      return;
                    }
                    const hour = parseInt(val, 10);
                    if (isNaN(hour) || hour < 0 || hour > 23) return;
                    // Build a UTC time.Time at today's date with this hour, preserving the date portion
                    const now = new Date();
                    const utcDate = new Date(Date.UTC(now.getUTCFullYear(), now.getUTCMonth(), now.getUTCDate(), hour, 0, 0, 0));
                    field.onChange(utcDate.toISOString());
                  }}
                />
              </FormControl>
              <FormMessage />
            </FormItem>
          );
        }

        // Day-based durations: show a date+time picker
        return (
          <FormItem className='w-[240px]'>
            <div className='flex items-center gap-1'>
              <FormLabel className='sr-only'>{t('channels.dialogs.rateLimit.fields.windowAnchor.label')}</FormLabel>
              <span className='text-sm text-muted-foreground whitespace-nowrap'>{t('channels.dialogs.rateLimit.fields.windowAnchor.startTime')}</span>
              <Tooltip>
                <TooltipTrigger asChild>
                  <Info className='h-3.5 w-3.5 text-muted-foreground cursor-help shrink-0' />
                </TooltipTrigger>
                <TooltipContent side='top' className='max-w-[260px]'>
                  {t('channels.dialogs.rateLimit.fields.windowAnchor.dateTooltip')}
                </TooltipContent>
              </Tooltip>
            </div>
            <FormControl>
              <Input
                type='datetime-local'
                value={utcToLocalDatetime(anchorValue)}
                onChange={(e) => {
                  field.onChange(localDatetimeToUtc(e.target.value));
                }}
                placeholder={t('channels.dialogs.rateLimit.fields.windowAnchor.placeholder')}
              />
            </FormControl>
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

  const form = useForm<RateLimitFormValues>({
    resolver: zodResolver(rateLimitFormSchema),
    defaultValues: {
      rpm: currentRow.settings?.rateLimit?.rpm ?? '',
      rpmDuration: currentRow.settings?.rateLimit?.rpmDuration ?? 'ONE_MIN',
      rpmWindowAnchor: currentRow.settings?.rateLimit?.rpmWindowAnchor ?? null,
      tpm: currentRow.settings?.rateLimit?.tpm ?? '',
      tpmDuration: currentRow.settings?.rateLimit?.tpmDuration ?? 'ONE_MIN',
      tpmWindowAnchor: currentRow.settings?.rateLimit?.tpmWindowAnchor ?? null,
      cost: currentRow.settings?.rateLimit?.cost ?? '',
      costDuration: currentRow.settings?.rateLimit?.costDuration ?? 'ONE_WEEK',
      costWindowAnchor: currentRow.settings?.rateLimit?.costWindowAnchor ?? null,
      maxConcurrent: currentRow.settings?.rateLimit?.maxConcurrent ?? '',
      modelConcurrent: modelConcurrentToArray(currentRow.settings?.rateLimit, currentRow.supportedModels),
    },
  });

  const { fields, append, remove } = useFieldArray({
    control: form.control,
    name: 'modelConcurrent',
  });

  const addModelEntry = useCallback(() => {
    append({ model: '', limit: '' });
  }, [append]);

  useEffect(() => {
    if (open) {
      form.reset({
        rpm: currentRow.settings?.rateLimit?.rpm ?? '',
        rpmDuration: currentRow.settings?.rateLimit?.rpmDuration ?? 'ONE_MIN',
        rpmWindowAnchor: currentRow.settings?.rateLimit?.rpmWindowAnchor ?? null,
        tpm: currentRow.settings?.rateLimit?.tpm ?? '',
        tpmDuration: currentRow.settings?.rateLimit?.tpmDuration ?? 'ONE_MIN',
        tpmWindowAnchor: currentRow.settings?.rateLimit?.tpmWindowAnchor ?? null,
        cost: currentRow.settings?.rateLimit?.cost ?? '',
        costDuration: currentRow.settings?.rateLimit?.costDuration ?? 'ONE_WEEK',
        costWindowAnchor: currentRow.settings?.rateLimit?.costWindowAnchor ?? null,
        maxConcurrent: currentRow.settings?.rateLimit?.maxConcurrent ?? '',
        modelConcurrent: modelConcurrentToArray(currentRow.settings?.rateLimit, currentRow.supportedModels),
      });
    }
  }, [open, currentRow, form]);

  const onSubmit = async (values: RateLimitFormValues) => {
    try {
      const finalRpm = values.rpm === '' || values.rpm == null ? null : values.rpm;
      const finalTpm = values.tpm === '' || values.tpm == null ? null : values.tpm;
      const finalCost = values.cost === '' || values.cost == null ? null : values.cost;

      const rateLimit: Record<string, unknown> = {
        rpm: finalRpm,
        rpmDuration: finalRpm != null ? values.rpmDuration : null,
        rpmWindowAnchor: finalRpm != null ? values.rpmWindowAnchor : null,
        tpm: finalTpm,
        tpmDuration: finalTpm != null ? values.tpmDuration : null,
        tpmWindowAnchor: finalTpm != null ? values.tpmWindowAnchor : null,
        cost: finalCost,
        costDuration: finalCost != null ? values.costDuration : null,
        costWindowAnchor: finalCost != null ? values.costWindowAnchor : null,
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

      const hasModelConcurrent = !!rateLimit.modelConcurrent && (rateLimit.modelConcurrent as { model: string; limit: number | null }[]).length > 0;
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
      toast.success(t('channels.messages.updateSuccess'));
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
                        <div className='flex gap-2 items-start'>
                          <FormControl>
                            <Input
                              type='number'
                              className='w-[100px]'
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
                                <Select
                                  value={durationField.value ?? 'ONE_MIN'}
                                  onValueChange={durationField.onChange}
                                >
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
                          <WindowAnchorField control={form.control} name='rpmWindowAnchor' duration={form.watch('rpmDuration')} />
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
                        <div className='flex gap-2 items-start'>
                          <FormControl>
                            <Input
                              type='number'
                              className='w-[100px]'
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
                                <Select
                                  value={durationField.value ?? 'ONE_MIN'}
                                  onValueChange={durationField.onChange}
                                >
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
                          <WindowAnchorField control={form.control} name='tpmWindowAnchor' duration={form.watch('tpmDuration')} />
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
                        <div className='flex gap-2 items-start'>
                          <FormControl>
                            <Input
                              type='number'
                              className='w-[100px]'
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
                                <Select
                                  value={durationField.value ?? 'ONE_WEEK'}
                                  onValueChange={durationField.onChange}
                                >
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
                          <WindowAnchorField control={form.control} name='costWindowAnchor' duration={form.watch('costDuration')} />
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
                              <Select
                                value={field.value ?? ''}
                                onValueChange={field.onChange}
                              >
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
                  <FormField
                    control={form.control}
                    name='modelConcurrent'
                    render={() => <FormMessage />}
                  />
                  <Button
                    type='button'
                    variant='outline'
                    size='sm'
                    onClick={addModelEntry}
                  >
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
