import { useState, useCallback, useRef, useEffect } from 'react';
import { toast } from 'sonner';
import { useTranslation } from 'react-i18next';
import {
  copilotOAuthStart,
  copilotOAuthPoll,
  DeviceFlowStartResult,
  DeviceFlowPollResult,
} from '../data/copilot';

export interface UseDeviceFlowOptions {
  /**
   * Optional project ID to include in headers
   */
  projectId?: string | null;

  /**
   * Callback when access token is successfully obtained
   */
  onSuccess?: (accessToken: string) => void;
}

export interface UseDeviceFlowState {
  userCode: string | null;
  verificationUri: string | null;
  sessionId: string | null;
  expiresAt: number | null;
  interval: number;
  isPolling: boolean;
  error: string | null;
  isComplete: boolean;
}

export interface UseDeviceFlowActions {
  start: () => Promise<void>;
  reset: () => void;
}

/**
 * A hook for managing GitHub Copilot OAuth device flow.
 *
 * Device flow is used when the user cannot be redirected to a callback URL.
 * Instead, the user receives a code to enter on GitHub's device activation page.
 *
 * @example
 * ```tsx
 * const deviceFlow = useDeviceFlow({
 *   projectId: selectedProjectId,
 *   onSuccess: (token) => form.setValue('credentials.apiKey', token),
 * });
 *
 * // Display user code and verification URI
 * {deviceFlow.userCode && (
 *   <div>
 *     <p>Enter code: <strong>{deviceFlow.userCode}</strong></p>
 *     <a href={deviceFlow.verificationUri} target="_blank">
 *       Go to {deviceFlow.verificationUri}
 *     </a>
 *   </div>
 * )}
 *
 * // Start the flow
 * <Button onClick={deviceFlow.start} disabled={deviceFlow.isPolling}>
 *   {deviceFlow.isPolling ? 'Waiting for authorization...' : 'Start Device Flow'}
 * </Button>
 * ```
 */
export function useDeviceFlow(
  options: UseDeviceFlowOptions = {}
): UseDeviceFlowState & UseDeviceFlowActions {
  const { projectId, onSuccess } = options;
  const { t } = useTranslation();

  const [userCode, setUserCode] = useState<string | null>(null);
  const [verificationUri, setVerificationUri] = useState<string | null>(null);
  const [sessionId, setSessionId] = useState<string | null>(null);
  const [expiresAt, setExpiresAt] = useState<number | null>(null);
  const [interval, setInterval] = useState(5);
  const [isPolling, setIsPolling] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [isComplete, setIsComplete] = useState(false);

  const pollingIntervalRef = useRef<number | null>(null);
  const currentIntervalRef = useRef(5);

  useEffect(() => {
    return () => {
      if (pollingIntervalRef.current) {
        clearInterval(pollingIntervalRef.current);
      }
    };
  }, []);

  const start = useCallback(async () => {
    if (!projectId) {
      toast.error(t('channels.dialogs.oauth.errors.projectRequired'));
      return;
    }

    setIsPolling(true);
    setError(null);

    try {
      const result: DeviceFlowStartResult = await copilotOAuthStart({
        'X-Project-ID': projectId,
      });

      setUserCode(result.user_code);
      setVerificationUri(result.verification_uri);
      setSessionId(result.session_id);
      setExpiresAt(Date.now() + result.expires_in * 1000);
      setInterval(result.interval);
      currentIntervalRef.current = result.interval;

      poll(result.session_id, result.interval, Date.now() + result.expires_in * 1000);
    } catch (err) {
      const errorMessage = err instanceof Error ? err.message : String(err);
      setError(errorMessage);
      setIsPolling(false);
    }
  }, [projectId, t]);

  const poll = useCallback(
    async (sessionId: string, pollInterval: number, expiry: number) => {
      if (!projectId) {
        return;
      }

      if (pollingIntervalRef.current) {
        clearInterval(pollingIntervalRef.current);
      }

      // @ts-expect-error - setInterval return type mismatch between browser and Node.js types
      pollingIntervalRef.current = window.setInterval(async () => {
        if (Date.now() >= expiry) {
          if (pollingIntervalRef.current) {
            clearInterval(pollingIntervalRef.current);
            pollingIntervalRef.current = null;
          }
          setIsPolling(false);
          setError(t('channels.dialogs.oauth.errors.deviceFlowExpired'));
          return;
        }

        try {
          const result: DeviceFlowPollResult = await copilotOAuthPoll(
            { session_id: sessionId },
            { 'X-Project-ID': projectId }
          );

          if (result.access_token) {
            if (pollingIntervalRef.current) {
              clearInterval(pollingIntervalRef.current);
              pollingIntervalRef.current = null;
            }
            setIsPolling(false);
            setIsComplete(true);

            if (onSuccess) {
              onSuccess(result.access_token);
            }

            toast.success(t('channels.dialogs.oauth.messages.credentialsImported'));
          } else if (result.status) {
            if (result.status === 'pending') {
              // Authorization still pending, continue polling
              return;
            } else if (result.status === 'slow_down') {
              const newInterval = currentIntervalRef.current * 2;
              currentIntervalRef.current = newInterval;
              setInterval(newInterval);

              if (pollingIntervalRef.current) {
                clearInterval(pollingIntervalRef.current);
              }
              // @ts-expect-error - setInterval return type mismatch between browser and Node.js types
              pollingIntervalRef.current = window.setInterval(() => {
                poll(sessionId, newInterval, expiry);
              }, newInterval * 1000);
            } else {
              // Other error statuses
              if (pollingIntervalRef.current) {
                clearInterval(pollingIntervalRef.current);
                pollingIntervalRef.current = null;
              }
              setIsPolling(false);
              setError(result.message || result.status);
            }
          }
        } catch (err) {
          const errorMessage = err instanceof Error ? err.message : String(err);
          if (pollingIntervalRef.current) {
            clearInterval(pollingIntervalRef.current);
            pollingIntervalRef.current = null;
          }
          setIsPolling(false);
          setError(errorMessage);
        }
      }, pollInterval * 1000);
    },
    [projectId, onSuccess, t]
  );

  const reset = useCallback(() => {
    if (pollingIntervalRef.current) {
      clearInterval(pollingIntervalRef.current);
      pollingIntervalRef.current = null;
    }
    setUserCode(null);
    setVerificationUri(null);
    setSessionId(null);
    setExpiresAt(null);
    setInterval(5);
    currentIntervalRef.current = 5;
    setIsPolling(false);
    setError(null);
    setIsComplete(false);
  }, []);

  return {
    userCode,
    verificationUri,
    sessionId,
    expiresAt,
    interval,
    isPolling,
    error,
    isComplete,
    start,
    reset,
  };
}
