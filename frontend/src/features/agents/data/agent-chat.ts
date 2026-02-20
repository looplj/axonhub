import { useMutation, useQuery } from '@tanstack/react-query';
import { graphqlRequest } from '@/gql/graphql';
import { useSelectedProjectId } from '@/stores/projectStore';

export type AgentChatMessage = {
  id: string;
  agentID: string;
  threadID: string;
  direction: 'to_runtime' | 'to_user';
  senderType: 'user' | 'runtime' | 'system';
  senderID?: number | null;
  text: string;
  sequence: number;
  status: 'pending' | 'acked' | 'expired';
  createdAt: string | Date;
};

const SEND_AGENT_MESSAGE_MUTATION = `
  mutation SendAgentMessage($input: SendAgentMessageInput!) {
    sendAgentMessage(input: $input) {
      id
      agentID
      threadID
      direction
      senderType
      senderID
      text
      sequence
      status
      createdAt
    }
  }
`;

const PULL_TO_USER_QUERY = `
  query PullAgentMessagesToUser($agentID: ID!, $threadID: String!, $afterSequence: Int, $limit: Int) {
    pullAgentMessagesToUser(agentID: $agentID, threadID: $threadID, afterSequence: $afterSequence, limit: $limit) {
      id
      agentID
      threadID
      direction
      senderType
      senderID
      text
      sequence
      status
      createdAt
    }
  }
`;

const THREAD_MESSAGES_QUERY = `
  query AgentThreadMessages($agentID: ID!, $threadID: String!, $afterSequence: Int, $limit: Int) {
    agentThreadMessages(agentID: $agentID, threadID: $threadID, afterSequence: $afterSequence, limit: $limit) {
      id
      agentID
      threadID
      direction
      senderType
      senderID
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
    mutationFn: async (input: { agentID: string; threadID: string; text: string }) => {
      const headers = selectedProjectId ? { 'X-Project-ID': selectedProjectId } : undefined;
      const data = await graphqlRequest<{ sendAgentMessage: AgentChatMessage }>(SEND_AGENT_MESSAGE_MUTATION, { input }, headers);
      return data.sendAgentMessage;
    },
  });
}

export function useAgentThreadMessages(agentID: string, threadID: string) {
  const selectedProjectId = useSelectedProjectId();

  return useQuery({
    queryKey: ['agentThreadMessages', agentID, threadID, selectedProjectId],
    queryFn: async () => {
      const headers = selectedProjectId ? { 'X-Project-ID': selectedProjectId } : undefined;
      const data = await graphqlRequest<{ agentThreadMessages: AgentChatMessage[] }>(
        THREAD_MESSAGES_QUERY,
        { agentID, threadID, afterSequence: null, limit: 200 },
        headers
      );
      return data.agentThreadMessages ?? [];
    },
    enabled: Boolean(selectedProjectId && agentID && threadID),
  });
}

export function usePullAgentMessagesToUser(agentID: string, threadID: string, afterSequence: number) {
  const selectedProjectId = useSelectedProjectId();

  return useQuery({
    queryKey: ['pullAgentMessagesToUser', agentID, threadID, afterSequence, selectedProjectId],
    queryFn: async () => {
      const headers = selectedProjectId ? { 'X-Project-ID': selectedProjectId } : undefined;
      const data = await graphqlRequest<{ pullAgentMessagesToUser: AgentChatMessage[] }>(
        PULL_TO_USER_QUERY,
        { agentID, threadID, afterSequence, limit: 50 },
        headers
      );
      return data.pullAgentMessagesToUser ?? [];
    },
    enabled: Boolean(selectedProjectId && agentID && threadID),
    refetchInterval: 1500,
  });
}

