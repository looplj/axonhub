import { useEffect, useMemo, useState } from 'react'
import { Command as CommandPrimitive } from 'cmdk'
import { Check } from 'lucide-react'
import { cn } from '@/lib/utils'
import { Command, CommandEmpty, CommandGroup, CommandItem, CommandList } from './ui/command'
import { Input } from './ui/input'
import { Popover, PopoverAnchor, PopoverContent } from './ui/popover'
import { Skeleton } from './ui/skeleton'

type Props<T extends string> = {
  selectedValue: T
  onSelectedValueChange: (value: T) => void
  items: { value: T; label: string }[]
  isLoading?: boolean
  emptyMessage?: string
  placeholder?: string
}

// AutoCompleteSelect: strictly selects from provided items. No free-form values are allowed.
export function AutoCompleteSelect<T extends string>({
  selectedValue,
  onSelectedValueChange,
  items,
  isLoading,
  emptyMessage = 'No items.',
  placeholder = 'Search...',
}: Props<T>) {
  const [open, setOpen] = useState(false)
  const [searchValue, setSearchValue] = useState('')

  // map value -> label for quick lookup
  const labels = useMemo(
    () =>
      items.reduce(
        (acc, item) => {
          acc[item.value] = item.label
          return acc
        },
        {} as Record<string, string>
      ),
    [items]
  )

  // Filter items locally based on search string
  const filtered = useMemo(() => {
    if (!searchValue) return items
    const q = searchValue.toLowerCase()
    return items.filter((it) => it.label.toLowerCase().includes(q))
  }, [items, searchValue])

  const onInputBlur = (e: React.FocusEvent<HTMLInputElement>) => {
    // Strict mode: on blur, revert input text to the selected label
    // unless focus moves within the list
    if (!e.relatedTarget?.hasAttribute('cmdk-list')) {
      setSearchValue(labels[selectedValue] ?? '')
    }
  }

  const onSelectItem = (inputValue: string) => {
    // Only accept values from list
    const exists = items.some((it) => it.value === inputValue)
    if (!exists) return
    onSelectedValueChange(inputValue as T)
    setSearchValue(labels[inputValue] ?? '')
    setOpen(false)
  }

  // Keep the search field in sync with the selected item when selection changes externally
  // This also initializes the field with the current label.
  useEffect(() => {
    setSearchValue(labels[selectedValue] ?? '')
  }, [labels, selectedValue])

  return (
    <div className='flex items-center'>
      <Popover open={open} onOpenChange={setOpen}>
        <Command shouldFilter={false}>
          <PopoverAnchor asChild>
            <CommandPrimitive.Input
              asChild
              value={searchValue}
              onValueChange={setSearchValue}
              onKeyDown={(e) => setOpen(e.key !== 'Escape')}
              onMouseDown={() => setOpen((prev) => !!searchValue || !prev)}
              onFocus={() => setOpen(true)}
              onBlur={onInputBlur}
            >
              <Input placeholder={placeholder} />
            </CommandPrimitive.Input>
          </PopoverAnchor>
          {!open && <CommandList aria-hidden='true' className='hidden' />}
          <PopoverContent
            asChild
            onOpenAutoFocus={(e) => e.preventDefault()}
            onInteractOutside={(e) => {
              if (e.target instanceof Element && e.target.hasAttribute('cmdk-input')) {
                e.preventDefault()
              }
            }}
            className='w-[var(--radix-popover-trigger-width)] max-w-[var(--radix-popover-trigger-width)] overflow-hidden p-0'
          >
            <CommandList className='max-h-72 overflow-auto'>
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
                      className='max-w-full w-full min-w-0'
                    >
                      <Check className={cn('mr-2 h-4 w-4', selectedValue === option.value ? 'opacity-100' : 'opacity-0')} />
                      <span className='truncate flex-1 min-w-0'>{option.label}</span>
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
  )
}

