import { DateRange } from 'react-day-picker';

export function buildDateRangeWhereClause(dateRange: DateRange | undefined) {
  const where: { createdAtGTE?: string; createdAtLTE?: string } = {};

  if (dateRange?.from) {
    where.createdAtGTE = dateRange.from.toISOString();
  }
  if (dateRange?.to) {
    const endDate = new Date(dateRange.to);
    endDate.setHours(23, 59, 59, 999);
    where.createdAtLTE = endDate.toISOString();
  }

  return where;
}
