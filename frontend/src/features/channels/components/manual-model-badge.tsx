import { useTranslation } from 'react-i18next';
import { Badge } from '@/components/ui/badge';
import { cn } from '@/lib/utils';

interface ManualModelBadgeProps {
  isManual?: boolean;
  className?: string;
}

/**
 * Badge component to indicate whether a model is manually added or auto-synced.
 *
 * - Manual models: User-added models that are preserved during sync
 * - Auto models: Models fetched from provider API
 */
export function ManualModelBadge({ isManual = false, className }: ManualModelBadgeProps) {
  const { t } = useTranslation();

  if (!isManual) {
    return null;
  }

  return (
    <Badge
      variant="secondary"
      className={cn(
        'bg-amber-100 text-amber-800 border-amber-200 dark:bg-amber-900/30 dark:text-amber-300 dark:border-amber-800',
        className
      )}
    >
      {t('channels.models.manual')}
    </Badge>
  );
}
