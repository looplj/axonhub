import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { graphqlRequest } from '@/gql/graphql';
import { useSelectedProjectId } from '@/stores/projectStore';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';
import { Prompt, PromptConnection, CreatePromptInput, UpdatePromptInput, promptConnectionSchema, promptSchema } from './schema';

const PROMPTS_QUERY = `
  query GetPrompts(
    $first: Int
    $after: Cursor
    $last: Int
    $before: Cursor
    $where: PromptWhereInput
    $orderBy: PromptOrder
  ) {
    prompts(first: $first, after: $after, last: $last, before: $before, where: $where, orderBy: $orderBy) {
      edges {
        node {
          id
          createdAt
          updatedAt
          projectID
          name
          description
          role
          content
          status
          order
          settings {
            action {
              type
            }
            conditions {
              conditions {
                type
                modelId
                modelPattern
              }
            }
          }
        }
        cursor
      }
      pageInfo {
        hasNextPage
        hasPreviousPage
        startCursor
        endCursor
      }
      totalCount
    }
  }
`;

const CREATE_PROMPT_MUTATION = `
  mutation CreatePrompt($input: CreatePromptInput!) {
    createPrompt(input: $input) {
      id
      createdAt
      updatedAt
      projectID
      name
      description
      role
      content
      status
      order
      settings {
        action {
          type
        }
        conditions {
          conditions {
            type
            modelId
            modelPattern
          }
        }
      }
    }
  }
`;

const UPDATE_PROMPT_MUTATION = `
  mutation UpdatePrompt($id: ID!, $input: UpdatePromptInput!) {
    updatePrompt(id: $id, input: $input) {
      id
      createdAt
      updatedAt
      projectID
      name
      description
      role
      content
      status
      order
      settings {
        action {
          type
        }
        conditions {
          conditions {
            type
            modelId
            modelPattern
          }
        }
      }
    }
  }
`;

const DELETE_PROMPT_MUTATION = `
  mutation DeletePrompt($id: ID!) {
    deletePrompt(id: $id)
  }
`;

const UPDATE_PROMPT_STATUS_MUTATION = `
  mutation UpdatePromptStatus($id: ID!, $status: PromptStatus!) {
    updatePromptStatus(id: $id, status: $status)
  }
`;

const BULK_DELETE_PROMPTS_MUTATION = `
  mutation BulkDeletePrompts($ids: [ID!]!) {
    bulkDeletePrompts(ids: $ids)
  }
`;

const BULK_DISABLE_PROMPTS_MUTATION = `
  mutation BulkDisablePrompts($ids: [ID!]!) {
    bulkDisablePrompts(ids: $ids)
  }
`;

const BULK_ENABLE_PROMPTS_MUTATION = `
  mutation BulkEnablePrompts($ids: [ID!]!) {
    bulkEnablePrompts(ids: $ids)
  }
`;

interface QueryPromptsArgs {
  first?: number;
  after?: string;
  last?: number;
  before?: string;
  where?: Record<string, any>;
  orderBy?: {
    field: 'CREATED_AT' | 'UPDATED_AT' | 'ORDER';
    direction: 'ASC' | 'DESC';
  };
}

export function useQueryPrompts(args: QueryPromptsArgs) {
  const selectedProjectId = useSelectedProjectId();

  return useQuery({
    queryKey: ['prompts', args, selectedProjectId],
    queryFn: async () => {
      const headers = selectedProjectId ? { 'X-Project-ID': selectedProjectId } : undefined;
      const data = await graphqlRequest<{ prompts: PromptConnection }>(PROMPTS_QUERY, args, headers);
      return promptConnectionSchema.parse(data.prompts);
    },
    enabled: !!selectedProjectId,
  });
}

export function useCreatePrompt() {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const selectedProjectId = useSelectedProjectId();

  return useMutation({
    mutationFn: async (input: CreatePromptInput) => {
      const headers = selectedProjectId ? { 'X-Project-ID': selectedProjectId } : undefined;
      const data = await graphqlRequest<{ createPrompt: Prompt }>(CREATE_PROMPT_MUTATION, { input }, headers);
      return promptSchema.parse(data.createPrompt);
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['prompts'] });
      toast.success(t('prompts.messages.createSuccess'));
    },
    onError: (error: Error) => {
      toast.error(t('prompts.messages.createError', { error: error.message }));
    },
  });
}

