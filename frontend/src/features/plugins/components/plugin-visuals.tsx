'use client';

import type { ReactNode } from 'react';
import type { TFunction } from 'i18next';
import { useTranslation } from 'react-i18next';
import { Skeleton } from '@/components/ui/skeleton';
import { cn } from '@/lib/utils';

const AVATAR_TONES = [
  'bg-orange-500/15 text-orange-700 dark:text-orange-300',
  'bg-sky-500/15 text-sky-700 dark:text-sky-300',
  'bg-emerald-500/15 text-emerald-700 dark:text-emerald-300',
  'bg-violet-500/15 text-violet-700 dark:text-violet-300',
  'bg-rose-500/15 text-rose-700 dark:text-rose-300',
  'bg-amber-500/15 text-amber-700 dark:text-amber-300',
];

/** Letter tile with a tone derived from the plugin ID, so a plugin keeps its color across pages. */
export function PluginAvatar({ id, name, className }: { id: string; name: string; className?: string }) {
  let hash = 0;
  for (const char of id) {
    hash = (hash * 31 + char.charCodeAt(0)) >>> 0;
  }
  const letter = (name || id).trim().charAt(0).toUpperCase() || '?';

  return (
    <div
      aria-hidden
      className={cn(
        'flex size-10 shrink-0 items-center justify-center rounded-xl text-base font-semibold',
        AVATAR_TONES[hash % AVATAR_TONES.length],
        className
      )}
    >
      {letter}
    </div>
  );
}

export function runtimeStatusLabel(t: TFunction, status: string) {
  return status === 'active' || status === 'disabled' || status === 'error' ? t(`plugins.status.${status}`) : status;
}

export function PluginStatus({ status, className }: { status: string; className?: string }) {
  const { t } = useTranslation();

  return (
    <span className={cn('inline-flex items-center gap-1.5 text-xs font-medium', className)}>
      <span
        className={cn(
          'size-2 rounded-full',
          status === 'active' && 'bg-emerald-500 shadow-[0_0_0_3px] shadow-emerald-500/20',
          status === 'error' && 'bg-destructive shadow-destructive/20 shadow-[0_0_0_3px]',
          status !== 'active' && status !== 'error' && 'bg-muted-foreground/40'
        )}
      />
      <span className={cn(status === 'error' ? 'text-destructive' : status === 'active' ? 'text-foreground' : 'text-muted-foreground')}>
        {runtimeStatusLabel(t, status)}
      </span>
    </span>
  );
}

export function PluginGrid({ children }: { children: ReactNode }) {
  return <div className='grid gap-4 sm:grid-cols-2 xl:grid-cols-3'>{children}</div>;
}

export function PluginGridSkeleton({ count = 3 }: { count?: number }) {
  return (
    <PluginGrid>
      {Array.from({ length: count }).map((_, index) => (
        <Skeleton key={index} className='h-44 rounded-2xl' />
      ))}
    </PluginGrid>
  );
}

export function PluginEmptyState({
  icon,
  title,
  description,
  action,
}: {
  icon: ReactNode;
  title: string;
  description?: string;
  action?: ReactNode;
}) {
  return (
    <div className='flex flex-col items-center justify-center gap-3 rounded-2xl border border-dashed px-6 py-16 text-center'>
      <div className='bg-muted text-muted-foreground flex size-12 items-center justify-center rounded-2xl [&>svg]:size-6'>{icon}</div>
      <div className='space-y-1'>
        <p className='font-medium'>{title}</p>
        {description && <p className='text-muted-foreground text-sm'>{description}</p>}
      </div>
      {action}
    </div>
  );
}

export function matchesPluginSearch(search: string, ...fields: (string | null | undefined)[]) {
  const query = search.trim().toLowerCase();
  if (!query) {
    return true;
  }
  return fields.some((field) => field?.toLowerCase().includes(query));
}
