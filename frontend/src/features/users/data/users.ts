import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { graphqlRequest } from '@/gql/graphql';
import { USERS_QUERY, CREATE_USER_MUTATION, UPDATE_USER_MUTATION, UPDATE_USER_STATUS_MUTATION, DELETE_USER_MUTATION } from '@/gql/users';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';
import { useErrorHandler } from '@/hooks/use-error-handler';
import { User, UserConnection, CreateUserInput, UpdateUserInput, userConnectionSchema, userSchema } from './schema';

// Query hooks
export function useUsers(
  variables?: {
    first?: number;
    after?: string;
    orderBy?: { field: 'CREATED_AT'; direction: 'ASC' | 'DESC' };
    where?: Record<string, any>;
  },
  options?: {
    disableAutoFetch?: boolean;
  }
) {
  const { t } = useTranslation();
  const { handleError } = useErrorHandler();

  const queryVariables = {
    ...variables,
    orderBy: variables?.orderBy || { field: 'CREATED_AT', direction: 'DESC' },
  };

  return useQuery({
    queryKey: ['users', queryVariables],
    queryFn: async () => {
      try {
        const data = await graphqlRequest<{ users: UserConnection }>(USERS_QUERY, queryVariables);
        return userConnectionSchema.parse(data?.users);
      } catch (error) {
        handleError(error, t('common.errors.loadFailed'));
        throw error;
      }
    },
    enabled: !options?.disableAutoFetch,
  });
}

export function useUser(id: string) {
  const { t } = useTranslation();
  const { handleError } = useErrorHandler();

  return useQuery({
    queryKey: ['user', id],
    queryFn: async () => {
      try {
        const data = await graphqlRequest<{ users: UserConnection }>(USERS_QUERY, { where: { id } });
        const user = data.users.edges[0]?.node;
        if (!user) {
          throw new Error(t('users.messages.userNotFound'));
        }
        return userSchema.parse(user);
      } catch (error) {
        handleError(error, t('common.errors.loadFailed'));
        throw error;
      }
    },
    enabled: !!id,
  });
}

// Mutation hooks
export function useCreateUser() {
  const { t } = useTranslation();
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (input: CreateUserInput) => {
      const data = await graphqlRequest<{ createUser: User }>(CREATE_USER_MUTATION, { input });
      return userSchema.parse(data.createUser);
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['users'] });
      toast.success(t('users.messages.createSuccess'));
    },
    onError: () => {
      toast.error(t('common.errors.internalServerError'));
    },
  });
}

export function useUpdateUser() {
  const { t } = useTranslation();
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async ({ id, input }: { id: string; input: UpdateUserInput }) => {
      const data = await graphqlRequest<{ updateUser: User }>(UPDATE_USER_MUTATION, { id, input });
      return userSchema.parse(data.updateUser);
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['users'] });
      toast.success(t('users.messages.updateSuccess'));
    },
    onError: () => {
      toast.error(t('common.errors.internalServerError'));
    },
  });
}

export function useUpdateUserStatus() {
  const { t } = useTranslation();
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async ({ id, status }: { id: string; status: 'activated' | 'deactivated' }) => {
      const response = await graphqlRequest<{ updateUserStatus: boolean }>(UPDATE_USER_STATUS_MUTATION, { id, status });
      return response.updateUserStatus;
    },
    onSuccess: (_data, variables) => {
      queryClient.invalidateQueries({ queryKey: ['users'] });
      queryClient.invalidateQueries({ queryKey: ['user', variables.id] });
      const statusText = variables.status === 'activated' ? t('users.status.activated') : t('users.status.deactivated');
      toast.success(t('users.messages.statusUpdateSuccess', { status: statusText }));
    },
    onError: () => {
      toast.error(t('common.errors.internalServerError'));
    },
  });
}

export function useDeleteUser() {
  const { t } = useTranslation();
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (id: string) => {
      const data = await graphqlRequest<{ deleteUser: boolean }>(DELETE_USER_MUTATION, { id });
      return data.deleteUser;
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['users'] });
      toast.success(t('users.messages.deleteSuccess'));
    },
    onError: (error: any) => {
      const message = error?.response?.errors?.[0]?.message || t('common.errors.internalServerError');
      toast.error(message);
    },
  });
}

// Export users for compatibility
export const users = {
  useUsers,
  useUser,
  useCreateUser,
  useUpdateUser,
  useDeleteUser,
};
