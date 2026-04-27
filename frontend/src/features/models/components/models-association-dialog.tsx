import { useEffect, useMemo, useCallback, useState, useRef } from 'react';
import { z } from 'zod';
import { useForm, useFieldArray, useWatch } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { IconPlus, IconTrash, IconChevronDown, IconChevronUp } from '@tabler/icons-react';
import { useQueryModels } from '@/gql/models';
import { useTranslation } from 'react-i18next';
import { extractNumberIDAsNumber } from '@/lib/utils';
import { useDebounce } from '@/hooks/use-debounce';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Form, FormControl, FormField, FormItem, FormLabel, FormMessage } from '@/components/ui/form';
import { Input } from '@/components/ui/input';
import { Switch } from '@/components/ui/switch';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { TagsAutocompleteInput } from '@/components/ui/tags-autocomplete-input';
import { AutoComplete } from '@/components/auto-complete';
import { AutoCompleteSelect } from '@/components/auto-complete-select';
import { FilterBuilder, type FilterBuilderCondition, type FilterBuilderField, type FilterBuilderGroupListValue } from '@/components/filter-builder';
import { useAllChannelSummarys, useAllChannelTags } from '@/features/channels/data/channels';
import { useModels } from '../context/models-context';
import { useQueryModelChannelConnections, ModelAssociationInput, ModelChannelConnection } from '../data/models';
import { useUpdateModel } from '../data/models';
import { ModelAssociation } from '../data/schema';
import { toast } from 'sonner';
import { ChannelModelsList } from './channel-models-list';

const whenFilterFields: FilterBuilderField[] = [
  {
    value: 'prompt_tokens',
    label: 'Prompt tokens',
    type: 'number',
    placeholder: 'Enter token threshold',
    operators: [
      { value: 'lt', label: '< Less than' },
      { value: 'lte', label: '<= Less than or equal' },
      { value: 'gt', label: '> Greater than' },
      { value: 'gte', label: '>= Greater than or equal' },
    ],
  },
  {
    value: 'stream',
    label: 'Stream',
    type: 'boolean',
    operators: [
      { value: 'eq', label: '= Equals' },
      { value: 'ne', label: '!= Not equal' },
    ],
  },
];

function isValidConditionOperator(field: string, operator: string): boolean {
  const fieldConfig = whenFilterFields.find((f) => f.value === field);
  if (!fieldConfig) return false;
  return fieldConfig.operators.some((op) => op.value === operator);
}

const DEFAULT_WHEN_CONDITION: FilterBuilderGroupListValue = {
  groups: [],
};
const MAX_WHEN_CONDITION_DEPTH = 1;
const DEFAULT_WHEN_GROUP: FilterBuilderCondition = {
  type: 'group',
  logic: 'and',
  conditions: [],
};

function hasConditionNodeData(condition?: FilterBuilderCondition): boolean {
  if (!condition) {
    return false;
  }

  if (condition.type === 'group') {
    return (condition.conditions || []).some((item) => hasConditionNodeData(item));
  }

  return Boolean(condition.field && condition.operator && condition.value !== '');
}

function hasGroupListData(value?: FilterBuilderGroupListValue) {
  return (value?.groups || []).some((group) => hasConditionNodeData(group));
}

function validateWhenConditionNode(
  condition: FilterBuilderCondition,
  ctx: z.RefinementCtx,
  path: (string | number)[],
  depth = 1,
  maxDepth = MAX_WHEN_CONDITION_DEPTH
) {
  if (condition.type === 'group') {
    const conditions = condition.conditions || [];
    if (conditions.length === 0) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: 'At least one condition is required',
        path,
      });
    }

    if (depth > maxDepth) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: `Condition nesting cannot exceed ${maxDepth} level${maxDepth > 1 ? 's' : ''}`,
        path,
      });
    }

    conditions.forEach((nestedCondition, index) =>
      validateWhenConditionNode(nestedCondition, ctx, [...path, 'conditions', index], depth + 1, maxDepth)
    );
    return;
  }

  if (!condition.field) {
    ctx.addIssue({
      code: z.ZodIssueCode.custom,
      message: 'Field is required',
      path: [...path, 'field'],
    });
  }
  if (!condition.operator) {
    ctx.addIssue({
      code: z.ZodIssueCode.custom,
      message: 'Operator is required',
      path: [...path, 'operator'],
    });
  }
  if (condition.value === '') {
    ctx.addIssue({
      code: z.ZodIssueCode.custom,
      message: 'Value is required',
      path: [...path, 'value'],
    });
  }
  if (condition.field === 'prompt_tokens' && typeof condition.value !== 'number') {
    ctx.addIssue({
      code: z.ZodIssueCode.custom,
      message: 'Value must be a number',
      path: [...path, 'value'],
    });
  }
  if (condition.field === 'stream' && typeof condition.value !== 'boolean') {
    ctx.addIssue({
      code: z.ZodIssueCode.custom,
      message: 'Value must be a boolean',
      path: [...path, 'value'],
    });
  }
}

function validateWhenGroupList(value: FilterBuilderGroupListValue, ctx: z.RefinementCtx, path: (string | number)[]) {
  const groups = value.groups || [];

  if (groups.length === 0) {
    ctx.addIssue({
      code: z.ZodIssueCode.custom,
      message: 'At least one condition group is required',
      path,
    });
    return;
  }

  if (groups.length > 1) {
    ctx.addIssue({
      code: z.ZodIssueCode.custom,
      message: 'Only one condition group is allowed',
      path,
    });
  }

  groups.slice(0, 1).forEach((group, index) => validateWhenConditionNode(group, ctx, [...path, 'groups', index], 1, MAX_WHEN_CONDITION_DEPTH));
}

