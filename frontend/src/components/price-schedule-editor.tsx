import { memo, useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { useFieldArray, useFormContext, useWatch, type Control, type FieldArrayPath, type FieldPath } from 'react-hook-form';
import { IconPlus, IconTrash, IconClock, IconCalendar, IconChevronDown, IconCheck } from '@tabler/icons-react';
import { format } from 'date-fns';
import { useTranslation } from 'react-i18next';
import { cn } from '@/lib/utils';
import { useClickOutside } from '@/hooks/use-click-outside';
import { Button } from '@/components/ui/button';
import { FormControl, FormField, FormItem, FormLabel, FormMessage } from '@/components/ui/form';
import { Input } from '@/components/ui/input';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Switch } from '@/components/ui/switch';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import { Calendar } from '@/components/ui/calendar';
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from '@/components/ui/collapsible';
import { AutoCompleteSelect } from '@/components/auto-complete-select';
import { GMTTimeZoneOptions } from '@/features/system/data/timezones';
import { Clock } from 'lucide-react';

const WEEKDAYS = [1, 2, 3, 4, 5, 6, 7] as const;
const HOURS = Array.from({ length: 24 }, (_, i) => i);
const MINUTES = Array.from({ length: 60 }, (_, i) => i);

type PricingMode = 'flat_fee' | 'usage_per_unit' | 'usage_tiered' | 'usage_volume';
const priceItemCodes = ['prompt_tokens', 'completion_tokens', 'prompt_cached_tokens', 'prompt_write_cached_tokens'] as const;

type DateRangeValue = { start: string; end: string };

type ScheduleFormValues = {
  prices: Array<{
    modelId: string;
    price: {
      items: Array<{
        itemCode: (typeof priceItemCodes)[number];
        pricing: {
          mode: PricingMode;
          flatFee?: string | null;
          usagePerUnit?: string | null;
          usageTiered?: {
            tiers: Array<{
              upTo?: number | null;
              pricePerUnit: string;
            }>;
          } | null;
        };
      }>;
      schedule?: {
        timezone: string;
        overrides: Array<{
          name: string;
          priority: number;
          when: {
            dailyTime?: { start: string; end: string } | null;
            weekdays?: number[] | null;
            dateRange?: DateRangeValue | null;
          };
          items: Array<{
            itemCode: (typeof priceItemCodes)[number];
            pricing: {
              mode: PricingMode;
              flatFee?: string | null;
              usagePerUnit?: string | null;
              usageTiered?: {
                tiers: Array<{
                  upTo?: number | null;
                  pricePerUnit: string;
                }>;
              } | null;
            };
          }>;
        }>;
      } | null;
    };
  }>;
};

function asFieldPath(path: string) {
  return path as unknown as FieldPath<ScheduleFormValues>;
}

function asFieldArrayPath(path: string) {
  return path as unknown as FieldArrayPath<ScheduleFormValues>;
}

function useScheduleWatch<TValue>(control: Control<ScheduleFormValues>, name: string) {
  // compute + deepEqual guard: value-unchanged broadcasts (e.g. deleting a card elsewhere)
  // must not force this subscriber to re-render.
  return useWatch({
    control,
    name: asFieldPath(name),
    compute: (value) => value,
  }) as unknown as TValue;
}

function pad2(n: number) {
  return String(n).padStart(2, '0');
}

// ==================== Main Editor ====================

type PriceScheduleEditorProps = {
  control: Control<ScheduleFormValues>;
  priceIndex: number;
  currencyCode?: string;
  defaultTimezone?: string;
};

export const PriceScheduleEditor = memo(function PriceScheduleEditor({
  control,
  priceIndex,
  currencyCode,
  defaultTimezone = 'UTC',
}: PriceScheduleEditorProps) {
  const { t } = useTranslation();
  const { setValue } = useFormContext<ScheduleFormValues>();

  const schedule = useScheduleWatch<ScheduleFormValues['prices'][number]['price']['schedule'] | null | undefined>(
    control,
    `prices.${priceIndex}.price.schedule`
  );

  const isEnabled = schedule != null;

  const handleToggle = useCallback(
    (checked: boolean) => {
      if (checked) {
        setValue(asFieldPath(`prices.${priceIndex}.price.schedule`) as FieldPath<ScheduleFormValues>, {
          timezone: defaultTimezone,
          overrides: [],
        } as never, { shouldDirty: true, shouldValidate: true });
      } else {
        setValue(asFieldPath(`prices.${priceIndex}.price.schedule`) as FieldPath<ScheduleFormValues>, null as never, {
          shouldDirty: true,
          shouldValidate: true,
        });
      }
    },
    [defaultTimezone, priceIndex, setValue]
  );

  return (
    <div className='mt-3 space-y-3'>
      <div className='flex items-center gap-2'>
        <Switch checked={isEnabled} onCheckedChange={handleToggle} />
        <IconClock size={14} className={isEnabled ? 'text-primary' : 'text-muted-foreground'} />
        <span className='text-muted-foreground text-sm'>{t('price.schedule.title')}</span>
      </div>

      {isEnabled && (
        <ScheduleContent
          control={control}
          priceIndex={priceIndex}
          currencyCode={currencyCode}
          defaultTimezone={defaultTimezone}
        />
      )}
    </div>
  );
});

