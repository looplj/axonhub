import { z } from 'zod';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { graphqlRequest } from '@/gql/graphql';
import { pageInfoSchema } from '@/gql/pagination';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';
import { useErrorHandler } from '@/hooks/use-error-handler';
import { overrideOperationSchema } from './schema';

// Zod Schemas for Template Types
export const channelOverrideTemplateSchema = z.object({
  id: z.string(),
  createdAt: z.coerce.date(),
  updatedAt: z.coerce.date(),
  userID: z.string(),
  name: z.string(),
  description: z.string().optional().nullable(),
  headerOverrideOperations: z.array(overrideOperationSchema),
  bodyOverrideOperations: z.array(overrideOperationSchema),
});
export type ChannelOverrideTemplate = z.infer<typeof channelOverrideTemplateSchema>;

export const channelOverrideTemplateConnectionSchema = z.object({
  edges: z.array(
    z.object({
      node: channelOverrideTemplateSchema,
      cursor: z.string(),
    })
  ),
  pageInfo: pageInfoSchema,
  totalCount: z.number(),
});
export type ChannelOverrideTemplateConnection = z.infer<typeof channelOverrideTemplateConnectionSchema>;

export const createChannelOverrideTemplateInputSchema = z.object({
  name: z.string().min(1, 'Template name is required'),
  description: z.string().optional(),
  headerOverrideOperations: z.array(overrideOperationSchema).optional(),
  bodyOverrideOperations: z.array(overrideOperationSchema).optional(),
});
export type CreateChannelOverrideTemplateInput = z.infer<typeof createChannelOverrideTemplateInputSchema>;

export const updateChannelOverrideTemplateInputSchema = z.object({
  name: z.string().min(1, 'Template name is required').optional(),
  description: z.string().optional(),
  clearDescription: z.boolean().optional(),
  headerOverrideOperations: z.array(overrideOperationSchema).optional(),
  bodyOverrideOperations: z.array(overrideOperationSchema).optional(),
});
export type UpdateChannelOverrideTemplateInput = z.infer<typeof updateChannelOverrideTemplateInputSchema>;

export const applyChannelOverrideTemplateInputSchema = z.object({
  templateID: z.string(),
  channelIDs: z.array(z.string()).min(1, 'At least one channel is required'),
  mode: z.enum(['MERGE']).optional(),
});
export type ApplyChannelOverrideTemplateInput = z.infer<typeof applyChannelOverrideTemplateInputSchema>;

export const applyChannelOverrideTemplatePayloadSchema = z.object({
  success: z.boolean(),
  updated: z.number(),
  channels: z.array(z.any()), // Channel schema is complex, just mark as any here
});
export type ApplyChannelOverrideTemplatePayload = z.infer<typeof applyChannelOverrideTemplatePayloadSchema>;

// GraphQL Fragments
const TEMPLATE_FRAGMENT = `
  fragment TemplateFields on ChannelOverrideTemplate {
    id
    createdAt
    updatedAt
    userID
    name
    description
    overrideParameters
    overrideHeaders{
      key
      value
    }
    headerOverrideOperations {
      op
      path
      from
      to
      value
      condition
    }
    bodyOverrideOperations {
      op
      path
      from
      to
      value
      condition
    }
  }
`;

// GraphQL Queries
const QUERY_CHANNEL_OVERRIDE_TEMPLATES = `
  ${TEMPLATE_FRAGMENT}
  query ChannelOverrideTemplates(
    $after: Cursor
    $first: Int
    $before: Cursor
    $last: Int
    $where: ChannelOverrideTemplateWhereInput
  ) {
    channelOverrideTemplates(
      after: $after
      first: $first
      before: $before
      last: $last
      where: $where
    ) {
      edges {
        node {
          ...TemplateFields
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

// GraphQL Mutations
const CREATE_CHANNEL_OVERRIDE_TEMPLATE = `
  ${TEMPLATE_FRAGMENT}
  mutation CreateChannelOverrideTemplate($input: CreateChannelOverrideTemplateInput!) {
    createChannelOverrideTemplate(input: $input) {
      ...TemplateFields
    }
  }
`;

const UPDATE_CHANNEL_OVERRIDE_TEMPLATE = `
  ${TEMPLATE_FRAGMENT}
  mutation UpdateChannelOverrideTemplate($id: ID!, $input: UpdateChannelOverrideTemplateInput!) {
    updateChannelOverrideTemplate(id: $id, input: $input) {
      ...TemplateFields
    }
  }
`;

const DELETE_CHANNEL_OVERRIDE_TEMPLATE = `
  mutation DeleteChannelOverrideTemplate($id: ID!) {
    deleteChannelOverrideTemplate(id: $id)
  }
