/**
MIT License

Copyright (c) 2025 Leonardo Montini

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
 */
import { useMemo, useState } from 'react';
import { Command as CommandPrimitive } from 'cmdk';
import { Check } from 'lucide-react';
import { cn } from '@/lib/utils';
import { Command, CommandEmpty, CommandGroup, CommandItem, CommandList } from './ui/command';
import { Input } from './ui/input';
import { Popover, PopoverAnchor, PopoverContent } from './ui/popover';
import { Skeleton } from './ui/skeleton';

type Props<T extends string> = {
  selectedValue: T;
  onSelectedValueChange: (value: T) => void;
  searchValue: string;
  onSearchValueChange: (value: string) => void;
  items: { value: T; label: string }[];
  isLoading?: boolean;
  emptyMessage?: string;
  placeholder?: string;
  /** 指定 Popover Portal 的容器元素，用于解决在 Dialog 内无法滚动的问题 */
  portalContainer?: HTMLElement | null;
};

export function AutoComplete<T extends string>({
  selectedValue,
  onSelectedValueChange,
  searchValue,
  onSearchValueChange,
  items,
  isLoading,
  emptyMessage = 'No items.',
  placeholder = 'Search...',
  portalContainer,
}: Props<T>) {
  const [open, setOpen] = useState(false);

  const labels = useMemo(
    () =>
      items.reduce(
        (acc, item) => {
          acc[item.value] = item.label;
          return acc;
        },
        {} as Record<string, string>
      ),
    [items]
  );

  const reset = () => {
    onSelectedValueChange('' as T);
    onSearchValueChange('');
  };

  const onInputBlur = (e: React.FocusEvent<HTMLInputElement>) => {
    // Don't reset if clicking on the command list
    if (e.relatedTarget?.hasAttribute('cmdk-list')) {
      return;
    }
    // If searchValue is empty, clear the selected value
    if (searchValue === '') {
      onSelectedValueChange('' as T);
      return;
    }
    // Keep the current search value as the selected value for custom inputs
    if (searchValue && labels[selectedValue] !== searchValue) {
      onSelectedValueChange(searchValue as T);
    }
  };

  const filtered = useMemo(() => {
    if (!searchValue) return items;
    const q = searchValue.toLowerCase();
    return items.filter((it) => it.label.toLowerCase().includes(q) || it.value.toLowerCase().includes(q));
  }, [items, searchValue]);

  const onSelectItem = (inputValue: string) => {
    if (inputValue === selectedValue) {
      reset();
    } else {
      onSelectedValueChange(inputValue as T);
      const item = items.find((it) => it.value === inputValue);
      onSearchValueChange(item?.label ?? '');
    }
    setOpen(false);
  };

  return (
    <div className='flex w-full items-center'>
      <Popover open={open} onOpenChange={setOpen}>
        <Command shouldFilter={false} className='flex-1 bg-transparent'>
          <PopoverAnchor asChild>
            <CommandPrimitive.Input
              asChild
              value={searchValue}
              onValueChange={onSearchValueChange}
              onKeyDown={(e) => setOpen(e.key !== 'Escape')}
              onMouseDown={() => setOpen((open) => !!searchValue || !open)}
              onFocus={() => setOpen(true)}
              onBlur={onInputBlur}
            >
              <Input placeholder={placeholder} className='w-full' />
            </CommandPrimitive.Input>
          </PopoverAnchor>
          {!open && <CommandList aria-hidden='true' className='hidden' />}
          <PopoverContent
            onOpenAutoFocus={(e) => e.preventDefault()}
            onInteractOutside={(e) => {
              if (e.target instanceof Element && e.target.hasAttribute('cmdk-input')) {
                e.preventDefault();
              }
            }}
            className='w-[var(--radix-popover-trigger-width)] max-w-[var(--radix-popover-trigger-width)] p-0'
            container={portalContainer}
          >
            <CommandList>
              {isLoading && (
                <CommandPrimitive.Loading>
                  <div className='p-1'>
                    <Skeleton className='h-6 w-full' />
                  </div>
                </CommandPrimitive.Loading>
              )}
              {filtered.length > 0 && !isLoading ? (
                <CommandGroup>
                  {filtered.map((option) => (
                    <CommandItem
                      key={option.value}
                      value={option.value}
                      onMouseDown={(e) => e.preventDefault()}
                      onSelect={onSelectItem}
                      className='w-full max-w-full min-w-0'
                    >
                      <Check className={cn('mr-2 h-4 w-4', selectedValue === option.value ? 'opacity-100' : 'opacity-0')} />
                      <span className='min-w-0 flex-1 truncate'>{option.label}</span>
                    </CommandItem>
                  ))}
                </CommandGroup>
              ) : null}
              {!isLoading ? <CommandEmpty>{emptyMessage ?? 'No items.'}</CommandEmpty> : null}
            </CommandList>
          </PopoverContent>
        </Command>
      </Popover>
    </div>
  );
}
