'use client';

import { IconDots, IconShieldLock, IconTrash, IconWorld, IconKey } from '@tabler/icons-react';
import { useTranslation } from 'react-i18next';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import { Switch } from '@/components/ui/switch';
import { cn } from '@/lib/utils';
import type { Plugin } from '../data/plugins';
import { PluginAvatar, PluginStatus } from './plugin-visuals';

interface PluginCardProps {
  plugin: Plugin;
  canManage: boolean;
  togglePending: boolean;
  onOpen: (plugin: Plugin) => void;
  onEnable: (plugin: Plugin) => void;
  onDisable: (plugin: Plugin) => void;
  onEditPermissions: (plugin: Plugin) => void;
  onDelete: (plugin: Plugin) => void;
}

export function PluginCard({
  plugin,
  canManage,
  togglePending,
  onOpen,
  onEnable,
  onDisable,
  onEditPermissions,
  onDelete,
}: PluginCardProps) {
  const { t } = useTranslation();
  const name = plugin.name || plugin.pluginID;
  const hasMenu = canManage && (plugin.enabled || !plugin.builtin);
  const overloaded = plugin.overloaded ?? 0;
  const protectedDrops = plugin.protectedDrops ?? 0;
  const overridden = plugin.overriddenByPassThrough ?? 0;

  return (
    <div
      data-testid='plugin-row'
      className={cn(
        'bg-card group flex flex-col overflow-hidden rounded-2xl border shadow-xs transition-all',
        'hover:border-primary/30 hover:shadow-md',
        plugin.runtimeStatus === 'error' && 'border-destructive/40'
      )}
    >
      <button
        type='button'
        onClick={() => onOpen(plugin)}
        className='focus-visible:ring-ring/50 flex flex-1 flex-col gap-3 p-4 text-left outline-none focus-visible:ring-[3px]'
      >
        <div className='flex items-start gap-3'>
          <PluginAvatar id={plugin.pluginID} name={plugin.name} className={cn(!plugin.enabled && 'grayscale')} />
          <div className='min-w-0 flex-1'>
            <div className='flex items-center gap-2'>
              <span className='truncate font-semibold'>{name}</span>
              {plugin.builtin && (
                <Badge variant='outline' className='h-5'>
                  {t('plugins.builtin')}
                </Badge>
              )}
            </div>
            <div className='text-muted-foreground truncate font-mono text-xs'>{plugin.pluginID}</div>
          </div>
          <span className='text-muted-foreground bg-muted shrink-0 rounded-md px-1.5 py-0.5 font-mono text-[11px]'>{plugin.version}</span>
        </div>
        <p className='text-muted-foreground line-clamp-2 min-h-10 text-sm'>{plugin.description}</p>
        {plugin.runtimeError && (
          <p className='text-destructive bg-destructive/10 line-clamp-2 rounded-md px-2 py-1 text-xs' title={plugin.runtimeError}>
            {plugin.runtimeError}
          </p>
        )}
        {(overloaded > 0 || overridden > 0 || protectedDrops > 0) && (
          <div className='flex flex-wrap gap-1.5'>
            {overloaded > 0 && (
              <Badge variant='secondary' className='font-normal'>
                {t('plugins.fields.overloaded')} {overloaded}
              </Badge>
            )}
            {protectedDrops > 0 && (
              <Badge variant='secondary' className='font-normal'>
                {t('plugins.fields.protectedDrops')} {protectedDrops}
              </Badge>
            )}
            {overridden > 0 && (
              <Badge variant='secondary' className='font-normal'>
                {t('plugins.fields.overriddenByPassThrough')} {overridden}
              </Badge>
            )}
          </div>
        )}
      </button>

      <div className='bg-muted/30 flex items-center gap-3 border-t px-4 py-2.5'>
        <PluginStatus status={plugin.runtimeStatus} />
        <div className='text-muted-foreground flex items-center gap-2.5 text-xs'>
          {plugin.grantedPermissions.length > 0 && (
            <span className='inline-flex items-center gap-1' title={t('plugins.fields.grantedPermissions')}>
              <IconKey className='size-3.5' />
              {plugin.grantedPermissions.length}
            </span>
          )}
          {plugin.grantedNetwork.length > 0 && (
            <span className='inline-flex items-center gap-1' title={t('plugins.fields.grantedNetwork')}>
              <IconWorld className='size-3.5' />
              {plugin.grantedNetwork.length}
            </span>
          )}
        </div>
        <div className='ml-auto flex items-center gap-1'>
          {hasMenu && (
            <DropdownMenu modal={false}>
              <DropdownMenuTrigger asChild>
                <Button type='button' variant='ghost' size='icon' className='size-8' aria-label={t('plugins.columns.actions')}>
                  <IconDots className='size-4' />
                </Button>
              </DropdownMenuTrigger>
              <DropdownMenuContent align='end'>
                {plugin.enabled && (
                  <DropdownMenuItem onClick={() => onEditPermissions(plugin)}>
                    <IconShieldLock className='size-4' />
                    {t('plugins.editPermissions')}
                  </DropdownMenuItem>
                )}
                {plugin.enabled && !plugin.builtin && <DropdownMenuSeparator />}
                {!plugin.builtin && (
                  <DropdownMenuItem className='text-destructive focus:text-destructive' onClick={() => onDelete(plugin)}>
                    <IconTrash className='size-4' />
                    {t('plugins.delete')}
                  </DropdownMenuItem>
                )}
              </DropdownMenuContent>
            </DropdownMenu>
          )}
          {canManage && (
            <Switch
              checked={plugin.enabled}
              disabled={togglePending}
              aria-label={plugin.enabled ? t('plugins.disable') : t('plugins.enable')}
              onCheckedChange={(checked) => (checked ? onEnable(plugin) : onDisable(plugin))}
            />
          )}
        </div>
      </div>
    </div>
  );
}
