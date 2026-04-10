'use client';

import { useEffect } from 'react';
import { Copy, ExternalLink, Loader2, CheckCircle2, AlertCircle, RefreshCw } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';
import { Button } from '@/components/ui/button';
import { FormLabel } from '@/components/ui/form';
import { useDeviceFlow } from '../hooks/use-device-flow';

interface CopilotDeviceFlowProps {
  onSuccess: (credentials: string) => void;
  onError?: (error: string) => void;
  existingCredentials?: string;
}

export function CopilotDeviceFlow({ onSuccess, onError, existingCredentials }: CopilotDeviceFlowProps) {
  const { t } = useTranslation();
  const hasExistingCredentials = existingCredentials && existingCredentials.trim().length > 0;
  const deviceFlow = useDeviceFlow({
    onSuccess,
  });
  useEffect(() => {
    if (deviceFlow.error && onError) {
      onError(deviceFlow.error);
    }
  }, [deviceFlow.error, onError]);

  const handleOpenGitHub = () => {
    if (deviceFlow.verificationUri) {
      window.open(deviceFlow.verificationUri, '_blank', 'noopener,noreferrer');
    }
  };

  const handleReset = () => {
    deviceFlow.reset();
  };

  const handleReconnect = () => {
    deviceFlow.reset();
    deviceFlow.start();
  };

  // Show already authenticated state
  if (hasExistingCredentials && !deviceFlow.userCode && !deviceFlow.isComplete && !deviceFlow.error) {
    return (
      <div className='mt-3 space-y-2'>
        <div className='rounded-md border border-green-500/50 bg-green-50/10 p-3'>
          <div className='flex items-center gap-2 text-green-600'>
            <CheckCircle2 className='h-5 w-5' />
            <span className='font-medium'>
              {t('channels.dialogs.github_copilot.messages.alreadyConnected')}
            </span>
          </div>
          <p className='text-muted-foreground mt-1 text-xs'>
            {t('channels.dialogs.github_copilot.messages.credentialsStored')}
          </p>
          <Button
            type='button'
            onClick={handleReconnect}
            variant='outline'
            size='sm'
            className='mt-2'
          >
            <RefreshCw className='mr-2 h-4 w-4' />
            {t('channels.dialogs.github_copilot.buttons.reauthenticate')}
          </Button>
        </div>
      </div>
    );
  }

  // Show completed state
  if (deviceFlow.isComplete) {
    return (
      <div className='mt-3 space-y-2'>
        <div className='rounded-md border border-green-500 p-3'>
          <div className='flex items-center gap-2 text-green-600'>
            <CheckCircle2 className='h-5 w-5' />
            <span className='font-medium'>
              {t('channels.dialogs.github_copilot.messages.authSuccess')}
            </span>
          </div>
          <p className='text-muted-foreground mt-1 text-xs'>
            {t('channels.dialogs.github_copilot.messages.credentialsImported')}
          </p>
        </div>
      </div>
    );
  }

  // Show error state with retry
  if (deviceFlow.error) {
    return (
      <div className='mt-3 space-y-2'>
        <div className='rounded-md border border-destructive p-3'>
          <div className='flex items-center gap-2 text-destructive'>
            <AlertCircle className='h-5 w-5' />
            <span className='font-medium'>{t('common.error')}</span>
          </div>
          <p className='text-muted-foreground mt-1 text-xs'>{deviceFlow.error}</p>
          <Button
            type='button'
            onClick={handleReset}
            variant='outline'
            size='sm'
            className='mt-2'
          >
            <RefreshCw className='mr-2 h-4 w-4' />
            {t('common.buttons.retry')}
          </Button>
        </div>
      </div>
    );
  }

  // Show waiting state with user code
  if (deviceFlow.userCode) {
    return (
      <div className='mt-3 space-y-2'>
        <div className='rounded-md border p-3'>
          <div className='flex items-center gap-2'>
            {deviceFlow.isPolling && <Loader2 className='h-5 w-5 animate-spin' />}
            <span className='font-medium'>
              {t('channels.dialogs.github_copilot.messages.waitingForAuth')}
            </span>
          </div>

          <div className='mt-3 space-y-2'>
            <FormLabel className='text-sm font-medium'>
              {t('channels.dialogs.github_copilot.labels.userCode')}
            </FormLabel>
            <div className='flex items-center gap-2'>
              <div className='flex-1 bg-muted p-3 rounded-md text-center'>
                <span className='text-2xl font-mono font-bold tracking-wider'>
                  {deviceFlow.userCode}
                </span>
              </div>

              <Button
                type='button'
                onClick={() => {
                  if (deviceFlow.userCode) {
                    navigator.clipboard.writeText(deviceFlow.userCode);
                    toast.success(t('channels.messages.credentialsCopied'));
                  }
                }}
                variant='outline'
                size='icon'
                title={t('copilot_device.copy_code')}
              >
                <Copy className='h-4 w-4' />
              </Button>
            </div>
          </div>

          <div className='mt-3 space-y-2'>
            <Button
              type='button'
              variant='secondary'
              onClick={handleOpenGitHub}
              className='w-full'
            >
              <ExternalLink className='mr-2 h-4 w-4' />
              {t('channels.dialogs.github_copilot.buttons.openGitHub')}
            </Button>
            <p className='text-xs text-center text-muted-foreground'>
              {deviceFlow.verificationUri}
            </p>
          </div>

          <div className='bg-muted/50 p-3 rounded-md mt-3'>
            <ol className='text-sm space-y-1 list-decimal list-inside text-muted-foreground'>
              <li>{t('copilot_device.step_1')}</li>
              <li>{t('copilot_device.step_2')}</li>
              <li>{t('copilot_device.step_3')}</li>
              <li>{t('copilot_device.step_4')}</li>
            </ol>
          </div>


          <Button
            type='button'
            onClick={handleReset}
            variant='ghost'
            size='sm'
            className='mt-2 w-full'
          >
            <RefreshCw className='mr-2 h-4 w-4' />
            {t('common.buttons.retry')}
          </Button>
        </div>
      </div>
    );
  }

  // Initial state - Start OAuth button
  return (
    <div className='mt-3 space-y-2'>
      <div className='rounded-md border p-3'>
        <Button
          type='button'
          variant='secondary'
          onClick={deviceFlow.start}
          disabled={deviceFlow.isPolling}
        >
          {deviceFlow.isPolling
            ? t('channels.dialogs.oauth.buttons.starting')
            : t('channels.dialogs.github_copilot.buttons.startOAuth')}
        </Button>
      </div>
    </div>
  );
}
