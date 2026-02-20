import { useEffect, useMemo } from 'react';
import { useForm, useFieldArray } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { z } from 'zod';
import { useTranslation } from 'react-i18next';
import { IconPlus, IconTrash } from '@tabler/icons-react';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Textarea } from '@/components/ui/textarea';
import { Form, FormControl, FormField, FormItem, FormLabel, FormMessage } from '@/components/ui/form';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { useAgents } from '../context/agents-context';
import { useCreateAgent, useUpdateAgent } from '../data/agents';
import type { AgentBuiltinToolInput } from '../data/schema';
import { builtinToolOptions } from '../data/agent-tools';

const builtinToolSchema = z.object({
  name: z.string().min(1),
  enabled: z.boolean().default(true),
  order: z.coerce.number().int().default(0),
});

const createAgentSchema = z.object({
  name: z.string().min(1),
  description: z.string().optional(),
  status: z.enum(['enabled', 'disabled', 'archived']).optional(),
  model: z.string().optional(),
  systemPrompt: z.string().min(1),
  skillsPolicyAdd: z.enum(['open', 'approval_required', 'registry_only']).default('open'),
  builtinTools: z.array(builtinToolSchema).optional(),
});

const updateAgentSchema = z.object({
  name: z.string().optional(),
  description: z.string().optional(),
  status: z.enum(['enabled', 'disabled', 'archived']).optional(),
  model: z.string().optional(),
  systemPrompt: z.string().optional(),
  skillsPolicyAdd: z.enum(['open', 'approval_required', 'registry_only']).optional(),
  builtinTools: z.array(builtinToolSchema).optional(),
});

type FormData = z.infer<typeof createAgentSchema>;

