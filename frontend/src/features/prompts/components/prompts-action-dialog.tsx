import { useEffect, useCallback, useState, useRef, useMemo } from 'react';
import { useForm, useFieldArray, useWatch } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { useTranslation } from 'react-i18next';
import { z } from 'zod';
import { IconPlus, IconTrash } from '@tabler/icons-react';
import { Button } from '@/components/ui/button';
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle, DialogFooter } from '@/components/ui/dialog';
import { Form, FormControl, FormDescription, FormField, FormItem, FormLabel, FormMessage } from '@/components/ui/form';
import { Input } from '@/components/ui/input';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Textarea } from '@/components/ui/textarea';
import { AutoComplete } from '@/components/auto-complete';
import { useQueryModels } from '@/gql/models';
import { usePrompts } from '../context/prompts-context';
import { useCreatePrompt, useUpdatePrompt } from '../data/prompts';
import { CreatePromptInput, UpdatePromptInput } from '../data/schema';
import { useSelectedProjectId } from '@/stores/projectStore';

const conditionSchema = z.object({
  type: z.enum(['model_id', 'model_pattern']),
  value: z.string().min(1, 'Condition value is required'),
});

const conditionGroupSchema = z.object({
  conditions: z.array(conditionSchema).min(1, 'At least one condition is required per group'),
});

const createPromptSchema = z.object({
  name: z.string().min(1, 'Name is required'),
  description: z.string().optional(),
  role: z.string().min(1, 'Role is required'),
  content: z.string().min(1, 'Content is required'),
  actionType: z.enum(['prepend', 'append']),
  conditionGroups: z.array(conditionGroupSchema).optional(),
});

const updatePromptSchema = z.object({
  name: z.string().min(1, 'Name is required').optional(),
  description: z.string().optional(),
  role: z.string().optional(),
  content: z.string().optional(),
  actionType: z.enum(['prepend', 'append']).optional(),
  conditionGroups: z.array(conditionGroupSchema).optional(),
});

type FormData = z.infer<typeof createPromptSchema>;

interface ModelAutoCompleteWrapperProps {
  field: any;
  modelOptions: Array<{ value: string; label: string }>;
  portalContainer: HTMLDivElement | null;
}

function ModelAutoCompleteWrapper({ field, modelOptions,  portalContainer }: ModelAutoCompleteWrapperProps) {
  const { t } = useTranslation();
  const [searchValue, setSearchValue] = useState('');

  useEffect(() => {
    const selectedOption = modelOptions.find(opt => opt.value === field.value);
    setSearchValue(selectedOption?.label || field.value || '');
  }, [field.value, modelOptions]);

  return (
    <AutoComplete
      selectedValue={field.value || ''}
      onSelectedValueChange={field.onChange}
      searchValue={searchValue}
      onSearchValueChange={setSearchValue}
      items={modelOptions}
      placeholder={t('prompts.fields.conditionValuePlaceholder')}
      emptyMessage={t('prompts.fields.noModels')}
      portalContainer={portalContainer}
    />
  );
}

interface ConditionGroupProps {
  groupIndex: number;
  form: any;
  onRemoveGroup: () => void;
  t: any;
  modelOptions: Array<{ value: string; label: string }>;
  dialogContentRef: React.RefObject<HTMLDivElement | null>;
}

