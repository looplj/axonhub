'use client';

import { useState } from 'react';
import { IconBuildingStore, IconPlugConnected, IconRefresh, IconSearch, IconUpload } from '@tabler/icons-react';
import { useTranslation } from 'react-i18next';
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
import { Input } from '@/components/ui/input';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { PluginCard } from './components/plugin-card';
import { PluginDetailSheet } from './components/plugin-detail-sheet';
import { PluginGrantDialog } from './components/plugin-grant-dialog';
import { PluginInstallDialog } from './components/plugin-install-dialog';
import { PluginMarket } from './components/plugin-market';
import { matchesPluginSearch, PluginEmptyState, PluginGrid, PluginGridSkeleton } from './components/plugin-visuals';
import { useDeletePlugin, useDisablePlugin, usePluginMarket, usePlugins, type Plugin } from './data/plugins';
import { usePluginGrantFlow } from './hooks/use-plugin-grant-flow';

export type PluginsTab = 'installed' | 'market';

interface PluginsManagementProps {
  tab: PluginsTab;
  onTabChange: (tab: PluginsTab) => void;
}

export default function PluginsManagement({ tab, onTabChange }: PluginsManagementProps) {
  const { t } = useTranslation();
  const { isOwner, hasSystemScope } = usePermissions();
  const canWrite = hasSystemScope('write_plugins');
  const pluginsQuery = usePlugins();
  // Only drives the toolbar refresh button; the market tab owns its own render.
  const marketQuery = usePluginMarket(tab === 'market');
  const flow = usePluginGrantFlow();
  const disablePlugin = useDisablePlugin();
  const deletePlugin = useDeletePlugin();
  const [installOpen, setInstallOpen] = useState(false);
  const [selectedFile, setSelectedFile] = useState<File | null>(null);
  const [pendingDelete, setPendingDelete] = useState<Plugin | null>(null);
  const [detailId, setDetailId] = useState<string | null>(null);
  const [search, setSearch] = useState('');

  const runtimeStatuses = new Map((pluginsQuery.data?.pluginRuntimeStatuses ?? []).map((status) => [status.id, status]));
  const plugins = (pluginsQuery.data?.plugins ?? []).map((plugin) => {
    const status = runtimeStatuses.get(plugin.id);
    return {
      ...plugin,
      overloaded: status?.overloaded ?? 0,
      protectedDrops: status?.protectedDrops ?? 0,
      overriddenByPassThrough: status?.overriddenByPassThrough ?? 0,
    };
  });
  // Resolved from the query on every render so the sheet shows fresh data after a save.
  const detailPlugin = plugins.find((plugin) => plugin.id === detailId) ?? null;
  const visiblePlugins = plugins.filter((plugin) => matchesPluginSearch(search, plugin.name, plugin.pluginID, plugin.description));

  const upload = async () => {
    if (!selectedFile) {
      return;
    }
    if (await flow.install({ kind: 'file', file: selectedFile }, t('plugins.upload.success'))) {
      setInstallOpen(false);
      setSelectedFile(null);
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
        <Tabs value={tab} onValueChange={(value) => onTabChange(value as PluginsTab)} className='w-full gap-4'>
          <div className='flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between'>
            <TabsList>
              <TabsTrigger value='installed' className='px-3'>
                <IconPlugConnected />
                {t('plugins.tabs.installed')}
                {/* bg-current keeps the pill legible on both the muted and the primary (active) tab background. */}
                {pluginsQuery.isSuccess && (
                  <span className='min-w-5 rounded-full bg-current/15 px-1.5 text-center text-xs leading-5 tabular-nums'>
                    {plugins.length}
                  </span>
                )}
              </TabsTrigger>
              <TabsTrigger value='market' className='px-3'>
                <IconBuildingStore />
                {t('plugins.tabs.market')}
              </TabsTrigger>
            </TabsList>
            <div className='flex items-center gap-2'>
              <div className='relative flex-1 sm:w-64 sm:flex-none'>
                <IconSearch className='text-muted-foreground pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2' />
                <Input
                  value={search}
                  onChange={(event) => setSearch(event.target.value)}
                  placeholder={t('plugins.search')}
                  aria-label={t('plugins.search')}
                  className='pl-9'
                />
              </div>
              {tab === 'market' && (
                <Button
                  type='button'
                  variant='outline'
                  size='icon'
                  disabled={marketQuery.isFetching}
                  aria-label={t('plugins.market.refresh')}
                  title={t('plugins.market.refresh')}
                  onClick={() => marketQuery.refetch()}
                >
                  <IconRefresh className={marketQuery.isFetching ? 'size-4 animate-spin' : 'size-4'} />
                </Button>
              )}
            </div>
          </div>

          <TabsContent value='installed'>
            {pluginsQuery.isLoading && <PluginGridSkeleton />}
            {pluginsQuery.isError && (
              <PluginEmptyState
                icon={<IconPlugConnected />}
                title={t('plugins.loadFailed')}
                description={pluginsQuery.error instanceof Error ? pluginsQuery.error.message : undefined}
                action={
                  <Button type='button' variant='outline' size='sm' onClick={() => pluginsQuery.refetch()}>
                    {t('plugins.page.retry')}
                  </Button>
                }
              />
            )}
            {pluginsQuery.isSuccess && plugins.length === 0 && (
              <PluginEmptyState
                icon={<IconPlugConnected />}
                title={t('plugins.empty')}
                description={t('plugins.emptyHint')}
                action={
                  <Button type='button' variant='outline' size='sm' onClick={() => onTabChange('market')}>
                    <IconBuildingStore className='size-4' />
                    {t('plugins.tabs.market')}
                  </Button>
                }
              />
            )}
            {plugins.length > 0 && visiblePlugins.length === 0 && <PluginEmptyState icon={<IconSearch />} title={t('plugins.noMatch')} />}
            {visiblePlugins.length > 0 && (
              <PluginGrid>
                {visiblePlugins.map((plugin) => (
                  <PluginCard
                    key={plugin.id}
                    plugin={plugin}
                    canManage={isOwner}
                    togglePending={disablePlugin.isPending || flow.busy}
                    onOpen={(target) => setDetailId(target.id)}
                    onEnable={(target) => {
                      if (!flow.enable(target)) {
                        setDetailId(target.id);
                      }
                    }}
                    onDisable={(target) => disablePlugin.mutate(target.id)}
                    onEditPermissions={flow.editPermissions}
                    onDelete={setPendingDelete}
                  />
                ))}
              </PluginGrid>
            )}
          </TabsContent>
          <TabsContent value='market'>
            <PluginMarket
              search={search}
              canManage={isOwner}
              busy={flow.busy}
              onInstall={(entry, intent) =>
                flow.install(
                  { kind: 'market', pluginID: entry.id, version: entry.version },
                  intent === 'upgrade' ? t('plugins.market.upgraded') : t('plugins.market.success')
                )
              }
            />
          </TabsContent>
        </Tabs>
      </Main>

      <PluginDetailSheet
        plugin={detailPlugin}
        canWrite={canWrite}
        canManage={isOwner}
        onEditPermissions={flow.editPermissions}
        onOpenChange={(open) => !open && setDetailId(null)}
      />

      <PluginInstallDialog
        open={installOpen}
        onOpenChange={(open) => {
          setInstallOpen(open);
          if (!open) {
            setSelectedFile(null);
          }
        }}
        file={selectedFile}
        onFileChange={setSelectedFile}
        pending={flow.pending}
        onSubmit={upload}
      />

      <PluginGrantDialog
        open={flow.grant !== null}
        onOpenChange={(open) => !open && flow.cancel()}
        mode={flow.grant?.mode ?? 'install'}
        pluginID={flow.grant?.plugin.pluginID ?? ''}
        pluginName={flow.grant ? flow.grant.plugin.name || flow.grant.plugin.pluginID : ''}
        manifestHooks={flow.grant?.plugin.manifestHooks}
        permissions={flow.grant?.plugin.manifestPermissions ?? []}
        network={flow.grant?.plugin.manifestNetwork ?? []}
        initialGranted={flow.grant?.plugin.grantedPermissions ?? []}
        initialGrantedNetwork={flow.grant?.plugin.grantedNetwork ?? []}
        preview={flow.grant?.preview}
        pending={flow.pending}
        onConfirm={flow.confirm}
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
