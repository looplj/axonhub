import { createFileRoute } from '@tanstack/react-router';
import ModelsManagement from '@/features/models';

export const Route = createFileRoute('/_authenticated/models/')({
  component: ModelsManagement,
});
