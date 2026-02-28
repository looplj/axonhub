import { useEffect, useState, useMemo } from 'react';
import { useForm, useFieldArray } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { z } from 'zod';
import { useTranslation } from 'react-i18next';
import { useNavigate, useParams } from '@tanstack/react-router';
import { ArrowLeft, Plus, Trash2, Info, Terminal, Settings, GripVertical, Key, Eye, EyeOff } from 'lucide-react';
import { DndContext, closestCenter, KeyboardSensor, PointerSensor, useSensor, useSensors, type DragEndEvent } from '@dnd-kit/core';
import { arrayMove, SortableContext, sortableKeyboardCoordinates, verticalListSortingStrategy } from '@dnd-kit/sortable';
import { useSortable } from '@dnd-kit/sortable';
import { CSS } from '@dnd-kit/utilities';
import { Header } from '@/components/layout/header';
import { Main } from '@/components/layout/main';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Textarea } from '@/components/ui/textarea';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Separator } from '@/components/ui/separator';
import { Form, FormControl, FormField, FormItem, FormLabel, FormMessage } from '@/components/ui/form';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import { Switch } from '@/components/ui/switch';
import { AutoComplete } from '@/components/auto-complete';
import { CopyButton } from '@/components/ui/copy-button';
import { useQueryModels } from '@/gql/models';
import { useCreateAgent, useUpdateAgent } from '../data/agents';
import { useAgentDetail } from '../data/agent-detail';
import type { AgentBuiltinToolInput } from '../data/schema';
import { builtinToolOptions, builtinToolDefaultEnabled } from '../data/agent-tools';
import { defaultSystemPrompt } from '../data/prompt';


const builtinToolSchema = z.object({
  name: z.string().min(1),
  enabled: z.boolean(),
  order: z.number().int(),
});

const agentFormSchema = z.object({
  name: z.string().min(1),
  description: z.string().optional(),
  model: z.string().min(1, 'Model is required'),
  systemPrompt: z.string().min(1),
  skillsPolicyAdd: z.enum(['open', 'approval_required', 'registry_only']),
  builtinTools: z.array(builtinToolSchema),
});

type FormData = z.infer<typeof agentFormSchema>;

interface AgentFormPageProps {
  mode: 'create' | 'edit';
}

interface SortableToolItemProps {
  field: { id: string; name: string; enabled: boolean; order: number };
  index: number;
  availableOptions: string[];
  onRemove: (index: number) => void;
  form: any;
  t: (key: string) => string;
}

