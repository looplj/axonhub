'use client';

import * as React from 'react';
import * as DialogPrimitive from '@radix-ui/react-dialog';
import { cva, type VariantProps } from 'class-variance-authority';
import { XIcon } from 'lucide-react';
import { cn } from '@/lib/utils';

function Dialog({ ...props }: React.ComponentProps<typeof DialogPrimitive.Root>) {
  return <DialogPrimitive.Root data-slot='dialog' {...props} />;
}

function DialogTrigger({ ...props }: React.ComponentProps<typeof DialogPrimitive.Trigger>) {
  return <DialogPrimitive.Trigger data-slot='dialog-trigger' {...props} />;
}

function DialogPortal({ ...props }: React.ComponentProps<typeof DialogPrimitive.Portal>) {
  return <DialogPrimitive.Portal data-slot='dialog-portal' {...props} />;
}

function DialogClose({ ...props }: React.ComponentProps<typeof DialogPrimitive.Close>) {
  return <DialogPrimitive.Close data-slot='dialog-close' {...props} />;
}

function DialogOverlay({ className, ...props }: React.ComponentProps<typeof DialogPrimitive.Overlay>) {
  return (
    <DialogPrimitive.Overlay
      data-slot='dialog-overlay'
      className={cn(
        'data-[state=open]:animate-dialog-in data-[state=closed]:animate-dialog-out fixed inset-0 z-50 flex items-center justify-center bg-black/50',
        className
      )}
      {...props}
    />
  );
}

const DialogContent = React.forwardRef<
  React.ElementRef<typeof DialogPrimitive.Content>,
  React.ComponentProps<typeof DialogPrimitive.Content> & {
    showCloseButton?: boolean;
  }
>(({ className, children, showCloseButton = true, ...props }, ref) => {
  return (
    <DialogPortal data-slot='dialog-portal'>
      <DialogOverlay>
        <DialogPrimitive.Content
          ref={ref}
          data-slot='dialog-content'
          className={cn(
            'bg-background data-[state=open]:animate-dialog-in data-[state=closed]:animate-dialog-out relative z-50 grid max-h-[calc(100dvh-2rem)] w-full max-w-[calc(100%-2rem)] gap-4 overflow-y-auto rounded-lg border p-6 shadow-lg duration-200 sm:max-w-lg',
            className
          )}
          onInteractOutside={(e) => {
            e.preventDefault();
          }}
          {...props}
        >
          {children}
          {showCloseButton && (
            <DialogPrimitive.Close
              data-slot='dialog-close'
              className="ring-offset-background focus:ring-ring data-[state=open]:bg-accent data-[state=open]:text-muted-foreground absolute top-4 right-4 rounded-xs opacity-70 transition-opacity hover:opacity-100 focus:ring-2 focus:ring-offset-2 focus:outline-hidden disabled:pointer-events-none [&_svg]:pointer-events-none [&_svg]:shrink-0 [&_svg:not([class*='size-'])]:size-4"
            >
              <XIcon />
              <span className='sr-only'>Close</span>
            </DialogPrimitive.Close>
          )}
        </DialogPrimitive.Content>
      </DialogOverlay>
    </DialogPortal>
  );
});

function DialogHeader({ className, ...props }: React.ComponentProps<'div'>) {
  return <div data-slot='dialog-header' className={cn('flex flex-col gap-2 text-center sm:text-left', className)} {...props} />;
}

// Footers stack full width below `sm` (primary action first) and become a row
// above it. Call sites pick spacing and chrome from these variants instead of
// re-declaring flex direction, alignment, and gaps.
// Buttons grow to a 44px touch target on phones, where they are the primary way
// out of the dialog, and return to the normal control height on wider screens.
const dialogFooterVariants = cva('flex flex-col-reverse gap-2 sm:flex-row sm:justify-end [&_button]:min-h-11 sm:[&_button]:min-h-0', {
  variants: {
    // `inline` keeps a footer on one line at every width. Only for footers with
    // content on both ends (e.g. Reset on the left, Done on the right).
    // `content` keeps source order below `sm` instead of reversing it, for
    // footers whose first child is explanatory text rather than a secondary
    // action — reversing those would push the message below the buttons.
    layout: {
      stacked: '',
      inline: 'flex-row',
      content: 'flex-col',
    },
    align: {
      end: '',
      between: 'sm:justify-between',
    },
    gap: {
      default: '',
      // Tightening below sm keeps phones compact; `xl` holds the gap at every width.
      lg: 'gap-3 sm:gap-2',
      xl: 'gap-3',
    },
    divided: {
      true: 'border-t pt-4',
    },
    pinned: {
      true: 'shrink-0',
    },
  },
  compoundVariants: [
    // An inline footer split across both ends needs the spacing at every width;
    // `sm:justify-between` alone would leave the two ends packed together below sm.
    { layout: 'inline', align: 'between', class: 'justify-between' },
  ],
  defaultVariants: {
    align: 'end',
    gap: 'default',
  },
});

function DialogFooter({
  className,
  layout,
  align,
  gap,
  divided,
  pinned,
  ...props
}: React.ComponentProps<'div'> & VariantProps<typeof dialogFooterVariants>) {
  return (
    <div data-slot='dialog-footer' className={cn(dialogFooterVariants({ layout, align, gap, divided, pinned }), className)} {...props} />
  );
}

function DialogTitle({ className, ...props }: React.ComponentProps<typeof DialogPrimitive.Title>) {
  return <DialogPrimitive.Title data-slot='dialog-title' className={cn('text-lg leading-none font-semibold', className)} {...props} />;
}

function DialogDescription({ className, ...props }: React.ComponentProps<typeof DialogPrimitive.Description>) {
  return (
    <DialogPrimitive.Description data-slot='dialog-description' className={cn('text-muted-foreground text-sm', className)} {...props} />
  );
}

export {
  dialogFooterVariants,
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogOverlay,
  DialogPortal,
  DialogTitle,
  DialogTrigger,
};
