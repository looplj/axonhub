import { createFileRoute } from '@tanstack/react-router';
import { StatisticsPage } from '@/features/statistics';

export const Route = createFileRoute('/_authenticated/admin/statistics/')({
  component: StatisticsPage,
});
