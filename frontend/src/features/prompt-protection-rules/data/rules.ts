import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';
import { graphqlRequest } from '@/gql/graphql';
import { useErrorHandler } from '@/hooks/use-error-handler';
import {
  CreatePromptProtectionRuleInput,
  PromptProtectionRule,
  PromptProtectionRuleConnection,
  UpdatePromptProtectionRuleInput,
  promptProtectionRuleConnectionSchema,
  promptProtectionRuleSchema,
} from './schema';

const RULES_QUERY = `
  query GetPromptProtectionRules(
    $first: Int
    $after: Cursor
    $last: Int
    $before: Cursor
    $where: PromptProtectionRuleWhereInput
    $orderBy: PromptProtectionRuleOrder
  ) {
    promptProtectionRules(first: $first, after: $after, last: $last, before: $before, where: $where, orderBy: $orderBy) {
      edges {
        node {
          id
          createdAt
          updatedAt
          name
          description
          pattern
          status
          settings {
            action
            replacement
            scopes
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

const CREATE_RULE_MUTATION = `
  mutation CreatePromptProtectionRule($input: CreatePromptProtectionRuleInput!) {
    createPromptProtectionRule(input: $input) {
      id
      createdAt
      updatedAt
      name
      description
      pattern
      status
      settings {
        action
        replacement
        scopes
      }
    }
  }
`;

const UPDATE_RULE_MUTATION = `
  mutation UpdatePromptProtectionRule($id: ID!, $input: UpdatePromptProtectionRuleInput!) {
    updatePromptProtectionRule(id: $id, input: $input) {
      id
      createdAt
      updatedAt
      name
      description
      pattern
      status
      settings {
        action
        replacement
        scopes
      }
    }
  }
`;

const DELETE_RULE_MUTATION = `
  mutation DeletePromptProtectionRule($id: ID!) {
    deletePromptProtectionRule(id: $id)
  }
`;

const UPDATE_RULE_STATUS_MUTATION = `
  mutation UpdatePromptProtectionRuleStatus($id: ID!, $status: PromptProtectionRuleStatus!) {
    updatePromptProtectionRuleStatus(id: $id, status: $status)
  }
`;

const BULK_DELETE_RULES_MUTATION = `
  mutation BulkDeletePromptProtectionRules($ids: [ID!]!) {
    bulkDeletePromptProtectionRules(ids: $ids)
  }
`;

const BULK_ENABLE_RULES_MUTATION = `
  mutation BulkEnablePromptProtectionRules($ids: [ID!]!) {
    bulkEnablePromptProtectionRules(ids: $ids)
  }
`;

const BULK_DISABLE_RULES_MUTATION = `
  mutation BulkDisablePromptProtectionRules($ids: [ID!]!) {
    bulkDisablePromptProtectionRules(ids: $ids)
  }
`;

interface QueryRulesArgs {
  first?: number;
  after?: string;
  last?: number;
  before?: string;
  where?: Record<string, any>;
  orderBy?: {
    field: 'CREATED_AT' | 'UPDATED_AT' | 'NAME';
    direction: 'ASC' | 'DESC';
  };
}

export function useQueryPromptProtectionRules(args: QueryRulesArgs) {
  return useQuery({
    queryKey: ['prompt-protection-rules', args],
    queryFn: async () => {
      const data = await graphqlRequest<{ promptProtectionRules: PromptProtectionRuleConnection }>(RULES_QUERY, args);
      return promptProtectionRuleConnectionSchema.parse(data.promptProtectionRules);
    },
  });
}

export function useCreatePromptProtectionRule() {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const { handleError } = useErrorHandler();

  return useMutation({
    mutationFn: async (input: CreatePromptProtectionRuleInput) => {
      try {
        const data = await graphqlRequest<{ createPromptProtectionRule: PromptProtectionRule }>(CREATE_RULE_MUTATION, { input });
        return promptProtectionRuleSchema.parse(data.createPromptProtectionRule);
      } catch (error) {
        handleError(error, { context: t('promptProtectionRules.dialogs.create.title') });
        throw error;
      }
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['prompt-protection-rules'] });
      toast.success(t('promptProtectionRules.messages.createSuccess'));
    },
  });
}

export function useUpdatePromptProtectionRule() {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const { handleError } = useErrorHandler();

  return useMutation({
    mutationFn: async ({ id, input }: { id: string; input: UpdatePromptProtectionRuleInput }) => {
      try {
        const data = await graphqlRequest<{ updatePromptProtectionRule: PromptProtectionRule }>(UPDATE_RULE_MUTATION, { id, input });
        return promptProtectionRuleSchema.parse(data.updatePromptProtectionRule);
      } catch (error) {
        handleError(error, { context: t('promptProtectionRules.dialogs.edit.title') });
        throw error;
      }
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['prompt-protection-rules'] });
      toast.success(t('promptProtectionRules.messages.updateSuccess'));
    },
  });
}

export function useDeletePromptProtectionRule() {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const { handleError } = useErrorHandler();

  return useMutation({
    mutationFn: async (id: string) => {
      try {
        await graphqlRequest(DELETE_RULE_MUTATION, { id });
      } catch (error) {
        handleError(error, { context: 'Delete Prompt Protection Rule' });
        throw error;
      }
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['prompt-protection-rules'] });
      toast.success(t('promptProtectionRules.messages.deleteSuccess'));
    },
  });
}

export function useUpdatePromptProtectionRuleStatus() {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const { handleError } = useErrorHandler();

  return useMutation({
    mutationFn: async ({ id, status }: { id: string; status: 'enabled' | 'disabled' }) => {
      try {
        await graphqlRequest(UPDATE_RULE_STATUS_MUTATION, { id, status });
      } catch (error) {
        handleError(error, { context: 'Update Prompt Protection Rule Status' });
        throw error;
      }
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['prompt-protection-rules'] });
      toast.success(t('promptProtectionRules.messages.statusUpdateSuccess'));
    },
  });
}

export function useBulkDeletePromptProtectionRules() {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const { handleError } = useErrorHandler();

  return useMutation({
    mutationFn: async (ids: string[]) => {
      try {
        await graphqlRequest(BULK_DELETE_RULES_MUTATION, { ids });
      } catch (error) {
        handleError(error, { context: 'Bulk Delete Prompt Protection Rules' });
        throw error;
      }
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['prompt-protection-rules'] });
      toast.success(t('promptProtectionRules.messages.bulkDeleteSuccess'));
    },
  });
}

export function useBulkEnablePromptProtectionRules() {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const { handleError } = useErrorHandler();

  return useMutation({
    mutationFn: async (ids: string[]) => {
      try {
        await graphqlRequest(BULK_ENABLE_RULES_MUTATION, { ids });
      } catch (error) {
        handleError(error, { context: 'Bulk Enable Prompt Protection Rules' });
        throw error;
      }
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['prompt-protection-rules'] });
      toast.success(t('promptProtectionRules.messages.bulkEnableSuccess'));
    },
  });
}

export function useBulkDisablePromptProtectionRules() {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const { handleError } = useErrorHandler();

  return useMutation({
    mutationFn: async (ids: string[]) => {
      try {
        await graphqlRequest(BULK_DISABLE_RULES_MUTATION, { ids });
      } catch (error) {
        handleError(error, { context: 'Bulk Disable Prompt Protection Rules' });
        throw error;
      }
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['prompt-protection-rules'] });
      toast.success(t('promptProtectionRules.messages.bulkDisableSuccess'));
    },
  });
}
