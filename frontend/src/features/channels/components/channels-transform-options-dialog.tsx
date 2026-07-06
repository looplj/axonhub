'use client';

import { useEffect, useState } from 'react';
import { z } from 'zod';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { Plus, X } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Checkbox } from '@/components/ui/checkbox';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Form, FormField, FormItem, FormLabel, FormMessage, FormControl } from '@/components/ui/form';
import { Input } from '@/components/ui/input';
import { useUpdateChannel } from '../data/channels';
import { Channel, ReasoningEffortMapping, TransformOptions } from '../data/schema';
import { mergeChannelSettingsForUpdate } from '../utils/merge';

interface Props {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  currentRow: Channel;
}

// Form schema: reasoningEffortMapping is a list of {from, to} entries, validated for
// uniqueness of `from` (duplicate from values would make the result order-dependent).
const transformOptionsFormSchema = z.object({
  forceArrayInstructions: z.boolean().optional(),
  forceArrayInputs: z.boolean().optional(),
  replaceDeveloperRoleWithSystem: z.boolean().optional(),
  reasoningEffortMapping: z
    .array(
      z.object({
        from: z.string().min(1),
        to: z.string().min(1),
      })
    )
    .refine(
      (mappings) => {
        const fromValues = mappings.map((m) => m.from);
        return new Set(fromValues).size === fromValues.length;
      },
      { message: 'Each source effort can only be mapped once' }
    )
    .optional(),
});

type TransformOptionsFormValues = z.infer<typeof transformOptionsFormSchema>;

const EFFORT_SUGGESTIONS = ['none', 'low', 'medium', 'high', 'xhigh', 'max'];

export function ChannelsTransformOptionsDialog({ open, onOpenChange, currentRow }: Props) {
  const { t } = useTranslation();
  const updateChannel = useUpdateChannel();

  const form = useForm<TransformOptionsFormValues>({
    resolver: zodResolver(transformOptionsFormSchema),
    defaultValues: {
      forceArrayInstructions: currentRow.settings?.transformOptions?.forceArrayInstructions || false,
      forceArrayInputs: currentRow.settings?.transformOptions?.forceArrayInputs || false,
      replaceDeveloperRoleWithSystem: currentRow.settings?.transformOptions?.replaceDeveloperRoleWithSystem || false,
      reasoningEffortMapping: currentRow.settings?.transformOptions?.reasoningEffortMapping || [],
    },
  });

  // Draft row for the add-new controls. Kept separate from the form list so partial
  // input never lands in the validated list (empty rows are filtered on submit anyway).
  const [draft, setDraft] = useState<ReasoningEffortMapping>({ from: '', to: '' });

  useEffect(() => {
    if (open) {
      form.reset({
        forceArrayInstructions: currentRow.settings?.transformOptions?.forceArrayInstructions || false,
        forceArrayInputs: currentRow.settings?.transformOptions?.forceArrayInputs || false,
        replaceDeveloperRoleWithSystem: currentRow.settings?.transformOptions?.replaceDeveloperRoleWithSystem || false,
        reasoningEffortMapping: currentRow.settings?.transformOptions?.reasoningEffortMapping || [],
      });
      setDraft({ from: '', to: '' });
    }
  }, [open, currentRow, form]);

  const mappings = form.watch('reasoningEffortMapping') || [];

  const addMapping = () => {
    const sanitized = { from: draft.from.trim(), to: draft.to.trim() };
    if (!sanitized.from || !sanitized.to) {
      return;
    }
    form.setValue('reasoningEffortMapping', [...mappings, sanitized], {
      shouldValidate: true,
      shouldDirty: true,
    });
    setDraft({ from: '', to: '' });
  };

  const removeMapping = (index: number) => {
    form.setValue(
      'reasoningEffortMapping',
      mappings.filter((_, i) => i !== index),
      { shouldValidate: true, shouldDirty: true }
    );
  };

  const onSubmit = async (values: TransformOptionsFormValues) => {
    try {
      const transformOptions: TransformOptions = {
        forceArrayInstructions: values.forceArrayInstructions,
        forceArrayInputs: values.forceArrayInputs,
        replaceDeveloperRoleWithSystem: values.replaceDeveloperRoleWithSystem,
      };
      // Empty list is treated as "clear": send [] so the backend removes the mapping.
      // undefined would mean "don't touch", but the dialog is the sole editor here.
      transformOptions.reasoningEffortMapping = values.reasoningEffortMapping || [];
      const nextSettings = mergeChannelSettingsForUpdate(currentRow.settings, {
        transformOptions,
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

                  <FormField
                    control={form.control}
                    name='reasoningEffortMapping'
                    render={() => (
                      <FormItem className='space-y-2'>
                        <FormLabel className='text-sm font-normal'>
                          {t('channels.dialogs.fields.transformOptions.reasoningEffortMapping.label')}
                        </FormLabel>
                        <p className='text-muted-foreground text-xs'>
                          {t('channels.dialogs.fields.transformOptions.reasoningEffortMapping.description')}
                        </p>
                        <FormControl>
                          <div className='space-y-2'>
                            <div className='grid grid-cols-1 gap-2 md:grid-cols-[minmax(0,1fr)_auto_minmax(0,1fr)_auto] md:items-center'>
                              <Input
                                list='reasoning-effort-from-suggestions'
                                placeholder={t('channels.dialogs.fields.transformOptions.reasoningEffortMapping.fromPlaceholder')}
                                value={draft.from}
                                onChange={(e) => setDraft({ ...draft, from: e.target.value })}
                                className='min-w-0'
                              />
                              <span className='text-muted-foreground hidden justify-center md:flex'>→</span>
                              <Input
                                list='reasoning-effort-to-suggestions'
                                placeholder={t('channels.dialogs.fields.transformOptions.reasoningEffortMapping.toPlaceholder')}
                                value={draft.to}
                                onChange={(e) => setDraft({ ...draft, to: e.target.value })}
                                className='min-w-0'
                              />
                              <Button
                                type='button'
                                size='sm'
                                onClick={addMapping}
                                disabled={!draft.from.trim() || !draft.to.trim()}
                              >
                                <Plus size={16} />
                              </Button>
                              <datalist id='reasoning-effort-from-suggestions'>
                                {EFFORT_SUGGESTIONS.map((v) => (
                                  <option key={v} value={v} />
                                ))}
                              </datalist>
                              <datalist id='reasoning-effort-to-suggestions'>
                                {EFFORT_SUGGESTIONS.map((v) => (
                                  <option key={v} value={v} />
                                ))}
                              </datalist>
                            </div>

                            {mappings.length === 0 ? (
                              <p className='text-muted-foreground py-2 text-center text-sm'>
                                {t('channels.dialogs.fields.transformOptions.reasoningEffortMapping.noMappings')}
                              </p>
                            ) : (
                              mappings.map((mapping, index) => (
                                <div
                                  key={index}
                                  className='flex items-center justify-between rounded-lg border p-2'
                                >
                                  <div className='flex flex-1 items-center gap-2'>
                                    <span className='text-sm'>{mapping.from}</span>
                                    <span className='text-muted-foreground'>→</span>
                                    <span className='text-sm'>{mapping.to}</span>
                                  </div>
                                  <Button
                                    type='button'
                                    variant='ghost'
                                    size='sm'
                                    onClick={() => removeMapping(index)}
                                    className='text-destructive hover:text-destructive'
                                  >
                                    <X size={16} />
                                  </Button>
                                </div>
                              ))
                            )}
                          </div>
                        </FormControl>
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
