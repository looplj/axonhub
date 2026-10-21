import { useEffect, useMemo, useCallback, useState, useRef, type ReactNode } from 'react';
import { z } from 'zod';
import { useForm, useFieldArray, useWatch, type Resolver } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import {
  IconPlus,
  IconTrash,
  IconChevronDown,
  IconChevronUp,
  IconInfoCircle,
  IconGripVertical,
  IconSnowflake,
  IconSnowflakeOff,
} from '@tabler/icons-react';
import { useQueryModels } from '@/gql/models';
import { DndContext, closestCenter, KeyboardSensor, PointerSensor, useSensor, useSensors, type DragEndEvent } from '@dnd-kit/core';
import { SortableContext, sortableKeyboardCoordinates, useSortable, verticalListSortingStrategy } from '@dnd-kit/sortable';
import { CSS } from '@dnd-kit/utilities';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';
import { extractNumberIDAsNumber } from '@/lib/utils';
import { useDebounce } from '@/hooks/use-debounce';
import { Alert, AlertDescription } from '@/components/ui/alert';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Form, FormControl, FormField, FormItem, FormLabel, FormMessage } from '@/components/ui/form';
import { Input } from '@/components/ui/input';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Switch } from '@/components/ui/switch';
import { TagsAutocompleteInput } from '@/components/ui/tags-autocomplete-input';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import { AutoComplete } from '@/components/auto-complete';
import { AutoCompleteSelect } from '@/components/auto-complete-select';
import {
  FilterBuilder,
  type FilterBuilderCondition,
  type FilterBuilderField,
  type FilterBuilderGroupListValue,
} from '@/components/filter-builder';
import {
  useAllChannelSummarys,
  useAllChannelTags,
  useClearChannelModelCooldown,
  useFreezeChannelModel,
} from '@/features/channels/data/channels';
import { useModelSettings, useUpdateModelSettings } from '@/features/system/data/system';
import { useModels } from '../context/models-context';
import {
  MAX_ASSOCIATION_PRIORITY,
  associationPrioritySchema,
  hasInvalidAssociationPriority,
  nextAssociationPriority,
} from '../data/association-priority';
import { computeAssociationReorder } from '../data/association-reorder';
import { useQueryModelChannelConnections, ModelAssociationInput, ModelChannelConnection } from '../data/models';
import { useUpdateModel } from '../data/models';
import { ModelAssociation, normalizeModelRoutingPolicyValue } from '../data/schema';
import { ChannelModelsCard } from './channel-models-list';

const requestFormatConditionOptions = [
  'openai/chat_completions',
  'openai/completions',
  'openai/responses',
  'openai/responses_compact',
  'openai/image_generation',
  'openai/image_edit',
  'openai/image_variation',
  'openai/embeddings',
  'openai/video',
  'openai/moderations',
  'openai/alpha_search',
  'openai/decisions',
  'openai/audio_speech',
  'openai/audio_transcriptions',
  'openai/audio_translations',
  'anthropic/messages',
  'gemini/contents',
  'gemini/embeddings',
  'aisdk/text',
  'aisdk/datastream',
  'jina/rerank',
  'jina/embeddings',
  'typesafe/systemone',
  'ollama/chat',
  'seedance/video',
] as const;

const contentFeatureConditionFields = [
  { value: 'has_image', label: 'Has image' },
  { value: 'has_video', label: 'Has video' },
  { value: 'has_document', label: 'Has document' },
  { value: 'has_audio', label: 'Has audio' },
] as const;

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
  ...contentFeatureConditionFields.map((field) => ({
    value: field.value,
    label: field.label,
    type: 'boolean' as const,
    operators: [
      { value: 'eq', label: '= Equals' },
      { value: 'ne', label: '!= Not equal' },
    ],
  })),
  {
    value: 'request_format',
    label: 'Request format',
    type: 'string',
    placeholder: 'Select request format',
    operators: [
      { value: 'eq', label: '= Equals' },
      { value: 'ne', label: '!= Not equal' },
    ],
    options: requestFormatConditionOptions.map((format) => ({
      value: format,
      label: format,
    })),
  },
  {
    value: 'reasoning_effort',
    label: 'Reasoning effort',
    type: 'string',
    placeholder: 'e.g. high, xhigh, max',
    operators: [
      { value: 'eq', label: '= Equals' },
      { value: 'ne', label: '!= Not equal' },
    ],
  },
  {
    value: 'daily_time',
    label: 'Daily time',
    type: 'string',
    placeholder: 'HH:mm-HH:mm',
    operators: [
      { value: 'within', label: 'Within' },
      { value: 'not_within', label: 'Not within' },
    ],
  },
  {
    value: 'request_header',
    label: 'Request header',
    type: 'string',
    placeholder: 'Header value',
    operators: [
      { value: 'eq', label: '= Equals' },
      { value: 'ne', label: '!= Not equal' },
      { value: 'contains', label: 'Contains' },
      { value: 'not_contains', label: 'Does not contain' },
      { value: 'start_with', label: 'Starts with' },
      { value: 'end_with', label: 'Ends with' },
    ],
    subField: {
      label: 'Header name',
      placeholder: 'e.g. X-Model',
      separator: '.',
      allowCustom: true,
      suggestions: [
        { value: 'User-Agent', label: 'User-Agent' },
        { value: 'X-Model', label: 'X-Model' },
        { value: 'X-Request-Id', label: 'X-Request-Id' },
        { value: 'X-Trace-Id', label: 'X-Trace-Id' },
        { value: 'Accept', label: 'Accept' },
        { value: 'Accept-Language', label: 'Accept-Language' },
        { value: 'Content-Type', label: 'Content-Type' },
        { value: 'Origin', label: 'Origin' },
        { value: 'Referer', label: 'Referer' },
      ],
    },
  },
];

const REQUEST_HEADER_FIELD_PREFIX = 'request_header.';

const dailyTimeRangePattern = /^([01]\d|2[0-3]):[0-5]\d-([01]\d|2[0-3]):[0-5]\d$/;

function dailyTimeRangeHasDifferentEndpoints(value: string): boolean {
  const [start, end] = value.split('-');
  return Boolean(start && end && start !== end);
}

function isValidConditionOperator(field: string, operator: string): boolean {
  const fieldConfig = resolveWhenFilterField(field);
  if (!fieldConfig) return false;
  return Boolean(fieldConfig.operators?.some((op) => op.value === operator));
}

function resolveWhenFilterField(field?: string) {
  if (!field) {
    return undefined;
  }

  const exact = whenFilterFields.find((f) => f.value === field);
  if (exact) {
    return exact;
  }

  return whenFilterFields.find((f) => f.subField && field.startsWith(f.value + f.subField.separator));
}

function isBooleanConditionField(field?: string): boolean {
  return field === 'stream' || contentFeatureConditionFields.some((item) => item.value === field);
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

function hasDailyTimeConditionNode(condition?: FilterBuilderCondition): boolean {
  if (!condition) {
    return false;
  }

  if (condition.type === 'group') {
    return (condition.conditions || []).some((item) => hasDailyTimeConditionNode(item));
  }

  return condition.field === 'daily_time';
}

function hasDailyTimeCondition(value?: FilterBuilderGroupListValue) {
  return (value?.groups || []).some((group) => hasDailyTimeConditionNode(group));
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
  if (isBooleanConditionField(condition.field) && typeof condition.value !== 'boolean') {
    ctx.addIssue({
      code: z.ZodIssueCode.custom,
      message: 'Value must be a boolean',
      path: [...path, 'value'],
    });
  }
  if (
    (condition.field === 'request_format' || condition.field === 'reasoning_effort' || condition.field === 'daily_time') &&
    typeof condition.value !== 'string'
  ) {
    ctx.addIssue({
      code: z.ZodIssueCode.custom,
      message: 'Value must be text',
      path: [...path, 'value'],
    });
  }
  if (condition.field === 'daily_time' && (typeof condition.value !== 'string' || !dailyTimeRangePattern.test(condition.value))) {
    ctx.addIssue({
      code: z.ZodIssueCode.custom,
      message: 'Value must use HH:mm-HH:mm format',
      path: [...path, 'value'],
    });
  }
  if (
    condition.field === 'daily_time' &&
    typeof condition.value === 'string' &&
    dailyTimeRangePattern.test(condition.value) &&
    !dailyTimeRangeHasDifferentEndpoints(condition.value)
  ) {
    ctx.addIssue({
      code: z.ZodIssueCode.custom,
      message: 'Start and end time must be different',
      path: [...path, 'value'],
    });
  }
  if (condition.field?.startsWith(REQUEST_HEADER_FIELD_PREFIX)) {
    const headerName = condition.field.slice(REQUEST_HEADER_FIELD_PREFIX.length);
    if (!headerName) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: 'Header name is required',
        path: [...path, 'field'],
      });
    }
    if (typeof condition.value !== 'string' || condition.value === '') {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: 'Value must be text',
        path: [...path, 'value'],
      });
    }
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

  groups
    .slice(0, 1)
    .forEach((group, index) => validateWhenConditionNode(group, ctx, [...path, 'groups', index], 1, MAX_WHEN_CONDITION_DEPTH));
}

