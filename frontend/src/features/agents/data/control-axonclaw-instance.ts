import { useMutation, useQueryClient } from '@tanstack/react-query';
import { graphqlRequest } from '@/gql/graphql';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';

type ControlAction = 'start' | 'stop' | 'restart' | 'redeploy';

const MUTATIONS: Record<ControlAction, string> = {
  start: `
    mutation StartAxonclawInstance($instanceID: ID!) {
      startAxonclawInstance(instanceID: $instanceID) {
        success
        error
        instance { id status lastHeartbeatAt updatedAt }
      }
    }
  `,
  stop: `
    mutation StopAxonclawInstance($instanceID: ID!) {
      stopAxonclawInstance(instanceID: $instanceID) {
        success
        error
        instance { id status lastHeartbeatAt updatedAt }
      }
    }
  `,
  restart: `
    mutation RestartAxonclawInstance($instanceID: ID!) {
      restartAxonclawInstance(instanceID: $instanceID) {
        success
        error
        instance { id status lastHeartbeatAt updatedAt }
      }
    }
  `,
  redeploy: `
    mutation RedeployAxonclawInstance($instanceID: ID!, $axonhubBaseUrl: String) {
      redeployAxonclawInstance(instanceID: $instanceID, axonhubBaseUrl: $axonhubBaseUrl) {
        success
        error
        instance { id status lastHeartbeatAt updatedAt }
      }
    }
  `,
};

type ControlResult = {
  success: boolean;
  error?: string | null;
  instance?: { id: string; status: string } | null;
};

function resultKey(action: ControlAction) {
  switch (action) {
    case 'start':
      return 'startAxonclawInstance';
    case 'stop':
      return 'stopAxonclawInstance';
    case 'restart':
      return 'restartAxonclawInstance';
    case 'redeploy':
      return 'redeployAxonclawInstance';
  }
}

export function useControlAxonclawInstance(agentID: string) {
  const queryClient = useQueryClient();
  const { t } = useTranslation();

  return useMutation({
    mutationFn: async (input: { instanceID: string; action: ControlAction; axonhubBaseUrl?: string }) => {
      const mutation = MUTATIONS[input.action];
      const key = resultKey(input.action);
      const variables: Record<string, any> = {
        instanceID: input.instanceID,
      };
      if (input.action === 'redeploy' && input.axonhubBaseUrl !== undefined) {
        variables.axonhubBaseUrl = input.axonhubBaseUrl;
      }
      const data = await graphqlRequest<Record<string, ControlResult>>(mutation, variables);
      return data[key];
    },
    onSuccess: (data, variables) => {
      queryClient.invalidateQueries({ queryKey: ['agentDetail', agentID] });

      const messageKey =
        variables.action === 'start'
          ? 'agents.messages.instanceStartSuccess'
          : variables.action === 'stop'
            ? 'agents.messages.instanceStopSuccess'
            : variables.action === 'restart'
              ? 'agents.messages.instanceRestartSuccess'
              : 'agents.messages.instanceRedeploySuccess';

      const errorKey =
        variables.action === 'start'
          ? 'agents.messages.instanceStartError'
          : variables.action === 'stop'
            ? 'agents.messages.instanceStopError'
            : variables.action === 'restart'
              ? 'agents.messages.instanceRestartError'
              : 'agents.messages.instanceRedeployError';

      if (data?.success) {
        toast.success(t(messageKey));
      } else {
        toast.error(t(errorKey, { error: data?.error || 'Unknown error' }));
      }
    },
    onError: (error, variables) => {
      const errorKey =
        variables.action === 'start'
          ? 'agents.messages.instanceStartError'
          : variables.action === 'stop'
            ? 'agents.messages.instanceStopError'
            : variables.action === 'restart'
              ? 'agents.messages.instanceRestartError'
              : 'agents.messages.instanceRedeployError';
      toast.error(t(errorKey, { error: error.message }));
    },
  });
}

export function useRedeployAxonclawInstance(agentID: string) {
  const controlInstance = useControlAxonclawInstance(agentID);

  return useMutation({
    mutationFn: async (input: { instanceID: string; axonhubBaseUrl?: string }) => {
      return controlInstance.mutateAsync({
        instanceID: input.instanceID,
        action: 'redeploy',
        axonhubBaseUrl: input.axonhubBaseUrl,
      });
    },
  });
}

