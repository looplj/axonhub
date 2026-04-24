import { createFileRoute } from '@tanstack/react-router';
import DashboardChannelSuccessRates from '@/features/dashboard/channel-success-rates';

export const Route = createFileRoute('/_authenticated/dashboard/channel-success-rates')({
  component: DashboardChannelSuccessRates,
});
