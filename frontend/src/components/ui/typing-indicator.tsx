import { Dot } from 'lucide-react';

export function TypingIndicator() {
  return (
    <div className='justify-left flex space-x-1'>
      <div className='bg-muted rounded-lg p-3'>
        <div className='flex -space-x-2.5'>
          <Dot className='animate-typing-dot-bounce h-5 w-5' />
          <Dot className='animate-typing-dot-bounce h-5 w-5 [animation-delay:90ms]' />
          <Dot className='animate-typing-dot-bounce h-5 w-5 [animation-delay:180ms]' />
        </div>
      </div>
    </div>
  );
}
