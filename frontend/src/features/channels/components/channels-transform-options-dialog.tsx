'use client';

import { useEffect } from 'react';
import { z } from 'zod';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Checkbox } from '@/components/ui/checkbox';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Form, FormField, FormItem, FormLabel, FormMessage, FormControl } from '@/components/ui/form';
import { useUpdateChannel } from '../data/channels';
import { Channel, TransformOptions } from '../data/schema';
import { mergeChannelSettingsForUpdate } from '../utils/merge';

interface Props {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  currentRow: Channel;
}

const transformOptionsFormSchema = z.object({
  forceArrayInstructions: z.boolean().optional(),
  forceArrayInputs: z.boolean().optional(),
  replaceDeveloperRoleWithSystem: z.boolean().optional(),
});

export function ChannelsTransformOptionsDialog({ open, onOpenChange, currentRow }: Props) {
  const { t } = useTranslation();
  const updateChannel = useUpdateChannel();
  const showDeveloperRoleOption = currentRow.type === 'bailian';

  const form = useForm<TransformOptions>({
    resolver: zodResolver(transformOptionsFormSchema),
    defaultValues: {
      forceArrayInstructions: currentRow.settings?.transformOptions?.forceArrayInstructions || false,
      forceArrayInputs: currentRow.settings?.transformOptions?.forceArrayInputs || false,
      replaceDeveloperRoleWithSystem: currentRow.settings?.transformOptions?.replaceDeveloperRoleWithSystem || false,
    },
  });

  useEffect(() => {
    if (open) {
      form.reset({
        forceArrayInstructions: currentRow.settings?.transformOptions?.forceArrayInstructions || false,
        forceArrayInputs: currentRow.settings?.transformOptions?.forceArrayInputs || false,
        replaceDeveloperRoleWithSystem: currentRow.settings?.transformOptions?.replaceDeveloperRoleWithSystem || false,
      });
    }
  }, [open, currentRow, form]);

  const onSubmit = async (values: TransformOptions) => {
    try {
      const nextSettings = mergeChannelSettingsForUpdate(currentRow.settings, {
        transformOptions: values,
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
      toast.error(t('channels.messages.updateError'));
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
          <DialogTitle>{t('channels.dialogs.transformOptions.title')}</DialogTitle>
          <DialogDescription>{t('channels.dialogs.transformOptions.description', { name: currentRow.name })}</DialogDescription>
        </DialogHeader>

        <div className='space-y-6'>
          <Card>
            <CardHeader>
              <CardTitle className='text-lg'>{t('channels.dialogs.transformOptions.options.title')}</CardTitle>
              <CardDescription>{t('channels.dialogs.transformOptions.options.description')}</CardDescription>
            </CardHeader>
            <CardContent className='space-y-4'>
              <Form {...form}>
                <form className='space-y-4'>
                  <FormField
                    control={form.control}
                    name='forceArrayInstructions'
                    render={({ field }) => (
                      <FormItem className='flex items-center gap-2'>
                        <FormControl>
                          <Checkbox checked={field.value || false} onCheckedChange={field.onChange} />
                        </FormControl>
                        <div className='space-y-0.5'>
                          <FormLabel className='cursor-pointer text-sm font-normal'>
                            {t('channels.dialogs.fields.transformOptions.forceArrayInstructions.label')}
                          </FormLabel>
                          <p className='text-muted-foreground text-xs'>
                            {t('channels.dialogs.fields.transformOptions.forceArrayInstructions.description')}
                          </p>
                        </div>
                        <FormMessage />
                      </FormItem>
                    )}
                  />

                  <FormField
                    control={form.control}
                    name='forceArrayInputs'
                    render={({ field }) => (
                      <FormItem className='flex items-center gap-2'>
                        <FormControl>
                          <Checkbox checked={field.value || false} onCheckedChange={field.onChange} />
                        </FormControl>
                        <div className='space-y-0.5'>
                          <FormLabel className='cursor-pointer text-sm font-normal'>
                            {t('channels.dialogs.fields.transformOptions.forceArrayInputs.label')}
                          </FormLabel>
                          <p className='text-muted-foreground text-xs'>
                            {t('channels.dialogs.fields.transformOptions.forceArrayInputs.description')}
                          </p>
                        </div>
                        <FormMessage />
                      </FormItem>
                    )}
                  />

                  {showDeveloperRoleOption ? (
                    <FormField
                      control={form.control}
                      name='replaceDeveloperRoleWithSystem'
                      render={({ field }) => (
                        <FormItem className='flex items-center gap-2'>
                          <FormControl>
                            <Checkbox checked={field.value || false} onCheckedChange={field.onChange} />
                          </FormControl>
                          <div className='space-y-0.5'>
                            <FormLabel className='cursor-pointer text-sm font-normal'>
                              {t('channels.dialogs.fields.transformOptions.replaceDeveloperRoleWithSystem.label')}
                            </FormLabel>
                            <p className='text-muted-foreground text-xs'>
                              {t('channels.dialogs.fields.transformOptions.replaceDeveloperRoleWithSystem.description')}
                            </p>
                          </div>
                          <FormMessage />
                        </FormItem>
                      )}
                    />
                  ) : null}
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
