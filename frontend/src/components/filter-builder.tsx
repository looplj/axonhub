import { IconPlus, IconTrash } from '@tabler/icons-react';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { cn } from '@/lib/utils';

export type FilterBuilderFieldType = 'number' | 'string' | 'boolean';

export type FilterBuilderOperator = {
  label: string;
  value: string;
};

export type FilterBuilderField = {
  label: string;
  operators?: FilterBuilderOperator[];
  placeholder?: string;
  type: FilterBuilderFieldType;
  value: string;
};

export type FilterBuilderCondition = {
  type: 'condition' | 'group';
  logic?: string;
  conditions?: FilterBuilderCondition[];
  field?: string;
  operator?: string;
  value?: string | number | boolean;
};

export type FilterBuilderGroupListValue = {
  groups: FilterBuilderCondition[];
};

type FilterBuilderProps = {
  addLabel: string;
  addGroupLabel?: string;
  className?: string;
  disabled?: boolean;
  fieldLabel: string;
  fields: FilterBuilderField[];
  logicLabel?: string;
  logicOptions?: Array<{ label: string; value: string }>;
  maxConditionsPerGroup?: number;
  maxDepth?: number;
  onChange: (value: FilterBuilderGroupListValue) => void;
  operatorLabel: string;
  value: FilterBuilderGroupListValue;
  valueLabel: string;
  allowNestedGroups?: boolean;
  groupJoinLabel?: string;
  singleGroup?: boolean;
};

const defaultOperatorsByType: Record<FilterBuilderFieldType, FilterBuilderOperator[]> = {
  number: [
    { value: 'eq', label: '=' },
    { value: 'ne', label: '!=' },
    { value: 'lt', label: '<' },
    { value: 'lte', label: '<=' },
    { value: 'gt', label: '>' },
    { value: 'gte', label: '>=' },
  ],
  string: [
    { value: 'eq', label: '=' },
    { value: 'ne', label: '!=' },
  ],
  boolean: [
    { value: 'eq', label: '=' },
    { value: 'ne', label: '!=' },
  ],
};

function getFieldDefinition(fields: FilterBuilderField[], fieldValue?: string) {
  return fields.find((field) => field.value === fieldValue) ?? fields[0];
}

function buildLeafCondition(fields: FilterBuilderField[]): FilterBuilderCondition | null {
  const firstField = fields[0];
  if (!firstField) {
    return null;
  }

  const operators = firstField.operators && firstField.operators.length > 0 ? firstField.operators : defaultOperatorsByType[firstField.type];
  const firstOperator = operators[0];

  return {
    type: 'condition',
    field: firstField.value,
    operator: firstOperator?.value || '',
    value: firstField.type === 'boolean' ? true : '',
  };
}

function buildGroupCondition(): FilterBuilderCondition {
  return {
    type: 'group',
    logic: 'and',
    conditions: [],
  };
}

function normalizeConditionNode(
  node: FilterBuilderCondition,
  depth: number,
  allowNestedGroups: boolean,
  maxDepth: number
) : FilterBuilderCondition[] {
  if (node.type === 'group') {
    const normalizedConditions = (node.conditions || []).flatMap((condition) => normalizeConditionNode(condition, depth + 1, allowNestedGroups, maxDepth));

    if (!allowNestedGroups || depth >= maxDepth) {
      return normalizedConditions;
    }

    return [
      {
        type: 'group',
        logic: node.logic || 'and',
        conditions: normalizedConditions,
      },
    ];
  }

  return [
    {
      type: 'condition',
      field: node.field || '',
      operator: node.operator || '',
      value: node.value ?? '',
    },
  ];
}

function normalizeNode(node: FilterBuilderCondition, allowNestedGroups: boolean, maxDepth: number): FilterBuilderCondition {
  return {
    type: 'group',
    logic: node.logic || 'and',
    conditions: (node.conditions || []).flatMap((condition) => normalizeConditionNode(condition, 1, allowNestedGroups, maxDepth)),
  };
}

function normalizeValue(
  value: FilterBuilderGroupListValue | undefined,
  allowNestedGroups: boolean,
  maxDepth: number,
  singleGroup: boolean
): FilterBuilderGroupListValue {
  const groups = (value?.groups || [])
    .filter((group) => group.type === 'group')
    .map((group) => normalizeNode(group, allowNestedGroups, maxDepth));

  return {
    groups: singleGroup ? groups.slice(0, 1) : groups,
  };
}