// ==================== Schedule Content (only rendered when enabled) ====================

const ScheduleContent = memo(function ScheduleContent({
  control,
  priceIndex,
  currencyCode,
  defaultTimezone,
}: {
  control: Control<ScheduleFormValues>;
  priceIndex: number;
  currencyCode?: string;
  defaultTimezone: string;
}) {
  const { t } = useTranslation();
  const { setValue } = useFormContext<ScheduleFormValues>();

  const timezoneItems = useMemo(() => GMTTimeZoneOptions, []);

  const {
    fields: overrideFields,
    append: appendOverride,
    remove: removeOverride,
  } = useFieldArray({
    control,
    name: asFieldArrayPath(`prices.${priceIndex}.price.schedule.overrides`),
  });

  const handleAddOverride = useCallback(() => {
    appendOverride({
      name: '',
      priority: 0,
      when: {},
      items: [
        {
          itemCode: 'prompt_tokens',
          pricing: { mode: 'usage_per_unit', usagePerUnit: '0' },
        },
      ],
    });
  }, [appendOverride]);

  const timezoneValue = useScheduleWatch<string>(control, `prices.${priceIndex}.price.schedule.timezone`);

  return (
    <div className='space-y-4 rounded-md border border-dashed p-4'>
      <div className='max-w-xs'>
        <label className='text-xs font-medium'>{t('price.schedule.timezone')}</label>
        <div className='mt-1'>
          <AutoCompleteSelect
            selectedValue={timezoneValue || defaultTimezone}
            onSelectedValueChange={(v) =>
              setValue(asFieldPath(`prices.${priceIndex}.price.schedule.timezone`) as FieldPath<ScheduleFormValues>, v as never, {
                shouldDirty: true,
                shouldValidate: true,
              })
            }
            items={timezoneItems}
            placeholder={t('system.general.timezone.placeholder', { defaultValue: 'Select timezone...' })}
          />
        </div>
      </div>

      <div className='space-y-3'>
        <div className='flex items-center justify-between'>
          <span className='text-muted-foreground text-xs font-medium'>{t('price.schedule.overrides')}</span>
          <Button type='button' variant='outline' size='icon-sm' onClick={handleAddOverride}>
            <IconPlus size={14} />
          </Button>
        </div>

        {overrideFields.map((field, overrideIndex) => (
          <OverrideCard
            key={field.id}
            control={control}
            priceIndex={priceIndex}
            overrideIndex={overrideIndex}
            currencyCode={currencyCode}
            onRemove={() => removeOverride(overrideIndex)}
          />
        ))}

        {overrideFields.length === 0 && (
          <p className='text-muted-foreground py-4 text-center text-xs'>
            {t('price.schedule.noOverrides')}
          </p>
        )}
      </div>
    </div>
  );
});

// ==================== Override Card ====================

type ConditionType = 'dailyTime' | 'weekdays' | 'dateRange';

const CONDITION_TYPES: ConditionType[] = ['dailyTime', 'weekdays', 'dateRange'];