function SortableToolItem({ field, index, availableOptions, onRemove, form, t }: SortableToolItemProps) {
  const { attributes, listeners, setNodeRef, transform, transition, isDragging } = useSortable({ id: field.id });

  const style = {
    transform: CSS.Transform.toString(transform),
    transition,
    opacity: isDragging ? 0.5 : 1,
  };

  const currentName = form.getValues(`builtinTools.${index}.name`);
  const description = t(`agents.toolDescriptions.${currentName}`);

  return (
    <div
      ref={setNodeRef}
      style={style}
      className={`group flex items-center gap-2 rounded-md border p-2 hover:shadow-sm ${
        isDragging ? 'ring-primary/20 relative z-50 shadow-xl ring-2' : 'hover:border-primary/20'
      }`}
    >
      {/* Drag Handle */}
      <div
        className='text-muted-foreground hover:text-foreground flex cursor-grab items-center active:cursor-grabbing'
        {...attributes}
        {...listeners}
      >
        <GripVertical className='h-4 w-4' />
      </div>

      {/* Tool Select with Tooltip */}
      <Tooltip>
        <TooltipTrigger asChild>
          <div className='flex flex-1'>
            <FormField
              control={form.control}
              name={`builtinTools.${index}.name`}
              render={({ field }) => (
                <FormItem className='w-full space-y-0'>
                  <Select onValueChange={field.onChange} value={field.value}>
                    <FormControl>
                      <SelectTrigger className='h-9 w-full'>
                        <SelectValue />
                      </SelectTrigger>
                    </FormControl>
                    <SelectContent>
                      {availableOptions.map((opt) => (
                        <SelectItem key={opt} value={opt}>
                          {opt}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </FormItem>
              )}
            />
          </div>
        </TooltipTrigger>
        <TooltipContent side='top' className='max-w-xs'>
          <p>{description}</p>
        </TooltipContent>
      </Tooltip>

      {/* Enabled/Disabled Toggle */}
      <FormField
        control={form.control}
        name={`builtinTools.${index}.enabled`}
        render={({ field }) => (
          <FormItem className='flex items-center space-y-0'>
            <Switch checked={field.value} onCheckedChange={field.onChange} />
          </FormItem>
        )}
      />

      {/* Delete Button */}
      <Button type='button' variant='ghost' size='icon' onClick={() => onRemove(index)} className='h-9 w-9 shrink-0'>
        <Trash2 className='h-4 w-4' />
      </Button>
    </div>
  );
}

export function AgentFormPage({ mode }: AgentFormPageProps) {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const createAgent = useCreateAgent();
  const updateAgent = useUpdateAgent();
  const { data: availableModels, mutateAsync: fetchModels } = useQueryModels();

  const params = useParams({ strict: false }) as { agentId?: string };
  const agentId = mode === 'edit' ? params.agentId : undefined;
  const { data: agent, isLoading: agentLoading } = useAgentDetail(agentId ?? '');

  const isEdit = mode === 'edit';

  // Model autocomplete state
  const [modelSearch, setModelSearch] = useState('');

  const modelOptions = useMemo(() => {
    return (availableModels || []).map((model) => ({ value: model.id, label: model.id }));
  }, [availableModels]);

  useEffect(() => {
    fetchModels({
      statusIn: ['enabled'],
      includeMapping: true,
      includePrefix: true,
    });
  }, [fetchModels]);

  const form = useForm<FormData>({
    resolver: zodResolver(agentFormSchema) as any,
    defaultValues: {
      name: '',
      description: '',
      model: '',
      systemPrompt: defaultSystemPrompt,
      skillsPolicyAdd: 'open',
      builtinTools: builtinToolOptions
        .filter((name) => builtinToolDefaultEnabled[name])
        .map((name, index) => ({
          name,
          enabled: true,
          order: index,
        })),
    },
  });

  const { fields, append, remove, move } = useFieldArray({
    control: form.control,
    name: 'builtinTools',
  });

  const sensors = useSensors(
    useSensor(PointerSensor),
    useSensor(KeyboardSensor, {
      coordinateGetter: sortableKeyboardCoordinates,
    })
  );

  const handleDragEnd = (event: DragEndEvent) => {
    const { active, over } = event;

    if (!over || active.id === over.id) {
      return;
    }

    const oldIndex = fields.findIndex((field) => field.id === active.id);
    const newIndex = fields.findIndex((field) => field.id === over.id);

    if (oldIndex !== -1 && newIndex !== -1) {
      move(oldIndex, newIndex);
      // Update order values after move
      const currentTools = form.getValues('builtinTools') || [];
      currentTools.forEach((tool, idx) => {
        form.setValue(`builtinTools.${idx}.order`, idx);
      });
    }
  };

  useEffect(() => {
    if (isEdit && agent) {
      const existingBuiltinTools = Array.isArray(agent.agentBuiltinTools) ? agent.agentBuiltinTools : [];
      const existingSkillsPolicyAdd =
        agent.skillsPolicy && typeof agent.skillsPolicy === 'object' && 'add' in agent.skillsPolicy
          ? (agent.skillsPolicy as any).add
          : 'open';

      form.reset({
        name: agent.name,
        description: agent.description || '',
        model: agent.model || '',
        systemPrompt: agent.prompt?.content || '',
        skillsPolicyAdd: existingSkillsPolicyAdd,
        builtinTools: existingBuiltinTools,
      });
      setModelSearch(agent.model || '');
    }
  }, [isEdit, agent, form]);

  const handleBack = () => {
    navigate({ to: '/project/agents' as any });
  };

  const onSubmit = async (values: FormData) => {
    const builtinTools = (values.builtinTools || []) as AgentBuiltinToolInput[];
    const skillsPolicy = values.skillsPolicyAdd ? { add: values.skillsPolicyAdd } : undefined;

    if (isEdit && agentId) {
      await updateAgent.mutateAsync({
        id: agentId,
        input: {
          name: values.name,
          description: values.description,
          model: values.model,
          systemPrompt: values.systemPrompt,
          builtinTools,
          skillsPolicy,
        },
      });
    } else {
      await createAgent.mutateAsync({
        name: values.name,
        description: values.description,
        model: values.model,
        systemPrompt: values.systemPrompt,
        builtinTools,
        skillsPolicy,
      });
    }

    handleBack();
  };

  // Get selected tool names for filtering
  const selectedToolNames = useMemo(() => {
    return new Set(fields.map((_, i) => form.getValues(`builtinTools.${i}.name`)));
  }, [fields, form]);

  // Check if all tools are added
  const canAddMore = fields.length < builtinToolOptions.length;

  if (isEdit && agentLoading) {
    return (
      <div className='flex h-screen flex-col'>
        <Header className='border-b' />
        <Main className='flex-1'>
          <div className='flex h-full items-center justify-center'>
            <div className='space-y-4 text-center'>
              <div className='border-primary mx-auto h-12 w-12 animate-spin rounded-full border-b-2' />
              <p className='text-muted-foreground text-lg'>{t('common.loading')}</p>
            </div>
          </div>
        </Main>
      </div>
    );
  }

  return (
    <div className='flex h-screen flex-col overflow-hidden'>
      <Header className='bg-background/95 supports-[backdrop-filter]:bg-background/60 border-b backdrop-blur'>
        <div className='flex w-full items-center justify-between'>
          <div className='flex items-center space-x-4'>
            <Button variant='ghost' size='sm' onClick={handleBack} className='hover:bg-accent'>
              <ArrowLeft className='mr-2 h-4 w-4' />
              {t('common.back')}
            </Button>
            <Separator orientation='vertical' className='h-6' />
            <h1 className='text-lg leading-none font-semibold'>
              {isEdit ? t('agents.dialogs.edit.title') : t('agents.dialogs.create.title')}
            </h1>
          </div>
          <div className='flex shrink-0 items-center gap-3'>
            <Button type='button' variant='outline' onClick={handleBack}>
              {t('common.buttons.cancel')}
            </Button>
            <Button
              type='button'
              disabled={createAgent.isPending || updateAgent.isPending}
              onClick={form.handleSubmit(onSubmit)}
            >
              {isEdit ? t('common.buttons.saveChanges') : t('common.buttons.create')}
            </Button>
          </div>
        </div>
      </Header>

      <Main className='flex-1 overflow-y-auto'>
        <div>
          <Form {...form}>
            <form onSubmit={form.handleSubmit(onSubmit)} className='container mx-auto max-w-7xl'>
              <div className='grid grid-cols-1 items-start gap-6 lg:grid-cols-3'>
                {/* Left column – Basic Info & System Prompt (2/3) */}
                <div className='flex flex-col gap-6 lg:col-span-2'>
                  <Card className='border-0 shadow-sm'>
                    <CardHeader className='pb-3'>
                      <CardTitle className='flex items-center gap-2 text-base'>
                        <Info className='h-4 w-4' />
                        {t('agents.form.basicInfo')}
                      </CardTitle>
                    </CardHeader>
                    <CardContent className='space-y-4'>
                      {/* Name and Model in one row */}
                      <div className='grid grid-cols-1 gap-4 md:grid-cols-2'>
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
                          name='model'
                          render={({ field }) => (
                            <FormItem>
                              <FormLabel>{t('agents.fields.model')}</FormLabel>
                              <FormControl>
                                <AutoComplete
                                  selectedValue={field.value || ''}
                                  onSelectedValueChange={field.onChange}
                                  searchValue={modelSearch}
                                  onSearchValueChange={setModelSearch}
                                  items={modelOptions}
                                  placeholder={t('agents.fields.modelPlaceholder')}
                                  emptyMessage={t('agents.fields.noModelsFound')}
                                />
                              </FormControl>
                              <FormMessage />
                            </FormItem>
                          )}
                        />
                      </div>

                      {/* Description in separate row */}
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
                    </CardContent>
                  </Card>

                  {/* System Prompt Card */}
                  <Card className='flex min-h-0 flex-1 flex-col border-0 shadow-sm'>
                    <CardHeader className='pb-3'>
                      <CardTitle className='flex items-center gap-2 text-base'>
                        <Terminal className='h-4 w-4' />
                        {t('agents.fields.systemPrompt')}
                      </CardTitle>
                    </CardHeader>
                    <CardContent className='flex min-h-0 flex-1 flex-col'>
                      <FormField
                        control={form.control}
                        name='systemPrompt'
                        render={({ field }) => (
                          <FormItem className='flex min-h-0 flex-1 flex-col'>
                            <FormControl>
                              <Textarea
                                {...field}
                                placeholder={t('agents.fields.systemPromptPlaceholder')}
                                className='min-h-[600px] flex-1 resize-none overscroll-none font-mono text-sm ![field-sizing:fixed]'
                              />
                            </FormControl>
                            <FormMessage />
                          </FormItem>
                        )}
                      />
                    </CardContent>
                  </Card>
                </div>

                {/* Right column – Skills Policy & Builtin Tools (1/3) */}
                <div className='space-y-6 lg:col-span-1'>
                  {/* Skills Policy Card */}
                  <Card className='border-0 shadow-sm'>
                    <CardHeader className='pb-3'>
                      <CardTitle className='flex items-center gap-2 text-base'>
                        <Settings className='h-4 w-4' />
                        {t('agents.fields.skillsPolicy')}
                      </CardTitle>
                    </CardHeader>
                    <CardContent>
                      <FormField
                        control={form.control}
                        name='skillsPolicyAdd'
                        render={({ field }) => (
                          <FormItem>
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
                    </CardContent>
                  </Card>

                  {/* Builtin Tools Card */}
                  <Card className='flex flex-col border-0 shadow-sm'>
                    <CardHeader className='pb-3'>
                      <div className='flex items-center justify-between'>
                        <CardTitle className='flex items-center gap-2 text-base'>
                          <Terminal className='h-4 w-4' />
                          {t('agents.fields.builtinTools')}
                        </CardTitle>
                        <Button
                          type='button'
                          variant='outline'
                          size='sm'
                          onClick={() => {
                            const availableTool = builtinToolOptions.find((opt) => !selectedToolNames.has(opt));
                            if (availableTool) {
                              append({ name: availableTool, enabled: true, order: fields.length });
                            }
                          }}
                          disabled={!canAddMore}
                          className='gap-2'
                        >
                          <Plus className='h-4 w-4' />
                          {t('agents.actions.addTool')}
                        </Button>
                      </div>
                    </CardHeader>
                    <CardContent className='flex-1'>
                      <div className='space-y-2 pr-1'>
                        {fields.length === 0 ? (
                          <p className='text-muted-foreground text-sm'>{t('agents.form.noTools')}</p>
                        ) : (
                          <DndContext sensors={sensors} collisionDetection={closestCenter} onDragEnd={handleDragEnd}>
                            <SortableContext items={fields.map((f) => f.id)} strategy={verticalListSortingStrategy}>
                              <div className='space-y-2'>
                                {fields.map((field, index) => {
                                  const otherSelectedNames = new Set(
                                    fields
                                      .map((_, i) => form.getValues(`builtinTools.${i}.name`))
                                      .filter((name) => name !== field.name)
                                  );
                                  const availableOptions = builtinToolOptions.filter((opt) => !otherSelectedNames.has(opt));

                                  return (
                                    <SortableToolItem
                                      key={field.id}
                                      field={field}
                                      index={index}
                                      availableOptions={availableOptions}
                                      onRemove={remove}
                                      form={form}
                                      t={t}
                                    />
                                  );
                                })}
                              </div>
                            </SortableContext>
                          </DndContext>
                        )}
                      </div>
                    </CardContent>
                  </Card>
                </div>
              </div>
            </form>
          </Form>
        </div>
      </Main>
    </div>
  );
}
