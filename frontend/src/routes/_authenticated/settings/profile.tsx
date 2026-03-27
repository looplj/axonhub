import { createFileRoute } from '@tanstack/react-router';
import SettingsProfile from '@/features/settings/profile';

export const Route = createFileRoute('/_authenticated/settings/profile')({
  component: SettingsProfile,
  validateSearch: (search: Record<string, unknown>) => {
    return {
      oidc_link: search.oidc_link as string | undefined,
    };
  },
});
