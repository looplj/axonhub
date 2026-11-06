import type React from 'react';
import { cn } from '@/lib/utils';
import { Label } from '@/components/ui/label';
import { Switch } from '@/components/ui/switch';

// Building blocks for settings dialogs that show one section at a time with a
// left navigation (a horizontal strip on small screens). Give the dialog a fixed
// height (see SETTINGS_DIALOG_CONTENT_CLASS) so switching sections does not
// resize it; the section content scrolls instead.

export const SETTINGS_DIALOG_CONTENT_CLASS =
  'flex h-[min(680px,85vh)] max-h-[85vh] w-full max-w-full flex-col overflow-hidden sm:max-w-[760px]';

export interface SettingsDialogNavItem<K extends string> {
  key: K;
  title: string;
  icon: React.ElementType;
}

interface SettingsDialogBodyProps<K extends string> {
  items: SettingsDialogNavItem<K>[];
  active: K;
  onSelect: (key: K) => void;
  children: React.ReactNode;
}

export function SettingsDialogBody<K extends string>({ items, active, onSelect, children }: SettingsDialogBodyProps<K>) {
  return (
    <div className='flex min-h-0 flex-1 flex-col gap-4 sm:flex-row sm:gap-6'>
      <nav className='flex shrink-0 gap-1 overflow-x-auto sm:w-44 sm:flex-col'>
        {items.map((item) => {
          const isActive = item.key === active;
          return (
            <button
              key={item.key}
              type='button'
              aria-current={isActive ? 'page' : undefined}
              onClick={() => onSelect(item.key)}
              className={cn(
                'flex shrink-0 items-center gap-2 rounded-lg px-3 py-2 text-left text-sm whitespace-nowrap transition-colors sm:w-full',
                isActive ? 'bg-muted text-foreground font-medium' : 'text-muted-foreground hover:bg-accent/50 hover:text-foreground'
              )}
            >
              <item.icon className='h-4 w-4 shrink-0' />
              {item.title}
            </button>
          );
        })}
      </nav>

      <div className='min-h-0 min-w-0 flex-1 overflow-y-auto pr-1'>{children}</div>
    </div>
  );
}

interface SettingsSectionProps {
  title: string;
  description?: string;
  children: React.ReactNode;
}

// A titled group of rows separated by dividers.
export function SettingsSection({ title, description, children }: SettingsSectionProps) {
  return (
    <section className='space-y-3'>
      <div>
        <h3 className='text-base font-semibold'>{title}</h3>
        {description && <p className='text-muted-foreground text-xs'>{description}</p>}
      </div>
      <div className='border-border divide-border divide-y rounded-xl border'>{children}</div>
    </section>
  );
}

interface SettingsSwitchRowProps {
  id: string;
  label: string;
  description: React.ReactNode;
  checked: boolean;
  onCheckedChange: (checked: boolean) => void;
  disabled?: boolean;
  // Extra controls shown under the row, e.g. options that depend on the switch.
  children?: React.ReactNode;
}

export function SettingsSwitchRow({ id, label, description, checked, onCheckedChange, disabled, children }: SettingsSwitchRowProps) {
  return (
    <div className='space-y-3 p-4'>
      <div className='flex items-start justify-between gap-4'>
        <div className='space-y-1'>
          <Label htmlFor={id} className='text-sm font-medium'>
            {label}
          </Label>
          <div className='text-muted-foreground text-xs leading-relaxed'>{description}</div>
        </div>
        <Switch id={id} checked={checked} onCheckedChange={onCheckedChange} disabled={disabled} className='mt-0.5 shrink-0' />
      </div>
      {children}
    </div>
  );
}

// Nested options under a row, visually tied to it with a left rule.
export function SettingsSubOptions({ children }: { children: React.ReactNode }) {
  return <div className='border-border space-y-1.5 border-l-2 pl-3'>{children}</div>;
}
