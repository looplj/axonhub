import { useCallback, useEffect, useReducer, useRef } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { graphqlRequest } from '@/gql/graphql';
import { toast } from 'sonner';
import { useAuthStore } from '@/stores/authStore';
import i18n from '@/lib/i18n';
import { useErrorHandler } from '@/hooks/use-error-handler';
import { usePermissions } from '@/hooks/usePermissions';
import {
  codexAccess,
  codexTestReducer,
  idleCodexTest,
  sortCodexChannels,
  visibleCodexTest,
  type CodexAccess,
  type CodexCatalogChannel,
  type CodexCatalogTestResult,
  type CodexCompatibilitySettings,
  type UpdateCodexCompatibilitySettingsInput,
} from './codex-compatibility';

const CODEX_COMPATIBILITY_SETTINGS_QUERY = `
  query CodexCompatibilitySettings {
    codexCompatibilitySettings {
      enabled
      channelID
    }
  }
`;

const CODEX_CATALOG_CHANNELS_QUERY = `
  query CodexCatalogChannels {
    codexCatalogChannels {
      id
      name
      status
    }
  }
`;

const UPDATE_CODEX_COMPATIBILITY_SETTINGS_MUTATION = `
  mutation UpdateCodexCompatibilitySettings($input: UpdateCodexCompatibilitySettingsInput!) {
    updateCodexCompatibilitySettings(input: $input)
  }
`;

const TEST_CODEX_CATALOG_MUTATION = `
  mutation TestCodexCatalog($channelID: Int!) {
    testCodexCatalog(channelID: $channelID) {
      success
      modelCount
      upstreamStatus
      error
    }
  }
`;

// Query keys carry the signed-in user and the scope flags so one user's cached data is never reused by another.
function useCodexQueryIdentity(): { authUserId: string | null; canReadSettings: boolean; access: CodexAccess } {
  const { hasSystemScope } = usePermissions();
  const authUserId = useAuthStore((state) => state.auth.user?.id ?? null);
  return { authUserId, canReadSettings: hasSystemScope('read_settings'), access: codexAccess(hasSystemScope) };
}

export function useCodexCompatibilitySettings() {
  const { handleError } = useErrorHandler();
  const { authUserId, canReadSettings } = useCodexQueryIdentity();

  return useQuery({
    queryKey: ['codexCompatibilitySettings', authUserId ?? 'signed-out', canReadSettings],
    enabled: canReadSettings,
    queryFn: async () => {
      try {
        const data = await graphqlRequest<{ codexCompatibilitySettings: CodexCompatibilitySettings }>(CODEX_COMPATIBILITY_SETTINGS_QUERY);
        return data.codexCompatibilitySettings;
      } catch (error) {
        handleError(error, i18n.t('common.errors.internalServerError'));
        throw error;
      }
    },
  });
}

export function useCodexCatalogChannels() {
  const { handleError } = useErrorHandler();
  const { authUserId, access } = useCodexQueryIdentity();

  return useQuery({
    queryKey: ['codexCatalogChannels', authUserId ?? 'signed-out', access.canListChannels],
    enabled: access.canListChannels,
    queryFn: async () => {
      try {
        const data = await graphqlRequest<{ codexCatalogChannels: CodexCatalogChannel[] }>(CODEX_CATALOG_CHANNELS_QUERY);
        return sortCodexChannels(data.codexCatalogChannels);
      } catch (error) {
        handleError(error, i18n.t('common.errors.internalServerError'));
        throw error;
      }
    },
  });
}

export function useUpdateCodexCompatibilitySettings() {
  const queryClient = useQueryClient();
  const { handleError } = useErrorHandler();

  return useMutation({
    mutationFn: async (input: UpdateCodexCompatibilitySettingsInput) => {
      const data = await graphqlRequest<{ updateCodexCompatibilitySettings: boolean }>(UPDATE_CODEX_COMPATIBILITY_SETTINGS_MUTATION, {
        input,
      });
      return data.updateCodexCompatibilitySettings;
    },
    // Resolving only after the refetch lets the form drop its draft without flashing the previous values.
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ['codexCompatibilitySettings'] });
      toast.success(i18n.t('common.success.systemUpdated'));
    },
    onError: (error) => {
      handleError(error, i18n.t('common.errors.systemUpdateFailed'));
    },
  });
}

// Runs the catalog test for the selected (possibly unsaved) channel. Only the request that is still current
// may publish a result: switching channels, cancelling, or starting another test discards older responses.
export function useCodexCatalogTest(selectedChannelID: number | null) {
  const [state, dispatch] = useReducer(codexTestReducer, idleCodexTest);
  const seqRef = useRef(0);
  const abortRef = useRef<AbortController | null>(null);

  useEffect(() => {
    dispatch({ type: 'selectionChanged', channelID: selectedChannelID });
    return () => abortRef.current?.abort();
  }, [selectedChannelID]);

  const run = useCallback(async () => {
    if (selectedChannelID === null) {
      return;
    }
    abortRef.current?.abort();
    const controller = new AbortController();
    abortRef.current = controller;
    seqRef.current += 1;
    const seq = seqRef.current;
    dispatch({ type: 'started', seq, channelID: selectedChannelID });
    try {
      const data = await graphqlRequest<{ testCodexCatalog: CodexCatalogTestResult }>(
        TEST_CODEX_CATALOG_MUTATION,
        { channelID: selectedChannelID },
        undefined,
        { signal: controller.signal }
      );
      dispatch({ type: 'finished', seq, result: data.testCodexCatalog });
    } catch (error) {
      if (controller.signal.aborted) {
        return;
      }
      dispatch({ type: 'rejected', seq, message: error instanceof Error ? error.message : i18n.t('common.errors.unknownError') });
    }
  }, [selectedChannelID]);

  const cancel = useCallback(() => {
    abortRef.current?.abort();
    dispatch({ type: 'cancelled' });
  }, []);

  return { state: visibleCodexTest(state, selectedChannelID), run, cancel };
}
