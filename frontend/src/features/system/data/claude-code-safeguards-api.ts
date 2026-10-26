import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { graphqlRequest } from '@/gql/graphql';
import { toast } from 'sonner';
import { useAuthStore } from '@/stores/authStore';
import i18n from '@/lib/i18n';
import { useErrorHandler } from '@/hooks/use-error-handler';
import { usePermissions } from '@/hooks/usePermissions';
import type { ClaudeCodeSafeguardsConfig, ClaudeCodeSafeguardsSettings } from './claude-code-safeguards';

const query = `
  query ClaudeCodeSafeguardsSettings {
    claudeCodeSafeguardsSettings {
      usingServerDefaults
      defaultPolicy
      config {
        classifierModel policy fallbackModels escalateOnDenial
        attemptTimeoutSeconds timeoutSeconds maxTokens transcriptCharBudget
      }
    }
  }
`;

export function useClaudeCodeSafeguardsSettings() {
  const userID = useAuthStore((state) => state.auth.user?.id ?? null);
  const { hasSystemScope } = usePermissions();
  const canRead = hasSystemScope('read_settings');
  return useQuery({
    queryKey: ['claudeCodeSafeguardsSettings', userID, canRead],
    enabled: userID !== null && canRead,
    queryFn: async () => {
      const data = await graphqlRequest<{ claudeCodeSafeguardsSettings: ClaudeCodeSafeguardsSettings }>(query);
      return data.claudeCodeSafeguardsSettings;
    },
  });
}

export function useSaveClaudeCodeSafeguardsSettings() {
  const client = useQueryClient();
  const { handleError } = useErrorHandler();
  return useMutation({
    mutationFn: async (input: ClaudeCodeSafeguardsConfig | null) => {
      if (input === null) {
        await graphqlRequest(`mutation ResetClaudeCodeSafeguardsSettings { resetClaudeCodeSafeguardsSettings }`);
      } else {
        await graphqlRequest(
          `
          mutation UpdateClaudeCodeSafeguardsSettings($input: UpdateClaudeCodeSafeguardsInput!) {
            updateClaudeCodeSafeguardsSettings(input: $input)
          }
        `,
          { input }
        );
      }
    },
    onSuccess: async () => {
      await client.invalidateQueries({ queryKey: ['claudeCodeSafeguardsSettings'] });
      toast.success(i18n.t('system.claudeCode.saved'));
    },
    onError: (error) => handleError(error, i18n.t('system.claudeCode.saveFailed')),
  });
}
