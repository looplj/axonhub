'use client';

import { useEffect } from 'react';
import { z } from 'zod';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';
import { AlertTriangle } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Form, FormField, FormItem, FormLabel, FormMessage, FormControl, FormDescription } from '@/components/ui/form';
import { Input } from '@/components/ui/input';
import { useUpdateChannel } from '../data/channels';
import { Channel } from '../data/schema';
import { mergeChannelSettingsForUpdate } from '../utils/merge';

interface Props {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  currentRow: Channel;
}

const numericField = z
  .union([z.number().int().nonnegative(), z.literal('')])
  .optional()
  .nullable();

const rateLimitFormSchema = z
  .object({
    rpm: numericField,
    tpm: numericField,
    maxConcurrent: numericField,
    queueSize: numericField,
    queueTimeoutMs: numericField,
  })
  .superRefine((values, ctx) => {
    const queueSize = values.queueSize;
    const maxConcurrent = values.maxConcurrent;

    if (typeof queueSize === 'number' && queueSize > 0) {
      if (typeof maxConcurrent !== 'number' || maxConcurrent <= 0) {
        ctx.addIssue({
          code: z.ZodIssueCode.custom,
          path: ['queueSize'],
          message: 'queueRequiresMaxConcurrent',
        });
      }
    }
  });

type RateLimitFormValues = z.infer<typeof rateLimitFormSchema>;

const emptyDefaults: RateLimitFormValues = {
  rpm: '',
  tpm: '',
  maxConcurrent: '',
  queueSize: '',
  queueTimeoutMs: '',
};

function valuesFromChannel(currentRow: Channel): RateLimitFormValues {
  return {
    rpm: currentRow.settings?.rateLimit?.rpm ?? '',
    tpm: currentRow.settings?.rateLimit?.tpm ?? '',
    maxConcurrent: currentRow.settings?.rateLimit?.maxConcurrent ?? '',
    queueSize: currentRow.settings?.rateLimit?.queueSize ?? '',
    queueTimeoutMs: currentRow.settings?.rateLimit?.queueTimeoutMs ?? '',
  };
}

function normalize(value: number | '' | null | undefined): number | null {
  return value === '' || value == null ? null : value;
}