function ConditionGroup({ groupIndex, form, onRemoveGroup, t, modelOptions, dialogContentRef }: ConditionGroupProps) {
  const { fields, append, remove } = useFieldArray({
    control: form.control,
    name: `conditionGroups.${groupIndex}.conditions`,
  });

  const handleAddCondition = useCallback(() => {
    append({ type: 'model_id', value: '' });
  }, [append]);

  const conditions = useWatch({
    control: form.control,
    name: `conditionGroups.${groupIndex}.conditions`,
  });

  return (
    <div className='rounded-lg border bg-muted/30 p-3'>
      <div className='mb-2 flex items-center justify-between'>
        <span className='text-xs font-medium text-muted-foreground'>
          {t('prompts.conditions.group')} {groupIndex + 1}
        </span>
        <div className='flex gap-1'>
          <Button
            type='button'
            variant='ghost'
            size='sm'
            onClick={handleAddCondition}
            className='h-7 px-2 text-xs'
          >
            <IconPlus className='mr-1 h-3 w-3' />
            {t('prompts.conditions.add')}
          </Button>
          <Button
            type='button'
            variant='ghost'
            size='sm'
            onClick={onRemoveGroup}
            className='h-7 px-2 text-xs text-muted-foreground hover:text-destructive'
          >
            <IconTrash className='h-3 w-3' />
          </Button>
        </div>
      </div>
      <div className='space-y-3'>
        {fields.map((field, conditionIndex) => (
          <div key={field.id}>
            {conditionIndex > 0 && (
              <div className='relative mb-3 mt-1'>
                <div className='absolute inset-0 flex items-center' aria-hidden='true'>
                  <div className='w-full border-t border-dashed'></div>
                </div>
                <div className='relative flex justify-center'>
                  <span className='bg-muted/50 px-2 text-[10px] font-semibold uppercase tracking-wider text-muted-foreground'>
                    {t('prompts.conditions.or')}
                  </span>
                </div>
              </div>
            )}
            <div className='grid grid-cols-[10rem_1fr_2.5rem] items-center gap-2'>
              <FormField
                 control={form.control}
                 name={`conditionGroups.${groupIndex}.conditions.${conditionIndex}.type`}
                 render={({ field }) => (
                   <FormItem className='space-y-0'>
                     <Select onValueChange={field.onChange} value={field.value}>
                       <FormControl>
                         <SelectTrigger className='h-10 w-full text-xs'>
                           <SelectValue />
                         </SelectTrigger>
                       </FormControl>
                       <SelectContent>
                         <SelectItem value='model_id'>{t('prompts.conditionTypes.model_id')}</SelectItem>
                         <SelectItem value='model_pattern'>{t('prompts.conditionTypes.model_pattern')}</SelectItem>
                       </SelectContent>
                     </Select>
                   </FormItem>
                 )}
               />
               <FormField
                 control={form.control}
                 name={`conditionGroups.${groupIndex}.conditions.${conditionIndex}.value`}
                 render={({ field }) => (
                   <FormItem className='space-y-0'>
                     <FormControl>
                       {conditions?.[conditionIndex]?.type === 'model_id' ? (
                         <ModelAutoCompleteWrapper
                           field={field}
                           modelOptions={modelOptions}
                           portalContainer={dialogContentRef.current}
                         />
                       ) : (
                         <Input
                           {...field}
                           placeholder={t('prompts.fields.conditionValuePlaceholder')}
                           className='h-10 text-xs'
                         />
                       )}
                     </FormControl>
                   </FormItem>
                 )}
               />
               <Button
                 type='button'
                 variant='ghost'
                 size='icon'
                 onClick={() => remove(conditionIndex)}
                 className='h-10 w-10 text-muted-foreground hover:text-destructive'
               >
                 <IconTrash className='h-4 w-4' />
               </Button>
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}

export function PromptsActionDialog() {
  const { t } = useTranslation();
  const { open, setOpen, currentRow } = usePrompts();
  const createPrompt = useCreatePrompt();
  const updatePrompt = useUpdatePrompt();
  const selectedProjectId = useSelectedProjectId();
  const { data: availableModels, mutateAsync: fetchModels } = useQueryModels();
  const dialogContentRef = useRef<HTMLDivElement>(null);

  const isEdit = open === 'edit';
  const isOpen = open === 'create' || open === 'edit';

  const modelOptions = useMemo(() => {
    if (!availableModels) return [];
    return availableModels.map((model) => ({
      value: model.id,
      label: model.id,
    }));
  }, [availableModels]);

  const form = useForm<FormData>({
    resolver: zodResolver(isEdit ? updatePromptSchema : createPromptSchema) as any,
    defaultValues: {
      name: '',
      description: '',
      role: 'system',
      content: '',
      actionType: 'prepend',
      conditionGroups: [],
    },
  });

  const { fields: groupFields, append: appendGroup, remove: removeGroup } = useFieldArray({
    control: form.control,
    name: 'conditionGroups',
  });

  useEffect(() => {
    if (isOpen) {
      fetchModels({
        statusIn: ['enabled'],
        includeMapping: true,
        includePrefix: true,
      });
    }
  }, [isOpen, fetchModels]);

  useEffect(() => {
    if (isEdit && currentRow) {
      const conditionGroups = currentRow.settings?.conditions?.map((composite: any) => ({
        conditions: composite.conditions?.map((condition: any) => ({
          type: condition.type,
          value: condition.modelId || condition.modelPattern || '',
        })) || []
      })) || [];
      form.reset({
        name: currentRow.name,
        description: currentRow.description || '',
        role: currentRow.role,
        content: currentRow.content,
        actionType: currentRow.settings?.action?.type || 'prepend',
        conditionGroups,
      });
    } else if (!isEdit) {
      form.reset({
        name: '',
        description: '',
        role: 'system',
        content: '',
        actionType: 'prepend',
        conditionGroups: [],
      });
    }
  }, [isEdit, currentRow, form, isOpen]);

  const onSubmit = useCallback(
    async (data: FormData) => {
      if (!selectedProjectId) return;

      const conditions = data.conditionGroups?.map(group => ({
        conditions: group.conditions.map((condition) => ({
          type: condition.type,
          ...(condition.type === 'model_id' ? { modelId: condition.value } : { modelPattern: condition.value }),
        }))
      })) || [];

      if (isEdit && currentRow) {
        const input: UpdatePromptInput = {
          name: data.name,
          description: data.description,
          role: data.role,
          content: data.content,
          settings: {
            action: { type: data.actionType },
            conditions,
          },
        };
        await updatePrompt.mutateAsync({ id: currentRow.id, input });
      } else {
        const input: CreatePromptInput = {
          name: data.name!,
          description: data.description,
          role: data.role!,
          content: data.content!,
          settings: {
            action: { type: data.actionType! },
            conditions,
          },
        };
        await createPrompt.mutateAsync(input);
      }

      setOpen(null);
      form.reset();
    },
    [isEdit, currentRow, selectedProjectId, createPrompt, updatePrompt, setOpen, form]
  );

  const handleClose = useCallback(() => {
    setOpen(null);
    form.reset();
  }, [setOpen, form]);

  const handleAddGroup = useCallback(() => {
    appendGroup({ conditions: [{ type: 'model_id', value: '' }] });
  }, [appendGroup]);

  return (
    <Dialog open={isOpen} onOpenChange={(open) => !open && handleClose()}>
      <DialogContent className='flex h-[85vh] max-h-[85vh] flex-col overflow-hidden sm:max-w-6xl' ref={dialogContentRef}>
        <DialogHeader className='flex-shrink-0'>
          <DialogTitle>{isEdit ? t('prompts.dialogs.edit.title') : t('prompts.dialogs.create.title')}</DialogTitle>
          <DialogDescription>
            {isEdit ? t('prompts.dialogs.edit.description') : t('prompts.dialogs.create.description')}
          </DialogDescription>
        </DialogHeader>

        <Form {...form}>
          <form onSubmit={form.handleSubmit(onSubmit)} className='flex flex-1 flex-col overflow-hidden'>
            <div className='flex flex-1 gap-6 overflow-hidden pb-4'>
              <div className='w-1/3 flex-shrink-0 space-y-4 overflow-y-auto pr-2'>
            <FormField
              control={form.control}
              name='name'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('prompts.fields.name')}</FormLabel>
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
                  <FormLabel>{t('prompts.fields.description')}</FormLabel>
                  <FormControl>
                    <Input {...field} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='role'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('prompts.fields.role')}</FormLabel>
                  <Select onValueChange={field.onChange} value={field.value}>
                    <FormControl>
                      <SelectTrigger>
                        <SelectValue placeholder={t('prompts.fields.rolePlaceholder')} />
                      </SelectTrigger>
                    </FormControl>
                    <SelectContent>
                      <SelectItem value='system'>system</SelectItem>
                      <SelectItem value='user'>user</SelectItem>
                      <SelectItem value='assistant'>assistant</SelectItem>
                    </SelectContent>
                  </Select>
                  <FormMessage />
                </FormItem>
              )}
            />

                <FormField
                  control={form.control}
                  name='actionType'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('prompts.fields.actionType')}</FormLabel>
                      <Select onValueChange={field.onChange} value={field.value}>
                        <FormControl>
                          <SelectTrigger>
                            <SelectValue placeholder={t('prompts.fields.actionTypePlaceholder')} />
                          </SelectTrigger>
                        </FormControl>
                        <SelectContent>
                          <SelectItem value='prepend'>{t('prompts.actionTypes.prepend')}</SelectItem>
                          <SelectItem value='append'>{t('prompts.actionTypes.append')}</SelectItem>
                        </SelectContent>
                      </Select>
                      <FormDescription>
                        {field.value === 'prepend' 
                          ? t('prompts.fields.actionTypeHelpPrepend') 
                          : t('prompts.fields.actionTypeHelpAppend')}
                      </FormDescription>
                      <FormMessage />
                    </FormItem>
                  )}
                />
              </div>

              <div className='flex flex-1 flex-col space-y-4 overflow-hidden'>
                <FormField
                  control={form.control}
                  name='content'
                  render={({ field }) => (
                    <FormItem className='flex min-h-0 flex-1 flex-col'>
                      <FormLabel>{t('prompts.fields.content')}</FormLabel>
                      <FormControl className='flex-1'>
                        <Textarea
                          {...field}
                          placeholder={t('prompts.fields.contentPlaceholder')}
                          className='h-full min-h-0 resize-none'
                        />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />

                <div className='bg-background flex min-h-0 flex-[2] flex-col overflow-hidden rounded-xl border'>
                  <div className='flex-shrink-0 p-4 pb-2'>
                    <div className='flex items-center justify-between'>
                      <div className='space-y-1'>
                        <h3 className='text-sm font-medium leading-none'>{t('prompts.conditions.title')}</h3>
                        <p className='text-muted-foreground text-xs'>{t('prompts.conditions.description')}</p>
                      </div>
                      <Button type='button' variant='outline' size='sm' onClick={handleAddGroup} className='h-8'>
                        <IconPlus className='mr-1 h-3.5 w-3.5' />
                        {t('prompts.conditions.addGroup')}
                      </Button>
                    </div>
                  </div>
                  <div className='flex-1 overflow-y-auto px-4 pb-4'>
                    {groupFields.length === 0 ? (
                      <div className='text-muted-foreground rounded-lg border border-dashed bg-muted/20 p-8 text-center text-sm'>
                        {t('prompts.conditions.empty')}
                      </div>
                    ) : (
                      <div className='space-y-3'>
                        {groupFields.map((groupField, groupIndex) => (
                          <div key={groupField.id} className='space-y-3'>
                            {groupIndex > 0 && (
                              <div className='relative py-2'>
                                <div className='absolute inset-0 flex items-center' aria-hidden='true'>
                                  <div className='w-full border-t border-muted-foreground/20'></div>
                                </div>
                                <div className='relative flex justify-center'>
                                  <span className='bg-background px-3 text-xs font-bold uppercase tracking-widest text-primary'>
                                    {t('prompts.conditions.and')}
                                  </span>
                                </div>
                              </div>
                            )}
                            <ConditionGroup
                              groupIndex={groupIndex}
                              form={form}
                              onRemoveGroup={() => removeGroup(groupIndex)}
                              t={t}
                              modelOptions={modelOptions}
                              dialogContentRef={dialogContentRef}
                            />
                          </div>
                        ))}
                      </div>
                    )}
                  </div>
                </div>
              </div>
            </div>

            <DialogFooter className='flex-shrink-0 border-t pt-4'>
              <Button type='button' variant='outline' onClick={handleClose}>
                {t('common.buttons.cancel')}
              </Button>
              <Button type='submit' disabled={createPrompt.isPending || updatePrompt.isPending}>
                {isEdit ? t('common.buttons.save') : t('common.buttons.create')}
              </Button>
            </DialogFooter>
          </form>
        </Form>
      </DialogContent>
    </Dialog>
  );
}
