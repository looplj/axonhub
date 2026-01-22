import type { RefObject } from 'react'
import { Clock } from 'lucide-react'
import { cn } from '@/lib/utils'
import type { TimeValue } from '@/utils/date-range'
import { TimeDropdown } from './time-dropdown'
import { timeToString } from './utils'

interface TimeFieldProps {
  label: string
  value: TimeValue
  active: boolean
  open: boolean
  onToggle: () => void
  onChange: (next: TimeValue) => void
  onClose: () => void
  closeLabel?: string
  wrapperRef?: RefObject<HTMLDivElement | null>
}

export function TimeField({
  label,
  value,
  active,
  open,
  onToggle,
  onChange,
  onClose,
  closeLabel,
  wrapperRef,
}: TimeFieldProps) {
  const inputClass = cn(
    'w-full cursor-pointer border-none bg-transparent p-0 text-sm transition-colors focus:ring-0',
    active ? 'font-semibold text-gray-900 dark:text-gray-100' : 'font-medium text-gray-500 dark:text-gray-600'
  )

  const iconClass = cn(
    'ml-2 h-5 w-5 transition-colors',
    active ? 'text-gray-600 dark:text-gray-300' : 'text-gray-400 dark:text-gray-600'
  )

  return (
    <div className='flex-1 space-y-3'>
      <label className='block text-[11px] font-bold uppercase tracking-[0.2em] text-gray-500'>{label}</label>

      <div className='relative' ref={wrapperRef}>
        <button
          type='button'
          className={cn(
            'flex w-full items-center rounded-md border border-gray-200 bg-white px-4 py-3.5 transition-all hover:border-white/20',
            'dark:border-white/10 dark:bg-[#121214]/60',
            open && 'active-glow'
          )}
          onClick={onToggle}
        >
          <input readOnly className={inputClass} value={timeToString(value)} placeholder='HH:mm:ss' />
          <Clock className={iconClass} />
        </button>

        {open && <TimeDropdown value={value} onChange={onChange} onClose={onClose} closeLabel={closeLabel} />}
      </div>
    </div>
  )
}