type GroupEditorProps = {
  addGroupLabel?: string;
  addLabel: string;
  allowNestedGroups: boolean;
  depth: number;
  disabled?: boolean;
  fieldLabel: string;
  fields: FilterBuilderField[];
  group: FilterBuilderCondition;
  groupLabel?: string;
  logicLabel?: string;
  logicOptions?: Array<{ label: string; value: string }>;
  maxConditionsPerGroup: number;
  maxDepth: number;
  onChange: (group: FilterBuilderCondition) => void;
  onRemove?: () => void;
  operatorLabel: string;
  valueLabel: string;
};

function GroupEditor({
  addGroupLabel,
  addLabel,
  allowNestedGroups,
  depth,
  disabled,
  fieldLabel,
  fields,
  group,
  groupLabel,
  logicLabel,
  logicOptions,
  maxConditionsPerGroup,
  maxDepth,
  onChange,
  onRemove,
  operatorLabel,
  valueLabel,
}: GroupEditorProps) {
  const conditions = group.conditions || [];
  const selectedLogicLabel =
    logicOptions?.find((option) => option.value === (group.logic || logicOptions[0]?.value))?.label || group.logic || '';

  const updateConditions = (nextConditions: FilterBuilderCondition[]) => {
    onChange({
      type: 'group',
      logic: group.logic || 'and',
      conditions: nextConditions,
    });
  };

  const addCondition = () => {
    const nextCondition = buildLeafCondition(fields);
    if (!nextCondition) {
      return;
    }
    updateConditions([...conditions, nextCondition]);
  };

  const addGroup = () => {
    updateConditions([...conditions, buildGroupCondition()]);
  };

  return (
    <div className='rounded-lg border bg-muted/30 p-3'>
      <div className='mb-2 flex items-center justify-between gap-2'>
        <span className='text-xs font-medium text-muted-foreground'>{groupLabel}</span>
        <div className='flex gap-1'>
          {allowNestedGroups && depth < maxDepth && addGroupLabel ? (
            <Button type='button' variant='ghost' size='sm' onClick={addGroup} disabled={disabled} className='h-7 px-2 text-xs'>
              <IconPlus className='mr-1 h-3 w-3' />
              {addGroupLabel}
            </Button>
          ) : null}
          <Button type='button' variant='ghost' size='sm' onClick={addCondition} disabled={disabled || conditions.length >= maxConditionsPerGroup} className='h-7 px-2 text-xs'>
            <IconPlus className='mr-1 h-3 w-3' />
            {addLabel}
          </Button>
          {onRemove ? (
            <Button type='button' variant='ghost' size='sm' onClick={onRemove} disabled={disabled} className='h-7 px-2 text-xs text-muted-foreground hover:text-destructive'>
              <IconTrash className='h-3 w-3' />
            </Button>
          ) : null}
        </div>
      </div>

      <div className='mb-3 flex items-center gap-2'>
        {logicLabel ? <span className='text-xs font-medium text-muted-foreground'>{logicLabel}</span> : null}
        {logicOptions && logicOptions.length > 0 ? (
          <Select value={group.logic || logicOptions[0]?.value} onValueChange={(value) => onChange({ ...group, logic: value })} disabled={disabled}>
            <SelectTrigger className='h-8 w-[10rem] text-xs'>
              <SelectValue placeholder={logicLabel} />
            </SelectTrigger>
            <SelectContent>
              {logicOptions.map((option) => (
                <SelectItem key={option.value} value={option.value}>
                  {option.label}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        ) : null}
      </div>

      <div className='space-y-3'>
        {conditions.length === 0 ? <div className='text-xs text-muted-foreground'>No conditions</div> : null}
        {conditions.map((condition, index) => {
          const itemKey = `${depth}-${index}-${condition.type}-${condition.field || 'group'}`;

          if (condition.type === 'group') {
            return (
              <div key={itemKey}>
                {index > 0 && selectedLogicLabel ? (
                  <div className='relative mb-3 mt-1'>
                    <div className='absolute inset-0 flex items-center' aria-hidden='true'>
                      <div className='w-full border-t border-dashed' />
                    </div>
                    <div className='relative flex justify-center'>
                      <span className='bg-muted/50 px-2 text-[10px] font-semibold uppercase tracking-wider text-muted-foreground'>
                        {selectedLogicLabel}
                      </span>
                    </div>
                  </div>
                ) : null}
                <div className='pl-4'>
                  <GroupEditor
                    addGroupLabel={addGroupLabel}
                    addLabel={addLabel}
                    allowNestedGroups={allowNestedGroups}
                    depth={depth + 1}
                    disabled={disabled}
                    fieldLabel={fieldLabel}
                    fields={fields}
                    group={normalizeNode(condition, allowNestedGroups, maxDepth)}
                    groupLabel={`Group ${depth + 1}`}
                    logicLabel={logicLabel}
                    logicOptions={logicOptions}
                    maxConditionsPerGroup={maxConditionsPerGroup}
                    maxDepth={maxDepth}
                    onChange={(nextGroup) => {
                      updateConditions(conditions.map((item, itemIndex) => (itemIndex === index ? nextGroup : item)));
                    }}
                    onRemove={() => updateConditions(conditions.filter((_, itemIndex) => itemIndex !== index))}
                    operatorLabel={operatorLabel}
                    valueLabel={valueLabel}
                  />
                </div>
              </div>
            );
          }

          const field = getFieldDefinition(fields, condition.field);
          if (!field) {
            return null;
          }

          const operators = field.operators && field.operators.length > 0 ? field.operators : defaultOperatorsByType[field.type];

          return (
            <div key={itemKey}>
              {index > 0 && selectedLogicLabel ? (
                <div className='relative mb-3 mt-1'>
                  <div className='absolute inset-0 flex items-center' aria-hidden='true'>
                    <div className='w-full border-t border-dashed' />
                  </div>
                  <div className='relative flex justify-center'>
                    <span className='bg-muted/50 px-2 text-[10px] font-semibold uppercase tracking-wider text-muted-foreground'>
                      {selectedLogicLabel}
                    </span>
                  </div>
                </div>
              ) : null}
              <div className='grid grid-cols-[10rem_10rem_1fr_2.5rem] items-center gap-2'>
                <Select
                  value={condition.field}
                  onValueChange={(fieldValue) => {
                    const nextField = getFieldDefinition(fields, fieldValue);
                    const nextOperators =
                      nextField?.operators && nextField.operators.length > 0 ? nextField.operators : defaultOperatorsByType[nextField?.type || 'string'];

                    updateConditions(
                      conditions.map((item, itemIndex) =>
                        itemIndex === index
                          ? {
                              type: 'condition',
                              field: fieldValue,
                              operator: nextOperators[0]?.value || '',
                              value: nextField?.type === 'boolean' ? true : '',
                            }
                          : item
                      )
                    );
                  }}
                  disabled={disabled}
                >
                  <SelectTrigger className='h-10 w-full text-xs'>
                    <SelectValue placeholder={fieldLabel} />
                  </SelectTrigger>
                  <SelectContent>
                    {fields.map((item) => (
                      <SelectItem key={item.value} value={item.value}>
                        {item.label}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>

                <Select
                  value={condition.operator}
                  onValueChange={(operator) =>
                    updateConditions(conditions.map((item, itemIndex) => (itemIndex === index ? { ...item, operator } : item)))
                  }
                  disabled={disabled}
                >
                  <SelectTrigger className='h-10 w-full text-xs'>
                    <SelectValue placeholder={operatorLabel} />
                  </SelectTrigger>
                  <SelectContent>
                    {operators.map((operator) => (
                      <SelectItem key={operator.value} value={operator.value}>
                        {operator.label}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>

                {field.type === 'boolean' ? (
                  <Select
                    value={String(condition.value ?? true)}
                    onValueChange={(nextValue) =>
                      updateConditions(conditions.map((item, itemIndex) => (itemIndex === index ? { ...item, value: nextValue === 'true' } : item)))
                    }
                    disabled={disabled}
                  >
                    <SelectTrigger className='h-10 w-full text-xs'>
                      <SelectValue placeholder={valueLabel} />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value='true'>true</SelectItem>
                      <SelectItem value='false'>false</SelectItem>
                    </SelectContent>
                  </Select>
                ) : (
                  <Input
                    type={field.type === 'number' ? 'number' : 'text'}
                    min={field.type === 'number' ? 0 : undefined}
                    value={String(condition.value ?? '')}
                    disabled={disabled}
                    placeholder={field.placeholder}
                    onChange={(e) =>
                      updateConditions(
                        conditions.map((item, itemIndex) =>
                          itemIndex === index
                            ? {
                                ...item,
                                value: field.type === 'number' ? (e.target.value === '' ? '' : Number(e.target.value)) : e.target.value,
                              }
                            : item
                        )
                      )
                    }
                    className='h-10 text-xs'
                  />
                )}

                <Button
                  type='button'
                  variant='ghost'
                  size='icon'
                  disabled={disabled}
                  onClick={() => updateConditions(conditions.filter((_, itemIndex) => itemIndex !== index))}
                  className='h-10 w-10 text-muted-foreground hover:text-destructive'
                >
                  <IconTrash className='h-4 w-4' />
                </Button>
              </div>
            </div>
          );
        })}
      </div>
    </div>
  );
}

export function FilterBuilder({
  addLabel,
  addGroupLabel,
  className,
  disabled,
  fieldLabel,
  fields,
  logicLabel,
  logicOptions,
  maxConditionsPerGroup = 10,
  maxDepth = 1,
  onChange,
  operatorLabel,
  value,
  valueLabel,
  allowNestedGroups = false,
  groupJoinLabel,
  singleGroup = false,
}: FilterBuilderProps) {
  const normalizedValue = normalizeValue(value, allowNestedGroups, maxDepth, singleGroup);
  const displayGroups =
    singleGroup && normalizedValue.groups.length === 0 ? [buildGroupCondition()] : normalizedValue.groups;

  const updateGroups = (nextGroups: FilterBuilderCondition[]) => {
    onChange({
      groups: singleGroup ? nextGroups.slice(0, 1) : nextGroups,
    });
  };

  const addTopLevelGroup = () => {
    updateGroups([...normalizedValue.groups, buildGroupCondition()]);
  };

  return (
    <div className={cn('space-y-3', className)}>
      {!singleGroup ? (
        <div className='flex items-center justify-end'>
          <Button type='button' variant='outline' size='sm' onClick={addTopLevelGroup} disabled={disabled} className='h-8'>
            <IconPlus className='mr-1 h-3.5 w-3.5' />
            {addGroupLabel}
          </Button>
        </div>
      ) : null}

      {displayGroups.length === 0 ? (
        <div className='text-muted-foreground rounded-lg border border-dashed bg-muted/20 p-6 text-center text-sm'>
          No conditions
        </div>
      ) : (
        <div className='space-y-3'>
          {displayGroups.map((group, groupIndex) => (
            <div key={`top-group-${groupIndex}`} className='space-y-3'>
              {groupIndex > 0 && groupJoinLabel ? (
                <div className='relative py-2'>
                  <div className='absolute inset-0 flex items-center' aria-hidden='true'>
                    <div className='w-full border-t border-muted-foreground/20' />
                  </div>
                  <div className='relative flex justify-center'>
                    <span className='bg-background px-3 text-xs font-bold uppercase tracking-widest text-primary'>
                      {groupJoinLabel}
                    </span>
                  </div>
                </div>
              ) : null}
              <GroupEditor
                addGroupLabel={addGroupLabel}
                addLabel={addLabel}
                allowNestedGroups={allowNestedGroups}
                depth={1}
                disabled={disabled}
                fieldLabel={fieldLabel}
                fields={fields}
                group={group}
                groupLabel={`Group ${groupIndex + 1}`}
                logicLabel={logicLabel}
                logicOptions={logicOptions}
                maxConditionsPerGroup={maxConditionsPerGroup}
                maxDepth={maxDepth}
                onChange={(nextGroup) => updateGroups(displayGroups.map((item, itemIndex) => (itemIndex === groupIndex ? nextGroup : item)))}
                onRemove={singleGroup ? undefined : () => updateGroups(displayGroups.filter((_, itemIndex) => itemIndex !== groupIndex))}
                operatorLabel={operatorLabel}
                valueLabel={valueLabel}
              />
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
