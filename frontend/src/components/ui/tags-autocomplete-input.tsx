'use client'

import { forwardRef, useCallback, useEffect, useRef, useState } from 'react'
import { Check } from 'lucide-react'
import { X } from 'lucide-react'
import { cn } from '@/lib/utils'
import { Popover, PopoverContent, PopoverTrigger } from './popover'
import { TruncatedText } from '@/components/truncated-text'

interface TagsAutocompleteInputProps {
  value: string[]
  onChange: (tags: string[]) => void
  placeholder?: string
  className?: string
  suggestions?: string[]
  isLoading?: boolean
}

export const TagsAutocompleteInput = forwardRef<HTMLDivElement, TagsAutocompleteInputProps>(
  ({ value = [], onChange, placeholder, className, suggestions = [], isLoading }, ref) => {
    const [inputValue, setInputValue] = useState('')
    const [open, setOpen] = useState(false)
    const [isComposing, setIsComposing] = useState(false)
    const containerRef = useRef<HTMLDivElement>(null)
    const inputRef = useRef<HTMLInputElement>(null)

    // Filter suggestions based on input and not already selected
    const filteredSuggestions = suggestions.filter((s) => {
      if (value.includes(s)) return false
      if (!inputValue.trim()) return true
      return s.toLowerCase().includes(inputValue.toLowerCase())
    })

    const handleInputChange = (e: React.ChangeEvent<HTMLInputElement>) => {
      setInputValue(e.target.value)
      if (e.target.value && !open && suggestions.length > 0) {
        setOpen(true)
      } else if (!e.target.value) {
        setOpen(false)
      }
    }

    const handleKeyDown = (e: React.KeyboardEvent<HTMLInputElement>) => {
      if (isComposing) return

      if (e.key === 'Enter' || e.key === ',') {
        e.preventDefault()
        const newTag = inputValue.trim()
        if (newTag && !value.includes(newTag)) {
          onChange([...value, newTag])
        }
        setInputValue('')
        setOpen(false)
      } else if (e.key === 'Backspace' && !inputValue && value.length > 0) {
        onChange(value.slice(0, -1))
      } else if (e.key === 'Escape') {
        setOpen(false)
      }
    }

    const removeTag = (tagToRemove: string) => {
      onChange(value.filter((tag) => tag !== tagToRemove))
    }

    const handleSelectSuggestion = (suggestion: string) => {
      if (!value.includes(suggestion)) {
        onChange([...value, suggestion])
      }
      setInputValue('')
      setOpen(false)
      inputRef.current?.focus()
    }

    const handleInputBlur = () => {
      if (isComposing) return
      const newTag = inputValue.trim()
      if (newTag && !value.includes(newTag)) {
        onChange([...value, newTag])
      }
      setInputValue('')
      setOpen(false)
    }

    // Focus input when clicking on the container
    const handleContainerClick = () => {
      inputRef.current?.focus()
      if (suggestions.length > 0 && inputValue) {
        setOpen(true)
      }
    }

    // Close popover when clicking outside
    useEffect(() => {
      const handleClickOutside = (e: MouseEvent) => {
        const target = e.target as HTMLElement
        if (!target.closest('[data-tags-input-container]')) {
          setOpen(false)
        }
      }

      document.addEventListener('mousedown', handleClickOutside)
      return () => document.removeEventListener('mousedown', handleClickOutside)
    }, [])

    return (
      <div
        ref={containerRef}
        data-tags-input-container
        className={cn(
          'border-input bg-background ring-offset-background focus-within:ring-ring flex min-h-10 w-full flex-wrap gap-1 rounded-md border px-3 py-2 text-sm focus-within:ring-2 focus-within:ring-offset-2',
          className
        )}
        onClick={handleContainerClick}
      >
        {value.map((tag) => (
          <div key={tag} className='bg-secondary text-secondary-foreground flex items-center gap-1 rounded-sm px-2 py-0.5'>
            <span className='text-xs'>{tag}</span>
            <button
              type='button'
              onClick={() => removeTag(tag)}
              className='text-secondary-foreground/80 hover:text-secondary-foreground focus:outline-none'
              aria-label={`Remove ${tag} tag`}
            >
              <X className='h-3 w-3' />
            </button>
          </div>
        ))}
        <Popover open={open} onOpenChange={setOpen} modal={false}>
          <PopoverTrigger asChild>
            <input
              ref={inputRef}
              type='text'
              value={inputValue}
              onChange={handleInputChange}
              onKeyDown={handleKeyDown}
              onBlur={handleInputBlur}
              onFocus={() => {
                if (suggestions.length > 0 && inputValue) {
                  setOpen(true)
                }
              }}
              onCompositionStart={() => setIsComposing(true)}
              onCompositionEnd={() => setIsComposing(false)}
              placeholder={value.length === 0 ? placeholder : ''}
              className='placeholder:text-muted-foreground min-w-[80px] flex-1 bg-transparent outline-none'
            />
          </PopoverTrigger>
          {(isLoading || filteredSuggestions.length > 0 || inputValue.trim()) && (
            <PopoverContent
              className='w-[var(--radix-popover-trigger-width)] max-w-[var(--radix-popover-trigger-width)] p-0'
              align='start'
              onOpenAutoFocus={(e) => e.preventDefault()}
              container={containerRef.current ?? undefined}
            >
              <div className='max-h-[200px] overflow-y-auto p-1'>
                {isLoading ? (
                  <div className='p-2 text-sm text-muted-foreground'>Loading...</div>
                ) : filteredSuggestions.length > 0 ? (
                  filteredSuggestions.map((suggestion) => (
                    <div
                      key={suggestion}
                      className='hover:bg-accent flex cursor-pointer items-center gap-2 rounded-md px-2 py-1.5 text-sm'
                      onMouseDown={(e) => {
                        e.preventDefault()
                        handleSelectSuggestion(suggestion)
                      }}
                    >
                      <Check className='h-4 w-4 shrink-0 text-muted-foreground' />
                      <TruncatedText className='flex-1'>{suggestion}</TruncatedText>
                    </div>
                  ))
                ) : inputValue.trim() ? (
                  <div className='p-2 text-sm text-muted-foreground'>
                    Press Enter to add &quot;{inputValue}&quot;
                  </div>
                ) : null}
              </div>
            </PopoverContent>
          )}
        </Popover>
      </div>
    )
  }
)

TagsAutocompleteInput.displayName = 'TagsAutocompleteInput'
