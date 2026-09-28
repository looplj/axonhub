import { createFileRoute, redirect } from '@tanstack/react-router';
import SignIn from '@/features/auth/sign-in';
import { getTokenFromStorage } from '@/stores/authStore';

export const Route = createFileRoute('/(auth)/sign-in')({
  beforeLoad: () => {
    if (getTokenFromStorage()) {
      throw redirect({ to: '/' });
    }
  },
  component: SignIn,
});
