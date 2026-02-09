import { LanguageSwitch } from '@/components/language-switch';
import { ThemeSwitch } from '@/components/theme-switch';

export function MobileHeaderControls() {
  return (
    <div
      role='toolbar'
      aria-label='Settings controls'
      className='flex flex-row items-center justify-end gap-2'
      data-testid='mobile-header-controls'
    >
      <div data-testid='mobile-language-switch'>
        <LanguageSwitch />
      </div>

      <div data-testid='mobile-theme-switch'>
        <ThemeSwitch />
      </div>
    </div>
  );
}
