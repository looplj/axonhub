import * as React from 'react'
import { cn } from '@/lib/utils'
import type { TimeValue } from '@/utils/date-range'

interface TimeDropdownProps {
  value: TimeValue
  onChange: (next: TimeValue) => void
  onClose?: () => void
  closeLabel?: string
  hours?: number[]
  minutes?: number[]
  seconds?: number[]
}

function buildNumberList(min: number, max: number) {
  return Array.from({ length: max - min + 1 }, (_, i) => min + i)
}

const DEFAULT_HOURS = buildNumberList(0, 23)
const DEFAULT_MINUTES = buildNumberList(0, 59)
const DEFAULT_SECONDS = buildNumberList(0, 59)

function pad2(value: number | string) {
  return String(value).padStart(2, '0')
}

export function TimeDropdown({
  value,
  onChange,
  onClose,
  closeLabel = 'Close',
  hours = DEFAULT_HOURS,
  minutes = DEFAULT_MINUTES,
  seconds = DEFAULT_SECONDS,
}: TimeDropdownProps) {
  return (
    <div
      className={cn(
        'absolute left-0 top-[calc(100%+8px)] z-50 flex h-[220px] w-full overflow-hidden rounded-md',
        'border border-gray-200 bg-white shadow-2xl dark:border-white/10 dark:bg-[#121214]'
      )}
      role='dialog'
    >
      <TimeCol label='HH' items={hours} active={value.hh} onPick={(hh) => onChange({ ...value, hh })} />
      <div className='no-scrollbar flex-1 overflow-y-auto border-x border-gray-100 p-1 text-center dark:border-white/5'>
        <TimeColInner label='MM' items={minutes} active={value.mm} onPick={(mm) => onChange({ ...value, mm })} />
      </div>
      <TimeCol label='SS' items={seconds} active={value.ss} onPick={(ss) => onChange({ ...value, ss })} />

      {onClose && (
        <button
          type='button'
          className='absolute -top-8 right-0 text-[11px] font-semibold uppercase tracking-widest text-gray-400 hover:text-gray-200'
          onClick={onClose}
        >
          {closeLabel}
        </button>
      )}
    </div>
  )
}

function TimeCol({
  label,
  items,
  active,
  onPick,
}: {
  label: string
  items: number[]
  active: string
  onPick: (val: string) => void
}) {
  return (
    <div className='no-scrollbar flex-1 overflow-y-auto p-1 text-center'>
      <TimeColInner label={label} items={items} active={active} onPick={onPick} />
    </div>
  )
}

function TimeColInner({
  label,
  items,
  active,
  onPick,
}: {
  label: string
  items: number[]
  active: string
  onPick: (val: string) => void
}) {
  return (
    <>
      <span className='sr-only'>{label}</span>
      {items.map((v) => {
        const txt = pad2(v)
        const isActive = txt === active

        return (
          <button
            key={txt}
            type='button'
            className={cn(
              'w-full rounded-md py-2 text-sm transition-colors',
              isActive
                ? 'glass-highlight border border-primary/20 font-semibold text-primary'
                : 'text-gray-400 hover:bg-gray-100 dark:text-gray-500 dark:hover:bg-white/5'
            )}
            onClick={() => onPick(txt)}
          >
            {txt}
          </button>
        )
      })}
    </>
  )
}
