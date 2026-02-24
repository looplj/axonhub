import { useMutation, useQueryClient } from '@tanstack/react-query';
import { graphqlRequest } from '@/gql/graphql';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';

const DEPLOY_AXONCLAW_MUTATION = `
  mutation DeployAxonclaw($input: DeployAxonclawInput!) {
    deployAxonclaw(input: $input) {
      success
      error
      instance {
        id
        instanceID
        name
        platform
        version
        lastHeartbeatAt
        createdAt
        updatedAt
      }
    }
  }
`;

export interface DeployAxonclawInput {
  agentID: string;
  runtimeID: string;
  name: string;
  directory?: string;
}

export interface DeployAxonclawResult {
  success: boolean;
  error?: string;
  instance?: {
    id: string;
    instanceID: string;
    name: string;
    platform: string;
    version: string;
    lastHeartbeatAt: string;
    createdAt: string;
    updatedAt: string;
  };
}

export function useDeployAxonclaw() {
  const queryClient = useQueryClient();
  const { t } = useTranslation();

  return useMutation({
    mutationFn: async (input: DeployAxonclawInput) => {
      const data = await graphqlRequest<{ deployAxonclaw: DeployAxonclawResult }>(
        DEPLOY_AXONCLAW_MUTATION,
        { input }
      );
      return data.deployAxonclaw;
    },
    onSuccess: (data, variables) => {
      if (data.success) {
        queryClient.invalidateQueries({ queryKey: ['agentDetail', variables.agentID] });
        toast.success(t('agents.messages.deploySuccess'));
      } else {
        toast.error(t('agents.messages.deployError', { error: data.error || 'Unknown error' }));
      }
    },
    onError: (error) => {
      toast.error(t('agents.messages.deployError', { error: error.message }));
    },
  });
}