const associationFormSchema = z.object({
  associations: z
    .array(
      z.object({
        type: z.enum(['channel_model', 'channel_regex', 'model', 'regex', 'channel_tags_model', 'channel_tags_regex']),
        priority: z.number().min(0, 'Priority must be at least 0').max(10, 'Priority cannot exceed 10'),
        disabled: z.boolean().default(false),
        whenEnabled: z.boolean().default(false),
        whenCondition: z.custom<FilterBuilderGroupListValue>().default(DEFAULT_WHEN_CONDITION),
        channelId: z.number().optional(),
        channelTags: z.array(z.string()).optional(),
        modelId: z.string().optional(),
        pattern: z.string().optional(),
        excludeChannelNamePattern: z.string().optional(),
        excludeChannelIds: z.array(z.number()).optional(),
        excludeChannelTags: z.array(z.string()).optional(),
      })
    )
    .max(10, 'Cannot have more than 10 associations')
    .superRefine((associations, ctx) => {
      associations.forEach((assoc, index) => {
        if (assoc.type === 'channel_model' || assoc.type === 'channel_regex') {
          if (!assoc.channelId) {
            ctx.addIssue({
              code: z.ZodIssueCode.custom,
              message: 'Channel is required',
              path: [index, 'channelId'],
            });
          }
        }
        if (assoc.type === 'channel_tags_model' || assoc.type === 'channel_tags_regex') {
          if (!assoc.channelTags || assoc.channelTags.length === 0) {
            ctx.addIssue({
              code: z.ZodIssueCode.custom,
              message: 'Channel tags are required',
              path: [index, 'channelTags'],
            });
          }
        }
        if (assoc.type === 'channel_model' || assoc.type === 'model' || assoc.type === 'channel_tags_model') {
          if (!assoc.modelId || assoc.modelId.trim() === '') {
            ctx.addIssue({
              code: z.ZodIssueCode.custom,
              message: 'Model ID is required',
              path: [index, 'modelId'],
            });
          }
        }
        if (assoc.type === 'channel_regex' || assoc.type === 'regex' || assoc.type === 'channel_tags_regex') {
          if (!assoc.pattern || assoc.pattern.trim() === '') {
            ctx.addIssue({
              code: z.ZodIssueCode.custom,
              message: 'Pattern is required',
              path: [index, 'pattern'],
            });
          }
        }
        if (assoc.whenEnabled) {
          validateWhenGroupList(assoc.whenCondition || DEFAULT_WHEN_CONDITION, ctx, [index, 'whenCondition']);
        }
      });
    }),
});

type AssociationFormData = z.infer<typeof associationFormSchema>;