`;

const APPLY_CHANNEL_OVERRIDE_TEMPLATE = `
  mutation ApplyChannelOverrideTemplate($input: ApplyChannelOverrideTemplateInput!) {
    applyChannelOverrideTemplate(input: $input) {
      success
      updated
      channels {
        id
      }
    }
  }
`;

// React Query Hooks

export function useChannelOverrideTemplates(
  variables?: {
    search?: string;
    first?: number;
    after?: string;
  },
  options?: {
    enabled?: boolean;
  }
) {
  const { handleError } = useErrorHandler();
  const { t } = useTranslation();

  return useQuery({
    enabled: options?.enabled !== false,
    queryKey: ['channelOverrideTemplates', variables?.search, variables?.after],
    queryFn: async () => {
      try {
        const where: Record<string, unknown> = {};
        if (variables?.search) {
          where.nameContainsFold = variables.search;
        }
        const data = await graphqlRequest<{ channelOverrideTemplates: ChannelOverrideTemplateConnection }>(
          QUERY_CHANNEL_OVERRIDE_TEMPLATES,
          {
            first: variables?.first,
            after: variables?.after,
            where: Object.keys(where).length > 0 ? where : undefined,
          }
        );
        return channelOverrideTemplateConnectionSchema.parse(data?.channelOverrideTemplates);
      } catch (error) {
        handleError(error, t('channels.templates.errors.fetchList'));
        throw error;
      }
    },
  });
}

export function useCreateChannelOverrideTemplate() {
  const queryClient = useQueryClient();
  const { t } = useTranslation();

  return useMutation({
    mutationFn: async (input: CreateChannelOverrideTemplateInput) => {
      const data = await graphqlRequest<{ createChannelOverrideTemplate: ChannelOverrideTemplate }>(CREATE_CHANNEL_OVERRIDE_TEMPLATE, {
        input,
      });
      return channelOverrideTemplateSchema.parse(data.createChannelOverrideTemplate);
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['channelOverrideTemplates'] });
      toast.success(t('channels.templates.messages.createSuccess'));
    },
    onError: (error) => {
      toast.error(t('channels.templates.messages.createError', { error: error.message }));
    },
  });
}

export function useUpdateChannelOverrideTemplate() {
  const queryClient = useQueryClient();
  const { t } = useTranslation();

  return useMutation({
    mutationFn: async ({ id, input }: { id: string; input: UpdateChannelOverrideTemplateInput }) => {
      const data = await graphqlRequest<{ updateChannelOverrideTemplate: ChannelOverrideTemplate }>(UPDATE_CHANNEL_OVERRIDE_TEMPLATE, {
        id,
        input,
      });
      return channelOverrideTemplateSchema.parse(data.updateChannelOverrideTemplate);
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['channelOverrideTemplates'] });
      toast.success(t('channels.templates.messages.updateSuccess'));
    },
    onError: (error) => {
      toast.error(t('channels.templates.messages.updateError', { error: error.message }));
    },
  });
}

export function useDeleteChannelOverrideTemplate() {
  const queryClient = useQueryClient();
  const { t } = useTranslation();

  return useMutation({
    mutationFn: async (id: string) => {
      const data = await graphqlRequest<{ deleteChannelOverrideTemplate: boolean }>(DELETE_CHANNEL_OVERRIDE_TEMPLATE, { id });
      return data.deleteChannelOverrideTemplate;
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['channelOverrideTemplates'] });
      toast.success(t('channels.templates.messages.deleteSuccess'));
    },
    onError: (error) => {
      toast.error(t('channels.templates.messages.deleteError', { error: error.message }));
    },
  });
}

export function useApplyChannelOverrideTemplate() {
  const queryClient = useQueryClient();
  const { t } = useTranslation();

  return useMutation({
    mutationFn: async (input: ApplyChannelOverrideTemplateInput) => {
      const data = await graphqlRequest<{ applyChannelOverrideTemplate: ApplyChannelOverrideTemplatePayload }>(
        APPLY_CHANNEL_OVERRIDE_TEMPLATE,
        { input }
      );
      return applyChannelOverrideTemplatePayloadSchema.parse(data.applyChannelOverrideTemplate);
    },
    onSuccess: (data) => {
      queryClient.invalidateQueries({ queryKey: ['channels'] });
      toast.success(t('channels.templates.messages.applySuccess', { count: data.updated }));
    },
    onError: (error) => {
      toast.error(t('channels.templates.messages.applyError', { error: error.message }));
    },
  });
}
