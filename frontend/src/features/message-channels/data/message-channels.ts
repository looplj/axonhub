import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import { useTranslation } from 'react-i18next';
import { graphqlRequest } from '@/gql/graphql';
import { useSelectedProjectId } from '@/stores/projectStore';
import type {
  MessageChannel,
  MessageChannelConnection,
  CreateMessageChannelInput,
  UpdateMessageChannelInput,
  MessageChannelAgentInstanceBindingInput,
} from './schema';
import { messageChannelConnectionSchema, messageChannelSchema } from './schema';

const MESSAGE_CHANNELS_QUERY = `
  query GetMessageChannels($first: Int, $after: Cursor, $last: Int, $before: Cursor, $where: MessageChannelWhereInput, $orderBy: MessageChannelOrder) {
    messageChannels(first: $first, after: $after, last: $last, before: $before, where: $where, orderBy: $orderBy) {
      edges {
        node {
          id
          createdAt
          updatedAt
          projectID
          name
          description
          type
          status
          settings {
            feishu {
              appId
              appSecret
              encryptKey
              verificationToken
              allowFrom
              excludeKeywords
            }
          }
          agentInstanceBindings {
            edges {
              node {
                id
                createdAt
                updatedAt
                messageChannelID
                agentInstanceID
                enabled
                config {
                  chatType
                  chatID
                  allowFrom
                  excludeKeywords
                }
                agentInstance {
                  id
                  name
                  description
                  status
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

const MESSAGE_CHANNEL_QUERY = `
  query GetMessageChannel($id: ID!) {
    node(id: $id) {
      ... on MessageChannel {
        id
        createdAt
        updatedAt
        projectID
        name
        description
        type
        status
        settings {
          feishu {
            appId
            appSecret
            encryptKey
            verificationToken
            allowFrom
            excludeKeywords
          }
        }
        agentInstanceBindings {
          edges {
            node {
              id
              createdAt
              updatedAt
              messageChannelID
              agentInstanceID
              enabled
              config {
                chatType
                chatID
                allowFrom
                excludeKeywords
              }
              agentInstance {
                id
                name
                description
                status
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
    }
  }
`;

const CREATE_MESSAGE_CHANNEL_MUTATION = `
  mutation CreateMessageChannel($input: CreateMessageChannelInput!) {
    createMessageChannel(input: $input) {
      id
      createdAt
      updatedAt
      projectID
      name
      description
      type
      status
      settings {
        feishu {
          appId
          appSecret
          encryptKey
          verificationToken
          allowFrom
          excludeKeywords
        }
      }
    }
  }
`;

const UPDATE_MESSAGE_CHANNEL_MUTATION = `
  mutation UpdateMessageChannel($id: ID!, $input: UpdateMessageChannelInput!) {
    updateMessageChannel(id: $id, input: $input) {
      id
      createdAt
      updatedAt
      projectID
      name
      description
      type
      status
      settings {
        feishu {
          appId
          appSecret
          encryptKey
          verificationToken
          allowFrom
          excludeKeywords
        }
      }
    }
  }
`;

const DELETE_MESSAGE_CHANNEL_MUTATION = `
  mutation DeleteMessageChannel($id: ID!) {
    deleteMessageChannel(id: $id)
  }
`;

const BATCH_SAVE_MESSAGE_CHANNEL_BINDINGS_MUTATION = `
  mutation BatchSaveMessageChannelBindings($messageChannelID: ID!, $bindings: [BatchMessageChannelAgentInstanceBindingInput!]!) {
    batchSaveMessageChannelBindings(messageChannelID: $messageChannelID, bindings: $bindings) {
      id
      createdAt
      updatedAt
      projectID
      name
      description
      type
      status
      settings {
        feishu {
          appId
          appSecret
          encryptKey
          verificationToken
          allowFrom
          excludeKeywords
        }
      }
      agentInstanceBindings {
        edges {
          node {
            id
            createdAt
            updatedAt
            messageChannelID
            agentInstanceID
            enabled
            config {
              chatType
              chatID
              allowFrom
              excludeKeywords
            }
            agentInstance {
              id
              name
              description
              status
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
  }
`;

const CREATE_BINDING_REQUEST_MUTATION = `
  mutation CreateBindingRequest($input: CreateMessageChannelBindingRequestInput!) {
    createBindingRequest(input: $input)
  }
`;

interface QueryMessageChannelsArgs {
  first?: number;
  after?: string;
  last?: number;
  before?: string;
  where?: Record<string, unknown>;
  orderBy?: {
    field: 'CREATED_AT' | 'UPDATED_AT' | 'TYPE' | 'STATUS';
    direction: 'ASC' | 'DESC';
  };
}

export interface BatchBindingInput {
  agentInstanceID: string;
  enabled: boolean;
  config?: MessageChannelAgentInstanceBindingInput;
}

export interface CreateBindingRequestInput {
  messageChannelID: number;
  agentInstanceID: number;
  type: string;
}

export function useQueryMessageChannels(args: QueryMessageChannelsArgs) {
  const selectedProjectId = useSelectedProjectId();

  return useQuery({
    queryKey: ['messageChannels', args, selectedProjectId],
    queryFn: async () => {
      const headers = selectedProjectId ? { 'X-Project-ID': selectedProjectId } : undefined;
      const data = await graphqlRequest<{ messageChannels: MessageChannelConnection }>(
        MESSAGE_CHANNELS_QUERY,
        args,
        headers
      );
      return messageChannelConnectionSchema.parse(data.messageChannels);
    },
    enabled: !!selectedProjectId,
  });
}

export function useMessageChannel(id: string) {
  const selectedProjectId = useSelectedProjectId();

  return useQuery({
    queryKey: ['messageChannel', id, selectedProjectId],
    queryFn: async () => {
      const headers = selectedProjectId ? { 'X-Project-ID': selectedProjectId } : undefined;
      const data = await graphqlRequest<{ node: MessageChannel }>(
        MESSAGE_CHANNEL_QUERY,
        { id },
        headers
      );
      return data.node ? messageChannelSchema.parse(data.node) : null;
    },
    enabled: !!id && !!selectedProjectId,
  });
}

export function useCreateMessageChannel() {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const selectedProjectId = useSelectedProjectId();

  return useMutation({
    mutationFn: async (input: CreateMessageChannelInput) => {
      const headers = selectedProjectId ? { 'X-Project-ID': selectedProjectId } : undefined;
      const data = await graphqlRequest<{ createMessageChannel: MessageChannel }>(
        CREATE_MESSAGE_CHANNEL_MUTATION,
        { input },
        headers
      );
      return messageChannelSchema.parse(data.createMessageChannel);
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['messageChannels'] });
      toast.success(t('messageChannels.messages.createSuccess'));
    },
    onError: (error: Error) => {
      toast.error(t('messageChannels.messages.createError', { error: error.message }));
    },
  });
}

export function useUpdateMessageChannel() {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const selectedProjectId = useSelectedProjectId();

  return useMutation({
    mutationFn: async ({ id, input }: { id: string; input: UpdateMessageChannelInput }) => {
      const headers = selectedProjectId ? { 'X-Project-ID': selectedProjectId } : undefined;
      const data = await graphqlRequest<{ updateMessageChannel: MessageChannel }>(
        UPDATE_MESSAGE_CHANNEL_MUTATION,
        { id, input },
        headers
      );
      return messageChannelSchema.parse(data.updateMessageChannel);
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['messageChannels'] });
      toast.success(t('messageChannels.messages.updateSuccess'));
    },
    onError: (error: Error) => {
      toast.error(t('messageChannels.messages.updateError', { error: error.message }));
    },
  });
}

export function useDeleteMessageChannel() {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const selectedProjectId = useSelectedProjectId();

  return useMutation({
    mutationFn: async (id: string) => {
      const headers = selectedProjectId ? { 'X-Project-ID': selectedProjectId } : undefined;
      await graphqlRequest(DELETE_MESSAGE_CHANNEL_MUTATION, { id }, headers);
      return true;
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['messageChannels'] });
      toast.success(t('messageChannels.messages.deleteSuccess'));
    },
    onError: (error: Error) => {
      toast.error(t('messageChannels.messages.deleteError', { error: error.message }));
    },
  });
}

export function useBatchSaveMessageChannelBindings() {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const selectedProjectId = useSelectedProjectId();

  return useMutation({
    mutationFn: async ({
      messageChannelID,
      bindings,
    }: {
      messageChannelID: string;
      bindings: BatchBindingInput[];
    }) => {
      const headers = selectedProjectId ? { 'X-Project-ID': selectedProjectId } : undefined;
      const data = await graphqlRequest<{ batchSaveMessageChannelBindings: MessageChannel }>(
        BATCH_SAVE_MESSAGE_CHANNEL_BINDINGS_MUTATION,
        { messageChannelID, bindings },
        headers
      );
      return messageChannelSchema.parse(data.batchSaveMessageChannelBindings);
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['messageChannels'] });
      toast.success(t('messageChannels.messages.bindingsSaveSuccess'));
    },
    onError: (error: Error) => {
      toast.error(t('messageChannels.messages.bindingsSaveError', { error: error.message }));
    },
  });
}

export function useCreateBindingRequest() {
  const { t } = useTranslation();
  const selectedProjectId = useSelectedProjectId();

  return useMutation({
    mutationFn: async (input: CreateBindingRequestInput) => {
      const headers = selectedProjectId ? { 'X-Project-ID': selectedProjectId } : undefined;
      const data = await graphqlRequest<{ createBindingRequest: string }>(
        CREATE_BINDING_REQUEST_MUTATION,
        { input },
        headers
      );
      return { pairCode: data.createBindingRequest };
    },
    onSuccess: () => {
      toast.success(t('messageChannels.messages.bindingRequestCreated'));
    },
    onError: (error: Error) => {
      toast.error(t('messageChannels.messages.bindingRequestError', { error: error.message }));
    },
  });
}
