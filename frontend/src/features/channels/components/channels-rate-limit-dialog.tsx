'use client';

import { useEffect, useCallback } from 'react';
import { z } from 'zod';
import { useForm, useFieldArray } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';
import { Plus, X } from 'lucide-react';
import { Button } from '@/components/ui/button';
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



const rateLimitFormSchema = z.object({
  rpm: z.union([z.number().int().positive(), z.literal('')]).optional().nullable(),
  rpmDuration: z.enum(RATE_LIMIT_DURATIONS).optional().nullable(),
  tpm: z.union([z.number().int().positive(), z.literal('')]).optional().nullable(),
  tpmDuration: z.enum(RATE_LIMIT_DURATIONS).optional().nullable(),
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

/**
 * Convert modelConcurrent from server array format to form array format.
 * The server now returns an array directly: { modelConcurrent: [{ model: string, limit: number | null }] }
 */
function modelConcurrentToArray(
  rateLimit: { modelConcurrent?: { model: string; limit: number | null }[] | null } | null | undefined
): { model: string; limit: number | '' }[] {
  if (!rateLimit?.modelConcurrent) return [];
  return rateLimit.modelConcurrent.map((entry) => ({
    model: entry.model ?? '',
    limit: entry.limit ?? '',
  }));
}

export function ChannelsRateLimitDialog({ open, onOpenChange, currentRow }: Props) {
  const { t } = useTranslation();
  const updateChannel = useUpdateChannel();

  const form = useForm<RateLimitFormValues>({
    resolver: zodResolver(rateLimitFormSchema),
    defaultValues: {
      rpm: currentRow.settings?.rateLimit?.rpm ?? '',
      rpmDuration: currentRow.settings?.rateLimit?.rpmDuration ?? 'ONE_MIN',
      tpm: currentRow.settings?.rateLimit?.tpm ?? '',
      tpmDuration: currentRow.settings?.rateLimit?.tpmDuration ?? 'ONE_MIN',
      maxConcurrent: currentRow.settings?.rateLimit?.maxConcurrent ?? '',
      modelConcurrent: modelConcurrentToArray(currentRow.settings?.rateLimit),
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
        tpm: currentRow.settings?.rateLimit?.tpm ?? '',
        tpmDuration: currentRow.settings?.rateLimit?.tpmDuration ?? 'ONE_MIN',
        maxConcurrent: currentRow.settings?.rateLimit?.maxConcurrent ?? '',
        modelConcurrent: modelConcurrentToArray(currentRow.settings?.rateLimit),
      });
    }
  }, [open, currentRow, form]);

  const onSubmit = async (values: RateLimitFormValues) => {
    try {
      // Bug 5: Nullify duration when corresponding value is null/empty
      const finalRpm = values.rpm === '' || values.rpm == null ? null : values.rpm;
      const finalTpm = values.tpm === '' || values.tpm == null ? null : values.tpm;

      const rateLimit: Record<string, unknown> = {
        rpm: finalRpm,
        // Only include duration if rpm has a value
        rpmDuration: finalRpm != null ? values.rpmDuration : null,
        tpm: finalTpm,
        // Only include duration if tpm has a value
        tpmDuration: finalTpm != null ? values.tpmDuration : null,
        maxConcurrent: values.maxConcurrent === '' || values.maxConcurrent == null ? null : values.maxConcurrent,
      };

      // Bug 3: Empty limit should save as null, not 0
      if (values.modelConcurrent && values.modelConcurrent.length > 0) {
        const mcArray: { model: string; limit: number | null }[] = [];
        for (const entry of values.modelConcurrent) {
          const model = entry.model.trim().toLowerCase();
          if (!model) continue;
          // Empty limit should be null (unlimited), not 0 (block all)
          const limit = entry.limit != null && entry.limit !== '' ? entry.limit : null;
          mcArray.push({ model, limit });
        }
        if (mcArray.length > 0) {
          rateLimit.modelConcurrent = mcArray;
        }
      }

      // Bug 6: Include duration fields and modelConcurrent in null-cleanup check
      const hasModelConcurrent = !!rateLimit.modelConcurrent && (rateLimit.modelConcurrent as { model: string; limit: number | null }[]).length > 0;
      const rateLimitValue =
        rateLimit.rpm == null &&
        rateLimit.rpmDuration == null &&
        rateLimit.tpm == null &&
        rateLimit.tpmDuration == null &&
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
                        <div className='flex gap-2'>
                          <FormControl>
                            <Input
                              type='number'
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
                        <div className='flex gap-2'>
                          <FormControl>
                            <Input
                              type='number'
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
                        </div>
                        <FormDescription>{t('channels.dialogs.rateLimit.fields.tpm.description')}</FormDescription>
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
