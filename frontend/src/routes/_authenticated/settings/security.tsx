import { createFileRoute } from '@tanstack/react-router';
import SettingsSecurity from '@/features/settings/security';

export const Route = createFileRoute('/_authenticated/settings/security')({
  component: SettingsSecurity,
});