const OverrideCard = memo(function OverrideCard({
  control,
  priceIndex,
  overrideIndex,
  currencyCode,
  onRemove,
}: {
  control: Control<ScheduleFormValues>;
  priceIndex: number;
  overrideIndex: number;
  currencyCode?: string;
  onRemove: () => void;
}) {
  const { t } = useTranslation();
  const { setValue } = useFormContext<ScheduleFormValues>();

  const base = `prices.${priceIndex}.price.schedule.overrides.${overrideIndex}`;
  const when = useScheduleWatch<ScheduleFormValues['prices'][number]['price']['schedule']['overrides'][number]['when']>(
    control,
    `${base}.when`
  );

  // Build condition list from when object
  const conditions: ConditionType[] = [];
  if (when?.dailyTime) conditions.push('dailyTime');
  if (when?.weekdays?.length) conditions.push('weekdays');
  if (when?.dateRange) conditions.push('dateRange');

  const handleAddCondition = useCallback(() => {
    // Add the first available condition type
    const available = CONDITION_TYPES.filter((ct) => !conditions.includes(ct));
    if (available.length === 0) return;
    const nextType = available[0];
    if (nextType === 'dailyTime') {
      setValue(asFieldPath(`${base}.when.dailyTime`) as FieldPath<ScheduleFormValues>, {
        start: '00:00',
        end: '08:00',
      } as never, { shouldDirty: true, shouldValidate: true });
    } else if (nextType === 'weekdays') {
      setValue(asFieldPath(`${base}.when.weekdays`) as FieldPath<ScheduleFormValues>, [1] as never, {
        shouldDirty: true,
        shouldValidate: true,
      });
    } else if (nextType === 'dateRange') {
      setValue(asFieldPath(`${base}.when.dateRange`) as FieldPath<ScheduleFormValues>, {
        start: '',
        end: '',
      } as never, { shouldDirty: true, shouldValidate: true });
    }
  }, [base, conditions, setValue]);

  const handleChangeConditionType = useCallback(
    (oldType: ConditionType, newType: ConditionType) => {
      if (oldType === newType) return;
      // Clear old
      setValue(asFieldPath(`${base}.when.${oldType}`) as FieldPath<ScheduleFormValues>, null as never, {
        shouldDirty: true,
        shouldValidate: true,
      });
      // Set new with default value
      if (newType === 'dailyTime') {
        setValue(asFieldPath(`${base}.when.dailyTime`) as FieldPath<ScheduleFormValues>, {
          start: '00:00',
          end: '08:00',
        } as never, { shouldDirty: true, shouldValidate: true });
      } else if (newType === 'weekdays') {
        setValue(asFieldPath(`${base}.when.weekdays`) as FieldPath<ScheduleFormValues>, [1] as never, {
          shouldDirty: true,
          shouldValidate: true,
        });
      } else if (newType === 'dateRange') {
        setValue(asFieldPath(`${base}.when.dateRange`) as FieldPath<ScheduleFormValues>, {
          start: '',
          end: '',
        } as never, { shouldDirty: true, shouldValidate: true });
      }
    },
    [base, setValue]
  );

  const handleRemoveCondition = useCallback(
    (type: ConditionType) => {
      setValue(asFieldPath(`${base}.when.${type}`) as FieldPath<ScheduleFormValues>, null as never, {
        shouldDirty: true,
        shouldValidate: true,
      });
    },
    [base, setValue]
  );

  const handleToggleWeekday = useCallback(
    (day: number) => {
      const current = when?.weekdays || [];
      const next = current.includes(day) ? current.filter((d) => d !== day) : [...current, day].sort();
      setValue(
        asFieldPath(`${base}.when.weekdays`) as FieldPath<ScheduleFormValues>,
        next.length > 0 ? (next as never) : (null as never),
        { shouldDirty: true, shouldValidate: true }
      );
    },
    [base, setValue, when?.weekdays]
  );

  return (
    <Collapsible defaultOpen className='rounded-md border p-3'>
      <CollapsibleContent className='space-y-3'>
        <div className='grid grid-cols-[1fr_5rem_auto] items-end gap-3'>
          <FormField
            control={control}
            name={asFieldPath(`${base}.name`)}
            render={({ field }) => (
              <FormItem>
                <FormLabel className='text-xs'>{t('price.schedule.override.name')}</FormLabel>
                <FormControl>
                  <Input {...field} value={(field.value as string) || ''} placeholder={t('price.schedule.override.namePlaceholder')} className='h-8 text-sm' />
                </FormControl>
                <FormMessage className='text-[10px]' />
              </FormItem>
            )}
          />

          <FormField
            control={control}
            name={asFieldPath(`${base}.priority`)}
            render={({ field }) => (
              <FormItem>
                <FormLabel className='text-xs'>{t('price.schedule.override.priority')}</FormLabel>
                <FormControl>
                  <Input
                    type='number'
                    min={0}
                    {...field}
                    value={(field.value as number) ?? 0}
                    onChange={(e) => field.onChange(Math.max(0, parseInt(e.target.value) || 0))}
                    className='h-8 text-sm'
                  />
                </FormControl>
                <FormMessage className='text-[10px]' />
              </FormItem>
            )}
          />

          <Button type='button' variant='ghost' size='icon-sm' className='text-destructive' onClick={onRemove}>
            <IconTrash size={14} />
          </Button>
        </div>

        {/* Conditions - array style like override items */}
        <div className='space-y-2'>
          <div className='flex items-center justify-between'>
            <span className='text-muted-foreground text-xs font-medium'>{t('price.schedule.override.when')}</span>
            <Button type='button' variant='outline' size='icon-sm' onClick={handleAddCondition} disabled={conditions.length >= CONDITION_TYPES.length}>
              <IconPlus size={14} />
            </Button>
          </div>

          {conditions.map((condType) => (
            <ConditionRow
              key={condType}
              type={condType}
              allActiveTypes={conditions}
              control={control}
              priceIndex={priceIndex}
              overrideIndex={overrideIndex}
              weekdays={when?.weekdays}
              onTypeChange={(newType) => handleChangeConditionType(condType, newType)}
              onRemove={() => handleRemoveCondition(condType)}
              onToggleWeekday={handleToggleWeekday}
            />
          ))}

          {conditions.length === 0 && (
            <p className='text-muted-foreground py-2 text-center text-xs'>
              {t('price.schedule.when.atLeastOne')}
            </p>
          )}
        </div>

        <OverrideItemsEditor
          control={control}
          priceIndex={priceIndex}
          overrideIndex={overrideIndex}
          currencyCode={currencyCode}
        />
      </CollapsibleContent>
    </Collapsible>
  );
});

