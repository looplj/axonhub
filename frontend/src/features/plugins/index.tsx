'use client';

import { useRef, useState } from 'react';
import { IconUpload } from '@tabler/icons-react';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';
import { usePermissions } from '@/hooks/usePermissions';
import { Main } from '@/components/layout/main';
import { PageHeader } from '@/components/layout/page-header';
import { Button } from '@/components/ui/button';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { PluginDetailSheet } from './components/plugin-detail-sheet';
import { PluginGrantDialog } from './components/plugin-grant-dialog';
import { PluginInstallDialog } from './components/plugin-install-dialog';
import { PluginsTable } from './components/plugins-table';
import {
  useDeletePlugin,
  useDisablePlugin,
  useEnablePlugin,
  useInstallPlugin,
  usePlugins,
  type Plugin,
  type PluginUpgradePreview,
} from './data/plugins';

interface GrantState {
  mode: 'install' | 'upgrade';
  plugin: Plugin;
  preview: PluginUpgradePreview | null;
}

export default function PluginsManagement() {
  const { t } = useTranslation();
  const { isOwner, hasSystemScope } = usePermissions();
  const canWrite = hasSystemScope('write_plugins');
  const pluginsQuery = usePlugins();
  const installPlugin = useInstallPlugin();
  const enablePlugin = useEnablePlugin();
  const disablePlugin = useDisablePlugin();
  const deletePlugin = useDeletePlugin();
  const [installOpen, setInstallOpen] = useState(false);
  const [selectedFile, setSelectedFile] = useState<File | null>(null);
  const pendingFile = useRef<File | null>(null);
  const [grant, setGrant] = useState<GrantState | null>(null);
  const [pendingDelete, setPendingDelete] = useState<Plugin | null>(null);
  const [detailId, setDetailId] = useState<string | null>(null);

  const plugins = pluginsQuery.data?.plugins ?? [];
  // Resolved from the query on every render so the sheet shows fresh data after a save.
  const detailPlugin = plugins.find((plugin) => plugin.id === detailId) ?? null;

  const resetInstall = () => {
    pendingFile.current = null;
    setSelectedFile(null);
  };

  const upload = async () => {
    if (!selectedFile) {
      return;
    }
    pendingFile.current = selectedFile;
    const known = new Set(plugins.map((plugin) => plugin.pluginID));
    try {
      const result = await installPlugin.mutateAsync({ file: selectedFile, grantedPermissions: null, grantedNetwork: null });
      if (result.upgradePreview && result.plugin) {
        setInstallOpen(false);
        setGrant({ mode: 'upgrade', plugin: result.plugin, preview: result.upgradePreview });
        return;
      }
      if (result.plugin && !known.has(result.plugin.pluginID) && !result.plugin.enabled) {
        setInstallOpen(false);
        setGrant({ mode: 'install', plugin: result.plugin, preview: null });
        return;
      }
      toast.success(t('plugins.upload.success'));
      setInstallOpen(false);
      resetInstall();
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t('plugins.upload.failed'));
    }
  };

  const confirmGrant = async (grantedPermissions: string[], grantedNetwork: string[]) => {
    if (!grant) {
      return;
    }
    try {
      if (grant.mode === 'upgrade') {
        const file = pendingFile.current;
        if (!file) {
          return;
        }
        const result = await installPlugin.mutateAsync({ file, grantedPermissions, grantedNetwork });
        if (result.upgradePreview && result.plugin) {
          setGrant({ mode: 'upgrade', plugin: result.plugin, preview: result.upgradePreview });
          return;
        }
        toast.success(t('plugins.upload.success'));
      } else {
        await enablePlugin.mutateAsync({ id: grant.plugin.id, grantedPermissions, grantedNetwork });
      }
      setGrant(null);
      resetInstall();
    } catch (error) {
      if (grant.mode === 'upgrade') {
        toast.error(error instanceof Error ? error.message : t('plugins.actions.failed'));
      }
    }
  };

  return (
    <>
      <PageHeader
        title={t('plugins.page.title')}
        description={t('plugins.page.description')}
        actions={
          isOwner ? (
            <Button type='button' onClick={() => setInstallOpen(true)}>
              <IconUpload className='size-4' />
              {t('plugins.upload.title')}
            </Button>
          ) : undefined
        }
      />

      <Main>
        <div className='space-y-4'>
          {pluginsQuery.data?.pluginCountersArePerInstance && (
            <p className='text-muted-foreground text-sm'>{t('plugins.countersPerInstance')}</p>
          )}
          {pluginsQuery.isLoading && <p className='text-muted-foreground text-sm'>{t('plugins.loading')}</p>}
          {pluginsQuery.isError && (
            <p className='text-destructive text-sm'>
              {pluginsQuery.error instanceof Error ? pluginsQuery.error.message : t('plugins.loadFailed')}
            </p>
          )}
          {pluginsQuery.isSuccess && plugins.length === 0 && (
            <div className='text-muted-foreground rounded-2xl border border-dashed p-10 text-center text-sm'>{t('plugins.empty')}</div>
          )}
          {plugins.length > 0 && (
            <PluginsTable
              plugins={plugins}
              canManage={isOwner}
              togglePending={disablePlugin.isPending}
              onConfigure={(plugin) => setDetailId(plugin.id)}
              onEnable={(plugin) => setGrant({ mode: 'install', plugin, preview: null })}
              onDisable={(plugin) => disablePlugin.mutate(plugin.id)}
              onDelete={(plugin) => setPendingDelete(plugin)}
            />
          )}
        </div>
      </Main>

      <PluginDetailSheet plugin={detailPlugin} canWrite={canWrite} onOpenChange={(open) => !open && setDetailId(null)} />

      <PluginInstallDialog
        open={installOpen}
        onOpenChange={(open) => {
          setInstallOpen(open);
          if (!open && !grant) {
            resetInstall();
          }
        }}
        file={selectedFile}
        onFileChange={setSelectedFile}
        pending={installPlugin.isPending}
        onSubmit={upload}
      />

      <PluginGrantDialog
        open={grant !== null}
        onOpenChange={(open) => {
          if (!open) {
            setGrant(null);
            resetInstall();
          }
        }}
        mode={grant?.mode ?? 'install'}
        permissions={grant?.plugin.manifestPermissions ?? []}
        network={grant?.plugin.manifestNetwork ?? []}
        initialGranted={grant?.plugin.grantedPermissions ?? []}
        initialGrantedNetwork={grant?.plugin.grantedNetwork ?? []}
        preview={grant?.preview}
        pending={installPlugin.isPending || enablePlugin.isPending}
        onConfirm={confirmGrant}
      />

      <Dialog open={pendingDelete !== null} onOpenChange={(open) => !open && setPendingDelete(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t('plugins.deleteTitle')}</DialogTitle>
            <DialogDescription>
              {t('plugins.deleteDescription', { name: pendingDelete?.name || pendingDelete?.pluginID })}
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button type='button' variant='outline' onClick={() => setPendingDelete(null)}>
              {t('plugins.cancel')}
            </Button>
            <Button
              type='button'
              variant='destructive'
              disabled={deletePlugin.isPending}
              onClick={() => {
                if (!pendingDelete) {
                  return;
                }
                deletePlugin.mutate(pendingDelete.id, { onSuccess: () => setPendingDelete(null) });
              }}
            >
              {t('plugins.deleteConfirm')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  );
}
