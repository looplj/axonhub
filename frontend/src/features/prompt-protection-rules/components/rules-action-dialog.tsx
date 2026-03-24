import { useCallback, useEffect, useMemo } from 'react';
import { useForm } from 'react-hook-form';
import { z } from 'zod';
import { zodResolver } from '@hookform/resolvers/zod';
import { useTranslation } from 'react-i18next';
import { Button } from '@/components/ui/button';
import { Checkbox } from '@/components/ui/checkbox';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Form, FormControl, FormField, FormItem, FormLabel, FormMessage } from '@/components/ui/form';
import { Input } from '@/components/ui/input';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Textarea } from '@/components/ui/textarea';
import { usePromptProtectionRules } from '../context/rules-context';
import { useCreatePromptProtectionRule, useUpdatePromptProtectionRule } from '../data/rules';

const defaultValues = {
  name: '',
  description: '',
  pattern: '',
  action: 'mask' as const,
  replacement: '[MASKED]',
  scopes: ['user'],
};

const createFormSchema = (t: ReturnType<typeof useTranslation>['t']) =>
  z
    .object({
      name: z.string().min(1, t('promptProtectionRules.validation.nameRequired')),
      description: z.string().optional(),
      pattern: z.string().min(1, t('promptProtectionRules.validation.patternRequired')),
      action: z.enum(['mask', 'reject']),
      replacement: z.string().optional(),
      scopes: z.array(z.string()).default([]),
    })
    .superRefine((val, ctx) => {
      if (val.action === 'mask' && !val.replacement) {
        ctx.addIssue({
          code: z.ZodIssueCode.custom,
          path: ['replacement'],
          message: t('promptProtectionRules.validation.replacementRequired'),
        });
      }
    });

type FormData = z.infer<ReturnType<typeof createFormSchema>>;

const scopeOptions = ['system', 'developer', 'user', 'assistant', 'tool'] as const;

export function RulesActionDialog() {
  const { t } = useTranslation();
  const { open, setOpen, currentRow, setCurrentRow, resetRowSelection } = usePromptProtectionRules();
  const createMutation = useCreatePromptProtectionRule();
  const updateMutation = useUpdatePromptProtectionRule();

  const isEdit = open === 'edit';
  const isOpen = open === 'create' || open === 'edit';
  const formSchema = useMemo(() => createFormSchema(t), [t]);

  const form = useForm<FormData>({
    resolver: zodResolver(formSchema),
    defaultValues,
  });

  useEffect(() => {
    if (!isOpen) {
      return;
    }

    if (isEdit && currentRow) {
      form.reset({
        name: currentRow.name,
        description: currentRow.description || '',
        pattern: currentRow.pattern,
        action: currentRow.settings.action,
        replacement: currentRow.settings.replacement || '',
        scopes: currentRow.settings.scopes || [],
      });
      return;
    }

    form.reset(defaultValues);
  }, [currentRow, form, isEdit, isOpen]);

  const action = form.watch('action');

  const handleOpenChange = useCallback(
    (nextOpen: boolean) => {
      if (!nextOpen) {
        setOpen(null);
        setCurrentRow(null);
        form.reset(defaultValues);
      }
    },
    [form, setCurrentRow, setOpen]
  );

  const onSubmit = useCallback(
    async (values: FormData) => {
      const input = {
        name: values.name,
        description: values.description || '',
        pattern: values.pattern,
        settings: {
          action: values.action,
          replacement: values.action === 'mask' ? values.replacement || '' : undefined,
          scopes: values.scopes,
        },
      };

      if (isEdit && currentRow) {
        await updateMutation.mutateAsync({
          id: currentRow.id,
          input,
        });
      } else {
        await createMutation.mutateAsync(input);
      }

      setOpen(null);
      setCurrentRow(null);
      resetRowSelection?.();
      form.reset(defaultValues);
    },
    [createMutation, currentRow, form, isEdit, resetRowSelection, setCurrentRow, setOpen, updateMutation]
  );

  return (
    <Dialog open={isOpen} onOpenChange={handleOpenChange}>
      <DialogContent className='sm:max-w-[680px]'>
        <DialogHeader>
          <DialogTitle>{isEdit ? t('promptProtectionRules.dialogs.edit.title') : t('promptProtectionRules.dialogs.create.title')}</DialogTitle>
          <DialogDescription>
            {isEdit ? t('promptProtectionRules.dialogs.edit.description') : t('promptProtectionRules.dialogs.create.description')}
          </DialogDescription>
        </DialogHeader>

        <Form {...form}>
          <form onSubmit={form.handleSubmit(onSubmit)} className='space-y-4'>
            <FormField
              control={form.control}
              name='name'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('promptProtectionRules.fields.name')}</FormLabel>
                  <FormControl>
                    <Input {...field} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='description'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('common.columns.description')}</FormLabel>
                  <FormControl>
                    <Input {...field} value={field.value ?? ''} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='pattern'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('promptProtectionRules.fields.pattern')}</FormLabel>
                  <FormControl>
                    <Textarea {...field} rows={4} className='font-mono text-xs' />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />

            <div className='grid gap-4 md:grid-cols-2'>
              <FormField
                control={form.control}
                name='action'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('promptProtectionRules.fields.action')}</FormLabel>
                    <Select onValueChange={field.onChange} value={field.value}>
                      <FormControl>
                        <SelectTrigger>
                          <SelectValue />
                        </SelectTrigger>
                      </FormControl>
                      <SelectContent>
                        <SelectItem value='mask'>{t('promptProtectionRules.actions.mask')}</SelectItem>
                        <SelectItem value='reject'>{t('promptProtectionRules.actions.reject')}</SelectItem>
                      </SelectContent>
                    </Select>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='replacement'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('promptProtectionRules.fields.replacement')}</FormLabel>
                    <FormControl>
                      <Input {...field} value={field.value ?? ''} disabled={action !== 'mask'} />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </div>

            <FormField
              control={form.control}
              name='scopes'
              render={() => (
                <FormItem>
                  <FormLabel>{t('promptProtectionRules.fields.scopes')}</FormLabel>
                  <div className='grid gap-2 md:grid-cols-5'>
                    {scopeOptions.map((scope) => (
                      <FormField
                        key={scope}
                        control={form.control}
                        name='scopes'
                        render={({ field }) => (
                          <FormItem className='flex items-center gap-2 rounded-md border px-3 py-2'>
                            <FormControl>
                              <Checkbox
                                checked={field.value?.includes(scope)}
                                onCheckedChange={(checked) => {
                                  const next = checked
                                    ? [...field.value, scope]
                                    : field.value.filter((value) => value !== scope);
                                  field.onChange(next);
                                }}
                              />
                            </FormControl>
                            <FormLabel className='m-0 cursor-pointer text-sm font-normal'>{t(`promptProtectionRules.scopes.${scope}`)}</FormLabel>
                          </FormItem>
                        )}
                      />
                    ))}
                  </div>
                  <FormMessage />
                </FormItem>
              )}
            />

            <DialogFooter>
              <Button type='button' variant='outline' onClick={() => handleOpenChange(false)}>
                {t('common.buttons.cancel')}
              </Button>
              <Button type='submit' disabled={createMutation.isPending || updateMutation.isPending}>
                {isEdit ? t('common.buttons.update') : t('common.buttons.create')}
              </Button>
            </DialogFooter>
          </form>
        </Form>
      </DialogContent>
    </Dialog>
  );
}