// ==================== Condition Row ====================

const ConditionRow = memo(function ConditionRow({
  type,
  allActiveTypes,
  control,
  priceIndex,
  overrideIndex,
  weekdays,
  onTypeChange,
  onRemove,
  onToggleWeekday,
}: {
  type: ConditionType;
  allActiveTypes: ConditionType[];
  control: Control<ScheduleFormValues>;
  priceIndex: number;
  overrideIndex: number;
  weekdays?: number[] | null;
  onTypeChange: (newType: ConditionType) => void;
  onRemove: () => void;
  onToggleWeekday: (day: number) => void;
}) {
  const { t } = useTranslation();

  const base = `prices.${priceIndex}.price.schedule.overrides.${overrideIndex}.when`;

  // Filter out types already used by OTHER rows, but keep current row's type
  const availableTypes = CONDITION_TYPES.filter(
    (ct) => ct === type || !allActiveTypes.includes(ct)
  );

  return (
    <div className='grid grid-cols-[7rem_1fr_auto_1fr_2rem] items-center gap-1'>
      {/* Col 1: type selector */}
      <Select value={type} onValueChange={(v) => onTypeChange(v as ConditionType)}>
        <SelectTrigger size='sm' className='h-8 w-full text-xs'>
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          {availableTypes.map((ct) => (
            <SelectItem key={ct} value={ct}>
              {t(`price.schedule.when.${ct}`)}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>

      {/* Col 2: first value */}
      {type === 'dailyTime' && (
        <HHMMTimePicker control={control} path={`${base}.dailyTime.start`} />
      )}
      {type === 'weekdays' && (
        <div className='col-span-3'>
          <WeekdaysEditor weekdays={weekdays} onToggle={onToggleWeekday} />
        </div>
      )}
      {type === 'dateRange' && (
        <DateRangeSinglePicker control={control} path={`${base}.dateRange.start`} placeholder={t('price.schedule.when.dateRange.start')} />
      )}

      {/* Col 3: arrow */}
      {type !== 'weekdays' && <span className='text-muted-foreground text-xs'>→</span>}

      {/* Col 4: second value */}
      {type === 'dailyTime' && (
        <HHMMTimePicker control={control} path={`${base}.dailyTime.end`} />
      )}
      {type === 'dateRange' && (
        <DateRangeSinglePicker control={control} path={`${base}.dateRange.end`} placeholder={t('price.schedule.when.dateRange.end')} />
      )}

      {/* Col 5: delete button */}
      <Button type='button' variant='ghost' size='icon-sm' className='text-destructive' onClick={onRemove}>
        <IconTrash size={14} />
      </Button>
    </div>
  );
});

// HH:MM time picker with the same visual style as TimeField from date-range-picker
// Reads/writes a single string in "HH:mm" format (e.g. "03:00")
const HHMMTimePicker = memo(function HHMMTimePicker({
  control,
  path,
}: {
  control: Control<ScheduleFormValues>;
  path: string;
}) {
  const { setValue } = useFormContext<ScheduleFormValues>();
  const value = useScheduleWatch<string>(control, path);
  const [open, setOpen] = useState(false);
  const wrapperRef = useRef<HTMLDivElement>(null);

  useClickOutside(wrapperRef, () => setOpen(false), open);

  const displayValue = value || '00:00';
  const [hh, mm] = displayValue.split(':');

  const handlePick = useCallback(
    (newHh: string, newMm: string) => {
      setValue(asFieldPath(path) as FieldPath<ScheduleFormValues>, `${newHh}:${newMm}` as never, { shouldDirty: true });
    },
    [path, setValue]
  );

  return (
    <div className='relative' ref={wrapperRef}>
      <button
        type='button'
        className={cn(
          'border-input bg-transparent',
          'flex h-8 w-full items-center justify-between rounded-md border px-2.5 text-xs',
          'focus:ring-ring focus:ring-2 focus:ring-offset-2 focus:outline-none',
          'disabled:cursor-not-allowed disabled:opacity-50',
          open && 'ring-ring ring-2 ring-offset-2'
        )}
        onClick={() => setOpen(!open)}
      >
        <span>{displayValue}</span>
        <Clock className='h-3.5 w-3.5 opacity-50' />
      </button>

      {open && (
        <div
          className={cn(
            'absolute left-0 top-[calc(100%+8px)] z-50 flex h-[220px] w-full overflow-hidden rounded-md',
            'border border-gray-200 bg-white shadow-2xl dark:border-white/10 dark:bg-[#121214]'
          )}
          role='dialog'
        >
          <div className='no-scrollbar flex-1 overflow-y-auto p-1 text-center'>
            <TimeColInner label='HH' items={HOURS} active={hh || '00'} onPick={(h) => handlePick(h, mm || '00')} />
          </div>
          <div className='no-scrollbar flex-1 overflow-y-auto border-x border-gray-100 p-1 text-center dark:border-white/5'>
            <TimeColInner label='MM' items={MINUTES} active={mm || '00'} onPick={(m) => handlePick(hh || '00', m)} />
          </div>
        </div>
      )}
    </div>
  );
});

function TimeColInner({
  label,
  items,
  active,
  onPick,
}: {
  label: string;
  items: number[];
  active: string;
  onPick: (val: string) => void;
}) {
  return (
    <>
      <span className='sr-only'>{label}</span>
      {items.map((v) => {
        const txt = pad2(v);
        const isActive = txt === active;

        return (
          <button
            key={txt}
            type='button'
            className={cn(
              'w-full rounded-md py-2 text-sm transition-colors',
              isActive
                ? 'glass-highlight border border-primary/20 font-semibold text-primary'
                : 'text-gray-400 hover:bg-gray-100 dark:text-gray-500 dark:hover:bg-white/5'
            )}
            onClick={() => onPick(txt)}
          >
            {txt}
          </button>
        );
      })}
    </>
  );
}

// ==================== Weekdays Editor (multi-select dropdown) ====================

const WeekdaysEditor = memo(function WeekdaysEditor({
  weekdays,
  onToggle,
}: {
  weekdays?: number[] | null;
  onToggle: (day: number) => void;
}) {
  const { t } = useTranslation();
  const [open, setOpen] = useState(false);

  const selected = weekdays || [];
  const displayText = selected.length > 0
    ? selected.map((d) => t(`price.schedule.weekdays.${d}`)).join(', ')
    : '';

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <button
          type='button'
          className={cn(
            'border-input bg-transparent',
            'flex h-8 w-full items-center justify-between rounded-md border px-2.5 text-xs',
            'focus:ring-ring focus:ring-2 focus:ring-offset-2 focus:outline-none'
          )}
        >
          <span className={cn('truncate', !displayText && 'text-muted-foreground')}>
            {displayText || t('price.schedule.when.weekdays')}
          </span>
          <IconChevronDown size={14} className='shrink-0 opacity-50' />
        </button>
      </PopoverTrigger>
      <PopoverContent className='w-48 p-1' align='start'>
        {WEEKDAYS.map((day) => {
          const isSelected = selected.includes(day);
          return (
            <button
              key={day}
              type='button'
              className='flex w-full items-center gap-2 rounded-sm px-2 py-1.5 text-xs hover:bg-accent'
              onClick={() => onToggle(day)}
            >
              <div
                className={cn(
                  'flex h-4 w-4 items-center justify-center rounded-sm border',
                  isSelected ? 'bg-primary border-primary text-primary-foreground' : 'border-input'
                )}
              >
                {isSelected && <IconCheck size={12} />}
              </div>
              <span>{t(`price.schedule.weekdays.${day}`)}</span>
            </button>
          );
        })}
      </PopoverContent>
    </Popover>
  );
});

// ==================== Single Date Picker ====================

const DateRangeSinglePicker = memo(function DateRangeSinglePicker({
  control,
  path,
  placeholder,
}: {
  control: Control<ScheduleFormValues>;
  path: string;
  placeholder: string;
}) {
  return (
    <FormField
      control={control}
      name={asFieldPath(path)}
      render={({ field }) => (
        <FormItem className='flex flex-col'>
          <Popover>
            <PopoverTrigger asChild>
              <FormControl>
                <Button
                  variant='outline'
                  className={`!bg-transparent h-8 w-full justify-start pl-2 text-left text-xs font-normal ${
                    !field.value && 'text-muted-foreground'
                  }`}
                >
                  {field.value || placeholder}
                  <IconCalendar className='ml-auto h-3.5 w-3.5 opacity-50' />
                </Button>
              </FormControl>
            </PopoverTrigger>
            <PopoverContent className='w-auto p-0' align='start'>
              <Calendar
                mode='single'
                selected={field.value ? new Date(field.value) : undefined}
                onSelect={(date) => field.onChange(date ? format(date, 'yyyy-MM-dd') : '')}
                initialFocus
              />
            </PopoverContent>
          </Popover>
          <FormMessage className='text-[10px]' />
        </FormItem>
      )}
    />
  );
});

// ==================== Override Items Editor ====================

const OverrideItemsEditor = memo(function OverrideItemsEditor({
  control,
  priceIndex,
  overrideIndex,
  currencyCode,
}: {
  control: Control<ScheduleFormValues>;
  priceIndex: number;
  overrideIndex: number;
  currencyCode?: string;
}) {
  const { t } = useTranslation();
  const { setValue } = useFormContext<ScheduleFormValues>();

  const items = useWatch({
    control,
    name: asFieldPath(`prices.${priceIndex}.price.schedule.overrides.${overrideIndex}.items`) as FieldPath<ScheduleFormValues>,
    compute: (value) => value,
  }) as unknown as ScheduleFormValues['prices'][number]['price']['schedule']['overrides'][number]['items'] | undefined;

  const handleAddItem = useCallback(() => {
    const currentItems = items || [];
    const existingCodes = new Set(currentItems.map((item) => item.itemCode));
    const nextCode = priceItemCodes.find((code) => !existingCodes.has(code));
    if (!nextCode) return;

    const path = `prices.${priceIndex}.price.schedule.overrides.${overrideIndex}.items` as string;
    setValue(asFieldPath(path) as FieldPath<ScheduleFormValues>, [
      ...currentItems,
      { itemCode: nextCode, pricing: { mode: 'usage_per_unit', usagePerUnit: '0' } },
    ] as never, { shouldDirty: true, shouldValidate: true });
  }, [items, overrideIndex, priceIndex, setValue]);

  const handleRemoveItem = useCallback(
    (itemIndex: number) => {
      const currentItems = items || [];
      const path = `prices.${priceIndex}.price.schedule.overrides.${overrideIndex}.items` as string;
      setValue(
        asFieldPath(path) as FieldPath<ScheduleFormValues>,
        currentItems.filter((_, i) => i !== itemIndex) as never,
        { shouldDirty: true, shouldValidate: true }
      );
    },
    [items, overrideIndex, priceIndex, setValue]
  );

  return (
    <div className='space-y-2'>
      <div className='flex items-center justify-between'>
        <span className='text-muted-foreground text-xs font-medium'>{t('price.schedule.override.items')}</span>
        <Button type='button' variant='outline' size='icon-sm' onClick={handleAddItem}>
          <IconPlus size={14} />
        </Button>
      </div>

      {(items || []).map((item, itemIndex) => (
        <OverrideItemRow
          key={itemIndex}
          control={control}
          priceIndex={priceIndex}
          overrideIndex={overrideIndex}
          itemIndex={itemIndex}
          itemCount={(items || []).length}
          currencyCode={currencyCode}
          onRemove={() => handleRemoveItem(itemIndex)}
        />
      ))}
    </div>
  );
});

// ==================== Override Item Row ====================

const OverrideItemRow = memo(function OverrideItemRow({
  control,
  priceIndex,
  overrideIndex,
  itemIndex,
  itemCount,
  currencyCode,
  onRemove,
}: {
  control: Control<ScheduleFormValues>;
  priceIndex: number;
  overrideIndex: number;
  itemIndex: number;
  itemCount: number;
  currencyCode?: string;
  onRemove: () => void;
}) {
  const { t } = useTranslation();
  const { setValue } = useFormContext<ScheduleFormValues>();
  const base = `prices.${priceIndex}.price.schedule.overrides.${overrideIndex}.items.${itemIndex}`;
  const itemsBase = `prices.${priceIndex}.price.schedule.overrides.${overrideIndex}.items`;

  const pricingMode = useScheduleWatch<string | undefined>(control, `${base}.pricing.mode`);
  const currentItems = useScheduleWatch<Array<{ itemCode: string }> | undefined>(control, itemsBase);

  const currentCode = currentItems?.[itemIndex]?.itemCode;
  const availableItemCodes = priceItemCodes.filter((code) => {
    if (code === currentCode) return true;
    return !currentItems?.some((item, i) => i !== itemIndex && item.itemCode === code);
  });

  // Tier management via useFieldArray
  const {
    fields: tierFields,
    append: appendTier,
    remove: removeTier,
  } = useFieldArray({
    control,
    name: asFieldArrayPath(`${base}.pricing.usageTiered.tiers`),
  });

  const tiers = useScheduleWatch<Array<{ upTo?: number | null; pricePerUnit: string }> | undefined>(
    control,
    `${base}.pricing.usageTiered.tiers`
  );

  // Auto-initialize tiers when switching to tiered/volume mode
  useEffect(() => {
    if ((pricingMode === 'usage_tiered' || pricingMode === 'usage_volume') && tierFields.length === 0) {
      appendTier({ upTo: null, pricePerUnit: '' });
    }
  }, [appendTier, pricingMode, tierFields.length]);

  // Ensure last tier's upTo is always null
  useEffect(() => {
    if (!tiers?.length) return;
    const lastIndex = tiers.length - 1;
    if (tiers[lastIndex]?.upTo !== null) {
      setValue(
        asFieldPath(`${base}.pricing.usageTiered.tiers.${lastIndex}.upTo`) as FieldPath<ScheduleFormValues>,
        null,
        { shouldDirty: true, shouldValidate: true }
      );
    }
  }, [base, setValue, tiers]);

  const currencyPrefix = currencyCode
    ? t('currencies.format', { val: 0, currency: currencyCode, locale: 'en-US' })
        .replace(/[0.,\s]+/g, '')
        .replace(/[A-Z]/g, '')
    : '';

  return (
    <div className='min-w-0 space-y-2'>
      <div className='flex items-end gap-2'>
        <FormField
          control={control}
          name={asFieldPath(`${base}.itemCode`)}
          render={({ field }) => (
            <FormItem className='flex-1'>
              <Select onValueChange={field.onChange} value={field.value as string}>
                <FormControl>
                  <SelectTrigger size='sm' className='h-7 text-xs'>
                    <SelectValue />
                  </SelectTrigger>
                </FormControl>
                <SelectContent>
                  {availableItemCodes.map((code) => (
                    <SelectItem key={code} value={code}>
                      {t(`price.itemCodes.${code}`, { defaultValue: code })}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </FormItem>
          )}
        />

        <FormField
          control={control}
          name={asFieldPath(`${base}.pricing.mode`)}
          render={({ field }) => (
            <FormItem className='flex-1'>
              <Select onValueChange={field.onChange} value={field.value as string}>
                <FormControl>
                  <SelectTrigger size='sm' className='h-7 text-xs'>
                    <SelectValue />
                  </SelectTrigger>
                </FormControl>
                <SelectContent>
                  <SelectItem value='flat_fee'>{t('price.mode_flat_fee')}</SelectItem>
                  <SelectItem value='usage_per_unit'>{t('price.mode_usage_per_unit')}</SelectItem>
                  <SelectItem value='usage_tiered'>{t('price.mode_usage_tiered')}</SelectItem>
                  <SelectItem value='usage_volume'>{t('price.mode_usage_volume')}</SelectItem>
                </SelectContent>
              </Select>
            </FormItem>
          )}
        />

        {pricingMode === 'usage_per_unit' && (
          <FormField
            control={control}
            name={asFieldPath(`${base}.pricing.usagePerUnit`)}
            render={({ field }) => (
              <FormItem className='flex-1'>
                <FormControl>
                  <div className='relative'>
                    {currencyPrefix && (
                      <span className='text-muted-foreground absolute top-1/2 left-2 -translate-y-1/2 text-[10px]'>
                        {currencyPrefix}
                      </span>
                    )}
                    <Input
                      {...field}
                      value={(field.value as string) || ''}
                      placeholder='0.00'
                      className={`h-7 text-right text-xs ${currencyPrefix ? 'pl-7' : ''}`}
                    />
                  </div>
                </FormControl>
              </FormItem>
            )}
          />
        )}

        {pricingMode === 'flat_fee' && (
          <FormField
            control={control}
            name={asFieldPath(`${base}.pricing.flatFee`)}
            render={({ field }) => (
              <FormItem className='flex-1'>
                <FormControl>
                  <div className='relative'>
                    {currencyPrefix && (
                      <span className='text-muted-foreground absolute top-1/2 left-2 -translate-y-1/2 text-[10px]'>
                        {currencyPrefix}
                      </span>
                    )}
                    <Input
                      {...field}
                      value={(field.value as string) || ''}
                      placeholder='0.00'
                      className={`h-7 text-right text-xs ${currencyPrefix ? 'pl-7' : ''}`}
                    />
                  </div>
                </FormControl>
              </FormItem>
            )}
          />
        )}

        {(pricingMode === 'usage_tiered' || pricingMode === 'usage_volume') && <div className='flex-1' />}

        <Button type='button' variant='ghost' size='icon-sm' className='text-destructive' onClick={onRemove} disabled={itemCount <= 1}>
          <IconTrash size={14} />
        </Button>
      </div>

      {/* Tier editor */}
      {(pricingMode === 'usage_tiered' || pricingMode === 'usage_volume') && (
        <div className='ml-4 space-y-2 rounded-md border border-dashed p-2'>
          <div className='text-muted-foreground flex items-center justify-between text-[10px]'>
            <span>{t('price.tiers')}</span>
            <Button
              type='button'
              variant='outline'
              size='icon-sm'
              className='h-5 w-5'
              onClick={() => appendTier({ upTo: null, pricePerUnit: '0' })}
            >
              <IconPlus size={10} />
            </Button>
          </div>
          {tierFields.map((field, tierIndex) => (
            <div key={field.id} className='flex items-center gap-1'>
              <FormField
                control={control}
                name={asFieldPath(`${base}.pricing.usageTiered.tiers.${tierIndex}.upTo`)}
                render={({ field }) => {
                  const isLastTier = tierIndex === tierFields.length - 1;
                  return (
                    <FormItem className='flex-1'>
                      <FormControl>
                        <Input
                          type='number'
                          {...field}
                          value={isLastTier ? '' : (field.value as number | null | undefined) ?? ''}
                          onChange={(e) =>
                            isLastTier
                              ? field.onChange(null)
                              : field.onChange(e.target.value ? parseInt(e.target.value) : null)
                          }
                          placeholder={isLastTier ? '∞' : t('price.upTo')}
                          disabled={isLastTier}
                          className='h-6 text-[10px]'
                        />
                      </FormControl>
                      {!isLastTier && <FormMessage className='text-[10px]' />}
                    </FormItem>
                  );
                }}
                rules={{
                  validate: (val) => {
                    const isLastTier = tierIndex === tierFields.length - 1;
                    if (isLastTier) return true;
                    return typeof val === 'number' ? true : t('price.validation.priceRequired');
                  },
                }}
              />
              <FormField
                control={control}
                name={asFieldPath(`${base}.pricing.usageTiered.tiers.${tierIndex}.pricePerUnit`)}
                render={({ field }) => (
                  <FormItem className='flex-1'>
                    <FormControl>
                      <div className='relative'>
                        {currencyPrefix && (
                          <span className='text-muted-foreground absolute top-1/2 left-1.5 -translate-y-1/2 text-[9px]'>
                            {currencyPrefix}
                          </span>
                        )}
                        <Input
                          {...field}
                          value={(field.value as string) || ''}
                          placeholder={t('price.pricePerUnit')}
                          className={`h-6 text-right text-[10px] ${currencyPrefix ? 'pl-6' : ''}`}
                        />
                      </div>
                    </FormControl>
                    <FormMessage className='text-[10px]' />
                  </FormItem>
                )}
                rules={{ required: t('price.validation.priceRequired') }}
              />
              <Button type='button' variant='ghost' size='icon-sm' onClick={() => removeTier(tierIndex)}>
                <IconTrash size={10} className='text-destructive' />
              </Button>
            </div>
          ))}
        </div>
      )}
    </div>
  );
});