export function useUpdatePrompt() {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const selectedProjectId = useSelectedProjectId();

  return useMutation({
    mutationFn: async ({ id, input }: { id: string; input: UpdatePromptInput }) => {
      const headers = selectedProjectId ? { 'X-Project-ID': selectedProjectId } : undefined;
      const data = await graphqlRequest<{ updatePrompt: Prompt }>(UPDATE_PROMPT_MUTATION, { id, input }, headers);
      return promptSchema.parse(data.updatePrompt);
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['prompts'] });
      toast.success(t('prompts.messages.updateSuccess'));
    },
    onError: (error: Error) => {
      toast.error(t('prompts.messages.updateError', { error: error.message }));
    },
  });
}

export function useDeletePrompt() {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const selectedProjectId = useSelectedProjectId();

  return useMutation({
    mutationFn: async (id: string) => {
      const headers = selectedProjectId ? { 'X-Project-ID': selectedProjectId } : undefined;
      await graphqlRequest(DELETE_PROMPT_MUTATION, { id }, headers);
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['prompts'] });
      toast.success(t('prompts.messages.deleteSuccess'));
    },
    onError: (error: Error) => {
      toast.error(t('prompts.messages.deleteError', { error: error.message }));
    },
  });
}

export function useUpdatePromptStatus() {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const selectedProjectId = useSelectedProjectId();

  return useMutation({
    mutationFn: async ({ id, status }: { id: string; status: 'enabled' | 'disabled' }) => {
      const headers = selectedProjectId ? { 'X-Project-ID': selectedProjectId } : undefined;
      await graphqlRequest(UPDATE_PROMPT_STATUS_MUTATION, { id, status }, headers);
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['prompts'] });
      toast.success(t('prompts.messages.statusUpdateSuccess'));
    },
    onError: (error: Error) => {
      toast.error(t('prompts.messages.statusUpdateError', { error: error.message }));
    },
  });
}

export function useBulkDeletePrompts() {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const selectedProjectId = useSelectedProjectId();

  return useMutation({
    mutationFn: async (ids: string[]) => {
      const headers = selectedProjectId ? { 'X-Project-ID': selectedProjectId } : undefined;
      const data = await graphqlRequest<{ bulkDeletePrompts: boolean }>(BULK_DELETE_PROMPTS_MUTATION, { ids }, headers);
      return data.bulkDeletePrompts;
    },
    onSuccess: (_data, variables) => {
      queryClient.invalidateQueries({ queryKey: ['prompts'] });
      toast.success(t('prompts.messages.bulkDeleteSuccess', { count: variables.length }));
    },
    onError: (error: Error) => {
      toast.error(t('prompts.messages.bulkDeleteError', { error: error.message }));
    },
  });
}

export function useBulkDisablePrompts() {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const selectedProjectId = useSelectedProjectId();

  return useMutation({
    mutationFn: async (ids: string[]) => {
      const headers = selectedProjectId ? { 'X-Project-ID': selectedProjectId } : undefined;
      const data = await graphqlRequest<{ bulkDisablePrompts: boolean }>(BULK_DISABLE_PROMPTS_MUTATION, { ids }, headers);
      return data.bulkDisablePrompts;
    },
    onSuccess: (_data, variables) => {
      queryClient.invalidateQueries({ queryKey: ['prompts'] });
      toast.success(t('prompts.messages.bulkDisableSuccess', { count: variables.length }));
    },
    onError: (error: Error) => {
      toast.error(t('prompts.messages.bulkDisableError', { error: error.message }));
    },
  });
}

export function useBulkEnablePrompts() {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const selectedProjectId = useSelectedProjectId();

  return useMutation({
    mutationFn: async (ids: string[]) => {
      const headers = selectedProjectId ? { 'X-Project-ID': selectedProjectId } : undefined;
      const data = await graphqlRequest<{ bulkEnablePrompts: boolean }>(BULK_ENABLE_PROMPTS_MUTATION, { ids }, headers);
      return data.bulkEnablePrompts;
    },
    onSuccess: (_data, variables) => {
      queryClient.invalidateQueries({ queryKey: ['prompts'] });
      toast.success(t('prompts.messages.bulkEnableSuccess', { count: variables.length }));
    },
    onError: (error: Error) => {
      toast.error(t('prompts.messages.bulkEnableError', { error: error.message }));
    },
  });
}