const associationFormSchema = z.object({
  disableDeveloperSettingsInheritance: z.boolean().default(false),
  loadBalancerStrategy: z.enum(['default', 'adaptive', 'failover', 'circuit-breaker', 'round-robin']).default('default'),
  traceStickyMode: z.enum(['default', 'disabled', 'prefer_previous_channel']).default('default'),
  associations: z
    .array(
      z.object({
        type: z.enum(['channel_model', 'channel_regex', 'model', 'regex', 'channel_tags_model', 'channel_tags_regex']),
        priority: associationPrioritySchema,
        disabled: z.boolean().default(false),
        whenEnabled: z.boolean().default(false),
        whenCondition: z.custom<FilterBuilderGroupListValue>().default(DEFAULT_WHEN_CONDITION),
        inheritModel: z.boolean().default(false),
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
        if (assoc.inheritModel && assoc.type !== 'channel_model' && assoc.type !== 'channel_tags_model') {
          ctx.addIssue({
            code: z.ZodIssueCode.custom,
            message: 'Developer rules can only select channels or channel tags',
            path: [index, 'type'],
          });
        }
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
          if (!assoc.inheritModel && (!assoc.modelId || assoc.modelId.trim() === '')) {
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
type AssociationFormRow = AssociationFormData['associations'][number];
type ExcludeInputList = NonNullable<ModelAssociationInput['regex']>['exclude'];
type ChannelOption = {
  value: number;
  label: string;
  type: string;
  status: string;
  tags: string[];
  allModelEntries: Array<{ requestModel: string; actualModel: string; source: string }>;
};

function associationModelID(assoc: AssociationFormRow, inheritedModelID?: string) {
  if (assoc.inheritModel) {
    return inheritedModelID ?? '';
  }

  return assoc.modelId ?? '';
}

function isCompleteAssociationFormRow(assoc: AssociationFormRow, inheritedModelID?: string) {
  if (assoc.type === 'channel_model') {
    return Boolean(assoc.channelId && (associationModelID(assoc, inheritedModelID) || assoc.inheritModel));
  }
  if (assoc.type === 'channel_regex') {
    return Boolean(assoc.channelId && assoc.pattern);
  }
  if (assoc.type === 'regex') {
    return Boolean(assoc.pattern);
  }
  if (assoc.type === 'model') {
    return Boolean(associationModelID(assoc, inheritedModelID) || assoc.inheritModel);
  }
  if (assoc.type === 'channel_tags_model') {
    return Boolean(
      assoc.channelTags && assoc.channelTags.length > 0 && (associationModelID(assoc, inheritedModelID) || assoc.inheritModel)
    );
  }
  if (assoc.type === 'channel_tags_regex') {
    return Boolean(assoc.channelTags && assoc.channelTags.length > 0 && assoc.pattern);
  }
  return false;
}

function buildExcludeInput(assoc: AssociationFormRow): ExcludeInputList {
  const hasExclude =
    assoc.excludeChannelNamePattern ||
    (assoc.excludeChannelIds && assoc.excludeChannelIds.length > 0) ||
    (assoc.excludeChannelTags && assoc.excludeChannelTags.length > 0);

  if (!hasExclude) {
    return undefined;
  }

  return [
    {
      channelNamePattern: assoc.excludeChannelNamePattern || null,
      channelIds: assoc.excludeChannelIds || null,
      channelTags: assoc.excludeChannelTags || null,
    },
  ];
}

function formAssociationToInput(assoc: AssociationFormRow, inheritedModelID?: string): ModelAssociationInput | undefined {
  if (!isCompleteAssociationFormRow(assoc, inheritedModelID)) {
    return undefined;
  }

  const modelID = associationModelID(assoc, inheritedModelID);
  const base = {
    priority: assoc.priority ?? 0,
    disabled: assoc.disabled ?? false,
    when: buildAssociationWhen(assoc.whenEnabled, assoc.whenCondition) || undefined,
  };

  if (assoc.type === 'channel_model') {
    return {
      ...base,
      type: 'channel_model',
      channelModel: {
        channelId: assoc.channelId!,
        modelId: modelID,
      },
    };
  }

  if (assoc.type === 'channel_regex') {
    return {
      ...base,
      type: 'channel_regex',
      channelRegex: {
        channelId: assoc.channelId!,
        pattern: assoc.pattern!,
      },
    };
  }

  if (assoc.type === 'regex') {
    return {
      ...base,
      type: 'regex',
      regex: {
        pattern: assoc.pattern!,
        exclude: buildExcludeInput(assoc),
      },
    };
  }

  if (assoc.type === 'model') {
    return {
      ...base,
      type: 'model',
      modelId: {
        modelId: modelID,
        exclude: buildExcludeInput(assoc),
      },
    };
  }

  if (assoc.type === 'channel_tags_model') {
    return {
      ...base,
      type: 'channel_tags_model',
      channelTagsModel: {
        channelTags: assoc.channelTags!,
        modelId: modelID,
      },
    };
  }

  return {
    ...base,
    type: 'channel_tags_regex',
    channelTagsRegex: {
      channelTags: assoc.channelTags!,
      pattern: assoc.pattern!,
    },
  };
}

function formAssociationToModelAssociation(assoc: AssociationFormRow): ModelAssociation {
  const input = formAssociationToInput(assoc);
  return {
    type: assoc.type,
    priority: assoc.priority ?? 0,
    disabled: assoc.disabled ?? false,
    when: input?.when ? { enabled: Boolean(input.when.enabled), condition: input.when.condition ?? null } : null,
    channelModel: input?.channelModel || null,
    channelRegex: input?.channelRegex || null,
    regex: input?.regex || null,
    modelId: input?.modelId || null,
    channelTagsModel: input?.channelTagsModel || null,
    channelTagsRegex: input?.channelTagsRegex || null,
  };
}

function modelAssociationToInput(assoc: ModelAssociation, inheritedModelID?: string): ModelAssociationInput {
  const channelModel = assoc.channelModel
    ? {
        ...assoc.channelModel,
        modelId: inheritedModelID ?? assoc.channelModel.modelId,
      }
    : undefined;
  const modelId = assoc.modelId
    ? {
        ...assoc.modelId,
        modelId: inheritedModelID ?? assoc.modelId.modelId,
      }
    : undefined;
  const channelTagsModel = assoc.channelTagsModel
    ? {
        ...assoc.channelTagsModel,
        modelId: inheritedModelID ?? assoc.channelTagsModel.modelId,
      }
    : undefined;

  return {
    type: assoc.type,
    priority: assoc.priority ?? 0,
    disabled: assoc.disabled ?? false,
    when: assoc.when
      ? {
          enabled: assoc.when.enabled,
          condition: assoc.when.condition || undefined,
        }
      : undefined,
    channelModel,
    channelRegex: assoc.channelRegex || undefined,
    regex: assoc.regex || undefined,
    modelId,
    channelTagsModel,
    channelTagsRegex: assoc.channelTagsRegex || undefined,
  };
}

function modelAssociationToFormRow(assoc: ModelAssociation, inheritModel = false): AssociationFormRow {
  const exclude = assoc.regex?.exclude?.[0] || assoc.modelId?.exclude?.[0];
  const promptTokensCondition = readPromptTokensCondition(assoc.when);
  return {
    type: assoc.type,
    priority: assoc.priority ?? 0,
    disabled: assoc.disabled ?? false,
    inheritModel,
    whenEnabled: promptTokensCondition.enabled,
    whenCondition:
      promptTokensCondition.enabled && (promptTokensCondition.condition.groups?.length || 0) === 0
        ? { groups: [DEFAULT_WHEN_GROUP] }
        : promptTokensCondition.condition,
    channelId: assoc.channelModel?.channelId || assoc.channelRegex?.channelId,
    channelTags: assoc.channelTagsModel?.channelTags || assoc.channelTagsRegex?.channelTags || [],
    modelId: inheritModel ? '' : assoc.channelModel?.modelId || assoc.modelId?.modelId || assoc.channelTagsModel?.modelId,
    pattern: assoc.channelRegex?.pattern || assoc.regex?.pattern || assoc.channelTagsRegex?.pattern,
    excludeChannelNamePattern: exclude?.channelNamePattern || '',
    excludeChannelIds: exclude?.channelIds || [],
    excludeChannelTags: exclude?.channelTags || [],
  };
}

function sortAssociationsByPriority<T extends { priority?: number }>(associations: T[]) {
  return [...associations].sort((a, b) => (a.priority ?? 0) - (b.priority ?? 0));
}

function buildDeveloperChannelPreview(associations: AssociationFormRow[], channelOptions: ChannelOption[]): ModelChannelConnection[] {
  return buildDeveloperChannelPreviewWithSources(associations, channelOptions).map((entry) => entry.connection);
}

// Same preview as buildDeveloperChannelPreview, but every entry remembers which
// association rows produced it so the preview list can be dragged and the drag
// can be written back as a priority change. sources is optional: when omitted
// the row index is used as the only source.
function buildDeveloperChannelPreviewWithSources(
  associations: AssociationFormRow[],
  channelOptions: ChannelOption[],
  sources?: number[]
): PreviewEntry[] {
  // One card per channel: several rules can produce the same channel, and
  // duplicated cards would both break sortable ids and look like stale rows
  // that never go away.
  const entriesByKey = new Map<string, PreviewEntry>();

  const addChannel = (channel: ChannelOption, sourceIndex?: number) => {
    const key = String(channel.value);
    const existing = entriesByKey.get(key);
    if (existing) {
      if (sourceIndex !== undefined && !existing.sourceIndices.includes(sourceIndex)) {
        existing.sourceIndices.push(sourceIndex);
      }
      return;
    }

    entriesByKey.set(key, {
      id: key,
      connection: {
        channel: {
          id: channel.value.toString(),
          name: channel.label,
          type: channel.type,
          status: channel.status,
          tags: channel.tags,
        },
        models: [],
      },
      sourceIndices: sourceIndex === undefined ? [] : [sourceIndex],
    });
  };

  sortAssociationsByPriority(associations).forEach((assoc) => {
    if (assoc.disabled) {
      return;
    }

    const originalIndex = associations.indexOf(assoc);
    const sourceIndex = sources?.[originalIndex] ?? originalIndex;

    if (assoc.type === 'channel_model' && assoc.channelId) {
      const channel = channelOptions.find((item) => item.value === assoc.channelId);
      if (channel) {
        addChannel(channel, sourceIndex);
      }
      return;
    }

    if (assoc.type === 'channel_tags_model' && assoc.channelTags && assoc.channelTags.length > 0) {
      channelOptions
        .filter((channel) => assoc.channelTags?.some((tag) => channel.tags.includes(tag)))
        .forEach((channel) => addChannel(channel, sourceIndex));
    }
  });

  return [...entriesByKey.values()];
}

// The preview lists one entry per channel, but a channel can be produced by
// several association rows. Each entry keeps the row indices that produced it so
// dragging it in the preview can rewrite those rows' priorities.
interface PreviewEntry {
  id: string;
  connection: ModelChannelConnection;
  sourceIndices: number[];
}

// Every rule that can produce one preview connection: channel_model matches the
// channel id, channel_tags_model matches channel tags, channel_regex matches the
// channel name. Collecting all of them lets a preview drag always rewrite at
// least one rule priority instead of silently doing nothing.
function collectAssociationSourceIndices(
  associations: AssociationFormRow[] | undefined,
  orderedFields: Array<{ index: number }>,
  connection: ModelChannelConnection,
  channelOptions: ChannelOption[],
): number[] {
  const numericChannelID = extractNumberIDAsNumber(String(connection.channel?.id ?? ''));
  const channelName = connection.channel?.name ?? '';
  // Older GraphQL selections may not carry channel tags; fall back to the
  // channel options loaded for the dialog.
  const channelTags =
    connection.channel?.tags ?? channelOptions.find((opt) => opt.value === numericChannelID)?.tags ?? [];

  const sources: number[] = [];
  for (const { index } of orderedFields) {
    const assoc = associations?.[index];
    if (!assoc || assoc.disabled) {
      continue;
    }

    const cardModels = (connection.models ?? []).map((model) => model.requestModel);

    // One card per channel + key group + model set: two channel_model rules on
    // the same channel with different models must stay two separate cards.
    if (
      assoc.type === 'channel_model' &&
      Number(assoc.channelId) === numericChannelID &&
      cardModels.includes(assoc.modelId ?? '')
    ) {
      sources.push(index);
      continue;
    }

    if (
      assoc.type === 'channel_tags_model' &&
      (assoc.channelTags ?? []).some((tag) => tag && channelTags.includes(tag)) &&
      cardModels.includes(assoc.modelId ?? '')
    ) {
      sources.push(index);
      continue;
    }

    if (assoc.type === 'channel_regex' && assoc.pattern && testRegexPattern(assoc.pattern, channelName)) {
      sources.push(index);
    }
  }

  return sources;
}

// Models of this connection that are currently frozen (channel x model
// cooldown). expiresAt == null means frozen until an operator unfreezes it.
function activeFrozenModels(connection: ModelChannelConnection | undefined, now: number): string[] {
  const cooldowns = connection?.channel?.modelCooldowns ?? [];
  return cooldowns
    .filter((cooldown) => !cooldown.expiresAt || new Date(cooldown.expiresAt).getTime() > now)
    .map((cooldown) => cooldown.model);
}

function testRegexPattern(pattern: string, value: string): boolean {
  try {
    return new RegExp(pattern).test(value);
  } catch {
    return false;
  }
}

// Sortable wrapper for the preview list: same drag mechanics as the rule rows, so
// the preview is reorderable in place instead of being read-only.
function SortablePreviewCard({ id, children }: { id: string; children: (handle: ReactNode, isDragging: boolean) => ReactNode }) {
  const { attributes, listeners, setNodeRef, transform, transition, isDragging } = useSortable({ id });

  const style = {
    transform: CSS.Transform.toString(transform),
    transition,
    opacity: isDragging ? 0.6 : 1,
  };

  const handle = (
    <button
      type='button'
      className='text-muted-foreground hover:text-foreground flex h-7 w-5 shrink-0 cursor-grab touch-none items-center justify-center active:cursor-grabbing'
      aria-label='Drag to reorder preview'
      {...attributes}
      {...listeners}
    >
      <IconGripVertical className='h-4 w-4' />
    </button>
  );

  return (
    <div ref={setNodeRef} style={style} className={isDragging ? 'relative z-50' : undefined}>
      {children(handle, isDragging)}
    </div>
  );
}

// Sortable wrapper: owns the drag handle and the dnd-kit transform, then renders
// the presentational AssociationRow. Kept separate so AssociationRow's props stay
// about the form, not about drag state.
interface SortableAssociationRowProps extends Omit<AssociationRowProps, 'dragHandle'> {
  id: string;
}

function SortableAssociationRow({ id, ...rowProps }: SortableAssociationRowProps) {
  const { attributes, listeners, setNodeRef, transform, transition, isDragging } = useSortable({ id });

  const style = {
    transform: CSS.Transform.toString(transform),
    transition,
    opacity: isDragging ? 0.6 : 1,
  };

  const dragHandle = (
    <button
      type='button'
      className='text-muted-foreground hover:text-foreground flex h-8 w-5 cursor-grab touch-none items-center justify-center active:cursor-grabbing'
      aria-label='Drag to reorder'
      {...attributes}
      {...listeners}
    >
      <IconGripVertical className='h-4 w-4' />
    </button>
  );

  return (
    <div ref={setNodeRef} style={style} className={isDragging ? 'relative z-50' : undefined}>
      <AssociationRow {...rowProps} dragHandle={dragHandle} />
    </div>
  );
}

export function ModelsAssociationDialog() {
  const { t, i18n } = useTranslation();
  const { open, setOpen, currentRow, currentDeveloper, setCurrentDeveloper } = useModels();
  const updateModel = useUpdateModel();
  const updateModelSettings = useUpdateModelSettings();
  const { data: settings, isLoading: isSettingsLoading } = useModelSettings();
  const { data: channelsData } = useAllChannelSummarys(undefined, { enabled: open === 'association' || open === 'developerAssociation' });
  const { data: availableModels, mutateAsync: fetchModels } = useQueryModels();
  const { data: allTags = [] } = useAllChannelTags();
  const { mutateAsync: queryConnections } = useQueryModelChannelConnections();
  const freezeChannelModel = useFreezeChannelModel();
  const clearChannelModelCooldown = useClearChannelModelCooldown();
  const [connections, setConnections] = useState<ModelChannelConnection[]>([]);
  const [channelFilter, setChannelFilter] = useState('');
  const dialogContentRef = useRef<HTMLDivElement>(null);

  const isOpen = open === 'association' || open === 'developerAssociation';
  const isDeveloperMode = open === 'developerAssociation';
  const activeDeveloper = isDeveloperMode ? currentDeveloper : currentRow?.developer || null;
  const developerLabel = useMemo(() => {
    if (!activeDeveloper) return '';
    const key = `models.developers.${activeDeveloper}`;
    return i18n.exists(key) ? t(key) : activeDeveloper;
  }, [activeDeveloper, i18n, t]);
  const developerAssociations = useMemo(
    () => settings?.developerSettings?.find((item) => item.developer === activeDeveloper)?.associations || [],
    [activeDeveloper, settings?.developerSettings]
  );

  useEffect(() => {
    if (isOpen) {
      fetchModels({
        statusIn: ['enabled'],
        includeAllChannelModels: true,
      });
    }
  }, [isOpen, fetchModels]);

  // Build channel options for select
  const channelOptions = useMemo((): ChannelOption[] => {
    if (!channelsData?.edges) return [];
    return channelsData.edges.map((edge) => ({
      value: extractNumberIDAsNumber(edge.node.id),
      label: edge.node.name,
      type: edge.node.type,
      status: edge.node.status,
      tags: edge.node.tags || [],
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
    resolver: zodResolver(associationFormSchema) as Resolver<AssociationFormData>,
    defaultValues: {
      disableDeveloperSettingsInheritance: false,
      loadBalancerStrategy: 'default',
      traceStickyMode: 'default',
      associations: [],
    },
  });

  const disableDeveloperSettingsInheritance = useWatch({
    control: form.control,
    name: 'disableDeveloperSettingsInheritance',
    defaultValue: false,
  });
  const inheritedAssociations = useMemo(
    () => (isDeveloperMode || disableDeveloperSettingsInheritance ? [] : developerAssociations),
    [disableDeveloperSettingsInheritance, isDeveloperMode, developerAssociations]
  );

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
  const debouncedPriorities = useDebounce(JSON.stringify(watchedAssociations.map((assoc) => assoc.priority ?? null)), 500);
  const debouncedAssociations = useMemo(() => {
    try {
      const rows: AssociationFormRow[] = JSON.parse(debouncedAssociationsString) || [];
      return { rows, hasInvalidPriority: hasInvalidAssociationPriority(JSON.parse(debouncedPriorities) || []) };
    } catch {
      return { rows: [], hasInvalidPriority: true };
    }
  }, [debouncedAssociationsString, debouncedPriorities]);

  // Query connections when associations change
  const connectionsFetchSequenceRef = useRef(0);

  useEffect(() => {
    if (!isOpen) {
      setConnections([]);
      return;
    }

    const { rows, hasInvalidPriority } = debouncedAssociations;

    // A fractional or out-of-range priority is rejected by the GraphQL Int input,
    // so the preview stays on the last valid result until the input is corrected.
    if (hasInvalidPriority) {
      return;
    }

    const fetchConnections = async () => {
      const sequence = ++connectionsFetchSequenceRef.current;
      try {
        if (isDeveloperMode) {
          setConnections(buildDeveloperChannelPreview(rows, channelOptions));
          return;
        }

        const inheritedInputs = inheritedAssociations.map((assoc) => modelAssociationToInput(assoc, currentRow?.modelID));
        const formInputs = sortAssociationsByPriority(rows)
          .map((assoc) => formAssociationToInput(assoc))
          .filter((item): item is ModelAssociationInput => item !== undefined);
        const associations = sortAssociationsByPriority([...formInputs, ...inheritedInputs]);

        if (associations.length > 0) {
          const result = await queryConnections(associations);
          // A slower older request must never overwrite a newer preview result.
          if (connectionsFetchSequenceRef.current !== sequence) {
            return;
          }
          setConnections(result);
        } else {
          setConnections([]);
        }
      } catch (_error) {
        if (connectionsFetchSequenceRef.current !== sequence) {
          return;
        }
        toast.error(t('common.errors.loadFailed'));
        setConnections([]);
      }
    };

    fetchConnections();
  }, [channelOptions, currentRow?.modelID, debouncedAssociations, inheritedAssociations, isOpen, isDeveloperMode, queryConnections]);

  useEffect(() => {
    if (isOpen) {
      const associations = isDeveloperMode ? developerAssociations : currentRow?.settings?.associations || [];
      form.reset({
        disableDeveloperSettingsInheritance: isDeveloperMode ? false : (currentRow?.settings?.disableDeveloperSettingsInheritance ?? false),
        loadBalancerStrategy: (isDeveloperMode
          ? 'default'
          : normalizeModelRoutingPolicyValue(currentRow?.settings?.loadBalancerStrategy)) as AssociationFormData['loadBalancerStrategy'],
        traceStickyMode: (isDeveloperMode
          ? 'default'
          : normalizeModelRoutingPolicyValue(currentRow?.settings?.traceStickyMode)) as AssociationFormData['traceStickyMode'],
        associations: associations
          .filter((assoc) => !isDeveloperMode || assoc.type === 'channel_model' || assoc.type === 'channel_tags_model')
          .map((assoc) => modelAssociationToFormRow(assoc, isDeveloperMode)),
      });
    }
  }, [isOpen, isDeveloperMode, currentRow, developerAssociations, form]);

  const onSubmit = async (data: AssociationFormData) => {
    if (isDeveloperMode && (!settings || !activeDeveloper)) return;
    if (!isDeveloperMode && !currentRow) return;

    try {
      const associations = sortAssociationsByPriority(data.associations).map(formAssociationToModelAssociation);

      if (isDeveloperMode) {
        const nextDeveloperSettings = [...(settings?.developerSettings || []).filter((item) => item.developer !== activeDeveloper)];
        if (associations.length > 0) {
          nextDeveloperSettings.push({
            developer: activeDeveloper!,
            associations,
          });
        }

        await updateModelSettings.mutateAsync({
          fallbackToChannelsOnModelNotFound: settings!.fallbackToChannelsOnModelNotFound,
          queryAllChannelModels: settings!.queryAllChannelModels,
          defaultModelAPIIncludeAll: settings!.defaultModelAPIIncludeAll,
          autoReasoningEffort: settings!.autoReasoningEffort,
          modelBlacklistRegex: settings!.modelBlacklistRegex,
          hideUnroutableModelsInList: settings!.hideUnroutableModelsInList,
          developerSettings: nextDeveloperSettings.sort((a, b) => a.developer.localeCompare(b.developer)),
        });
        handleClose();
        return;
      }

      await updateModel.mutateAsync({
        id: currentRow!.id,
        input: {
          settings: {
            disableDeveloperSettingsInheritance: data.disableDeveloperSettingsInheritance ?? false,
            associations,
            loadBalancerStrategy: data.loadBalancerStrategy,
            traceStickyMode: data.traceStickyMode,
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
    setCurrentDeveloper(null);
    form.reset();
    setConnections([]);
    setChannelFilter('');
  }, [setOpen, setCurrentDeveloper, form]);

  const handleAddAssociation = useCallback(() => {
    if (fields.length >= 10) return;

    const currentAssociations = form.getValues('associations') || [];

    append({
      type: 'channel_model',
      priority: nextAssociationPriority(currentAssociations.map((a) => a.priority)),
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
      inheritModel: isDeveloperMode,
    });
  }, [append, fields.length, form, isDeveloperMode]);

  // Rows are always displayed ordered by priority, so the visual order and the
  // priority order are the same thing. Dragging therefore simply rewrites every
  // rule's priority to its new 0-based position.
  // Derived from the watched rows, not from form.getValues: a drag rewrites only
  // the priority values, so a non-reactive read would keep the old order on screen
  // and the drag would look like it did nothing.
  const orderedAssociationFields = useMemo(
    () =>
      fields
        .map((field, index) => ({ field, index }))
        .sort((a, b) => {
          const priorityA = watchedAssociations?.[a.index]?.priority ?? a.index;
          const priorityB = watchedAssociations?.[b.index]?.priority ?? b.index;
          return priorityA - priorityB || a.index - b.index;
        }),
    [fields, watchedAssociations]
  );

  const sensors = useSensors(
    // A small activation distance keeps a plain click on the handle from being
    // treated as a drag.
    useSensor(PointerSensor, { activationConstraint: { distance: 4 } }),
    useSensor(KeyboardSensor, { coordinateGetter: sortableKeyboardCoordinates })
  );

  // The preview has its own sensor set so dragging there never collides with a
  // drag that started on a rule row.
  const previewSensors = useSensors(
    useSensor(PointerSensor, { activationConstraint: { distance: 4 } }),
    useSensor(KeyboardSensor, { coordinateGetter: sortableKeyboardCoordinates })
  );

  const handleAssociationDragEnd = (event: DragEndEvent) => {
    const { active, over } = event;
    if (!over) {
      return;
    }

    const ordered = orderedAssociationFields.map((entry) => ({ id: entry.field.id, index: entry.index }));
    const assignments = computeAssociationReorder(ordered, String(active.id), String(over.id));
    if (!assignments) {
      return;
    }

    // Rewrite priorities so the first row is 0, the second is 1, and so on. The
    // underlying field array is never reordered, so row identities (and their
    // component state) survive the drag.
    assignments.forEach(({ index: rowIndex, priority }) => {
      form.setValue(`associations.${rowIndex}.priority`, priority, {
        shouldDirty: true,
        shouldValidate: true,
      });
    });
  };

  // Filter connections by channel name
  const filteredConnections = useMemo(() => {
    if (!channelFilter.trim()) return connections;
    const filter = channelFilter.toLowerCase().trim();
    return connections.filter((conn) => conn.channel.name.toLowerCase().includes(filter));
  }, [connections, channelFilter]);

  // Preview entries carry the rule rows that produced them, so a drag on the
  // preview can be translated into a priority rewrite of the matching rules. In
  // developer mode the preview is derived locally, so the caller passes the
  // indices in directly.
  const previewEntries: PreviewEntry[] = useMemo(() => {
    if (isDeveloperMode) {
      const built = buildDeveloperChannelPreviewWithSources(
        orderedAssociationFields.map((entry) => entry.index).map((index) => form.getValues(`associations.${index}`)),
        channelOptions,
        orderedAssociationFields.map((entry) => entry.index)
      );
      return built;
    }

    // One card per channel + key group + model set. Several rules that produce
    // exactly the same (channel, group, models) collapse into one card; rules on
    // the same channel with different models stay separate cards.
    const entriesByKey = new Map<string, PreviewEntry>();

    filteredConnections.forEach((connection) => {
      const groupKey = connection.keyGroup ? String(connection.keyGroup.index) : 'default';
      const numericChannelID = extractNumberIDAsNumber(String(connection.channel.id));
      const modelsKey = (connection.models ?? [])
        .map((model) => model.requestModel)
        .sort()
        .join('|');
      const key = `${numericChannelID}:${groupKey}:${modelsKey}`;
      const sourceIndices = collectAssociationSourceIndices(
        watchedAssociations,
        orderedAssociationFields,
        connection,
        channelOptions,
      );

      const existing = entriesByKey.get(key);
      if (existing) {
        for (const sourceIndex of sourceIndices) {
          if (!existing.sourceIndices.includes(sourceIndex)) {
            existing.sourceIndices.push(sourceIndex);
          }
        }
        return;
      }

      entriesByKey.set(key, {
        id: key,
        connection,
        sourceIndices,
      });
    });

    return [...entriesByKey.values()];
  }, [channelOptions, filteredConnections, isDeveloperMode, orderedAssociationFields, watchedAssociations]);

  // Preview rows follow the producing rule's priority: a drop rewrites rule
  // priorities, so the list must reorder from rule order instead of waiting for
  // the debounced backend preview (which made the drag look like it did nothing).
  // Cards of the same channel but different models each carry their own rule, so
  // they can be reordered independently; cards produced by the same rule (e.g.
  // one tags rule matching several channels) keep backend order among themselves.
  const orderedPreviewEntries = useMemo(() => {
    const ruleRank = new Map<number, number>();
    orderedAssociationFields.forEach(({ index }, position) => ruleRank.set(index, position));

    return previewEntries
      .map((entry, index) => {
        const rank =
          entry.sourceIndices.length > 0
            ? Math.min(...entry.sourceIndices.map((ruleIndex) => ruleRank.get(ruleIndex) ?? Number.MAX_SAFE_INTEGER))
            : Number.MAX_SAFE_INTEGER;

        return { entry, index, rank };
      })
      .sort((a, b) => a.rank - b.rank || a.index - b.index)
      .map((item) => item.entry);
  }, [orderedAssociationFields, previewEntries]);

  // Reordering needs at least two cards and at least one card whose producing
  // rule is known, otherwise a drop could not be expressed as a priority change.
  const canReorderPreview = useMemo(() => {
    if (previewEntries.length < 2) {
      return false;
    }

    return previewEntries.some((entry) => entry.sourceIndices.length > 0);
  }, [previewEntries]);

  const handlePreviewDragEnd = (event: DragEndEvent) => {
    const { active, over } = event;
    if (!over || active.id === over.id) {
      return;
    }

    const movingIndex = orderedPreviewEntries.findIndex((entry) => entry.id === String(active.id));
    const targetIndex = orderedPreviewEntries.findIndex((entry) => entry.id === String(over.id));
    if (movingIndex < 0 || targetIndex < 0) {
      return;
    }

    const reordered = [...orderedPreviewEntries];
    const [moved] = reordered.splice(movingIndex, 1);
    reordered.splice(targetIndex, 0, moved);

    // A preview row can be produced by a rule the entry does not carry (a channel
    // matched by tags, or a channel_model rule whose id format differs). Fall back
    // to matching enabled channel_model rules by channel id so a drag always maps
    // onto at least one rule instead of silently doing nothing.
    // A preview row stands for a channel band. Every channel_model rule of that
    // channel has to follow the row, otherwise moving a channel that owns several
    // rules silently rewrites only one of them.
    const ruleIndicesForEntry = (entry: PreviewEntry): number[] => {
      const merged: number[] = [];
      const push = (index: number) => {
        if (!merged.includes(index)) {
          merged.push(index);
        }
      };

      entry.sourceIndices.forEach(push);
      if (merged.length > 0) {
        return merged;
      }

      // Fallback for entries whose rule cannot be derived (unknown id formats):
      // single-model cards match the channel_model rule of that exact model,
      // multi-model cards fall back to the whole channel band.
      const channelId = entry.connection?.channel?.id;
      if (channelId === undefined || channelId === null) {
        return merged;
      }

      const numericChannelID = extractNumberIDAsNumber(String(channelId));
      const cardModels = (entry.connection.models ?? []).map((model) => model.requestModel);
      orderedAssociationFields.forEach(({ index }) => {
        const assoc = watchedAssociations?.[index];
        if (!assoc || assoc.disabled || assoc.type !== 'channel_model' || Number(assoc.channelId) !== numericChannelID) {
          return;
        }
        if (cardModels.length === 1 && assoc.modelId && cardModels[0] !== assoc.modelId) {
          return;
        }
        push(index);
      });

      return merged;
    };

    // Walk the new preview order and hand out priorities row by row. A channel
    // backed by several rules advances the cursor once per rule, so the rules of
    // one channel keep a contiguous priority band.
    const assignments: { index: number; priority: number }[] = [];
    const assigned = new Set<number>();
    let cursor = 0;
    for (const entry of reordered) {
      for (const index of ruleIndicesForEntry(entry)) {
        if (assigned.has(index)) continue;
        assigned.add(index);
        assignments.push({ index, priority: cursor });
        cursor += 1;
      }
    }

    // Rules the preview cannot represent (regex/model rules, disabled rules) keep
    // a stable position after the reordered ones.
    const touched = new Set(assignments.map((item) => item.index));
    orderedAssociationFields.forEach(({ index }) => {
      if (touched.has(index)) return;
      assignments.push({ index, priority: cursor });
      cursor += 1;
    });

    assignments.forEach(({ index, priority }) => {
      form.setValue(`associations.${index}.priority`, priority, {
        shouldDirty: true,
        shouldValidate: true,
      });
    });
  };

  // Replace the cooldown list of one preview channel so the cards reflect a
  // freeze/unfreeze immediately without waiting for the next preview refetch.
  const applyChannelModelCooldowns = useCallback(
    (channelGUID: string, cooldowns: ModelChannelConnection['channel']['modelCooldowns']) => {
      setConnections((prev) =>
        prev.map((conn) =>
          String(conn.channel.id) === String(channelGUID)
            ? { ...conn, channel: { ...conn.channel, modelCooldowns: cooldowns } }
            : conn,
        ),
      );
    },
    [],
  );

  const handleFreezeModel = useCallback(
    async (connection: ModelChannelConnection, model: string, durationMinutes: number) => {
      if (isDeveloperMode || !currentRow) return;
      try {
        await freezeChannelModel.mutateAsync({ channelID: currentRow.id, model, durationMinutes });
        applyChannelModelCooldowns(connection.channel.id, [
          ...(connection.channel.modelCooldowns ?? []).filter(
            (cooldown) => cooldown.model !== model && cooldown.expiresAt && new Date(cooldown.expiresAt).getTime() > Date.now(),
          ),
          {
            model,
            reason: t('models.dialogs.association.cooldown.manualReason'),
            createdAt: new Date().toISOString(),
            expiresAt: durationMinutes > 0 ? new Date(Date.now() + durationMinutes * 60_000).toISOString() : null,
          },
        ]);
        toast.success(t('models.dialogs.association.cooldown.freezeSuccess'));
      } catch {
        // Error is handled by the hook.
      }
    },
    [applyChannelModelCooldowns, currentRow, freezeChannelModel, isDeveloperMode, t],
  );

  const handleUnfreezeModel = useCallback(
    async (connection: ModelChannelConnection, model: string) => {
      if (isDeveloperMode || !currentRow) return;
      try {
        await clearChannelModelCooldown.mutateAsync({ channelID: currentRow.id, model });
        applyChannelModelCooldowns(
          connection.channel.id,
          (connection.channel.modelCooldowns ?? []).filter((cooldown) => cooldown.model !== model),
        );
        toast.success(t('models.dialogs.association.cooldown.unfreezeSuccess'));
      } catch {
        // Error is handled by the hook.
      }
    },
    [applyChannelModelCooldowns, clearChannelModelCooldown, currentRow, isDeveloperMode, t],
  );

  const buildFreezeMenu = useCallback(
    (connection: ModelChannelConnection): ReactNode => {
      if (isDeveloperMode || (connection.models ?? []).length === 0) {
        return null;
      }

      const frozen = activeFrozenModels(connection, Date.now()).filter((model) =>
        (connection.models ?? []).some((entry) => entry.requestModel === model),
      );

      return (
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <button
              type='button'
              className='text-muted-foreground hover:text-foreground hover:bg-muted flex h-6 w-6 items-center justify-center rounded'
              aria-label={t('models.dialogs.association.cooldown.menuLabel')}
            >
              <IconSnowflake className='h-4 w-4' />
            </button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align='end' className='min-w-44'>
            <DropdownMenuItem onClick={() => connection.models.forEach((model) => handleFreezeModel(connection, model.requestModel, 30))}>
              {t('models.dialogs.association.cooldown.freeze30')}
            </DropdownMenuItem>
            <DropdownMenuItem onClick={() => connection.models.forEach((model) => handleFreezeModel(connection, model.requestModel, 1440))}>
              {t('models.dialogs.association.cooldown.freeze24h')}
            </DropdownMenuItem>
            <DropdownMenuItem
              onClick={() => connection.models.forEach((model) => handleFreezeModel(connection, model.requestModel, 0))}
            >
              {t('models.dialogs.association.cooldown.freezeForever')}
            </DropdownMenuItem>
            {frozen.length > 0 && (
              <>
                <DropdownMenuSeparator />
                {frozen.map((model) => (
                  <DropdownMenuItem key={model} onClick={() => handleUnfreezeModel(connection, model)} className='text-cyan-600!'>
                    <IconSnowflakeOff className='mr-1 h-4 w-4' />
                    {t('models.dialogs.association.cooldown.unfreezeItem', { model })}
                  </DropdownMenuItem>
                ))}
              </>
            )}
          </DropdownMenuContent>
        </DropdownMenu>
      );
    },
    [handleFreezeModel, handleUnfreezeModel, isDeveloperMode, t],
  );

  // Render-time snapshot for cooldown expiry checks; every state update in the
  // dialog re-runs this, so badges stay fresh enough for minute-level cooldowns.
  const nowMs = Date.now();

  return (
    <Dialog open={isOpen} onOpenChange={handleClose}>
      <DialogContent ref={dialogContentRef} className='flex h-[90vh] max-h-[800px] w-full max-w-full flex-col overflow-hidden sm:max-w-6xl'>
        <DialogHeader className='shrink-0 text-left'>
          <DialogTitle className='text-lg sm:text-xl'>
            {isDeveloperMode ? t('models.dialogs.developerAssociation.title') : t('models.dialogs.association.title')}
          </DialogTitle>
        </DialogHeader>

        <div className='flex min-h-0 flex-1 flex-col gap-6 overflow-y-auto sm:flex-row sm:overflow-hidden'>
          {/* Left Side - Association Rules */}
          <div className='flex min-w-0 flex-col sm:min-h-0 sm:flex-[2]'>
            <DialogDescription className='shrink-0 text-sm sm:text-base'>
              {isDeveloperMode
                ? t('models.dialogs.developerAssociation.description', { name: developerLabel })
                : t('models.dialogs.association.description', { name: currentRow?.name })}
            </DialogDescription>
            <Alert className='mt-3 shrink-0 py-2.5'>
              <IconInfoCircle className='h-4 w-4' />
              <AlertDescription className='text-xs sm:text-sm'>
                {isDeveloperMode
                  ? t('models.dialogs.developerAssociation.inheritanceHelp', { name: developerLabel })
                  : t('models.dialogs.association.inheritanceHelp')}
              </AlertDescription>
            </Alert>
            {!isDeveloperMode && (
              <div className='mt-3 flex shrink-0 items-start justify-between gap-4 rounded-lg border px-4 py-3'>
                <div className='space-y-1'>
                  <div className='text-sm font-medium'>{t('models.dialogs.association.disableDeveloperInheritance.label')}</div>
                  <p className='text-muted-foreground text-xs sm:text-sm'>
                    {t('models.dialogs.association.disableDeveloperInheritance.description')}
                  </p>
                </div>
                <Switch
                  checked={disableDeveloperSettingsInheritance}
                  onCheckedChange={(checked) =>
                    form.setValue('disableDeveloperSettingsInheritance', checked, {
                      shouldDirty: true,
                      shouldValidate: true,
                    })
                  }
                  className='mt-0.5 shrink-0'
                />
              </div>
            )}

            {/* Rules Section - scrolls internally on desktop; part of the single body scroll on mobile */}
            <div className='flex-1 overflow-y-auto py-4 sm:min-h-0'>
              <Form {...form}>
                <form id='association-form' onSubmit={form.handleSubmit(onSubmit)} className='space-y-3'>
                  {!isDeveloperMode && (
                    <div className='mb-4 grid gap-4 rounded-lg border p-4 sm:grid-cols-2'>
                      <FormField
                        control={form.control}
                        name='loadBalancerStrategy'
                        render={({ field }) => {
                          const description =
                            field.value === 'default'
                              ? t('models.fields.routingInheritanceDescription')
                              : t(`system.retry.loadBalancerStrategy.documentation.${field.value}`);

                          return (
                            <FormItem className='space-y-0'>
                              <div className='flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between sm:gap-3'>
                                <div className='flex items-center gap-1.5'>
                                  <FormLabel className='text-sm font-medium'>{t('models.fields.loadBalancerStrategy')}</FormLabel>
                                  <Tooltip>
                                    <TooltipTrigger asChild>
                                      <button type='button' className='text-muted-foreground hover:text-foreground inline-flex'>
                                        <IconInfoCircle className='h-3.5 w-3.5' />
                                        <span className='sr-only'>{description}</span>
                                      </button>
                                    </TooltipTrigger>
                                    <TooltipContent className='max-w-xs text-xs'>{description}</TooltipContent>
                                  </Tooltip>
                                </div>
                                <FormControl>
                                  <Select value={field.value} onValueChange={field.onChange}>
                                    <SelectTrigger className='w-full sm:w-[140px] sm:shrink-0'>
                                      <SelectValue />
                                    </SelectTrigger>
                                    <SelectContent>
                                      <SelectItem value='default'>{t('models.fields.routingSystemDefault')}</SelectItem>
                                      <SelectItem value='adaptive'>{t('system.retry.loadBalancerStrategy.options.adaptive')}</SelectItem>
                                      <SelectItem value='failover'>{t('system.retry.loadBalancerStrategy.options.failover')}</SelectItem>
                                      <SelectItem value='circuit-breaker'>
                                        {t('system.retry.loadBalancerStrategy.options.circuitBreaker')}
                                      </SelectItem>
                                      <SelectItem value='round-robin'>
                                        {t('system.retry.loadBalancerStrategy.options.roundRobin')}
                                      </SelectItem>
                                    </SelectContent>
                                  </Select>
                                </FormControl>
                              </div>
                              <FormMessage />
                            </FormItem>
                          );
                        }}
                      />

                      <FormField
                        control={form.control}
                        name='traceStickyMode'
                        render={({ field }) => {
                          const description =
                            field.value === 'default'
                              ? t('models.fields.routingInheritanceDescription')
                              : t('system.retry.traceStickyMode.description');

                          return (
                            <FormItem className='space-y-0'>
                              <div className='flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between sm:gap-3'>
                                <div className='flex items-center gap-1.5'>
                                  <FormLabel className='text-sm font-medium'>{t('models.fields.traceStickyMode')}</FormLabel>
                                  <Tooltip>
                                    <TooltipTrigger asChild>
                                      <button type='button' className='text-muted-foreground hover:text-foreground inline-flex'>
                                        <IconInfoCircle className='h-3.5 w-3.5' />
                                        <span className='sr-only'>{description}</span>
                                      </button>
                                    </TooltipTrigger>
                                    <TooltipContent className='max-w-xs text-xs'>{description}</TooltipContent>
                                  </Tooltip>
                                </div>
                                <FormControl>
                                  <Select value={field.value} onValueChange={field.onChange}>
                                    <SelectTrigger className='w-full sm:w-[160px] sm:shrink-0'>
                                      <SelectValue />
                                    </SelectTrigger>
                                    <SelectContent>
                                      <SelectItem value='default'>{t('models.fields.routingSystemDefault')}</SelectItem>
                                      <SelectItem value='prefer_previous_channel'>
                                        {t('system.retry.traceStickyMode.options.preferPreviousChannel')}
                                      </SelectItem>
                                      <SelectItem value='disabled'>{t('system.retry.traceStickyMode.options.disabled')}</SelectItem>
                                    </SelectContent>
                                  </Select>
                                </FormControl>
                              </div>
                              <FormMessage />
                            </FormItem>
                          );
                        }}
                      />
                    </div>
                  )}

                  {fields.length === 0 && (
                    <p className='text-muted-foreground py-8 text-center text-sm'>{t('models.dialogs.association.noRules')}</p>
                  )}

                  {fields.length > 0 && (
                    <div className='grid grid-cols-[1.5rem_2.5rem_4rem_1fr_2.5rem] items-center gap-2 border-b px-3 pb-2 sm:grid-cols-[1.5rem_2.25rem_3rem_14rem_1fr_2.25rem] sm:px-[13px]'>
                      <div />
                      <div />
                      <div className='text-muted-foreground text-center text-xs font-medium'>
                        {t('models.dialogs.association.priority')}
                      </div>
                      <div className='text-muted-foreground hidden text-center text-xs font-medium sm:block'>
                        {t('models.dialogs.association.type')}
                      </div>
                      <div className='text-muted-foreground text-center text-xs font-medium'>{t('models.dialogs.association.rule')}</div>
                      <div />
                    </div>
                  )}
                  {fields.length > 1 && (
                    <p className='text-muted-foreground pl-[13px] text-xs'>{t('models.dialogs.association.dragToReorder')}</p>
                  )}

                  <DndContext sensors={sensors} collisionDetection={closestCenter} onDragEnd={handleAssociationDragEnd}>
                    <SortableContext items={orderedAssociationFields.map((entry) => entry.field.id)} strategy={verticalListSortingStrategy}>
                      {orderedAssociationFields.map(({ field, index }) => (
                        <SortableAssociationRow
                          key={field.id}
                          id={field.id}
                          index={index}
                          form={form}
                          isDeveloperMode={isDeveloperMode}
                          channelOptions={channelOptions}
                          allModelOptions={allModelOptions}
                          allTags={allTags}
                          onRemove={() => remove(index)}
                          portalContainer={dialogContentRef.current}
                        />
                      ))}
                    </SortableContext>
                  </DndContext>
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
          <div className='flex min-w-0 flex-col border-t pt-4 sm:min-h-0 sm:flex-1 sm:border-t-0 sm:border-l sm:pt-0 sm:pl-6'>
            <div className='shrink-0 space-y-2 pb-4'>
              <h3 className='text-sm font-semibold'>{t('models.dialogs.association.preview')}</h3>
              <p className='text-muted-foreground text-xs'>
                {isDeveloperMode
                  ? t('models.dialogs.developerAssociation.previewDescription')
                  : t('models.dialogs.association.previewDescription')}
              </p>
              <Input
                placeholder={t('models.dialogs.association.filterByChannel')}
                value={channelFilter}
                onChange={(e) => setChannelFilter(e.target.value)}
                className='h-9 sm:h-8'
              />
            </div>
            {canReorderPreview && (
              <p className='text-muted-foreground shrink-0 pb-2 text-xs'>{t('models.dialogs.association.previewDragToReorder')}</p>
            )}
            <div className='flex-1 overflow-y-auto sm:min-h-0'>
              <DndContext sensors={previewSensors} collisionDetection={closestCenter} onDragEnd={handlePreviewDragEnd}>
                <SortableContext items={orderedPreviewEntries.map((entry) => entry.id)} strategy={verticalListSortingStrategy}>
                  <div className='space-y-3'>
                    {orderedPreviewEntries.map((entry) => (
                      <SortablePreviewCard key={entry.id} id={entry.id}>
                        {(handle) => (
                          <ChannelModelsCard
                            connection={entry.connection}
                            dragHandle={canReorderPreview ? handle : undefined}
                            frozenModels={activeFrozenModels(entry.connection, nowMs)}
                            freezeMenu={buildFreezeMenu(entry.connection)}
                          />
                        )}
                      </SortablePreviewCard>
                    ))}
                  </div>
                </SortableContext>
              </DndContext>
              {orderedPreviewEntries.length === 0 && (
                <p className='text-muted-foreground py-8 text-center text-sm'>
                  {channelFilter.trim()
                    ? t('models.dialogs.association.noFilteredConnections')
                    : t('models.dialogs.association.noConnections')}
                </p>
              )}
            </div>
          </div>
        </div>

        <DialogFooter className='flex shrink-0 flex-col gap-2 border-t pt-4 sm:flex-row sm:justify-end sm:gap-0'>
          <Button type='button' variant='outline' onClick={handleClose} className='w-full sm:w-auto'>
            {t('common.buttons.cancel')}
          </Button>
          <Button
            type='submit'
            form='association-form'
            disabled={updateModel.isPending || updateModelSettings.isPending || isSettingsLoading || !form.formState.isValid}
            className='w-full sm:w-auto'
          >
            {t('common.buttons.save')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

function readPromptTokensCondition(when: ModelAssociation['when']): { enabled: boolean; condition: FilterBuilderGroupListValue } {
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

  if (!condition.field || !condition.operator || !isValidConditionOperator(condition.field, condition.operator)) {
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

  if (condition.field.startsWith(REQUEST_HEADER_FIELD_PREFIX) && condition.field.slice(REQUEST_HEADER_FIELD_PREFIX.length) === '') {
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
  isDeveloperMode: boolean;
  channelOptions: ChannelOption[];
  allModelOptions: { value: string; label: string }[];
  allTags: string[];
  onRemove: () => void;
  portalContainer: HTMLElement | null;
  // Drag handle rendered as the first grid cell. Supplied by SortableAssociationRow
  // so the row itself stays presentational.
  dragHandle?: ReactNode;
}

function AssociationTypeSelectContent({ isDeveloperMode }: { isDeveloperMode: boolean }) {
  const { t } = useTranslation();

  if (isDeveloperMode) {
    return (
      <SelectContent>
        <SelectItem value='channel_model'>{t('models.dialogs.association.developerTypes.channel')}</SelectItem>
        <SelectItem value='channel_tags_model'>{t('models.dialogs.association.developerTypes.channelTags')}</SelectItem>
      </SelectContent>
    );
  }

  return (
    <SelectContent>
      <SelectItem value='channel_model'>{t('models.dialogs.association.types.channelModel')}</SelectItem>
      <SelectItem value='channel_regex'>{t('models.dialogs.association.types.channelRegex')}</SelectItem>
      <SelectItem value='channel_tags_model'>{t('models.dialogs.association.types.channelTagsModel')}</SelectItem>
      <SelectItem value='channel_tags_regex'>{t('models.dialogs.association.types.channelTagsRegex')}</SelectItem>
      <SelectItem value='model'>{t('models.dialogs.association.types.model')}</SelectItem>
      <SelectItem value='regex'>{t('models.dialogs.association.types.regex')}</SelectItem>
    </SelectContent>
  );
}

function AssociationRow({
  index,
  form,
  isDeveloperMode,
  channelOptions,
  allModelOptions,
  allTags,
  onRemove,
  portalContainer,
  dragHandle,
}: AssociationRowProps) {
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
  const showModel = !isDeveloperMode && (type === 'channel_model' || type === 'model' || type === 'channel_tags_model');
  const showPattern = !isDeveloperMode && (type === 'channel_regex' || type === 'regex' || type === 'channel_tags_regex');
  const showExclude = !isDeveloperMode && (type === 'regex' || type === 'model');
  const showModelPatternOnSecondRow = !isDeveloperMode && (type === 'channel_model' || type === 'channel_regex');
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
      <div className='grid grid-cols-[1.5rem_2.5rem_4rem_1fr_2.5rem] items-center gap-2 sm:grid-cols-[1.5rem_2.25rem_3rem_14rem_1fr_2.25rem]'>
        {/* Drag Handle */}
        <div className='flex items-center justify-center'>{dragHandle}</div>

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
                  max={MAX_ASSOCIATION_PRIORITY}
                  {...field}
                  value={field.value ?? ''}
                  onChange={(e) => {
                    // Keep the raw input so the schema reports out-of-range values instead of clamping them.
                    // An empty input becomes null (not undefined) so react-hook-form does not fall back to the default value.
                    field.onChange(e.target.value ? Number(e.target.value) : null);
                    // The form only validates on submit, but Save is disabled while invalid, so surface the error now.
                    void form.trigger(field.name);
                  }}
                  className='h-10 text-center [-moz-appearance:textfield] sm:h-9 [&::-webkit-inner-spin-button]:m-0 [&::-webkit-inner-spin-button]:hidden [&::-webkit-inner-spin-button]:appearance-none'
                  placeholder='0'
                />
              </FormControl>
              <FormMessage />
            </FormItem>
          )}
        />

        {/* Type Select */}
        <FormField
          control={form.control}
          name={`associations.${index}.type`}
          render={({ field }) => (
            <FormItem className='hidden min-w-0 gap-0 sm:block'>
              <FormControl>
                <Select value={field.value} onValueChange={field.onChange}>
                  <SelectTrigger className='h-10 w-full text-xs sm:h-9'>
                    <SelectValue />
                  </SelectTrigger>
                  <AssociationTypeSelectContent isDeveloperMode={isDeveloperMode} />
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
        <Button
          type='button'
          variant='ghost'
          size='sm'
          onClick={onRemove}
          className='text-destructive hover:text-destructive h-10 w-10 p-0 sm:h-9 sm:w-9'
        >
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
                  <AssociationTypeSelectContent isDeveloperMode={isDeveloperMode} />
                </Select>
              </FormControl>
              <FormMessage />
            </FormItem>
          )}
        />
      </div>

      {/* Model and Pattern on Second Row for channel_model and channel_regex */}
      {showModelPatternOnSecondRow && (
        <div className='ml-0 grid gap-2 sm:ml-[6.25rem]'>
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
        <div className='ml-0 grid gap-2 sm:ml-[6.25rem]'>
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
                    className='h-auto min-h-10 py-1 sm:min-h-9'
                  />
                </FormControl>
                {fieldState.error && <FormMessage>{fieldState.error.message}</FormMessage>}
              </FormItem>
            )}
          />
        </div>
      )}

      <div className='ml-0 border-t pt-2 sm:ml-[6.25rem]'>
        <Button
          type='button'
          variant='ghost'
          size='sm'
          onClick={() => setWhenExpanded(!whenExpanded)}
          className='text-muted-foreground hover:text-foreground mb-2 h-10 px-3 text-xs sm:h-7 sm:px-2'
        >
          {whenExpanded ? (
            <IconChevronUp className='mr-1 h-4 w-4 sm:h-3 sm:w-3' />
          ) : (
            <IconChevronDown className='mr-1 h-4 w-4 sm:h-3 sm:w-3' />
          )}
          {t('models.dialogs.association.conditions.section')}
          {hasWhenData && !whenExpanded && (
            <Badge variant='secondary' className='ml-2 h-5 px-2 text-xs sm:h-4 sm:px-1 sm:text-[10px]'>
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
                        placeholder: t(`models.dialogs.association.conditions.placeholders.${item.value}`, {
                          defaultValue: item.placeholder,
                        }),
                        operators: item.operators?.map((operator) => ({
                          value: operator.value,
                          label: t(`models.dialogs.association.conditions.operators.${operator.value}`),
                        })),
                        options: item.options?.map((option) => ({
                          ...option,
                          label: t(`models.dialogs.association.conditions.formatOptions.${option.value}`, { defaultValue: option.label }),
                        })),
                        subField: item.subField
                          ? {
                              ...item.subField,
                              label: t(`models.dialogs.association.conditions.subFields.${item.value}.label`, {
                                defaultValue: item.subField.label,
                              }),
                              placeholder: t(`models.dialogs.association.conditions.subFields.${item.value}.placeholder`, {
                                defaultValue: item.subField.placeholder,
                              }),
                            }
                          : undefined,
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
              <div className='space-y-1'>
                <p className='text-muted-foreground text-xs'>{t('models.dialogs.association.conditions.conditionsHint')}</p>
                {hasDailyTimeCondition(whenCondition) && (
                  <p className='text-muted-foreground text-xs'>{t('models.dialogs.association.conditions.dailyTimeTimezoneHint')}</p>
                )}
              </div>
            )}
          </div>
        )}
      </div>

      {/* Exclude Section */}
      {showExclude && (
        <div className='ml-0 border-t pt-2 sm:ml-[6.25rem]'>
          <Button
            type='button'
            variant='ghost'
            size='sm'
            onClick={() => setExcludeExpanded(!excludeExpanded)}
            className='text-muted-foreground hover:text-foreground mb-2 h-10 px-3 text-xs sm:h-7 sm:px-2'
          >
            {excludeExpanded ? (
              <IconChevronUp className='mr-1 h-4 w-4 sm:h-3 sm:w-3' />
            ) : (
              <IconChevronDown className='mr-1 h-4 w-4 sm:h-3 sm:w-3' />
            )}
            {t('models.dialogs.association.excludeSection')}
            {hasExcludeData && !excludeExpanded && (
              <Badge variant='secondary' className='ml-2 h-5 px-2 text-xs sm:h-4 sm:px-1 sm:text-[10px]'>
                {(excludeChannelNamePattern ? 1 : 0) + (excludeChannelIds?.length || 0) + (excludeChannelTags?.length || 0)}
              </Badge>
            )}
          </Button>
          {excludeExpanded && (
            <div className='space-y-3'>
              <div className='grid grid-cols-1 gap-3 sm:grid-cols-2'>
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
                          className='h-auto min-h-10 py-1 sm:min-h-9'
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
                        className='h-auto min-h-10 py-1 sm:min-h-9'
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
          if (isDeveloperMode && type === 'channel_model' && channelId) {
            hint = t('models.dialogs.association.developerRuleHints.channel', {
              channel: selectedChannel?.label || channelId.toString(),
            });
          } else if (isDeveloperMode && type === 'channel_tags_model' && channelTags && channelTags.length > 0) {
            hint = t('models.dialogs.association.developerRuleHints.channelTags', { tags: channelTags.join(', ') });
          } else if (type === 'channel_model' && channelId && modelId) {
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
            return <div className='text-muted-foreground ml-0 text-xs sm:ml-[6.25rem]'>{hint}</div>;
          }
          return null;
        })()}
    </div>
  );
}
