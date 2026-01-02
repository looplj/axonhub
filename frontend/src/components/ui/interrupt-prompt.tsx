'use client';

import { AnimatePresence, motion } from 'framer-motion';
import { X } from 'lucide-react';

interface InterruptPromptProps {
  isOpen: boolean;
  close: () => void;
}

export function InterruptPrompt({ isOpen, close }: InterruptPromptProps) {
  return (
    <AnimatePresence>
      {isOpen && (
        <motion.div
          initial={{ top: 0, filter: 'blur(5px)' }}
          animate={{
            top: -40,
            filter: 'blur(0px)',
            transition: {
              type: 'spring',
              filter: { type: 'tween' },
            },
          }}
          exit={{ top: 0, filter: 'blur(5px)' }}
          className='bg-background text-muted-foreground absolute left-1/2 flex -translate-x-1/2 overflow-hidden rounded-full border py-1 text-center text-sm whitespace-nowrap'
        >
          <span className='ml-2.5'>Press Enter again to interrupt</span>
          <button className='mr-2.5 ml-1 flex items-center' type='button' onClick={close} aria-label='Close'>
            <X className='h-3 w-3' />
          </button>
        </motion.div>
      )}
    </AnimatePresence>
  );
}
