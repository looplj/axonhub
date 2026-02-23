'use client';

import { useEffect } from 'react';
import { Copy, ExternalLink, Loader2, CheckCircle2, AlertCircle, RefreshCw } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';
import { useSelectedProjectId } from '@/stores/projectStore';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { useDeviceFlow } from '../hooks/use-device-flow';

interface CopilotDeviceFlowProps {
  onSuccess: (credentials: string) => void;
  onError?: (error: string) => void;
}

export function CopilotDeviceFlow({ onSuccess, onError }: CopilotDeviceFlowProps) {
  const { t } = useTranslation();
  const projectId = useSelectedProjectId();

  const deviceFlow = useDeviceFlow({
    projectId,
    onSuccess,
  });

  useEffect(() => {
    if (deviceFlow.error && onError) {
      onError(deviceFlow.error);
    }
  }, [deviceFlow.error, onError]);

  const handleCopyCode = async () => {
    if (deviceFlow.userCode) {
      try {
        await navigator.clipboard.writeText(deviceFlow.userCode);
        toast.success(t('channels.messages.credentialsCopied'));
      } catch (err) {
        toast.error('Failed to copy code');
      }
    }
  };

  const handleOpenGitHub = () => {
    if (deviceFlow.verificationUri) {
      window.open(deviceFlow.verificationUri, '_blank', 'noopener,noreferrer');
    }
  };

  const handleReset = () => {
    deviceFlow.reset();
  };

  if (!deviceFlow.userCode && !deviceFlow.isPolling && !deviceFlow.error) {
    return (
      <Card className="border-2 border-dashed">
        <CardHeader className="text-center pb-4">
          <CardTitle className="text-lg">{t('channels.dialogs.github_copilot.buttons.startOAuth')}</CardTitle>
          <CardDescription>
            Connect your GitHub account to enable Copilot access
          </CardDescription>
        </CardHeader>
        <CardContent className="flex justify-center">
          <Button
            onClick={deviceFlow.start}
            disabled={!projectId || deviceFlow.isPolling}
            size="lg"
            className="min-w-[200px]"
          >
            {t('channels.dialogs.github_copilot.buttons.startOAuth')}
          </Button>
        </CardContent>
      </Card>
    );
  }

  if (deviceFlow.error) {
    return (
      <Card className="border-destructive">
        <CardHeader>
          <CardTitle className="flex items-center gap-2 text-destructive">
            <AlertCircle className="h-5 w-5" />
            {t('common.error')}
          </CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          <p className="text-sm text-muted-foreground">{deviceFlow.error}</p>
          <div className="flex gap-2">
            <Button onClick={handleReset} variant="outline" className="flex-1">
              <RefreshCw className="mr-2 h-4 w-4" />
              {t('common.buttons.retry')}
            </Button>
          </div>
        </CardContent>
      </Card>
    );
  }

  if (deviceFlow.isComplete) {
    return (
      <Card className="border-green-500">
        <CardHeader>
          <CardTitle className="flex items-center gap-2 text-green-600">
            <CheckCircle2 className="h-5 w-5" />
            {t('channels.dialogs.github_copilot.messages.authSuccess')}
          </CardTitle>
        </CardHeader>
        <CardContent>
          <p className="text-sm text-muted-foreground">
            {t('channels.dialogs.github_copilot.messages.credentialsImported')}
          </p>
        </CardContent>
      </Card>
    );
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-2">
          {deviceFlow.isPolling && <Loader2 className="h-5 w-5 animate-spin" />}
          {t('channels.dialogs.github_copilot.messages.waitingForAuth')}
        </CardTitle>
        <CardDescription>
          Enter the code below on GitHub to authorize
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-6">
        <div className="space-y-2">
          <label className="text-sm font-medium">
            {t('channels.dialogs.github_copilot.labels.userCode')}
          </label>
          <div className="flex items-center gap-2">
            <div className="flex-1 bg-muted p-4 rounded-md text-center">
              <span className="text-3xl font-mono font-bold tracking-wider">
                {deviceFlow.userCode}
              </span>
            </div>
            <Button
              type="button"
              onClick={handleCopyCode}
              variant="outline"
              size="icon"
              title="Copy code"
            >
              <Copy className="h-4 w-4" />
            </Button>
          </div>
        </div>

        <div className="space-y-2">
          <Button
            type="button"
            onClick={handleOpenGitHub}
            className="w-full"
            size="lg"
          >
            <ExternalLink className="mr-2 h-4 w-4" />
            {t('channels.dialogs.github_copilot.buttons.openGitHub')}
          </Button>
          <p className="text-xs text-center text-muted-foreground">
            {deviceFlow.verificationUri}
          </p>
        </div>

        <div className="bg-muted/50 p-4 rounded-md space-y-2">
          <ol className="text-sm space-y-2 list-decimal list-inside text-muted-foreground">
            <li>Click the button above to open GitHub</li>
            <li>Enter the code shown above</li>
            <li>Authorize the application</li>
            <li>This page will update automatically</li>
          </ol>
        </div>

        <Button
          type="button"
          onClick={handleReset}
          variant="ghost"
          size="sm"
          className="w-full"
        >
          <RefreshCw className="mr-2 h-4 w-4" />
          {t('common.buttons.retry')}
        </Button>
      </CardContent>
    </Card>
  );
}
