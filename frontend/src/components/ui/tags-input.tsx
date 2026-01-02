'use client';

import { forwardRef, InputHTMLAttributes, useCallback, useRef, useState } from 'react';
import { X } from 'lucide-react';
import { cn } from '@/lib/utils';

export interface TagsInputProps extends Omit<InputHTMLAttributes<HTMLInputElement>, 'value' | 'onChange'> {
  value: string[];
  onChange: (tags: string[]) => void;
  placeholder?: string;
  className?: string;
}

export const TagsInput = forwardRef<HTMLDivElement, TagsInputProps>(({ value = [], onChange, placeholder, className, ...props }, ref) => {
  const [inputValue, setInputValue] = useState('');
  const inputRef = useRef<HTMLInputElement>(null);
  const isComposingRef = useRef(false);

  const handleInputChange = useCallback((e: React.ChangeEvent<HTMLInputElement>) => {
    setInputValue(e.target.value);
  }, []);

  const handleCompositionStart = useCallback(() => {
    isComposingRef.current = true;
  }, []);

  const handleCompositionEnd = useCallback(() => {
    isComposingRef.current = false;
  }, []);

  const handleInputKeyDown = useCallback(
    (e: React.KeyboardEvent<HTMLInputElement>) => {
      const nativeEvent = e.nativeEvent as unknown as { isComposing?: boolean; keyCode?: number };
      if (isComposingRef.current || nativeEvent.isComposing || nativeEvent.keyCode === 229) {
        return;
      }

      if (e.key === 'Enter' || e.key === ',' || e.key === ' ') {
        e.preventDefault();
        const newTag = inputValue.trim();
        if (newTag && !value.includes(newTag)) {
          onChange([...value, newTag]);
          setInputValue('');
        }
      } else if (e.key === 'Backspace' && !inputValue && value.length > 0) {
        // Remove the last tag when backspace is pressed on empty input
        onChange(value.slice(0, -1));
      }
    },
    [inputValue, value, onChange]
  );

  const removeTag = useCallback(
    (tagToRemove: string) => {
      // 标签在组件内已去重，按值删除更稳定（避免索引变更导致误删）
      onChange(value.filter((tag) => tag !== tagToRemove));
    },
    [value, onChange]
  );

  const handleInputBlur = useCallback(() => {
    if (isComposingRef.current) return;

    const newTag = inputValue.trim();
    if (newTag && !value.includes(newTag)) {
      onChange([...value, newTag]);
    }
    setInputValue('');
  }, [inputValue, value, onChange]);

  return (
    <div
      ref={ref}
      className={cn(
        'border-input bg-background ring-offset-background focus-within:ring-ring flex min-h-10 w-full flex-wrap gap-1 rounded-md border px-3 py-2 text-sm focus-within:ring-2 focus-within:ring-offset-2',
        className
      )}
      onClick={() => inputRef.current?.focus()}
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
      <input
        ref={inputRef}
        type='text'
        value={inputValue}
        onChange={handleInputChange}
        onKeyDown={handleInputKeyDown}
        onCompositionStart={handleCompositionStart}
        onCompositionEnd={handleCompositionEnd}
        onBlur={handleInputBlur}
        placeholder={value.length === 0 ? placeholder : ''}
        className='placeholder:text-muted-foreground min-w-[80px] flex-1 bg-transparent outline-none'
        {...props}
      />
    </div>
  );
});

TagsInput.displayName = 'TagsInput';
