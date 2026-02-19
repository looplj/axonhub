import * as React from 'react';
import * as ToggleGroupPrimitive from '@radix-ui/react-toggle-group';
import { type VariantProps, tv } from 'tailwind-variants';
import { cn } from '@/lib/utils';

const toggleGroupVariants = tv({
  base: 'inline-flex items-center justify-center gap-1 rounded-md bg-muted p-1 text-muted-foreground',
  variants: {
    variant: {
      default: 'bg-muted hover:bg-muted hover:text-foreground',
      outline: 'border border-input bg-transparent shadow-sm hover:bg-muted hover:text-foreground',
    },
    size: {
      default: 'h-9 px-2 min-[0]:place-self-center',
      sm: 'h-8 px-1.5',
      lg: 'h-10 px-2.5',
    },
  },
  defaultVariants: {
    variant: 'default',
    size: 'default',
  },
});

const toggleGroupItemVariants = tv({
  base: cn(
    'inline-flex items-center justify-center whitespace-nowrap rounded-md px-3 py-1.5 text-sm font-medium transition-all focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:pointer-events-none disabled:opacity-50',
    'hover:bg-primary hover:text-primary-foreground focus-visible:bg-primary focus-visible:text-primary-foreground',
    'data-[state=on]:bg-primary data-[state=on]:text-primary-foreground',
  ),
  variants: {
    variant: {
      default: '',
      outline: 'hover:bg-muted hover:text-foreground focus-visible:bg-muted focus-visible:text-foreground',
    },
    size: {
      default: '',
      sm: 'h-7 px-2 text-xs',
      lg: 'h-10 px-4 text-base',
    },
  },
  defaultVariants: {
    variant: 'default',
    size: 'default',
  },
});

interface ToggleGroupProps
  extends React.ComponentProps<typeof ToggleGroupPrimitive.Root>,
    VariantProps<typeof toggleGroupVariants> {
  type?: 'single' | 'multiple';
}

function ToggleGroup({ className, variant, size, children, ...props }: ToggleGroupProps) {
  return (
    <ToggleGroupPrimitive.Root
      data-slot="toggle-group"
      className={cn(toggleGroupVariants({ variant, size, className }))}
      {...props}
    >
      {children}
    </ToggleGroupPrimitive.Root>
  );
}

interface ToggleGroupItemProps
  extends React.ComponentProps<typeof ToggleGroupPrimitive.Item>,
    VariantProps<typeof toggleGroupItemVariants> {}

function ToggleGroupItem({ className, variant, size, children, ...props }: ToggleGroupItemProps) {
  return (
    <ToggleGroupPrimitive.Item
      data-slot="toggle-group-item"
      className={cn(toggleGroupItemVariants({ variant, size, className }))}
      {...props}
    >
      {children}
    </ToggleGroupPrimitive.Item>
  );
}

export { ToggleGroup, ToggleGroupItem, toggleGroupVariants, toggleGroupItemVariants };
