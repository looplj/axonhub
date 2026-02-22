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
  deviceCode: string | null;
  expiresAt: number | null;
  interval: number;
  isPolling: boolean;
  error: string | null;
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
  const [deviceCode, setDeviceCode] = useState<string | null>(null);
  const [expiresAt, setExpiresAt] = useState<number | null>(null);
  const [interval, setInterval] = useState(5);
  const [isPolling, setIsPolling] = useState(false);
  const [error, setError] = useState<string | null>(null);

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
      setDeviceCode(result.device_code);
      setExpiresAt(Date.now() + result.expires_in * 1000);
      setInterval(result.interval);
      currentIntervalRef.current = result.interval;

      poll(result.device_code, result.interval, Date.now() + result.expires_in * 1000);
    } catch (err) {
      const errorMessage = err instanceof Error ? err.message : String(err);
      setError(errorMessage);
      toast.error(errorMessage);
      setIsPolling(false);
    }
  }, [projectId, t]);

  const poll = useCallback(
    async (code: string, pollInterval: number, expiry: number) => {
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
          toast.error(t('channels.dialogs.oauth.errors.deviceFlowExpired'));
          return;
        }

        try {
          const result: DeviceFlowPollResult = await copilotOAuthPoll(
            { device_code: code },
            { 'X-Project-ID': projectId }
          );

          if (result.access_token) {
            if (pollingIntervalRef.current) {
              clearInterval(pollingIntervalRef.current);
              pollingIntervalRef.current = null;
            }
            setIsPolling(false);

            if (onSuccess) {
              onSuccess(result.access_token);
            }

            toast.success(t('channels.dialogs.oauth.messages.credentialsImported'));
          } else if (result.error) {
            if (result.error === 'authorization_pending') {
              return;
            } else if (result.error === 'slow_down') {
              const newInterval = currentIntervalRef.current * 2;
              currentIntervalRef.current = newInterval;
              setInterval(newInterval);

              if (pollingIntervalRef.current) {
                clearInterval(pollingIntervalRef.current);
              }
              // @ts-expect-error - setInterval return type mismatch between browser and Node.js types
              pollingIntervalRef.current = window.setInterval(() => {
                poll(code, newInterval, expiry);
              }, newInterval * 1000);
            } else {
              if (pollingIntervalRef.current) {
                clearInterval(pollingIntervalRef.current);
                pollingIntervalRef.current = null;
              }
              setIsPolling(false);
              setError(result.error);
              toast.error(result.error);
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
          toast.error(errorMessage);
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
    setDeviceCode(null);
    setExpiresAt(null);
    setInterval(5);
    currentIntervalRef.current = 5;
    setIsPolling(false);
    setError(null);
  }, []);

  return {
    userCode,
    verificationUri,
    deviceCode,
    expiresAt,
    interval,
    isPolling,
    error,
    start,
    reset,
  };
}
