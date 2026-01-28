'use client';

import React, { useState, useEffect } from 'react';
import { Download, Upload, Loader2, AlertCircle, CheckCircle2, Clock, Play, Link2 } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Label } from '@/components/ui/label';
import { Switch } from '@/components/ui/switch';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Input } from '@/components/ui/input';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import {
  useBackup,
  useRestore,
  useAutoBackupSettings,
  useUpdateAutoBackupSettings,
  useTestWebDAVConnection,
  useTriggerAutoBackup,
  BackupOptionsInput,
  RestoreOptionsInput,
  BackupFrequency,
} from '../data/system';

export function BackupSettings() {
  const { t } = useTranslation();
  const backup = useBackup();
  const restore = useRestore();
  const autoBackupSettings = useAutoBackupSettings();
  const updateAutoBackupSettings = useUpdateAutoBackupSettings();
  const testConnection = useTestWebDAVConnection();
  const triggerBackup = useTriggerAutoBackup();

  const [backupOptions, setBackupOptions] = useState<BackupOptionsInput>({
    includeChannels: true,
    includeModelPrices: true,
    includeModels: true,
    includeAPIKeys: false,
  });

  const [restoreOptions, setRestoreOptions] = useState<RestoreOptionsInput>({
    includeChannels: true,
    includeModelPrices: true,
    includeModels: true,
    includeAPIKeys: false,
    channelConflictStrategy: 'skip',
    modelConflictStrategy: 'skip',
    modelPriceConflictStrategy: 'skip',
    apiKeyConflictStrategy: 'skip',
  });

  const [selectedFile, setSelectedFile] = useState<File | null>(null);

  const [autoBackupForm, setAutoBackupForm] = useState({
    enabled: false,
    frequency: 'daily' as BackupFrequency,
    webdavUrl: '',
    webdavUsername: '',
    webdavPassword: '',
    webdavPath: '/backups/axonhub',
    insecureSkipTLS: false,
    includeChannels: true,
    includeModels: true,
    includeAPIKeys: false,
    includeModelPrices: true,
    retentionDays: 0,
  });

  const savedWebdavConfig = autoBackupSettings.data?.webdav;
  const canTriggerBackup =
    savedWebdavConfig?.url && savedWebdavConfig?.username && savedWebdavConfig?.password;

  useEffect(() => {
    if (autoBackupSettings.data) {
      setAutoBackupForm({
        enabled: autoBackupSettings.data.enabled,
        frequency: autoBackupSettings.data.frequency,
        webdavUrl: autoBackupSettings.data.webdav?.url || '',
        webdavUsername: autoBackupSettings.data.webdav?.username || '',
        webdavPassword: autoBackupSettings.data.webdav?.password || '',
        webdavPath: autoBackupSettings.data.webdav?.path || '/backups/axonhub',
        insecureSkipTLS: autoBackupSettings.data.webdav?.insecureSkipTLS || false,
        includeChannels: autoBackupSettings.data.includeChannels,
        includeModels: autoBackupSettings.data.includeModels,
        includeAPIKeys: autoBackupSettings.data.includeAPIKeys,
        includeModelPrices: autoBackupSettings.data.includeModelPrices,
        retentionDays: autoBackupSettings.data.retentionDays,
      });
    }
  }, [autoBackupSettings.data]);

  const handleBackup = () => {
    backup.mutate(backupOptions);
  };

  const handleFileChange = (event: React.ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0];
    if (file) {
      setSelectedFile(file);
    }
  };

  const handleRestore = () => {
    if (!selectedFile) return;
    restore.mutate({ file: selectedFile, input: restoreOptions });
  };

  const handleSaveAutoBackup = () => {
    updateAutoBackupSettings.mutate({
      enabled: autoBackupForm.enabled,
      frequency: autoBackupForm.frequency,
      webdav: {
        url: autoBackupForm.webdavUrl,
        username: autoBackupForm.webdavUsername,
        password: autoBackupForm.webdavPassword,
        insecureSkipTLS: autoBackupForm.insecureSkipTLS,
        path: autoBackupForm.webdavPath,
      },
      includeChannels: autoBackupForm.includeChannels,
      includeModels: autoBackupForm.includeModels,
      includeAPIKeys: autoBackupForm.includeAPIKeys,
      includeModelPrices: autoBackupForm.includeModelPrices,
      retentionDays: autoBackupForm.retentionDays,
    });
  };

  const handleTestConnection = () => {
    testConnection.mutate({
      url: autoBackupForm.webdavUrl,
      username: autoBackupForm.webdavUsername,
      password: autoBackupForm.webdavPassword,
      insecureSkipTLS: autoBackupForm.insecureSkipTLS,
      path: autoBackupForm.webdavPath,
    });
  };

  const handleTriggerBackup = () => {
    triggerBackup.mutate();
  };

  return (
    <div className="space-y-6">
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <Download className="h-5 w-5" />
            {t('system.backup.title')}
          </CardTitle>
          <CardDescription>{t('system.backup.description')}</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="space-y-4">
            <div className="flex items-center justify-between">
              <Label htmlFor="include-channels">{t('system.backup.includeChannels')}</Label>
              <Switch
                id="include-channels"
                checked={backupOptions.includeChannels}
                onCheckedChange={(checked) => setBackupOptions({ ...backupOptions, includeChannels: checked })}
              />
            </div>
            <div className="flex items-center justify-between">
              <Label htmlFor="include-model-prices">{t('system.backup.includeModelPrices')}</Label>
              <Switch
                id="include-model-prices"
                checked={backupOptions.includeModelPrices}
                onCheckedChange={(checked) => setBackupOptions({ ...backupOptions, includeModelPrices: checked })}
              />
            </div>
            <div className="flex items-center justify-between">
              <Label htmlFor="include-models">{t('system.backup.includeModels')}</Label>
              <Switch
                id="include-models"
                checked={backupOptions.includeModels}
                onCheckedChange={(checked) => setBackupOptions({ ...backupOptions, includeModels: checked })}
              />
            </div>
            <div className="flex items-center justify-between">
              <Label htmlFor="include-apikeys">{t('system.backup.includeAPIKeys')}</Label>
              <Switch
                id="include-apikeys"
                checked={backupOptions.includeAPIKeys}
                onCheckedChange={(checked) => setBackupOptions({ ...backupOptions, includeAPIKeys: checked })}
              />
            </div>
          </div>
          <Button onClick={handleBackup} disabled={backup.isPending} className="w-full">
            {backup.isPending ? (
              <>
                <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                {t('system.backup.backingUp')}
              </>
            ) : (
              <>
                <Download className="mr-2 h-4 w-4" />
                {t('system.backup.createBackup')}
              </>
            )}
          </Button>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <Upload className="h-5 w-5" />
            {t('system.restore.title')}
          </CardTitle>
          <CardDescription>{t('system.restore.description')}</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="space-y-2">
            <Label htmlFor="backup-file">{t('system.restore.selectFile')}</Label>
            <input
              id="backup-file"
              type="file"
              accept=".json"
              onChange={handleFileChange}
              className="flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background file:border-0 file:bg-transparent file:text-sm file:font-medium placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50"
            />
            {selectedFile && (
              <div className="flex items-center gap-2 text-sm text-muted-foreground">
                <CheckCircle2 className="h-4 w-4 text-green-500" />
                {selectedFile.name}
              </div>
            )}
          </div>
          <div className="space-y-4">
            <div className="flex items-center gap-4">
              <div className="flex flex-1 items-center justify-between">
                <Label htmlFor="restore-include-channels">{t('system.backup.includeChannels')}</Label>
                <Switch
                  id="restore-include-channels"
                  checked={restoreOptions.includeChannels}
                  onCheckedChange={(checked) => setRestoreOptions({ ...restoreOptions, includeChannels: checked })}
                  disabled={!selectedFile}
                />
              </div>
              <Select
                value={restoreOptions.channelConflictStrategy}
                onValueChange={(value: 'skip' | 'overwrite' | 'error') =>
                  setRestoreOptions({ ...restoreOptions, channelConflictStrategy: value })
                }
                disabled={!selectedFile || !restoreOptions.includeChannels}
              >
                <SelectTrigger id="channel-conflict-strategy" className="w-32">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="skip">{t('system.restore.strategies.skip')}</SelectItem>
                  <SelectItem value="overwrite">{t('system.restore.strategies.overwrite')}</SelectItem>
                  <SelectItem value="error">{t('system.restore.strategies.error')}</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div className="flex items-center gap-4">
              <div className="flex flex-1 items-center justify-between">
                <Label htmlFor="restore-include-models">{t('system.backup.includeModels')}</Label>
                <Switch
                  id="restore-include-models"
                  checked={restoreOptions.includeModels}
                  onCheckedChange={(checked) => setRestoreOptions({ ...restoreOptions, includeModels: checked })}
                  disabled={!selectedFile}
                />
              </div>
              <Select
                value={restoreOptions.modelConflictStrategy}
                onValueChange={(value: 'skip' | 'overwrite' | 'error') =>
                  setRestoreOptions({ ...restoreOptions, modelConflictStrategy: value })
                }
                disabled={!selectedFile || !restoreOptions.includeModels}
              >
                <SelectTrigger id="model-conflict-strategy" className="w-32">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="skip">{t('system.restore.strategies.skip')}</SelectItem>
                  <SelectItem value="overwrite">{t('system.restore.strategies.overwrite')}</SelectItem>
                  <SelectItem value="error">{t('system.restore.strategies.error')}</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div className="flex items-center gap-4">
              <div className="flex flex-1 items-center justify-between">
                <Label htmlFor="restore-include-apikeys">{t('system.backup.includeAPIKeys')}</Label>
                <Switch
                  id="restore-include-apikeys"
                  checked={restoreOptions.includeAPIKeys}
                  onCheckedChange={(checked) => setRestoreOptions({ ...restoreOptions, includeAPIKeys: checked })}
                  disabled={!selectedFile}
                />
              </div>
              <Select
                value={restoreOptions.apiKeyConflictStrategy}
                onValueChange={(value: 'skip' | 'overwrite' | 'error') =>
                  setRestoreOptions({ ...restoreOptions, apiKeyConflictStrategy: value })
                }
                disabled={!selectedFile || !restoreOptions.includeAPIKeys}
              >
                <SelectTrigger id="apikey-conflict-strategy" className="w-32">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="skip">{t('system.restore.strategies.skip')}</SelectItem>
                  <SelectItem value="overwrite">{t('system.restore.strategies.overwrite')}</SelectItem>
                  <SelectItem value="error">{t('system.restore.strategies.error')}</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div className="flex items-center gap-4">
              <div className="flex flex-1 items-center justify-between">
                <Label htmlFor="restore-include-model-prices">{t('system.backup.includeModelPrices')}</Label>
                <Switch
                  id="restore-include-model-prices"
                  checked={restoreOptions.includeModelPrices}
                  onCheckedChange={(checked) => setRestoreOptions({ ...restoreOptions, includeModelPrices: checked })}
                  disabled={!selectedFile}
                />
              </div>
              <Select
                value={restoreOptions.modelPriceConflictStrategy}
                onValueChange={(value: 'skip' | 'overwrite' | 'error') =>
                  setRestoreOptions({ ...restoreOptions, modelPriceConflictStrategy: value })
                }
                disabled={!selectedFile || !restoreOptions.includeModelPrices}
              >
                <SelectTrigger id="model-price-conflict-strategy" className="w-32">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="skip">{t('system.restore.strategies.skip')}</SelectItem>
                  <SelectItem value="overwrite">{t('system.restore.strategies.overwrite')}</SelectItem>
                  <SelectItem value="error">{t('system.restore.strategies.error')}</SelectItem>
                </SelectContent>
              </Select>
            </div>
          </div>
          <Button
            onClick={handleRestore}
            disabled={restore.isPending || !selectedFile}
            className="w-full"
            variant="destructive"
          >
            {restore.isPending ? (
              <>
                <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                {t('system.restore.restoring')}
              </>
            ) : (
              <>
                <Upload className="mr-2 h-4 w-4" />
                {t('system.restore.restoreBackup')}
              </>
            )}
          </Button>
          <div className="flex items-start gap-2 rounded-md bg-yellow-50 p-3 text-sm text-yellow-800 dark:bg-yellow-900/20 dark:text-yellow-200">
            <AlertCircle className="mt-0.5 h-4 w-4 flex-shrink-0" />
            <p>{t('system.restore.warning')}</p>
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <Clock className="h-5 w-5" />
            {t('system.autoBackup.title')}
          </CardTitle>
          <CardDescription>{t('system.autoBackup.description')}</CardDescription>
        </CardHeader>
        <CardContent className="space-y-6">
          <div className="flex items-center justify-between">
            <div className="space-y-0.5">
              <Label htmlFor="auto-backup-enabled">{t('system.autoBackup.enabled.label')}</Label>
              <p className="text-sm text-muted-foreground">{t('system.autoBackup.enabled.description')}</p>
            </div>
            <Switch
              id="auto-backup-enabled"
              checked={autoBackupForm.enabled}
              onCheckedChange={(checked) => setAutoBackupForm({ ...autoBackupForm, enabled: checked })}
            />
          </div>

          <div className="space-y-2">
            <Label htmlFor="backup-frequency">{t('system.autoBackup.frequency.label')}</Label>
            <Select
              value={autoBackupForm.frequency}
              onValueChange={(value: BackupFrequency) => setAutoBackupForm({ ...autoBackupForm, frequency: value })}
            >
              <SelectTrigger id="backup-frequency">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="daily">{t('system.autoBackup.frequency.daily')}</SelectItem>
                <SelectItem value="weekly">{t('system.autoBackup.frequency.weekly')}</SelectItem>
                <SelectItem value="monthly">{t('system.autoBackup.frequency.monthly')}</SelectItem>
              </SelectContent>
            </Select>
          </div>

          <div className="space-y-4 rounded-md border p-4">
            <div className="flex items-center gap-2">
              <Link2 className="h-4 w-4" />
              <Label className="text-base font-medium">{t('system.autoBackup.webdav.title')}</Label>
            </div>

            <div className="space-y-2">
              <Label htmlFor="webdav-url">{t('system.autoBackup.webdav.url')}</Label>
              <Input
                id="webdav-url"
                type="url"
                placeholder={t('system.autoBackup.webdav.urlPlaceholder')}
                value={autoBackupForm.webdavUrl}
                onChange={(e) => setAutoBackupForm({ ...autoBackupForm, webdavUrl: e.target.value })}
              />
            </div>

            <div className="space-y-2">
              <Label htmlFor="webdav-username">{t('system.autoBackup.webdav.username')}</Label>
              <Input
                id="webdav-username"
                type="text"
                placeholder={t('system.autoBackup.webdav.usernamePlaceholder')}
                value={autoBackupForm.webdavUsername}
                onChange={(e) => setAutoBackupForm({ ...autoBackupForm, webdavUsername: e.target.value })}
              />
            </div>

            <div className="space-y-2">
              <Label htmlFor="webdav-password">{t('system.autoBackup.webdav.password')}</Label>
              <Input
                id="webdav-password"
                type="password"
                placeholder={t('system.autoBackup.webdav.passwordPlaceholder')}
                value={autoBackupForm.webdavPassword}
                onChange={(e) => setAutoBackupForm({ ...autoBackupForm, webdavPassword: e.target.value })}
              />
            </div>

            <div className="space-y-2">
              <Label htmlFor="webdav-path">{t('system.autoBackup.webdav.path')}</Label>
              <Input
                id="webdav-path"
                type="text"
                placeholder={t('system.autoBackup.webdav.pathPlaceholder')}
                value={autoBackupForm.webdavPath}
                onChange={(e) => setAutoBackupForm({ ...autoBackupForm, webdavPath: e.target.value })}
              />
              <p className="text-sm text-muted-foreground">{t('system.autoBackup.webdav.pathDescription')}</p>
            </div>

            <div className="flex items-center justify-between">
              <div className="space-y-0.5">
                <Label htmlFor="insecure-skip-tls">{t('system.autoBackup.webdav.insecureSkipTLS')}</Label>
                <p className="text-sm text-muted-foreground">{t('system.autoBackup.webdav.insecureSkipTLSDescription')}</p>
              </div>
              <Switch
                id="insecure-skip-tls"
                checked={autoBackupForm.insecureSkipTLS}
                onCheckedChange={(checked) => setAutoBackupForm({ ...autoBackupForm, insecureSkipTLS: checked })}
              />
            </div>

            <Button
              variant="outline"
              onClick={handleTestConnection}
              disabled={testConnection.isPending || !autoBackupForm.webdavUrl}
              className="w-full"
            >
              {testConnection.isPending ? (
                <>
                  <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                  {t('system.autoBackup.testingConnection')}
                </>
              ) : (
                <>
                  <Link2 className="mr-2 h-4 w-4" />
                  {t('system.autoBackup.testConnection')}
                </>
              )}
            </Button>
          </div>

          <div className="space-y-4">
            <Label className="text-base font-medium">{t('system.autoBackup.options.title')}</Label>
            <div className="flex items-center justify-between">
              <Label htmlFor="auto-include-channels">{t('system.backup.includeChannels')}</Label>
              <Switch
                id="auto-include-channels"
                checked={autoBackupForm.includeChannels}
                onCheckedChange={(checked) => setAutoBackupForm({ ...autoBackupForm, includeChannels: checked })}
              />
            </div>
            <div className="flex items-center justify-between">
              <Label htmlFor="auto-include-models">{t('system.backup.includeModels')}</Label>
              <Switch
                id="auto-include-models"
                checked={autoBackupForm.includeModels}
                onCheckedChange={(checked) => setAutoBackupForm({ ...autoBackupForm, includeModels: checked })}
              />
            </div>
            <div className="flex items-center justify-between">
              <Label htmlFor="auto-include-apikeys">{t('system.backup.includeAPIKeys')}</Label>
              <Switch
                id="auto-include-apikeys"
                checked={autoBackupForm.includeAPIKeys}
                onCheckedChange={(checked) => setAutoBackupForm({ ...autoBackupForm, includeAPIKeys: checked })}
              />
            </div>
            <div className="flex items-center justify-between">
              <Label htmlFor="auto-include-model-prices">{t('system.backup.includeModelPrices')}</Label>
              <Switch
                id="auto-include-model-prices"
                checked={autoBackupForm.includeModelPrices}
                onCheckedChange={(checked) => setAutoBackupForm({ ...autoBackupForm, includeModelPrices: checked })}
              />
            </div>
          </div>

          <div className="space-y-2">
            <Label htmlFor="retention-days">{t('system.autoBackup.retentionDays')}</Label>
            <Input
              id="retention-days"
              type="number"
              min={0}
              max={365}
              value={autoBackupForm.retentionDays}
              onChange={(e) => setAutoBackupForm({ ...autoBackupForm, retentionDays: parseInt(e.target.value) || 0 })}
            />
            <p className="text-sm text-muted-foreground">{t('system.autoBackup.retentionDaysDescription')}</p>
          </div>

          {autoBackupSettings.data?.lastBackupAt && (
            <div className="rounded-md bg-muted p-3 text-sm">
              <div className="flex items-center gap-2">
                <CheckCircle2 className="h-4 w-4 text-green-500" />
                <span>
                  {t('system.autoBackup.lastBackup.time')}: {new Date(autoBackupSettings.data.lastBackupAt).toLocaleString()}
                </span>
              </div>
            </div>
          )}

          {autoBackupSettings.data?.lastBackupError && (
            <div className="flex items-start gap-2 rounded-md bg-red-50 p-3 text-sm text-red-800 dark:bg-red-900/20 dark:text-red-200">
              <AlertCircle className="mt-0.5 h-4 w-4 flex-shrink-0" />
              <p>{autoBackupSettings.data.lastBackupError}</p>
            </div>
          )}

          <div className="flex gap-2">
            <Button
              onClick={handleSaveAutoBackup}
              disabled={updateAutoBackupSettings.isPending}
              className="flex-1"
            >
              {updateAutoBackupSettings.isPending ? (
                <>
                  <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                  {t('system.buttons.saving')}
                </>
              ) : (
                t('system.buttons.save')
              )}
            </Button>
            <Tooltip>
              <TooltipTrigger asChild>
                <span>
                  <Button
                    variant="outline"
                    onClick={handleTriggerBackup}
                    disabled={triggerBackup.isPending || !canTriggerBackup}
                  >
                    {triggerBackup.isPending ? (
                      <>
                        <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                        {t('system.autoBackup.triggeringBackup')}
                      </>
                    ) : (
                      <>
                        <Play className="mr-2 h-4 w-4" />
                        {t('system.autoBackup.triggerNow')}
                      </>
                    )}
                  </Button>
                </span>
              </TooltipTrigger>
              {!canTriggerBackup && (
                <TooltipContent>
                  <p>{t('system.autoBackup.triggerNowTooltip')}</p>
                </TooltipContent>
              )}
            </Tooltip>
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
