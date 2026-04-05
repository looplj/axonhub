import { useMutation, useQuery } from '@tanstack/react-query';
import { graphqlRequest } from '@/gql/graphql';
import { useSelectedProjectId } from '@/stores/projectStore';

export type AgentMessageType = 'chat' | 'approval_request' | 'approval_result' | 'system_event';

export type AgentChatMessage = {
  id: string;
  agentID: string;
  agentInstanceID: string;
  direction: 'to_runtime' | 'to_user';
  senderType: 'user' | 'agent' | 'system';
  senderID?: number | null;
  type: AgentMessageType;
  correlationID: string;
  content: Record<string, unknown>;
  text: string;
  sequence: number;
  status: 'pending' | 'acked' | 'expired';
  createdAt: string | Date;
};

export type ApprovalScope = 'once' | 'thread' | 'workspace' | 'global';

const SEND_AGENT_MESSAGE_MUTATION = `
  mutation SendAgentMessage($input: SendAgentMessageInput!) {
    sendAgentMessage(input: $input) {
      id
      agentID
      agentInstanceID
      direction
      senderType
      senderID
      type
      correlationID
      content
      text
      sequence
      status
      createdAt
    }
  }
`;

const PULL_TO_USER_QUERY = `
  query PullAgentMessagesToUser($agentID: ID!, $agentInstanceID: ID, $afterSequence: Int, $limit: Int) {
    pullAgentMessagesToUser(agentID: $agentID, agentInstanceID: $agentInstanceID, afterSequence: $afterSequence, limit: $limit) {
      id
      agentID
      agentInstanceID
      direction
      senderType
      senderID
      type
      correlationID
      content
      text
      sequence
      status
      createdAt
    }
  }
`;

const THREAD_MESSAGES_QUERY = `
  query AgentChatMessages($agentID: ID!, $agentInstanceID: ID, $afterSequence: Int, $limit: Int) {
    agentChatMessages(agentID: $agentID, agentInstanceID: $agentInstanceID, afterSequence: $afterSequence, limit: $limit) {
      id
      agentID
      agentInstanceID
      direction
      senderType
      senderID
      type
      correlationID
      content
      text
      sequence
      status
      createdAt
    }
  }
`;

export function useSendAgentMessage() {
  const selectedProjectId = useSelectedProjectId();

  return useMutation({
    mutationFn: async (input: { agentID: string; agentInstanceID?: string; text: string }) => {
      const headers = selectedProjectId ? { 'X-Project-ID': selectedProjectId } : undefined;
      const data = await graphqlRequest<{ sendAgentMessage: AgentChatMessage }>(SEND_AGENT_MESSAGE_MUTATION, { input }, headers);
      return data.sendAgentMessage;
    },
  });
}

export function useAgentChatMessages(agentID: string, agentInstanceID?: string) {
  const selectedProjectId = useSelectedProjectId();

  return useQuery({
    queryKey: ['agentChatMessages', agentID, agentInstanceID, selectedProjectId],
    queryFn: async () => {
      const headers = selectedProjectId ? { 'X-Project-ID': selectedProjectId } : undefined;
      const data = await graphqlRequest<{ agentChatMessages: AgentChatMessage[] }>(
        THREAD_MESSAGES_QUERY,
        { agentID, agentInstanceID: agentInstanceID ?? null, afterSequence: null, limit: 200 },
        headers
      );
      return data.agentChatMessages ?? [];
    },
    enabled: Boolean(selectedProjectId && agentID),
  });
}

const RESOLVE_APPROVAL_MUTATION = `
  mutation ResolveApproval($input: ResolveApprovalInput!) {
    resolveApproval(input: $input)
  }
`;

const ACK_AGENT_MESSAGES_MUTATION = `
  mutation AckAgentMessages($input: AckAgentMessagesInput!) {
    ackAgentMessages(input: $input)
  }
`;

export function usePullAgentMessagesToUser(agentID: string, agentInstanceID: string | undefined, afterSequence: number) {
  const selectedProjectId = useSelectedProjectId();

  return useQuery({
    queryKey: ['pullAgentMessagesToUser', agentID, agentInstanceID, afterSequence, selectedProjectId],
    queryFn: async () => {
      const headers = selectedProjectId ? { 'X-Project-ID': selectedProjectId } : undefined;
      const data = await graphqlRequest<{ pullAgentMessagesToUser: AgentChatMessage[] }>(
        PULL_TO_USER_QUERY,
        { agentID, agentInstanceID: agentInstanceID ?? null, afterSequence, limit: 50 },
        headers
      );
      return data.pullAgentMessagesToUser ?? [];
    },
    enabled: Boolean(selectedProjectId && agentID),
    refetchInterval: 1500,
  });
}

export function useResolveApproval() {
  const selectedProjectId = useSelectedProjectId();

  return useMutation({
    mutationFn: async (input: { agentID: string; agentInstanceID?: string; requestID: string; granted: boolean; scope?: ApprovalScope; reason?: string; resourceIndices?: number[] }) => {
      const headers = selectedProjectId ? { 'X-Project-ID': selectedProjectId } : undefined;
      const data = await graphqlRequest<{ resolveApproval: boolean }>(RESOLVE_APPROVAL_MUTATION, { input }, headers);
      return data.resolveApproval;
    },
  });
}

export function useAckAgentMessages() {
  const selectedProjectId = useSelectedProjectId();

  return useMutation({
    mutationFn: async (input: { agentID: string; agentInstanceID?: string; messageIDs: string[] }) => {
      const headers = selectedProjectId ? { 'X-Project-ID': selectedProjectId } : undefined;
      const data = await graphqlRequest<{ ackAgentMessages: boolean }>(ACK_AGENT_MESSAGES_MUTATION, { input }, headers);
      return data.ackAgentMessages;
    },
  });
}
