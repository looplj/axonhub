'use client';

import React, { useState } from 'react';
import { Download, Upload, Loader2, AlertCircle, CheckCircle2 } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Label } from '@/components/ui/label';
import { Switch } from '@/components/ui/switch';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { useBackup, useRestore, BackupOptionsInput, RestoreOptionsInput } from '../data/system';

export function BackupSettings() {
  const { t } = useTranslation();
  const backup = useBackup();
  const restore = useRestore();

  const [backupOptions, setBackupOptions] = useState<BackupOptionsInput>({
    includeChannels: true,
    includeModels: true,
  });

  const [restoreOptions, setRestoreOptions] = useState<RestoreOptionsInput>({
    includeChannels: true,
    includeModels: true,
    channelConflictStrategy: 'SKIP',
    modelConflictStrategy: 'SKIP',
  });

  const [selectedFile, setSelectedFile] = useState<File | null>(null);

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
              <Label htmlFor="include-models">{t('system.backup.includeModels')}</Label>
              <Switch
                id="include-models"
                checked={backupOptions.includeModels}
                onCheckedChange={(checked) => setBackupOptions({ ...backupOptions, includeModels: checked })}
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
          <div className="space-y-4">
            <div className="flex items-center justify-between">
              <Label htmlFor="restore-include-channels">{t('system.backup.includeChannels')}</Label>
              <Switch
                id="restore-include-channels"
                checked={restoreOptions.includeChannels}
                onCheckedChange={(checked) => setRestoreOptions({ ...restoreOptions, includeChannels: checked })}
              />
            </div>
            <div className="flex items-center justify-between">
              <Label htmlFor="restore-include-models">{t('system.backup.includeModels')}</Label>
              <Switch
                id="restore-include-models"
                checked={restoreOptions.includeModels}
                onCheckedChange={(checked) => setRestoreOptions({ ...restoreOptions, includeModels: checked })}
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="channel-conflict-strategy">{t('system.restore.channelConflictStrategy')}</Label>
              <Select
                value={restoreOptions.channelConflictStrategy}
                onValueChange={(value: 'SKIP' | 'OVERWRITE' | 'ERROR') =>
                  setRestoreOptions({ ...restoreOptions, channelConflictStrategy: value })
                }
              >
                <SelectTrigger id="channel-conflict-strategy">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="SKIP">{t('system.restore.strategies.skip')}</SelectItem>
                  <SelectItem value="OVERWRITE">{t('system.restore.strategies.overwrite')}</SelectItem>
                  <SelectItem value="ERROR">{t('system.restore.strategies.error')}</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div className="space-y-2">
              <Label htmlFor="model-conflict-strategy">{t('system.restore.modelConflictStrategy')}</Label>
              <Select
                value={restoreOptions.modelConflictStrategy}
                onValueChange={(value: 'SKIP' | 'OVERWRITE' | 'ERROR') =>
                  setRestoreOptions({ ...restoreOptions, modelConflictStrategy: value })
                }
              >
                <SelectTrigger id="model-conflict-strategy">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="SKIP">{t('system.restore.strategies.skip')}</SelectItem>
                  <SelectItem value="OVERWRITE">{t('system.restore.strategies.overwrite')}</SelectItem>
                  <SelectItem value="ERROR">{t('system.restore.strategies.error')}</SelectItem>
                </SelectContent>
              </Select>
            </div>
          </div>
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
    </div>
  );
}
