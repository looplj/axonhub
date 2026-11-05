'use client';

import { IconDots, IconSettings } from '@tabler/icons-react';
import type { TFunction } from 'i18next';
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
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import type { Plugin } from '../data/plugins';

interface PluginsTableProps {
  plugins: Plugin[];
  canManage: boolean;
  togglePending: boolean;
  onConfigure: (plugin: Plugin) => void;
  onEnable: (plugin: Plugin) => void;
  onDisable: (plugin: Plugin) => void;
  onDelete: (plugin: Plugin) => void;
}

export function PluginsTable({ plugins, canManage, togglePending, onConfigure, onEnable, onDisable, onDelete }: PluginsTableProps) {
  const { t } = useTranslation();

  return (
    <div className='rounded-2xl border'>
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>{t('plugins.columns.name')}</TableHead>
            <TableHead>{t('plugins.fields.version')}</TableHead>
            <TableHead>{t('plugins.fields.enabled')}</TableHead>
            <TableHead>{t('plugins.fields.runtimeStatus')}</TableHead>
            <TableHead className='text-right'>{t('plugins.fields.priority')}</TableHead>
            <TableHead className='w-0 text-right'>{t('plugins.columns.actions')}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {plugins.map((plugin) => (
            <TableRow key={plugin.id} data-testid='plugin-row'>
              <TableCell className='max-w-md'>
                <div className='flex items-center gap-2'>
                  <span className='truncate font-medium'>{plugin.name || plugin.pluginID}</span>
                  {plugin.builtin && <Badge variant='outline'>{t('plugins.builtin')}</Badge>}
                </div>
                <div className='text-muted-foreground truncate text-xs'>{plugin.pluginID}</div>
                {plugin.description && <div className='text-muted-foreground mt-1 line-clamp-2 text-xs whitespace-normal'>{plugin.description}</div>}
              </TableCell>
              <TableCell>{plugin.version}</TableCell>
              <TableCell>
                <Badge variant={plugin.enabled ? 'default' : 'secondary'}>
                  {plugin.enabled ? t('plugins.enabledYes') : t('plugins.enabledNo')}
                </Badge>
              </TableCell>
              <TableCell className='max-w-xs'>
                <RuntimeStatus plugin={plugin} />
              </TableCell>
              <TableCell className='text-right'>{plugin.priority}</TableCell>
              <TableCell>
                <div className='flex items-center justify-end gap-1'>
                  <Button type='button' variant='ghost' size='sm' onClick={() => onConfigure(plugin)}>
                    <IconSettings className='size-4' />
                    {t('plugins.configure')}
                  </Button>
                  {canManage && (
                    <DropdownMenu modal={false}>
                      <DropdownMenuTrigger asChild>
                        <Button type='button' variant='ghost' size='icon' className='size-8' aria-label={t('plugins.columns.actions')}>
                          <IconDots className='size-4' />
                        </Button>
                      </DropdownMenuTrigger>
                      <DropdownMenuContent align='end'>
                        {plugin.enabled ? (
                          <DropdownMenuItem disabled={togglePending} onClick={() => onDisable(plugin)}>
                            {t('plugins.disable')}
                          </DropdownMenuItem>
                        ) : (
                          <DropdownMenuItem onClick={() => onEnable(plugin)}>{t('plugins.enable')}</DropdownMenuItem>
                        )}
                        {!plugin.builtin && (
                          <>
                            <DropdownMenuSeparator />
                            <DropdownMenuItem className='text-destructive focus:text-destructive' onClick={() => onDelete(plugin)}>
                              {t('plugins.delete')}
                            </DropdownMenuItem>
                          </>
                        )}
                      </DropdownMenuContent>
                    </DropdownMenu>
                  )}
                </div>
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  );
}

export function runtimeStatusLabel(t: TFunction, status: string) {
  return status === 'active' || status === 'disabled' || status === 'error' ? t(`plugins.status.${status}`) : status;
}

function RuntimeStatus({ plugin }: { plugin: Plugin }) {
  const { t } = useTranslation();
  const label = runtimeStatusLabel(t, plugin.runtimeStatus);

  return (
    <div className='space-y-1'>
      <Badge variant={plugin.runtimeStatus === 'error' ? 'destructive' : 'outline'}>{label}</Badge>
      {plugin.runtimeError && (
        <div className='text-destructive truncate text-xs' title={plugin.runtimeError}>
          {plugin.runtimeError}
        </div>
      )}
    </div>
  );
}
