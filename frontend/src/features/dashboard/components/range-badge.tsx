import { useTranslation } from 'react-i18next';
import { Badge } from '@/components/ui/badge';
import { coarseWindowLabelKey, type CoarseTimeWindow } from '../utils/time-window';

interface RangeBadgeProps {
  label?: string;
  timeWindow?: CoarseTimeWindow;
  className?: string;
}

/** Card-header badge stating which window a card's numbers cover, so no card on the
 * page is ambiguous about its range. */
export function RangeBadge({ label, timeWindow, className }: RangeBadgeProps) {
  const { t } = useTranslation();
  const text = label ?? (timeWindow ? t(coarseWindowLabelKey(timeWindow)) : null);

  if (!text) return null;

  return (
    <Badge variant='outline' className={`text-muted-foreground font-normal ${className ?? ''}`}>
      {text}
    </Badge>
  );
}
