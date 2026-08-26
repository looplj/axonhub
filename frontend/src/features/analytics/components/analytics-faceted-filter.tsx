import * as React from 'react';
import { CheckIcon, Cross2Icon, PlusCircledIcon } from '@radix-ui/react-icons';
import { useTranslation } from 'react-i18next';
import { cn } from '@/lib/utils';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Command, CommandEmpty, CommandGroup, CommandInput, CommandItem, CommandList, CommandSeparator } from '@/components/ui/command';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import { Separator } from '@/components/ui/separator';

interface AnalyticsFacetedFilterProps {
  title: string;
  options?: {
    label: string;
    value: string;
    icon?: React.ComponentType<{ className?: string }>;
  }[];
  selectedValues: string[];
  onSelectedValuesChange: (values: string[]) => void;
  isLoading?: boolean;
  searchValue?: string;
  onSearchValueChange?: (value: string) => void;
  hasMore?: boolean;
  isLoadingMore?: boolean;
  onLoadMore?: () => void | Promise<unknown>;
}

/** Renders a searchable faceted filter controlled by the analytics filter store. */
export function AnalyticsFacetedFilter({
  title,
  options = [],
  selectedValues,
  onSelectedValuesChange,
  isLoading = false,
  searchValue,
  onSearchValueChange,
  hasMore = false,
  isLoadingMore = false,
  onLoadMore,
}: AnalyticsFacetedFilterProps) {
  const { t } = useTranslation();
  const selectedValueSet = new Set(selectedValues);
  const optionValueSet = new Set(options.map((option) => option.value));
  const selectedOptions = Array.from(
    selectedValueSet,
    (value) => options.find((option) => option.value === value) ?? { label: value, value }
  );
  // Missing options are not available in the popover, so keep them individually removable beside the collapsed count.
  const removableSelectedOptions =
    selectedValueSet.size > 2 ? selectedOptions.filter((option) => !optionValueSet.has(option.value)) : selectedOptions;
  // Keep selected options at the top while preserving the source order within both groups.
  const orderedOptions = [
    ...options.filter((option) => selectedValueSet.has(option.value)),
    ...options.filter((option) => !selectedValueSet.has(option.value)),
  ];

  /** Removes one selected value without opening the sibling popover trigger. */
  const removeSelectedValue = (value: string) => {
    onSelectedValuesChange(selectedValues.filter((selectedValue) => selectedValue !== value));
  };

  return (
    <Popover
      onOpenChange={(open) => {
        if (!open) onSearchValueChange?.('');
      }}
    >
      <div className='bg-background dark:bg-input/30 dark:border-input flex h-8 w-fit items-center rounded-md border border-dashed shadow-xs'>
        <PopoverTrigger asChild>
          <Button variant='ghost' size='sm' className='h-full rounded-md border-0 shadow-none'>
            <PlusCircledIcon className='h-4 w-4' />
            {title}
          </Button>
        </PopoverTrigger>
        {selectedValueSet.size > 0 && (
          <>
            <Separator orientation='vertical' className='mx-2 h-4' />
            <div className='mr-1 flex space-x-1'>
              {selectedValueSet.size > 2 && (
                <Badge variant='secondary' className='rounded-sm px-1 font-normal'>
                  {t('common.selectedItems', { count: selectedValueSet.size })}
                </Badge>
              )}
              {removableSelectedOptions.map((option) => (
                <Badge variant='secondary' key={option.value} className='gap-0.5 rounded-sm py-0 pr-0.5 pl-1 font-normal'>
                  {option.label}
                  <button
                    type='button'
                    aria-label={`${t('common.clearFilters')}: ${option.label}`}
                    title={`${t('common.clearFilters')}: ${option.label}`}
                    className='text-muted-foreground hover:bg-muted-foreground/20 hover:text-foreground inline-flex size-4 items-center justify-center rounded-sm transition-colors'
                    onClick={() => removeSelectedValue(option.value)}
                  >
                    <Cross2Icon className='size-3' />
                  </button>
                </Badge>
              ))}
            </div>
          </>
        )}
      </div>
      <PopoverContent className='w-[200px] p-0' align='start'>
        <Command shouldFilter={onSearchValueChange ? false : undefined}>
          <CommandInput
            placeholder={title}
            value={onSearchValueChange ? (searchValue ?? '') : undefined}
            onValueChange={onSearchValueChange}
          />
          <CommandList hasMore={hasMore} isLoadingMore={isLoadingMore} onLoadMore={onLoadMore}>
            {!onSearchValueChange && <CommandEmpty>{isLoading ? t('common.loading') : t('common.noResultsFound')}</CommandEmpty>}
            {onSearchValueChange && orderedOptions.length === 0 ? (
              <div className='py-6 text-center text-sm'>{isLoading ? t('common.loading') : t('common.noResultsFound')}</div>
            ) : (
              <CommandGroup>
                {orderedOptions.map((option) => {
                  const isSelected = selectedValueSet.has(option.value);
                  return (
                    <CommandItem
                      key={option.value}
                      onSelect={() =>
                        onSelectedValuesChange(
                          isSelected
                            ? selectedValues.filter((selectedValue) => selectedValue !== option.value)
                            : [...selectedValues, option.value]
                        )
                      }
                    >
                      <div
                        className={cn(
                          'border-primary flex h-4 w-4 items-center justify-center rounded-sm border',
                          isSelected ? 'bg-primary text-primary-foreground' : 'opacity-50 [&_svg]:invisible'
                        )}
                      >
                        <CheckIcon className='h-4 w-4' />
                      </div>
                      {option.icon && <option.icon className='text-muted-foreground h-4 w-4' />}
                      <span>{option.label}</span>
                    </CommandItem>
                  );
                })}
              </CommandGroup>
            )}
            {isLoadingMore && <div className='py-2 text-center text-sm'>{t('common.loading')}</div>}
            {selectedValueSet.size > 0 && (
              <>
                <CommandSeparator />
                <CommandGroup>
                  <CommandItem onSelect={() => onSelectedValuesChange([])} className='justify-center text-center'>
                    {t('common.clearFilters')}
                  </CommandItem>
                </CommandGroup>
              </>
            )}
          </CommandList>
        </Command>
      </PopoverContent>
    </Popover>
  );
}
