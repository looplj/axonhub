import { useEffect } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useRouter } from '@tanstack/react-router';
import { pickFallbackNavUrl } from '@/config/nav-items';
import { graphqlRequest } from '@/gql/graphql';
import { ME_QUERY } from '@/gql/users';
import { toast } from 'sonner';
import { useAuthStore } from '@/stores/authStore';
import { AuthUser } from '@/stores/authStore';
import { useProjectStore } from '@/stores/projectStore';
import { getHiddenNavItems } from '@/stores/sidebarPrefsStore';
import { authApi } from '@/lib/api-client';
import i18n from '@/lib/i18n';
import { isProjectSelectionValid } from '@/lib/project-membership';

export interface SignInInput {
  email: string;
  password: string;
}

interface MeResponse {
  me: AuthUser;
}

export function useMe(enabled = true) {
  const { setUser } = useAuthStore((state) => state.auth);
  const sessionGeneration = useAuthStore((state) => state.auth.sessionGeneration);

  const query = useQuery({
    queryKey: ['me', sessionGeneration],
    queryFn: async () => {
      const data = await graphqlRequest<MeResponse>(ME_QUERY);
      return data.me;
    },
    enabled,
    retry: false,
  });

  // Update auth store when data changes
  useEffect(() => {
    if (query.data) {
      const userLanguage = query.data.preferLanguage || 'en';

      // The selected project is persisted per browser, but only valid for the
      // user it belonged to. Drop any project the current user is not a member
      // of so a stale selection from a previous account is never sent to the
      // server as X-Project-ID.
      const { selectedProjectId, clearSelectedProjectId } = useProjectStore.getState();
      if (!isProjectSelectionValid(query.data, selectedProjectId)) {
        clearSelectedProjectId();
      }

      setUser(query.data);

      // Initialize i18n with user's preferred language
      if (userLanguage !== i18n.language) {
        i18n.changeLanguage(userLanguage);
      }
    }
  }, [query.data, setUser]);

  return query;
}

export function useSignIn() {
  const { setUser, startSession } = useAuthStore((state) => state.auth);
  const router = useRouter();

  return useMutation({
    mutationFn: async (input: SignInInput) => {
      return await authApi.signIn(input);
    },
    onSuccess: (data) => {
      const userLanguage = data.user.preferLanguage || 'en';

      // Update auth store
      startSession();
      setUser(data.user);

      // Do not clear the persisted project here: the AuthGuard gates
      // project-scoped queries until the selected project is validated
      // against this user's memberships (useMe clears it when stale), so a
      // returning user keeps their last selection while another account's
      // stale selection can never leak into a request.
      // Initialize i18n with user's preferred language
      if (userLanguage !== i18n.language) {
        i18n.changeLanguage(userLanguage);
      }

      toast.success(i18n.t('common.success.signedIn'));

      // Redirect based on user role, skipping routes the user hid from the sidebar.
      // Owner users go to dashboard, non-owner users go to requests page.
      const baseRedirectPath = data.user.isOwner ? '/' : '/project/playground';
      const redirectPath = pickFallbackNavUrl(baseRedirectPath, getHiddenNavItems(), data.user.isOwner);
      router.navigate({ to: redirectPath });
    },
    onError: (error: any) => {
      const errorMessage = error.message || 'Failed to sign in';
      toast.error(errorMessage);
    },
  });
}

export function useSignOut() {
  const { reset } = useAuthStore((state) => state.auth);
  const router = useRouter();
  const queryClient = useQueryClient();

  return async () => {
    try {
      await authApi.signOut();
    } catch (error) {
      toast.error(error instanceof Error ? error.message : i18n.t('common.errors.internalServerError'));
      return;
    }
    reset();
    queryClient.clear();
    toast.success(i18n.t('common.success.signedOut'));
    router.navigate({ to: '/sign-in' });
  };
}

export function useOIDCProviders() {
  return useQuery({
    queryKey: ['oidc-providers'],
    queryFn: async () => {
      const response = await authApi.getOIDCProviders();
      return response.data || [];
    },
    staleTime: 5 * 60 * 1000, // 5 minutes
    retry: 1,
  });
}

export function useOIDCAuthorize() {
  return useMutation({
    mutationFn: async (providerId: string) => {
      return await authApi.getOIDCAuthorizeURL(providerId);
    },
    onSuccess: (response) => {
      if (response && response.data && response.data.url) {
        window.location.href = response.data.url;
      } else {
        toast.error('Invalid authorization URL received');
      }
    },
    onError: (error: unknown) => {
      const errorMessage = error instanceof Error ? error.message : 'Failed to initialize SSO login';
      toast.error(errorMessage);
    },
  });
}

export function useOIDCExchange() {
  const { setUser, startSession } = useAuthStore((state) => state.auth);
  const router = useRouter();

  return useMutation({
    mutationFn: async (code: string) => {
      return await authApi.exchangeOIDCCode(code);
    },
    onSuccess: (response) => {
      const data = response.data;

      const userLanguage = data.user.preferLanguage || 'en';

      // Update auth store
      startSession();
      setUser(data.user);

      // Do not clear the persisted project here: the AuthGuard gates
      // project-scoped queries until the selected project is validated
      // against this user's memberships (useMe clears it when stale), so a
      // returning user keeps their last selection while another account's
      // stale selection can never leak into a request.
      // Initialize i18n with user's preferred language
      if (userLanguage !== i18n.language) {
        i18n.changeLanguage(userLanguage);
      }

      toast.success(i18n.t('common.success.signedIn'));

      // Redirect based on user role
      const redirectPath = data.user.isOwner ? '/' : '/project/playground';
      router.navigate({ to: redirectPath });
    },
    onError: (error: unknown) => {
      const errorMessage = error instanceof Error ? error.message : 'SSO login failed';
      toast.error(errorMessage);
      router.navigate({ to: '/sign-in' });
    },
  });
}