export function ModelsAssociationDialog() {
  const { t } = useTranslation();
  const { open, setOpen, currentRow } = useModels();
  const updateModel = useUpdateModel();
  const { data: channelsData } = useAllChannelSummarys(undefined, { enabled: open === 'association' });
  const { data: availableModels, mutateAsync: fetchModels } = useQueryModels();
  const { data: allTags = [] } = useAllChannelTags();
  const { mutateAsync: queryConnections } = useQueryModelChannelConnections();
  const [connections, setConnections] = useState<ModelChannelConnection[]>([]);
  const [channelFilter, setChannelFilter] = useState('');
  const dialogContentRef = useRef<HTMLDivElement>(null);

  const isOpen = open === 'association';

  useEffect(() => {
    if (isOpen) {
      fetchModels({
        statusIn: ['enabled'],
        includeAllChannelModels: true,
      });
    }
  }, [isOpen, fetchModels]);

  // Build channel options for select
  const channelOptions = useMemo((): {
    value: number;
    label: string;
    allModelEntries: Array<{ requestModel: string; actualModel: string; source: string }>;
  }[] => {
    if (!channelsData?.edges) return [];
    return channelsData.edges.map((edge) => ({
      value: extractNumberIDAsNumber(edge.node.id),
      label: edge.node.name,
      allModelEntries: edge.node.allModelEntries || [],
    }));
  }, [channelsData]);

  // Build all available model options
  const allModelOptions = useMemo(() => {
    if (!availableModels) return [];
    return availableModels.map((model) => ({
      value: model.id,
      label: model.id,
    }));
  }, [availableModels]);

  const form = useForm<AssociationFormData>({
    resolver: zodResolver(associationFormSchema),
    defaultValues: {
      associations: [],
    },
  });

  const { fields, append, remove } = useFieldArray({
    control: form.control,
    name: 'associations',
  });

  // Watch associations for debounced preview - useWatch triggers re-renders
  const watchedAssociations = useWatch({
    control: form.control,
    name: 'associations',
    defaultValue: [],
  });
  // Serialize to string for stable comparison in debounce
  const associationsString = JSON.stringify(watchedAssociations);
  const debouncedAssociationsString = useDebounce(associationsString, 500);

  // Query connections when associations change
  useEffect(() => {
    if (!isOpen) {
      setConnections([]);
      return;
    }

    let debouncedAssociations;
    try {
      debouncedAssociations = JSON.parse(debouncedAssociationsString);
    } catch {
      setConnections([]);
      return;
    }

    if (!debouncedAssociations || debouncedAssociations.length === 0) {
      setConnections([]);
      return;
    }

    const fetchConnections = async () => {
      try {
        const sortedDebouncedAssociations = [...debouncedAssociations].sort((a: any, b: any) => (a.priority ?? 0) - (b.priority ?? 0));
        const associations: ModelAssociationInput[] = sortedDebouncedAssociations
          .filter((assoc: any) => {
            if (assoc.type === 'channel_model') {
              return assoc.channelId && assoc.modelId;
            } else if (assoc.type === 'channel_regex') {
              return assoc.channelId && assoc.pattern;
            } else if (assoc.type === 'regex') {
              return assoc.pattern;
            } else if (assoc.type === 'model') {
              return assoc.modelId;
            } else if (assoc.type === 'channel_tags_model') {
              return assoc.channelTags && assoc.channelTags.length > 0 && assoc.modelId;
            } else if (assoc.type === 'channel_tags_regex') {
              return assoc.channelTags && assoc.channelTags.length > 0 && assoc.pattern;
            }
            return false;
          })
          .map((assoc: any): ModelAssociationInput | undefined => {
            const hasExclude =
              assoc.excludeChannelNamePattern ||
              (assoc.excludeChannelIds && assoc.excludeChannelIds.length > 0) ||
              (assoc.excludeChannelTags && assoc.excludeChannelTags.length > 0);
            const exclude = hasExclude
              ? [
                {
                  channelNamePattern: assoc.excludeChannelNamePattern || null,
                  channelIds: assoc.excludeChannelIds || null,
                  channelTags: assoc.excludeChannelTags || null,
                },
              ]
              : undefined;

            if (assoc.type === 'channel_model') {
              return {
                type: 'channel_model' as const,
                disabled: assoc.disabled ?? false,
                when: buildAssociationWhen(assoc.whenEnabled, assoc.whenCondition),
                channelModel: {
                  channelId: assoc.channelId!,
                  modelId: assoc.modelId!,
                },
              };
            } else if (assoc.type === 'channel_regex') {
              return {
                type: 'channel_regex' as const,
                disabled: assoc.disabled ?? false,
                when: buildAssociationWhen(assoc.whenEnabled, assoc.whenCondition),
                channelRegex: {
                  channelId: assoc.channelId!,
                  pattern: assoc.pattern!,
                },
              };
            } else if (assoc.type === 'regex') {
              return {
                type: 'regex' as const,
                disabled: assoc.disabled ?? false,
                when: buildAssociationWhen(assoc.whenEnabled, assoc.whenCondition),
                regex: {
                  pattern: assoc.pattern!,
                  exclude,
                },
              };
            } else if (assoc.type === 'model') {
              return {
                type: 'model' as const,
                disabled: assoc.disabled ?? false,
                when: buildAssociationWhen(assoc.whenEnabled, assoc.whenCondition),
                modelId: {
                  modelId: assoc.modelId!,
                  exclude,
                },
              };
            } else if (assoc.type === 'channel_tags_model') {
              return {
                type: 'channel_tags_model' as const,
                disabled: assoc.disabled ?? false,
                when: buildAssociationWhen(assoc.whenEnabled, assoc.whenCondition),
                channelTagsModel: {
                  channelTags: assoc.channelTags!,
                  modelId: assoc.modelId!,
                },
              };
            } else if (assoc.type === 'channel_tags_regex') {
              return {
                type: 'channel_tags_regex' as const,
                disabled: assoc.disabled ?? false,
                when: buildAssociationWhen(assoc.whenEnabled, assoc.whenCondition),
                channelTagsRegex: {
                  channelTags: assoc.channelTags!,
                  pattern: assoc.pattern!,
                },
              };
            }
            return undefined;
          })
          .filter((item): item is ModelAssociationInput => item !== undefined);

        if (associations.length > 0) {
          const result = await queryConnections(associations);
          setConnections(result);
        } else {
          setConnections([]);
        }
      } catch (error) {
        toast.error(t('common.errors.loadFailed'));
        setConnections([]);
      }
    };

    fetchConnections();
  }, [debouncedAssociationsString, isOpen, queryConnections]);

  useEffect(() => {
    if (isOpen && currentRow) {
      const associations = currentRow.settings?.associations || [];
      form.reset({
        associations: associations.map((assoc) => {
          const exclude = assoc.regex?.exclude?.[0] || assoc.modelId?.exclude?.[0];
          const promptTokensCondition = readPromptTokensCondition(assoc.when);
          return {
            type: assoc.type,
            priority: assoc.priority ?? 0,
            disabled: assoc.disabled ?? false,
            whenEnabled: promptTokensCondition.enabled,
            whenCondition: promptTokensCondition.enabled && (promptTokensCondition.condition.groups?.length || 0) === 0
              ? { groups: [DEFAULT_WHEN_GROUP] }
              : promptTokensCondition.condition,
            channelId: assoc.channelModel?.channelId || assoc.channelRegex?.channelId,
            channelTags: assoc.channelTagsModel?.channelTags || assoc.channelTagsRegex?.channelTags || [],
            modelId: assoc.channelModel?.modelId || assoc.modelId?.modelId || assoc.channelTagsModel?.modelId,
            pattern: assoc.channelRegex?.pattern || assoc.regex?.pattern || assoc.channelTagsRegex?.pattern,
            excludeChannelNamePattern: exclude?.channelNamePattern || '',
            excludeChannelIds: exclude?.channelIds || [],
            excludeChannelTags: exclude?.channelTags || [],
          };
        }),
      });
    }
  }, [isOpen, currentRow, form]);

  const onSubmit = async (data: AssociationFormData) => {
    if (!currentRow) return;

    try {
      const sortedAssociations = [...data.associations].sort((a, b) => (a.priority ?? 0) - (b.priority ?? 0));
      const associations: ModelAssociation[] = sortedAssociations.map((assoc) => {
        if (assoc.type === 'channel_model') {
          return {
            type: 'channel_model',
            priority: assoc.priority ?? 0,
            disabled: assoc.disabled ?? false,
            when: buildAssociationWhen(assoc.whenEnabled, assoc.whenCondition),
            channelModel: {
              channelId: assoc.channelId || 0,
              modelId: assoc.modelId || '',
            },
            channelRegex: null,
            regex: null,
            modelId: null,
            channelTagsModel: null,
            channelTagsRegex: null,
          };
        } else if (assoc.type === 'channel_regex') {
          return {
            type: 'channel_regex',
            priority: assoc.priority ?? 0,
            disabled: assoc.disabled ?? false,
            when: buildAssociationWhen(assoc.whenEnabled, assoc.whenCondition),
            channelModel: null,
            channelRegex: {
              channelId: assoc.channelId || 0,
              pattern: assoc.pattern || '',
            },
            regex: null,
            modelId: null,
            channelTagsModel: null,
            channelTagsRegex: null,
          };
        } else if (assoc.type === 'channel_tags_model') {
          return {
            type: 'channel_tags_model',
            priority: assoc.priority ?? 0,
            disabled: assoc.disabled ?? false,
            when: buildAssociationWhen(assoc.whenEnabled, assoc.whenCondition),
            channelModel: null,
            channelRegex: null,
            regex: null,
            modelId: null,
            channelTagsModel: {
              channelTags: assoc.channelTags || [],
              modelId: assoc.modelId || '',
            },
            channelTagsRegex: null,
          };
        } else if (assoc.type === 'channel_tags_regex') {
          return {
            type: 'channel_tags_regex',
            priority: assoc.priority ?? 0,
            disabled: assoc.disabled ?? false,
            when: buildAssociationWhen(assoc.whenEnabled, assoc.whenCondition),
            channelModel: null,
            channelRegex: null,
            regex: null,
            modelId: null,
            channelTagsModel: null,
            channelTagsRegex: {
              channelTags: assoc.channelTags || [],
              pattern: assoc.pattern || '',
            },
          };
        } else if (assoc.type === 'regex') {
          const hasExclude =
            assoc.excludeChannelNamePattern ||
            (assoc.excludeChannelIds && assoc.excludeChannelIds.length > 0) ||
            (assoc.excludeChannelTags && assoc.excludeChannelTags.length > 0);
          const exclude = hasExclude
            ? [
              {
                channelNamePattern: assoc.excludeChannelNamePattern || null,
                channelIds: assoc.excludeChannelIds || null,
                channelTags: assoc.excludeChannelTags || null,
              },
            ]
            : null;
          return {
            type: 'regex',
            priority: assoc.priority ?? 0,
            disabled: assoc.disabled ?? false,
            when: buildAssociationWhen(assoc.whenEnabled, assoc.whenCondition),
            channelModel: null,
            channelRegex: null,
            regex: {
              pattern: assoc.pattern || '',
              exclude,
            },
            modelId: null,
            channelTagsModel: null,
            channelTagsRegex: null,
          };
        } else {
          const hasExclude =
            assoc.excludeChannelNamePattern ||
            (assoc.excludeChannelIds && assoc.excludeChannelIds.length > 0) ||
            (assoc.excludeChannelTags && assoc.excludeChannelTags.length > 0);
          const exclude = hasExclude
            ? [
              {
                channelNamePattern: assoc.excludeChannelNamePattern || null,
                channelIds: assoc.excludeChannelIds || null,
                channelTags: assoc.excludeChannelTags || null,
              },
            ]
            : null;
          return {
            type: 'model',
            priority: assoc.priority ?? 0,
            disabled: assoc.disabled ?? false,
            when: buildAssociationWhen(assoc.whenEnabled, assoc.whenCondition),
            channelModel: null,
            channelRegex: null,
            regex: null,
            modelId: {
              modelId: assoc.modelId || '',
              exclude,
            },
            channelTagsModel: null,
            channelTagsRegex: null,
          };
        }
      });

      await updateModel.mutateAsync({
        id: currentRow.id,
        input: {
          settings: {
            associations,
          },
        },
      });
      handleClose();
    } catch (_error) {
      // Error is handled by mutation
    }
  };

  const handleClose = useCallback(() => {
    setOpen(null);
    form.reset();
    setConnections([]);
    setChannelFilter('');
  }, [setOpen, form]);

  const handleAddAssociation = useCallback(() => {
    if (fields.length >= 10) return;

    // Get the priority of the last rule (highest priority)
    const currentAssociations = form.getValues('associations') || [];
    const lastPriority =
      currentAssociations.length > 0 ? Math.max(...currentAssociations.map((a) => a.priority ?? 0)) : 0;

    append({
      type: 'channel_model',
      priority: lastPriority,
      disabled: false,
      whenEnabled: false,
      whenCondition: DEFAULT_WHEN_CONDITION,
      channelId: undefined,
      channelTags: [],
      modelId: '',
      pattern: '',
      excludeChannelNamePattern: '',
      excludeChannelIds: [],
      excludeChannelTags: [],
    });
  }, [append, fields.length, form]);

  // Filter connections by channel name
  const filteredConnections = useMemo(() => {
    if (!channelFilter.trim()) return connections;
    const filter = channelFilter.toLowerCase().trim();
    return connections.filter((conn) => conn.channel.name.toLowerCase().includes(filter));
  }, [connections, channelFilter]);

  return (
    <Dialog open={isOpen} onOpenChange={handleClose}>
      <DialogContent ref={dialogContentRef} className='flex h-[90vh] max-h-[800px] flex-col w-full max-w-full sm:max-w-6xl'>
        <DialogHeader className='shrink-0 text-left'>
          <DialogTitle className='text-lg sm:text-xl'>{t('models.dialogs.association.title')}</DialogTitle>
          <DialogDescription className='text-sm sm:text-base'>{t('models.dialogs.association.description', { name: currentRow?.name })}</DialogDescription>
        </DialogHeader>

        <div className='flex min-h-0 flex-1 flex-col gap-6 sm:flex-row'>
          {/* Left Side - Association Rules */}
          <div className='flex min-h-0 flex-1 flex-col sm:flex-[2]'>
            {/* Scrollable Rules Section */}
            <div className='flex-1 overflow-y-auto py-4'>
              <Form {...form}>
                <form id='association-form' onSubmit={form.handleSubmit(onSubmit)} className='space-y-3'>
                  {fields.length === 0 && (
                    <p className='text-muted-foreground py-8 text-center text-sm'>{t('models.dialogs.association.noRules')}</p>
                  )}

                  {fields.length > 0 && (
                    <div className='grid grid-cols-[2.25rem_3rem_1fr_2.25rem] sm:grid-cols-[2.25rem_3rem_14rem_1fr_2.25rem] items-center gap-2 border-b px-3 sm:px-[13px] pb-2'>
                      <div />
                      <div className='text-muted-foreground text-center text-xs font-medium'>{t('models.dialogs.association.priority')}</div>
                      <div className='text-muted-foreground text-center text-xs font-medium sm:block hidden'>{t('models.dialogs.association.type')}</div>
                      <div className='text-muted-foreground text-center text-xs font-medium'>{t('models.dialogs.association.rule')}</div>
                      <div />
                    </div>
                  )}

                  {fields
                    .map((field, index) => ({ field, index }))
                    .sort((a, b) => {
                      const priorityA = form.getValues(`associations.${a.index}.priority`) ?? 0;
                      const priorityB = form.getValues(`associations.${b.index}.priority`) ?? 0;
                      return priorityA - priorityB;
                    })
                    .map(({ field, index }) => (
                      <AssociationRow
                        key={field.id}
                        index={index}
                        form={form}
                        channelOptions={channelOptions}
                        allModelOptions={allModelOptions}
                        allTags={allTags}
                        onRemove={() => remove(index)}
                        portalContainer={dialogContentRef.current}
                      />
                    ))}
                </form>
              </Form>
            </div>

            {/* Fixed Add Rule Section at Bottom */}
            <div className='bg-background shrink-0 border-t pt-4'>
              <Button type='button' variant='outline' onClick={handleAddAssociation} disabled={fields.length >= 10} className='w-full'>
                <IconPlus className='mr-2 h-4 w-4' />
                {t('models.dialogs.association.addRule')}
              </Button>
            </div>
          </div>

          {/* Right Side - Preview */}
          <div className='flex min-h-0 flex-1 flex-col border-t sm:border-t-0 sm:border-l pt-4 sm:pt-0 sm:pl-6'>
            <div className='shrink-0 space-y-2 pb-4'>
              <h3 className='text-sm font-semibold'>{t('models.dialogs.association.preview')}</h3>
              <p className='text-muted-foreground text-xs'>{t('models.dialogs.association.previewDescription')}</p>
              <Input
                placeholder={t('models.dialogs.association.filterByChannel')}
                value={channelFilter}
                onChange={(e) => setChannelFilter(e.target.value)}
                className='h-9 sm:h-8'
              />
            </div>
            <div className='flex-1 overflow-y-auto'>
              <ChannelModelsList
                channels={filteredConnections}
                emptyMessage={
                  channelFilter.trim()
                    ? t('models.dialogs.association.noFilteredConnections')
                    : t('models.dialogs.association.noConnections')
                }
              />
            </div>
          </div>
        </div>

        <DialogFooter className='shrink-0 border-t pt-4 flex flex-col sm:flex-row gap-2 sm:gap-0 sm:justify-end'>
          <Button type='button' variant='outline' onClick={handleClose} className='w-full sm:w-auto'>
            {t('common.buttons.cancel')}
          </Button>
          <Button type='submit' form='association-form' disabled={updateModel.isPending || !form.formState.isValid} className='w-full sm:w-auto'>
            {t('common.buttons.save')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

function readPromptTokensCondition(
  when: ModelAssociation['when']
): { enabled: boolean; condition: FilterBuilderGroupListValue } {
  if (!when) {
    return { enabled: false, condition: DEFAULT_WHEN_CONDITION };
  }

  const condition = when.condition;
  if (!when.enabled || !condition) {
    return { enabled: Boolean(when.enabled), condition: DEFAULT_WHEN_CONDITION };
  }

  return {
    enabled: Boolean(when.enabled),
    condition: {
      groups: normalizeWhenCondition(condition, 0, MAX_WHEN_CONDITION_DEPTH)?.conditions || [],
    },
  };
}

function normalizeWhenCondition(
  condition?: FilterBuilderCondition | null,
  depth = 0,
  maxDepth = MAX_WHEN_CONDITION_DEPTH
): FilterBuilderCondition | null {
  if (!condition) {
    return null;
  }

  if (condition.type === 'group') {
    const normalizedConditions = (condition.conditions || [])
      .map((nestedCondition) => normalizeWhenCondition(nestedCondition, depth + 1, maxDepth))
      .filter((item): item is FilterBuilderCondition => item !== null);

    if (depth >= maxDepth) {
      return {
        type: 'group',
        logic: condition.logic === 'or' ? 'or' : 'and',
        conditions: normalizedConditions.flatMap((item) => (item.type === 'group' ? item.conditions || [] : [item])),
      };
    }

    return {
      type: 'group',
      logic: condition.logic === 'or' ? 'or' : 'and',
      conditions: normalizedConditions,
    };
  }

  if (!isValidConditionOperator(condition.field, condition.operator)) {
    return null;
  }

  return {
    type: 'condition',
    field: condition.field,
    operator: condition.operator,
    value: condition.field === 'prompt_tokens' ? Number(condition.value) : condition.value,
  };
}

function sanitizeWhenCondition(condition?: FilterBuilderCondition): FilterBuilderCondition | null {
  if (!condition) {
    return null;
  }

  if (condition.type === 'group') {
    const conditions = (condition.conditions || [])
      .map((nestedCondition) => sanitizeWhenCondition(nestedCondition))
      .filter((item): item is FilterBuilderCondition => item !== null);

    if (conditions.length === 0) {
      return null;
    }

    return {
      type: 'group',
      logic: condition.logic === 'or' ? 'or' : 'and',
      conditions: conditions.flatMap((item) => (item.type === 'group' ? item.conditions || [] : [item])),
    };
  }

  if (!condition.field || !condition.operator || condition.value === '') {
    return null;
  }

  return {
    type: 'condition',
    field: condition.field,
    operator: condition.operator,
    value: condition.value,
  };
}

function buildAssociationWhen(enabled?: boolean, value?: FilterBuilderGroupListValue): ModelAssociationInput['when'] | null {
  const groups = (value?.groups || [])
    .map((group) => sanitizeWhenCondition(group))
    .filter((item): item is FilterBuilderCondition => item !== null);

  if (!enabled || groups.length === 0) {
    return null;
  }

  return {
    enabled: true,
    condition: {
      type: 'group',
      logic: 'and',
      conditions: groups,
    },
  };
}

interface AssociationRowProps {
  index: number;
  form: ReturnType<typeof useForm<AssociationFormData>>;
  channelOptions: { value: number; label: string; allModelEntries: Array<{ requestModel: string; actualModel: string; source: string }> }[];
  allModelOptions: { value: string; label: string }[];
  allTags: string[];
  onRemove: () => void;
  portalContainer: HTMLElement | null;
}

function AssociationRow({ index, form, channelOptions, allModelOptions, allTags, onRemove, portalContainer }: AssociationRowProps) {
  const { t } = useTranslation();

  const type = form.watch(`associations.${index}.type`);
  const channelId = form.watch(`associations.${index}.channelId`);
  const channelTags = form.watch(`associations.${index}.channelTags`);
  const modelId = form.watch(`associations.${index}.modelId`);
  const pattern = form.watch(`associations.${index}.pattern`);
  const excludeChannelIds = form.watch(`associations.${index}.excludeChannelIds`);
  const excludeChannelNamePattern = form.watch(`associations.${index}.excludeChannelNamePattern`);
  const excludeChannelTags = form.watch(`associations.${index}.excludeChannelTags`);
  const disabled = form.watch(`associations.${index}.disabled`);
  const whenEnabled = form.watch(`associations.${index}.whenEnabled`);
  const whenCondition = form.watch(`associations.${index}.whenCondition`);
  const [modelSearch, setModelSearch] = useState(modelId?.toString() || '');
  const [whenExpanded, setWhenExpanded] = useState(Boolean(whenEnabled || hasGroupListData(whenCondition)));
  const [excludeExpanded, setExcludeExpanded] = useState(false);

  useEffect(() => {
    setModelSearch(modelId?.toString() || '');
  }, [modelId]);

  const showChannel = type === 'channel_model' || type === 'channel_regex';
  const showChannelTags = type === 'channel_tags_model' || type === 'channel_tags_regex';
  const showModel = type === 'channel_model' || type === 'model' || type === 'channel_tags_model';
  const showPattern = type === 'channel_regex' || type === 'regex' || type === 'channel_tags_regex';
  const showExclude = type === 'regex' || type === 'model';
  const showModelPatternOnSecondRow = type === 'channel_model' || type === 'channel_regex';
  const hasExcludeData =
    excludeChannelNamePattern ||
    (excludeChannelIds && excludeChannelIds.length > 0) ||
    (excludeChannelTags && excludeChannelTags.length > 0);
  const hasWhenData = Boolean(whenEnabled || hasGroupListData(whenCondition));

  // Auto-expand if has exclude data
  useEffect(() => {
    if (hasExcludeData) {
      setExcludeExpanded(true);
    }
  }, [hasExcludeData]);

  // Auto-expand if has when data
  useEffect(() => {
    if (hasWhenData) {
      setWhenExpanded(true);
    }
  }, [hasWhenData]);

  // Filter model options based on selected channel's model entries
  const modelOptions = useMemo(() => {
    if (!showModel) {
      return [];
    }

    if (type === 'model' || type === 'channel_tags_model') {
      // For 'model' and 'channel_tags_model' types, show all available models
      return allModelOptions;
    }

    // For 'channel_model' type, use the selected channel's model entries
    if (!channelId) {
      return [];
    }

    const selectedChannel = channelOptions.find((option) => option.value === channelId);
    if (!selectedChannel?.allModelEntries?.length) {
      return [];
    }

    // Return model entries as options (using requestModel)
    return selectedChannel.allModelEntries.map((entry: { requestModel: string; actualModel: string; source: string }) => ({
      value: entry.requestModel,
      label: entry.requestModel,
    }));
  }, [channelId, channelOptions, allModelOptions, showModel, type]);

  return (
    <div className={`flex flex-col gap-3 rounded-lg border p-3 ${disabled ? 'opacity-50' : ''}`}>
      <div className='grid grid-cols-[2.5rem_4rem_1fr_2.5rem] sm:grid-cols-[2.25rem_3rem_14rem_1fr_2.25rem] items-center gap-2'>
        {/* Enable/Disable Switch */}
        <div className='flex items-center justify-center'>
          <Switch
            checked={!disabled}
            onCheckedChange={(checked) => form.setValue(`associations.${index}.disabled`, !checked)}
            className='scale-100 sm:scale-75'
          />
        </div>

        {/* Priority Input */}
        <FormField
          control={form.control}
          name={`associations.${index}.priority`}
          render={({ field }) => (
            <FormItem className='min-w-0 gap-0'>
              <FormControl>
                <Input
                  type='number'
                  min={0}
                  max={10}
                  {...field}
                  value={field.value ?? 0}
                  onChange={(e) => field.onChange(Math.max(0, Math.min(10, Number(e.target.value) || 0)))}
                  className='h-10 sm:h-9 text-center [-moz-appearance:textfield] [&::-webkit-inner-spin-button]:m-0 [&::-webkit-inner-spin-button]:hidden [&::-webkit-inner-spin-button]:appearance-none'
                  placeholder='0'
                />
              </FormControl>
            </FormItem>
          )}
        />

        {/* Type Select */}
        <FormField
          control={form.control}
          name={`associations.${index}.type`}
          render={({ field }) => (
            <FormItem className='min-w-0 gap-0 sm:block hidden'>
              <FormControl>
                <Select value={field.value} onValueChange={field.onChange}>
                  <SelectTrigger className='h-10 sm:h-9 w-full text-xs'>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value='channel_model'>{t('models.dialogs.association.types.channelModel')}</SelectItem>
                    <SelectItem value='channel_regex'>{t('models.dialogs.association.types.channelRegex')}</SelectItem>
                    <SelectItem value='channel_tags_model'>{t('models.dialogs.association.types.channelTagsModel')}</SelectItem>
                    <SelectItem value='channel_tags_regex'>{t('models.dialogs.association.types.channelTagsRegex')}</SelectItem>
                    <SelectItem value='model'>{t('models.dialogs.association.types.model')}</SelectItem>
                    <SelectItem value='regex'>{t('models.dialogs.association.types.regex')}</SelectItem>
                  </SelectContent>
                </Select>
              </FormControl>
              <FormMessage />
            </FormItem>
          )}
        />

        {/* Channel Select */}
        {showChannel && (
          <FormField
            control={form.control}
            name={`associations.${index}.channelId`}
            render={({ field, fieldState }) => (
              <FormItem className='min-w-0 gap-0'>
                <FormControl>
                  <AutoCompleteSelect
                    selectedValue={field.value?.toString() || ''}
                    onSelectedValueChange={(value) => field.onChange(Number(value))}
                    items={channelOptions.map((opt) => ({ value: opt.value.toString(), label: opt.label }))}
                    placeholder={t('models.dialogs.association.selectChannel')}
                    emptyMessage={t('models.dialogs.association.noModelsAvailable')}
                    portalContainer={portalContainer}
                  />
                </FormControl>
                {fieldState.error && <FormMessage>{fieldState.error.message}</FormMessage>}
              </FormItem>
            )}
          />
        )}

        {/* Model Select/AutoComplete - Only show if NOT on second row */}
        {showModel && !showModelPatternOnSecondRow && (
          <FormField
            control={form.control}
            name={`associations.${index}.modelId`}
            render={({ field }) => (
              <FormItem className='min-w-0 gap-0'>
                <FormControl>
                  <AutoComplete
                    selectedValue={field.value?.toString() || ''}
                    onSelectedValueChange={(value) => {
                      field.onChange(value);
                    }}
                    searchValue={modelSearch}
                    onSearchValueChange={setModelSearch}
                    items={modelOptions}
                    placeholder={t('models.dialogs.association.selectModel')}
                    emptyMessage={
                      modelOptions.length === 0 && channelId
                        ? t('models.dialogs.association.noChannelModelsAvailable')
                        : t('models.dialogs.association.selectChannelFirst')
                    }
                    portalContainer={portalContainer}
                  />
                </FormControl>
              </FormItem>
            )}
          />
        )}

        {/* Pattern Input - Only show if NOT on second row */}
        {showPattern && !showModelPatternOnSecondRow && (
          <FormField
            control={form.control}
            name={`associations.${index}.pattern`}
            render={({ field }) => (
              <FormItem className='min-w-0 gap-0'>
                <FormControl>
                  <Input
                    {...field}
                    value={field.value?.toString() || ''}
                    placeholder={t('models.dialogs.association.patternPlaceholder')}
                    className='h-10 sm:h-9'
                  />
                </FormControl>
              </FormItem>
            )}
          />
        )}

        {/* Delete Button */}
        <Button type='button' variant='ghost' size='sm' onClick={onRemove} className='text-destructive hover:text-destructive h-10 sm:h-9 w-10 sm:w-9 p-0'>
          <IconTrash className='h-5 w-5 sm:h-4 sm:w-4' />
        </Button>
      </div>

      {/* Type Select for mobile */}
      <div className='sm:hidden'>
        <FormField
          control={form.control}
          name={`associations.${index}.type`}
          render={({ field }) => (
            <FormItem className='min-w-0 gap-1'>
              <FormLabel className='text-xs'>{t('models.dialogs.association.type')}</FormLabel>
              <FormControl>
                <Select value={field.value} onValueChange={field.onChange}>
                  <SelectTrigger className='h-10 w-full text-xs'>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value='channel_model'>{t('models.dialogs.association.types.channelModel')}</SelectItem>
                    <SelectItem value='channel_regex'>{t('models.dialogs.association.types.channelRegex')}</SelectItem>
                    <SelectItem value='channel_tags_model'>{t('models.dialogs.association.types.channelTagsModel')}</SelectItem>
                    <SelectItem value='channel_tags_regex'>{t('models.dialogs.association.types.channelTagsRegex')}</SelectItem>
                    <SelectItem value='model'>{t('models.dialogs.association.types.model')}</SelectItem>
                    <SelectItem value='regex'>{t('models.dialogs.association.types.regex')}</SelectItem>
                  </SelectContent>
                </Select>
              </FormControl>
              <FormMessage />
            </FormItem>
          )}
        />
      </div>

      {/* Model and Pattern on Second Row for channel_model and channel_regex */}
      {showModelPatternOnSecondRow && (
        <div className='ml-0 sm:ml-[6.25rem] grid gap-2'>
          {showModel && (
            <FormField
              control={form.control}
              name={`associations.${index}.modelId`}
              render={({ field }) => (
                <FormItem className='min-w-0 gap-0'>
                  <FormControl>
                    <AutoComplete
                      selectedValue={field.value?.toString() || ''}
                      onSelectedValueChange={(value) => {
                        field.onChange(value);
                      }}
                      searchValue={modelSearch}
                      onSearchValueChange={setModelSearch}
                      items={modelOptions}
                      placeholder={t('models.dialogs.association.selectModel')}
                      emptyMessage={
                        modelOptions.length === 0 && channelId
                          ? t('models.dialogs.association.noChannelModelsAvailable')
                          : t('models.dialogs.association.selectChannelFirst')
                      }
                      portalContainer={portalContainer}
                    />
                  </FormControl>
                </FormItem>
              )}
            />
          )}
          {showPattern && (
            <FormField
              control={form.control}
              name={`associations.${index}.pattern`}
              render={({ field }) => (
                <FormItem className='min-w-0 gap-0'>
                  <FormControl>
                    <Input
                      {...field}
                      value={field.value?.toString() || ''}
                      placeholder={t('models.dialogs.association.patternPlaceholder')}
                      className='h-10 sm:h-9'
                    />
                  </FormControl>
                </FormItem>
              )}
            />
          )}
        </div>
      )}

      {/* Channel Tags Input - Second Row */}
      {showChannelTags && (
        <div className='ml-0 sm:ml-[6.25rem] grid gap-2'>
          <FormField
            control={form.control}
            name={`associations.${index}.channelTags`}
            render={({ field, fieldState }) => (
              <FormItem className='space-y-1'>
                <FormLabel className='text-xs'>{t('models.dialogs.association.selectChannelTags')}</FormLabel>
                <FormControl>
                  <TagsAutocompleteInput
                    value={field.value || []}
                    onChange={field.onChange}
                    placeholder={t('models.dialogs.association.selectChannelTags')}
                    suggestions={allTags}
                    className='h-auto min-h-10 sm:min-h-9 py-1'
                  />
                </FormControl>
                {fieldState.error && <FormMessage>{fieldState.error.message}</FormMessage>}
              </FormItem>
            )}
          />
        </div>
      )}

      <div className='ml-0 sm:ml-[6.25rem] border-t pt-2'>
        <Button
          type='button'
          variant='ghost'
          size='sm'
          onClick={() => setWhenExpanded(!whenExpanded)}
          className='text-muted-foreground hover:text-foreground mb-2 h-10 sm:h-7 px-3 sm:px-2 text-xs'
        >
          {whenExpanded ? <IconChevronUp className='mr-1 h-4 w-4 sm:h-3 sm:w-3' /> : <IconChevronDown className='mr-1 h-4 w-4 sm:h-3 sm:w-3' />}
          {t('models.dialogs.association.conditions.section')}
          {hasWhenData && !whenExpanded && (
            <Badge variant='secondary' className='ml-2 h-5 sm:h-4 px-2 sm:px-1 text-xs sm:text-[10px]'>
              1
            </Badge>
          )}
        </Button>
        {whenExpanded && (
          <div className='grid gap-3'>
            <FormField
              control={form.control}
              name={`associations.${index}.whenEnabled`}
              render={({ field }) => (
                <div className='flex items-center gap-3'>
                  <Switch
                    checked={field.value}
                    onCheckedChange={(checked) => {
                      field.onChange(checked);
                      if (checked && (form.getValues(`associations.${index}.whenCondition`)?.groups?.length || 0) === 0) {
                        form.setValue(
                          `associations.${index}.whenCondition`,
                          { groups: [DEFAULT_WHEN_GROUP] },
                          { shouldDirty: true, shouldValidate: true }
                        );
                      }
                    }}
                    className='scale-100 sm:scale-75'
                  />
                  <FormLabel className='text-xs'>{t('models.dialogs.association.conditions.enabled')}</FormLabel>
                </div>
              )}
            />
            <FormField
              control={form.control}
              name={`associations.${index}.whenCondition`}
              render={({ field, fieldState }) => (
                <FormItem className='space-y-1'>
                  <FormControl>
                    <FilterBuilder
                      logicLabel={t('models.dialogs.association.conditions.logicLabel')}
                      logicOptions={[
                        { value: 'and', label: t('models.dialogs.association.conditions.and') },
                        { value: 'or', label: t('models.dialogs.association.conditions.or') },
                      ]}
                      value={field.value || DEFAULT_WHEN_CONDITION}
                      onChange={field.onChange}
                      disabled={!whenEnabled}
                      allowNestedGroups={false}
                      maxDepth={MAX_WHEN_CONDITION_DEPTH}
                      singleGroup
                      fields={whenFilterFields.map((item) => ({
                        ...item,
                        label: t(`models.dialogs.association.conditions.fields.${item.value}`),
                        placeholder: t('models.dialogs.association.conditions.valuePlaceholder'),
                        operators: item.operators?.map((operator) => ({
                          value: operator.value,
                          label: t(`models.dialogs.association.conditions.operators.${operator.value}`),
                        })),
                      }))}
                      fieldLabel={t('models.dialogs.association.conditions.fieldLabel')}
                      operatorLabel={t('models.dialogs.association.conditions.operatorLabel')}
                      valueLabel={t('models.dialogs.association.conditions.valueLabel')}
                      addLabel={t('models.dialogs.association.conditions.add')}
                      addGroupLabel={t('prompts.conditions.addGroup')}
                      maxConditionsPerGroup={5}
                      groupJoinLabel={t('models.dialogs.association.conditions.and')}
                    />
                  </FormControl>
                  {fieldState.error && <FormMessage>{fieldState.error.message}</FormMessage>}
                </FormItem>
              )}
            />
            {hasGroupListData(whenCondition) && (
              <p className='text-muted-foreground text-xs'>{t('models.dialogs.association.conditions.conditionsHint')}</p>
            )}
          </div>
        )}
      </div>

      {/* Exclude Section */}
      {showExclude && (
        <div className='ml-0 sm:ml-[6.25rem] border-t pt-2'>
          <Button
            type='button'
            variant='ghost'
            size='sm'
            onClick={() => setExcludeExpanded(!excludeExpanded)}
            className='text-muted-foreground hover:text-foreground mb-2 h-10 sm:h-7 px-3 sm:px-2 text-xs'
          >
            {excludeExpanded ? <IconChevronUp className='mr-1 h-4 w-4 sm:h-3 sm:w-3' /> : <IconChevronDown className='mr-1 h-4 w-4 sm:h-3 sm:w-3' />}
            {t('models.dialogs.association.excludeSection')}
            {hasExcludeData && !excludeExpanded && (
              <Badge variant='secondary' className='ml-2 h-5 sm:h-4 px-2 sm:px-1 text-xs sm:text-[10px]'>
                {(excludeChannelNamePattern ? 1 : 0) + (excludeChannelIds?.length || 0) + (excludeChannelTags?.length || 0)}
              </Badge>
            )}
          </Button>
          {excludeExpanded && (
            <div className='space-y-3'>
              <div className='grid grid-cols-1 sm:grid-cols-2 gap-3'>
                <FormField
                  control={form.control}
                  name={`associations.${index}.excludeChannelNamePattern`}
                  render={({ field }) => (
                    <FormItem className='space-y-1'>
                      <FormLabel className='text-xs'>{t('models.dialogs.association.excludeChannelNamePattern')}</FormLabel>
                      <FormControl>
                        <Input
                          {...field}
                          value={field.value?.toString() || ''}
                          placeholder={t('models.dialogs.association.excludeChannelNamePattern')}
                          className='h-10 sm:h-9'
                        />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />
                <FormField
                  control={form.control}
                  name={`associations.${index}.excludeChannelTags`}
                  render={({ field }) => (
                    <FormItem className='space-y-1'>
                      <FormLabel className='text-xs'>{t('models.dialogs.association.excludeChannelTags')}</FormLabel>
                      <FormControl>
                        <TagsAutocompleteInput
                          value={field.value || []}
                          onChange={field.onChange}
                          placeholder={t('models.dialogs.association.excludeChannelTags')}
                          suggestions={allTags}
                          className='h-auto min-h-10 sm:min-h-9 py-1'
                        />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />
              </div>
              <FormField
                control={form.control}
                name={`associations.${index}.excludeChannelIds`}
                render={({ field }) => (
                  <FormItem className='space-y-1'>
                    <FormLabel className='text-xs'>{t('models.dialogs.association.excludeChannelIds')}</FormLabel>
                    <FormControl>
                      <TagsAutocompleteInput
                        value={(field.value || []).map((id: number) => {
                          const channel = channelOptions.find((opt) => opt.value === id);
                          return channel?.label || id.toString();
                        })}
                        onChange={(tags) => {
                          const ids = tags
                            .map((tag) => {
                              const channel = channelOptions.find((opt) => opt.label === tag);
                              return channel ? channel.value : parseInt(tag);
                            })
                            .filter((id) => !isNaN(id));
                          field.onChange(ids);
                        }}
                        placeholder={t('models.dialogs.association.excludeChannelIds')}
                        suggestions={channelOptions.map((opt) => opt.label)}
                        className='h-auto min-h-10 sm:min-h-9 py-1'
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </div>
          )}
        </div>
      )}

      {/* Hint */}
      {!showExclude &&
        (() => {
          let hint = null;
          const selectedChannel = channelOptions.find((c) => c.value === channelId);
          if (type === 'channel_model' && channelId && modelId) {
            hint = t('models.dialogs.association.ruleHints.channelModel', {
              model: modelId,
              channel: selectedChannel?.label || channelId.toString(),
            });
          } else if (type === 'channel_regex' && channelId && pattern) {
            hint = t('models.dialogs.association.ruleHints.channelRegex', {
              pattern,
              channel: selectedChannel?.label || channelId.toString(),
            });
          } else if (type === 'channel_tags_model' && channelTags && channelTags.length > 0 && modelId) {
            hint = t('models.dialogs.association.ruleHints.channelTagsModel', { model: modelId, tags: channelTags.join(', ') });
          } else if (type === 'channel_tags_regex' && channelTags && channelTags.length > 0 && pattern) {
            hint = t('models.dialogs.association.ruleHints.channelTagsRegex', { pattern, tags: channelTags.join(', ') });
          }
          if (hint) {
            return <div className='text-muted-foreground ml-0 sm:ml-[6.25rem] text-xs'>{hint}</div>;
          }
          return null;
        })()}
    </div>
  );
}
