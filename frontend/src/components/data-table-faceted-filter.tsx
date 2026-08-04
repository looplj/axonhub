import * as React from 'react';
import { CheckIcon, Cross2Icon, PlusCircledIcon } from '@radix-ui/react-icons';
import { Column } from '@tanstack/react-table';
import { useTranslation } from 'react-i18next';
import { cn } from '@/lib/utils';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Command, CommandEmpty, CommandGroup, CommandInput, CommandItem, CommandList, CommandSeparator } from '@/components/ui/command';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import { Separator } from '@/components/ui/separator';

interface DataTableFacetedFilterProps<TData, TValue> {
  column?: Column<TData, TValue>;
  title?: string;
  options?: {
    label: string;
    value: string;
    icon?: React.ComponentType<{ className?: string }>;
  }[];
  singleSelect?: boolean;
  footer?: React.ReactNode;
  selectedValues?: string[];
  onSelectedValuesChange?: (values: string[]) => void;
  isLoading?: boolean;
}

/** Renders the shared searchable faceted-filter UX for table-backed or controlled filters. */
export function DataTableFacetedFilter<TData, TValue>({
  column,
  title,
  options = [],
  singleSelect = false,
  footer,
  selectedValues: controlledSelectedValues,
  onSelectedValuesChange,
  isLoading = false,
}: DataTableFacetedFilterProps<TData, TValue>) {
  const { t } = useTranslation();

  const facets = column?.getFacetedUniqueValues() || new Map();
  const filterValue = column?.getFilterValue();
  const columnSelectedValues = singleSelect ? (filterValue ? [filterValue as string] : []) : ((filterValue || []) as string[]);
  const selectedValues = new Set(controlledSelectedValues ?? columnSelectedValues);

  /** Updates a controlled filter or falls back to the table column filter. */
  const updateSelectedValues = (values: string[]) => {
    if (onSelectedValuesChange) {
      onSelectedValuesChange(values);
      return;
    }

    column?.setFilterValue(singleSelect ? values[0] : values.length > 0 ? values : undefined);
  };

  /** Removes one visible selection without opening or closing the filter popover. */
  const removeSelectedValue = (value: string) => {
    updateSelectedValues(Array.from(selectedValues).filter((selectedValue) => selectedValue !== value));
  };

  return (
    <Popover>
      <PopoverTrigger asChild>
        <Button variant='outline' size='sm' className='h-8 border-dashed'>
          <PlusCircledIcon className='h-4 w-4' />
          {title}
          {selectedValues.size > 0 && (
            <>
              <Separator orientation='vertical' className='mx-2 h-4' />
              <Badge variant='secondary' className='rounded-sm px-1 font-normal lg:hidden'>
                {selectedValues.size}
              </Badge>
              <div className='hidden space-x-1 lg:flex'>
                {selectedValues.size > 2 ? (
                  <Badge variant='secondary' className='rounded-sm px-1 font-normal'>
                    {t('common.selectedItems', { count: selectedValues.size })}
                  </Badge>
                ) : (
                  options
                    ?.filter((option) => selectedValues.has(option.value))
                    .map((option) => (
                      <Badge variant='secondary' key={option.value} className='gap-0.5 rounded-sm py-0 pr-0.5 pl-1 font-normal'>
                        {option.label}
                        <span
                          role='button'
                          tabIndex={0}
                          aria-label={`${t('common.clearFilters')}: ${option.label}`}
                          title={`${t('common.clearFilters')}: ${option.label}`}
                          className='text-muted-foreground hover:bg-muted-foreground/20 hover:text-foreground inline-flex size-4 items-center justify-center rounded-sm transition-colors'
                          onPointerDown={(event) => {
                            // Removing a chip must not toggle the parent popover trigger.
                            event.preventDefault();
                            event.stopPropagation();
                          }}
                          onClick={(event) => {
                            event.preventDefault();
                            event.stopPropagation();
                            removeSelectedValue(option.value);
                          }}
                          onKeyDown={(event) => {
                            if (event.key !== 'Enter' && event.key !== ' ') return;

                            event.preventDefault();
                            event.stopPropagation();
                            removeSelectedValue(option.value);
                          }}
                        >
                          <Cross2Icon className='size-3' />
                        </span>
                      </Badge>
                    ))
                )}
              </div>
            </>
          )}
        </Button>
      </PopoverTrigger>
      <PopoverContent className='w-[200px] p-0' align='start'>
        <Command>
          <CommandInput placeholder={title} />
          <CommandList>
            <CommandEmpty>{isLoading ? t('common.loading') : t('common.noResultsFound')}</CommandEmpty>
            <CommandGroup>
              {options?.map((option) => {
                const isSelected = selectedValues.has(option.value);
                return (
                  <CommandItem
                    key={option.value}
                    onSelect={() => {
                      if (singleSelect) {
                        // Single-select filters clear the active value when it is selected again.
                        updateSelectedValues(isSelected ? [] : [option.value]);
                      } else {
                        // Multi-select filters keep the shared trigger badges in sync with the caller.
                        if (isSelected) {
                          selectedValues.delete(option.value);
                        } else {
                          selectedValues.add(option.value);
                        }
                        updateSelectedValues(Array.from(selectedValues));
                      }
                    }}
                  >
                    <div
                      className={cn(
                        'border-primary flex h-4 w-4 items-center justify-center rounded-sm border',
                        isSelected ? 'bg-primary text-primary-foreground' : 'opacity-50 [&_svg]:invisible'
                      )}
                    >
                      <CheckIcon className={cn('h-4 w-4')} />
                    </div>
                    {option.icon && <option.icon className='text-muted-foreground h-4 w-4' />}
                    <span>{option.label}</span>
                    {facets?.has(option.value) && (
                      <span className='ml-auto flex h-4 w-4 items-center justify-center font-mono text-xs'>{facets.get(option.value)}</span>
                    )}
                  </CommandItem>
                );
              })}
            </CommandGroup>
            {footer && (
              <>
                <CommandSeparator />
                <CommandGroup>{footer}</CommandGroup>
              </>
            )}
            {selectedValues.size > 0 && (
              <>
                <CommandSeparator />
                <CommandGroup>
                  <CommandItem onSelect={() => updateSelectedValues([])} className='justify-center text-center'>
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
