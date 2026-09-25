import { useEffect } from 'react';
import { useRouter } from '@tanstack/react-router';
import { isAuthError } from '@/gql/graphql';
import { useSelectedProjectId } from '@/stores/projectStore';
import { isProjectSelectionValid } from '@/lib/project-membership';
import { Skeleton } from '@/components/ui/skeleton';
import { useMe } from '@/features/auth/data/auth';

interface AuthGuardProps {
  children: React.ReactNode;
}

export function AuthGuard({ children }: AuthGuardProps) {
  const router = useRouter();
  const selectedProjectId = useSelectedProjectId();

  const { isLoading: isMeLoading, error: meError, data: meData } = useMe();

  // The selected project is persisted per browser. Until `me` has resolved and
  // the persisted project has been validated against the current user's
  // projects (and cleared when stale), keep showing the loading state so
  // descendant queries never carry a X-Project-ID the user has no membership in.
  // `meData` must be present: an unvalidated selection is never a reason to
  // render protected content, even when the `me` query itself errored.
  const projectReady = isProjectSelectionValid(meData, selectedProjectId);

  useEffect(() => {
    if (meError && isAuthError(meError)) {
      router.navigate({ to: '/sign-in' });
    }
  }, [meError, router]);

  // Show loading while checking auth
  if (isMeLoading || !meData || !projectReady) {
    return (
      <div className='flex h-screen items-center justify-center'>
        <div className='space-y-4'>
          <Skeleton className='h-8 w-48' />
          <Skeleton className='h-4 w-32' />
        </div>
      </div>
    );
  }

  return <>{children}</>;
}