export function ChannelsRateLimitDialog({ open, onOpenChange, currentRow }: Props) {
  const { t } = useTranslation();
  const updateChannel = useUpdateChannel();

  const form = useForm<RateLimitFormValues>({
    resolver: zodResolver(rateLimitFormSchema),
    defaultValues: valuesFromChannel(currentRow),
    mode: 'onChange',
  });

  useEffect(() => {
    if (open) {
      form.reset(valuesFromChannel(currentRow));
    }
  }, [open, currentRow, form]);

  // Soft-mode advisory: when the user sets MaxConcurrent without a queue, the
  // limiter only down-ranks the channel in load-balancer scoring — it does not
  // block excess requests. Surface this so users understand why the cap may
  // appear to be exceeded under load.
  const watchedMaxConcurrent = form.watch('maxConcurrent');
  const watchedQueueSize = form.watch('queueSize');
  const showSoftModeHint =
    typeof watchedMaxConcurrent === 'number' &&
    watchedMaxConcurrent > 0 &&
    (typeof watchedQueueSize !== 'number' || watchedQueueSize <= 0);

  const onSubmit = async (values: RateLimitFormValues) => {
    try {
      const rateLimit = {
        rpm: normalize(values.rpm),
        tpm: normalize(values.tpm),
        maxConcurrent: normalize(values.maxConcurrent),
        queueSize: normalize(values.queueSize),
        queueTimeoutMs: normalize(values.queueTimeoutMs),
      };

      const allEmpty =
        rateLimit.rpm == null &&
        rateLimit.tpm == null &&
        rateLimit.maxConcurrent == null &&
        rateLimit.queueSize == null &&
        rateLimit.queueTimeoutMs == null;

      const rateLimitValue = allEmpty ? null : rateLimit;

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

  const renderNumericField = (name: keyof RateLimitFormValues, labelKey: string, placeholderKey: string, descriptionKey: string) => (
    <FormField
      control={form.control}
      name={name}
      render={({ field }) => (
        <FormItem>
          <FormLabel>{t(labelKey)}</FormLabel>
          <FormControl>
            <Input
              type='number'
              min={0}
              placeholder={t(placeholderKey)}
              value={field.value === '' || field.value == null ? '' : field.value}
              onChange={(e) => {
                const val = e.target.value;
                field.onChange(val === '' ? '' : parseInt(val, 10));
              }}
            />
          </FormControl>
          <FormDescription>{t(descriptionKey)}</FormDescription>
          <FormMessage />
        </FormItem>
      )}
    />
  );

  return (
    <Dialog
      open={open}
      onOpenChange={(state) => {
        if (!state) {
          form.reset(emptyDefaults);
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
                  {renderNumericField(
                    'rpm',
                    'channels.dialogs.rateLimit.fields.rpm.label',
                    'channels.dialogs.rateLimit.fields.rpm.placeholder',
                    'channels.dialogs.rateLimit.fields.rpm.description',
                  )}

                  {renderNumericField(
                    'tpm',
                    'channels.dialogs.rateLimit.fields.tpm.label',
                    'channels.dialogs.rateLimit.fields.tpm.placeholder',
                    'channels.dialogs.rateLimit.fields.tpm.description',
                  )}

                  {renderNumericField(
                    'maxConcurrent',
                    'channels.dialogs.rateLimit.fields.maxConcurrent.label',
                    'channels.dialogs.rateLimit.fields.maxConcurrent.placeholder',
                    'channels.dialogs.rateLimit.fields.maxConcurrent.description',
                  )}

                  <FormField
                    control={form.control}
                    name='queueSize'
                    render={({ field, fieldState }) => (
                      <FormItem>
                        <FormLabel>{t('channels.dialogs.rateLimit.fields.queueSize.label')}</FormLabel>
                        <FormControl>
                          <Input
                            type='number'
                            min={0}
                            placeholder={t('channels.dialogs.rateLimit.fields.queueSize.placeholder')}
                            value={field.value === '' || field.value == null ? '' : field.value}
                            onChange={(e) => {
                              const val = e.target.value;
                              field.onChange(val === '' ? '' : parseInt(val, 10));
                            }}
                          />
                        </FormControl>
                        <FormDescription>{t('channels.dialogs.rateLimit.fields.queueSize.description')}</FormDescription>
                        {showSoftModeHint && (
                          <div className='mt-1 flex items-start gap-2 rounded-md border border-amber-300 bg-amber-50 p-2 text-xs text-amber-900 dark:border-amber-700/50 dark:bg-amber-950/40 dark:text-amber-200'>
                            <AlertTriangle className='mt-0.5 h-3.5 w-3.5 shrink-0' />
                            <span>{t('channels.dialogs.rateLimit.hints.softModeWarning')}</span>
                          </div>
                        )}
                        {fieldState.error?.message === 'queueRequiresMaxConcurrent' ? (
                          <p className='text-destructive text-sm'>
                            {t('channels.dialogs.rateLimit.errors.queueRequiresMaxConcurrent')}
                          </p>
                        ) : (
                          <FormMessage />
                        )}
                      </FormItem>
                    )}
                  />

                  {renderNumericField(
                    'queueTimeoutMs',
                    'channels.dialogs.rateLimit.fields.queueTimeoutMs.label',
                    'channels.dialogs.rateLimit.fields.queueTimeoutMs.placeholder',
                    'channels.dialogs.rateLimit.fields.queueTimeoutMs.description',
                  )}
                </form>
              </Form>
            </CardContent>
          </Card>
        </div>

        <DialogFooter>
          <Button type='button' variant='outline' onClick={() => onOpenChange(false)}>
            {t('common.buttons.cancel')}
          </Button>
          <Button
            type='button'
            onClick={form.handleSubmit(onSubmit)}
            disabled={updateChannel.isPending || !form.formState.isValid}
          >
            {updateChannel.isPending ? t('common.buttons.saving') : t('common.buttons.save')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
