'use client';

import { useEffect } from 'react';
import { z } from 'zod';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';
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

const rateLimitFormSchema = z.object({
  rpm: z.union([z.number().int().positive(), z.literal('')]).optional().nullable(),
  tpm: z.union([z.number().int().positive(), z.literal('')]).optional().nullable(),
  maxConcurrent: z.union([z.number().int().positive(), z.literal('')]).optional().nullable(),
});

type RateLimitFormValues = z.infer<typeof rateLimitFormSchema>;

export function ChannelsRateLimitDialog({ open, onOpenChange, currentRow }: Props) {
  const { t } = useTranslation();
  const updateChannel = useUpdateChannel();

  const form = useForm<RateLimitFormValues>({
    resolver: zodResolver(rateLimitFormSchema),
    defaultValues: {
      rpm: currentRow.settings?.rateLimit?.rpm ?? '',
      tpm: currentRow.settings?.rateLimit?.tpm ?? '',
      maxConcurrent: currentRow.settings?.rateLimit?.maxConcurrent ?? '',
    },
  });

  useEffect(() => {
    if (open) {
      form.reset({
        rpm: currentRow.settings?.rateLimit?.rpm ?? '',
        tpm: currentRow.settings?.rateLimit?.tpm ?? '',
        maxConcurrent: currentRow.settings?.rateLimit?.maxConcurrent ?? '',
      });
    }
  }, [open, currentRow, form]);

  const onSubmit = async (values: RateLimitFormValues) => {
    try {
      const rateLimit = {
        rpm: values.rpm === '' || values.rpm == null ? null : values.rpm,
        tpm: values.tpm === '' || values.tpm == null ? null : values.tpm,
        maxConcurrent: values.maxConcurrent === '' || values.maxConcurrent == null ? null : values.maxConcurrent,
      };

      // If all are null, set rateLimit to null to clean up
      const rateLimitValue = rateLimit.rpm == null && rateLimit.tpm == null && rateLimit.maxConcurrent == null ? null : rateLimit;

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
