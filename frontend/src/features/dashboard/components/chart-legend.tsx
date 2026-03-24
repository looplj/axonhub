import { cn } from '@/lib/utils';
import { useIsMobile } from '@/hooks/use-mobile';

export interface ChartLegendItem {
  name: string;
  index?: number;
  color?: string;
  primaryValue: string;
  secondaryValue?: string;
}

export interface ChartLegendProps {
  items: ChartLegendItem[];
  columns?: 1 | 2;
  showIndex?: boolean;
}

export function ChartLegend({ items, columns, showIndex = true }: ChartLegendProps) {
  const isMobile = useIsMobile();
  const effectiveColumns = columns ?? (isMobile ? 1 : 2);
  const rows = effectiveColumns === 2 ? Math.ceil(items.length / 2) : items.length;

  return (
    <div
      className={cn('grid gap-x-4 gap-y-4')}
      style={{
        gridTemplateRows: effectiveColumns === 2 ? `repeat(${rows}, auto)` : undefined,
        gridAutoFlow: effectiveColumns === 2 ? 'column' : undefined,
      }}
    >
      {items.map((item, index) => (
        <div key={`${item.name}-${index}`} className='grid w-full grid-cols-[auto_auto_1fr_auto] items-start gap-3'>
          {showIndex && item.index !== undefined && (
            <span className='text-muted-foreground w-8 text-right text-sm font-semibold tabular-nums'>
              {item.index.toString().padStart(2, '0')}.
            </span>
          )}
          {item.color && (
            <span className='mt-1 h-2.5 w-2.5 rounded-full' style={{ backgroundColor: item.color }} />
          )}
          <span className='text-foreground min-w-0 text-sm font-medium break-words'>{item.name}</span>
          <div className='text-right leading-tight'>
            <div className='text-foreground text-sm font-medium tabular-nums'>{item.primaryValue}</div>
            {item.secondaryValue && (
              <div className='text-muted-foreground text-xs tabular-nums'>{item.secondaryValue}</div>
            )}
          </div>
        </div>
      ))}
    </div>
  );
}