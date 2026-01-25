import { useState, useCallback } from 'react';
import { toast } from 'sonner';
import { useTranslation } from 'react-i18next';

export interface OAuthStartResult {
  session_id: string;
  auth_url: string;
}

export interface OAuthExchangeInput {
  session_id: string;
  callback_url: string;
}

export interface OAuthExchangeResult {
  credentials: string;
}

export interface OAuthFlowOptions {
  /**
   * The provider name (used for i18n keys: e.g., 'codex', 'claudecode')
   */
  provider: string;

  /**
   * Function to call the OAuth start endpoint
   */
  startFn: (headers?: Record<string, string>) => Promise<OAuthStartResult>;

  /**
   * Function to call the OAuth exchange endpoint
   */
  exchangeFn: (input: OAuthExchangeInput, headers?: Record<string, string>) => Promise<OAuthExchangeResult>;

  /**
   * Optional project ID to include in headers
   */
  projectId?: string | null;

  /**
   * Callback when credentials are successfully obtained
   */
  onSuccess?: (credentials: string) => void;
}

export interface OAuthFlowState {
  sessionId: string | null;
  authUrl: string | null;
  callbackUrl: string;
  isStarting: boolean;
  isExchanging: boolean;
}

export interface OAuthFlowActions {
  start: () => Promise<void>;
  exchange: () => Promise<void>;
  setCallbackUrl: (url: string) => void;
  reset: () => void;
}

/**
 * A reusable hook for managing OAuth flows (e.g., Codex, Claude Code).
 * This eliminates code duplication for different OAuth providers.
 *
 * @example
 * ```tsx
 * const codexOAuth = useOAuthFlow({
 *   provider: 'codex',
 *   startFn: codexOAuthStart,
 *   exchangeFn: codexOAuthExchange,
 *   projectId: selectedProjectId,
 *   onSuccess: (credentials) => form.setValue('credentials.apiKey', credentials),
 * });
 *
 * // Later in your component:
 * <Button onClick={codexOAuth.start} disabled={codexOAuth.isStarting}>
 *   {codexOAuth.isStarting ? 'Starting...' : 'Start OAuth'}
 * </Button>
 * ```
 */
export function useOAuthFlow(options: OAuthFlowOptions): OAuthFlowState & OAuthFlowActions {
  const { provider, startFn, exchangeFn, projectId, onSuccess } = options;
  const { t } = useTranslation();

  const [sessionId, setSessionId] = useState<string | null>(null);
  const [authUrl, setAuthUrl] = useState<string | null>(null);
  const [callbackUrl, setCallbackUrl] = useState('');
  const [isStarting, setIsStarting] = useState(false);
  const [isExchanging, setIsExchanging] = useState(false);

  const start = useCallback(async () => {
    if (!projectId) {
      toast.error(t(`channels.dialogs.${provider}.errors.projectRequired`));
      return;
    }

    setIsStarting(true);
    try {
      const result = await startFn({ 'X-Project-ID': projectId });
      setSessionId(result.session_id);
      setAuthUrl(result.auth_url);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : String(error));
    } finally {
      setIsStarting(false);
    }
  }, [projectId, provider, startFn, t]);

  const exchange = useCallback(async () => {
    if (!projectId) {
      toast.error(t(`channels.dialogs.${provider}.errors.projectRequired`));
      return;
    }

    if (!sessionId) {
      toast.error(t(`channels.dialogs.${provider}.errors.sessionMissing`));
      return;
    }

    if (!callbackUrl.trim()) {
      toast.error(t(`channels.dialogs.${provider}.errors.callbackUrlRequired`));
      return;
    }

    setIsExchanging(true);
    try {
      const result = await exchangeFn(
        {
          session_id: sessionId,
          callback_url: callbackUrl.trim(),
        },
        { 'X-Project-ID': projectId }
      );

      if (onSuccess) {
        onSuccess(result.credentials);
      }

      toast.success(t(`channels.dialogs.${provider}.messages.credentialsImported`));
    } catch (error) {
      toast.error(error instanceof Error ? error.message : String(error));
    } finally {
      setIsExchanging(false);
    }
  }, [projectId, sessionId, callbackUrl, provider, exchangeFn, onSuccess, t]);

  const reset = useCallback(() => {
    setSessionId(null);
    setAuthUrl(null);
    setCallbackUrl('');
    setIsStarting(false);
    setIsExchanging(false);
  }, []);

  return {
    sessionId,
    authUrl,
    callbackUrl,
    isStarting,
    isExchanging,
    start,
    exchange,
    setCallbackUrl,
    reset,
  };
}
