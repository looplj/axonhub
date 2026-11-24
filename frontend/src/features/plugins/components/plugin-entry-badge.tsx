// 职责: 在宿主导航和顶部动作上显示插件设置的计数或状态点。
// 关联文件:
//   frontend/src/features/plugins/data/plugin-ui.ts
//   frontend/src/features/plugins/components/plugin-header-actions.tsx
//   frontend/src/components/layout/nav-group.tsx
//   frontend/src/locales/en/plugins.json
//   frontend/src/locales/zh-CN/plugins.json
import { useTranslation } from 'react-i18next';
import { cn } from '@/lib/utils';
import type { PluginUIBadge } from '../data/plugin-ui';

const tones = {
  accent: 'bg-primary text-primary-foreground',
  warning: 'bg-amber-500 text-black',
  danger: 'bg-destructive text-white',
};

export function PluginEntryBadge({ badge, className }: { badge?: PluginUIBadge | null; className?: string }) {
  const { t } = useTranslation();
  if (!badge) return null;
  const count = badge.count ?? 0;

  return (
    <span
      role='img'
      aria-label={count > 0 ? t('plugins.badge.count', { count }) : t('plugins.badge.dot')}
      className={cn(
        'inline-flex shrink-0 items-center justify-center rounded-full',
        count > 0 ? 'h-4 min-w-4 px-1 text-[10px] leading-none tabular-nums' : 'size-2',
        tones[badge.tone],
        className
      )}
    >
      {count > 99 ? '99+' : count > 0 ? count : null}
    </span>
  );
}