export function AgentsActionDialog() {
  const { t } = useTranslation();
  const { open, setOpen, currentRow, setCurrentRow } = useAgents();
  const createAgent = useCreateAgent();
  const updateAgent = useUpdateAgent();

  const isEdit = open === 'edit';
  const isOpen = open === 'create' || open === 'edit';

  const form = useForm<FormData>({
    resolver: zodResolver(isEdit ? updateAgentSchema : createAgentSchema) as any,
    defaultValues: {
      name: '',
      description: '',
      status: 'enabled',
      model: '',
      systemPrompt: '',
      skillsPolicyAdd: 'open',
      builtinTools: [
        { name: 'read', enabled: true, order: 0 },
        { name: 'write', enabled: true, order: 0 },
      ],
    },
  });

  const { fields, append, remove } = useFieldArray({
    control: form.control,
    name: 'builtinTools',
  });

  useEffect(() => {
    if (!isOpen) return;
    if (isEdit && currentRow) {
      const existingBuiltinTools = Array.isArray(currentRow.agentBuiltinTools) ? currentRow.agentBuiltinTools : [];
      const existingSkillsPolicyAdd =
        currentRow.skillsPolicy && typeof currentRow.skillsPolicy === 'object' && 'add' in currentRow.skillsPolicy
          ? (currentRow.skillsPolicy as any).add
          : 'open';

      form.reset({
        name: currentRow.name,
        description: currentRow.description || '',
        status: currentRow.status,
        model: currentRow.model || '',
        systemPrompt: currentRow.prompt?.content || '',
        skillsPolicyAdd: existingSkillsPolicyAdd,
        builtinTools: existingBuiltinTools,
      });
    } else {
      form.reset();
    }
  }, [isOpen, isEdit, currentRow, form]);

  const onSubmit = async (values: FormData) => {
    const builtinTools = (values.builtinTools || []) as AgentBuiltinToolInput[];
    const skillsPolicy = values.skillsPolicyAdd ? { add: values.skillsPolicyAdd } : undefined;

    if (isEdit && currentRow) {
      await updateAgent.mutateAsync({
        id: currentRow.id,
        input: {
          name: values.name,
          description: values.description,
          status: values.status,
          model: values.model,
          systemPrompt: values.systemPrompt,
          builtinTools,
          skillsPolicy,
        },
      });
      setOpen(null);
      setCurrentRow(null);
      return;
    }

    await createAgent.mutateAsync({
      name: values.name,
      description: values.description,
      status: values.status,
      model: values.model,
      systemPrompt: values.systemPrompt,
      builtinTools,
      skillsPolicy,
    });

    setOpen(null);
    setCurrentRow(null);
  };

  const toolNameOptions = useMemo(() => builtinToolOptions, []);

  return (
    <Dialog
      open={isOpen}
      onOpenChange={(next) => {
        if (!next) {
          setOpen(null);
          setCurrentRow(null);
        }
      }}
    >
      <DialogContent className='max-h-[90vh] overflow-y-auto sm:max-w-2xl'>
        <DialogHeader>
          <DialogTitle>{isEdit ? t('agents.dialogs.edit.title') : t('agents.dialogs.create.title')}</DialogTitle>
          <DialogDescription>{isEdit ? t('agents.dialogs.edit.description') : t('agents.dialogs.create.description')}</DialogDescription>
        </DialogHeader>

        <Form {...form}>
          <form onSubmit={form.handleSubmit(onSubmit)} className='space-y-4'>
            <div className='grid grid-cols-2 gap-4'>
              <FormField
                control={form.control}
                name='name'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('agents.fields.name')}</FormLabel>
                    <FormControl>
                      <Input {...field} placeholder={t('agents.fields.namePlaceholder')} />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='status'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('common.columns.status')}</FormLabel>
                    <Select onValueChange={field.onChange} value={field.value}>
                      <FormControl>
                        <SelectTrigger>
                          <SelectValue />
                        </SelectTrigger>
                      </FormControl>
                      <SelectContent>
                        <SelectItem value='enabled'>{t('common.buttons.enable')}</SelectItem>
                        <SelectItem value='disabled'>{t('common.buttons.disable')}</SelectItem>
                        <SelectItem value='archived'>{t('common.buttons.archive')}</SelectItem>
                      </SelectContent>
                    </Select>
                  </FormItem>
                )}
              />
            </div>

            <FormField
              control={form.control}
              name='description'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('agents.fields.description')}</FormLabel>
                  <FormControl>
                    <Input {...field} placeholder={t('agents.fields.descriptionPlaceholder')} />
                  </FormControl>
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='model'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('agents.fields.model')}</FormLabel>
                  <FormControl>
                    <Input {...field} placeholder={t('agents.fields.modelPlaceholder')} />
                  </FormControl>
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='systemPrompt'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('agents.fields.systemPrompt')}</FormLabel>
                  <FormControl>
                    <Textarea {...field} placeholder={t('agents.fields.systemPromptPlaceholder')} className='min-h-32' />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='skillsPolicyAdd'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('agents.fields.skillsPolicy')}</FormLabel>
                  <Select onValueChange={field.onChange} value={field.value}>
                    <FormControl>
                      <SelectTrigger>
                        <SelectValue />
                      </SelectTrigger>
                    </FormControl>
                    <SelectContent>
                      <SelectItem value='open'>{t('agents.skillsPolicy.open')}</SelectItem>
                      <SelectItem value='approval_required'>{t('agents.skillsPolicy.approvalRequired')}</SelectItem>
                      <SelectItem value='registry_only'>{t('agents.skillsPolicy.registryOnly')}</SelectItem>
                    </SelectContent>
                  </Select>
                </FormItem>
              )}
            />

            <div className='space-y-2'>
              <div className='flex items-center justify-between'>
                <div className='text-sm font-medium'>{t('agents.fields.builtinTools')}</div>
                <Button
                  type='button'
                  variant='outline'
                  size='sm'
                  onClick={() => append({ name: 'read', enabled: true, order: 0 })}
                  className='gap-2'
                >
                  <IconPlus className='h-4 w-4' />
                  {t('agents.actions.addTool')}
                </Button>
              </div>

              <div className='space-y-2'>
                {fields.map((field, index) => (
                  <div key={field.id} className='grid grid-cols-[1fr_6rem_3rem] items-center gap-2'>
                    <FormField
                      control={form.control}
                      name={`builtinTools.${index}.name`}
                      render={({ field }) => (
                        <FormItem className='space-y-0'>
                          <Select onValueChange={field.onChange} value={field.value}>
                            <FormControl>
                              <SelectTrigger className='h-10'>
                                <SelectValue />
                              </SelectTrigger>
                            </FormControl>
                            <SelectContent>
                              {toolNameOptions.map((opt) => (
                                <SelectItem key={opt} value={opt}>
                                  {opt}
                                </SelectItem>
                              ))}
                            </SelectContent>
                          </Select>
                        </FormItem>
                      )}
                    />

                    <FormField
                      control={form.control}
                      name={`builtinTools.${index}.order`}
                      render={({ field }) => (
                        <FormItem className='space-y-0'>
                          <FormControl>
                            <Input {...field} type='number' className='h-10' />
                          </FormControl>
                        </FormItem>
                      )}
                    />

                    <Button type='button' variant='ghost' size='icon' onClick={() => remove(index)} className='h-10 w-10'>
                      <IconTrash className='h-4 w-4' />
                    </Button>
                  </div>
                ))}
              </div>
            </div>

            <DialogFooter>
              <Button type='button' variant='outline' onClick={() => setOpen(null)}>
                {t('common.buttons.cancel')}
              </Button>
              <Button type='submit' disabled={createAgent.isPending || updateAgent.isPending}>
                {isEdit ? t('common.buttons.saveChanges') : t('common.buttons.create')}
              </Button>
            </DialogFooter>
          </form>
        </Form>
      </DialogContent>
    </Dialog>
  );
}
