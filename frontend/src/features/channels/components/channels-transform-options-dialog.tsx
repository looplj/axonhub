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

// Form schema: reasoningEffortMapping is edited as a JSON string and parsed on submit.
const transformOptionsFormSchema = z.object({
  forceArrayInstructions: z.boolean().optional(),
  forceArrayInputs: z.boolean().optional(),
  replaceDeveloperRoleWithSystem: z.boolean().optional(),
  reasoningEffortMappingJSON: z.string().optional(),
});

type TransformOptionsFormValues = z.infer<typeof transformOptionsFormSchema>;

// Serialize the reasoning_effort mapping (Record<string,string>) to a JSON string for the textarea.
function reasoningEffortMappingToJSON(mapping?: Record<string, string> | null): string {
  if (!mapping || Object.keys(mapping).length === 0) {
    return '';
  }
  return JSON.stringify(mapping);
}

// Parse the textarea JSON string into a Record<string,string>. Returns null when empty,
// throws on invalid JSON (caught by the caller to show a validation error).
function parseReasoningEffortMapping(json: string): Record<string, string> | null {
  const trimmed = json.trim();
  if (trimmed === '') {
    return null;
  }
  const parsed = JSON.parse(trimmed);
  if (typeof parsed !== 'object' || parsed === null || Array.isArray(parsed)) {
    throw new Error('must be a JSON object like {"xhigh": "max"}');
  }
  const result: Record<string, string> = {};
  for (const [k, v] of Object.entries(parsed)) {
    if (typeof v !== 'string') {
      throw new Error(`value for key "${k}" must be a string`);
    }
    result[k] = v;
  }
  return result;
}

export function ChannelsTransformOptionsDialog({ open, onOpenChange, currentRow }: Props) {
  const { t } = useTranslation();
  const updateChannel = useUpdateChannel();

  const form = useForm<TransformOptionsFormValues>({
    resolver: zodResolver(transformOptionsFormSchema),
    defaultValues: {
      forceArrayInstructions: currentRow.settings?.transformOptions?.forceArrayInstructions || false,
      forceArrayInputs: currentRow.settings?.transformOptions?.forceArrayInputs || false,
      replaceDeveloperRoleWithSystem: currentRow.settings?.transformOptions?.replaceDeveloperRoleWithSystem || false,
      reasoningEffortMappingJSON: reasoningEffortMappingToJSON(currentRow.settings?.transformOptions?.reasoningEffortMapping),
    },
  });

  useEffect(() => {
    if (open) {
      form.reset({
        forceArrayInstructions: currentRow.settings?.transformOptions?.forceArrayInstructions || false,
        forceArrayInputs: currentRow.settings?.transformOptions?.forceArrayInputs || false,
        replaceDeveloperRoleWithSystem: currentRow.settings?.transformOptions?.replaceDeveloperRoleWithSystem || false,
        reasoningEffortMappingJSON: reasoningEffortMappingToJSON(currentRow.settings?.transformOptions?.reasoningEffortMapping),
      });
    }
  }, [open, currentRow, form]);

  const onSubmit = async (values: TransformOptionsFormValues) => {
    let reasoningEffortMapping: Record<string, string> | null;
    try {
      reasoningEffortMapping = parseReasoningEffortMapping(values.reasoningEffortMappingJSON || '');
    } catch (e) {
      toast.error(t('channels.dialogs.fields.transformOptions.reasoningEffortMapping.invalid', { error: (e as Error).message }));
      return;
    }

    try {
      const transformOptions: TransformOptions = {
        forceArrayInstructions: values.forceArrayInstructions,
        forceArrayInputs: values.forceArrayInputs,
        replaceDeveloperRoleWithSystem: values.replaceDeveloperRoleWithSystem,
      };
      if (reasoningEffortMapping) {
        transformOptions.reasoningEffortMapping = reasoningEffortMapping;
      }
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
                    name='reasoningEffortMappingJSON'
                    render={({ field }) => (
                      <FormItem className='space-y-1'>
                        <FormLabel className='text-sm font-normal'>
                          {t('channels.dialogs.fields.transformOptions.reasoningEffortMapping.label')}
                        </FormLabel>
                        <p className='text-muted-foreground text-xs'>
                          {t('channels.dialogs.fields.transformOptions.reasoningEffortMapping.description')}
                        </p>
                        <FormControl>
                          <textarea
                            {...field}
                            value={field.value || ''}
                            placeholder='{"xhigh": "max"}'
                            className='flex min-h-[80px] w-full rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2'
                          />
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
