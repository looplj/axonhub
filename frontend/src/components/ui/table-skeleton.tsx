import { Skeleton } from '@/components/ui/skeleton';
import { TableRow } from '@/components/ui/table';

interface TableSkeletonProps {
  rows?: number;
  columns?: number;
}

export function TableSkeleton({ rows = 5 }: TableSkeletonProps) {
  return (
    <>
      {Array.from({ length: rows }).map((_, rowIndex) => (
        <TableRow key={rowIndex} className='group/row rounded-xl border-0 !bg-[var(--table-background)]'>
          <td colSpan={100} className='border-0 bg-inherit px-4 py-3'>
            <Skeleton className='h-10 w-full rounded-lg' />
          </td>
        </TableRow>
      ))}
    </>
  );
}
