'use client';

import type { ReactNode } from 'react';
import { IconAlertTriangle, IconPencil } from '@tabler/icons-react';
import { useTranslation } from 'react-i18next';
import { Alert, AlertDescription } from '@/components/ui/alert';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { ScrollArea } from '@/components/ui/scroll-area';
import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle } from '@/components/ui/sheet';
import type { Plugin } from '../data/plugins';
import { PluginConfigForm } from './plugin-config-form';
import { PluginAvatar, PluginStatus } from './plugin-visuals';

interface PluginDetailSheetProps {
  plugin: Plugin | null;
  canWrite: boolean;
  canManage: boolean;
  onEditPermissions: (plugin: Plugin) => void;
  onOpenChange: (open: boolean) => void;
}

export function PluginDetailSheet({ plugin, canWrite, canManage, onEditPermissions, onOpenChange }: PluginDetailSheetProps) {
  const { t } = useTranslation();

  return (
    <Sheet open={plugin !== null} onOpenChange={onOpenChange}>
      <SheetContent className='w-full gap-0 sm:max-w-xl'>
        {plugin && (
          <>
            <SheetHeader className='gap-3 border-b p-5 pr-12'>
              <div className='flex items-start gap-3'>
                <PluginAvatar id={plugin.pluginID} name={plugin.name} className='size-12 text-lg' />
                <div className='min-w-0 flex-1 space-y-1'>
                  <SheetTitle className='flex items-center gap-2'>
                    <span className='truncate'>{plugin.name || plugin.pluginID}</span>
                    {plugin.builtin && (
                      <Badge variant='outline' className='h-5'>
                        {t('plugins.builtin')}
                      </Badge>
                    )}
                  </SheetTitle>
                  <div className='text-muted-foreground flex flex-wrap items-center gap-x-3 gap-y-1 text-xs'>
                    <span className='font-mono'>{plugin.pluginID}</span>
                    <span className='font-mono'>{plugin.version}</span>
                    <PluginStatus status={plugin.runtimeStatus} />
                  </div>
                </div>
              </div>
              {plugin.description && <SheetDescription>{plugin.description}</SheetDescription>}
            </SheetHeader>

            <ScrollArea className='min-h-0 flex-1'>
              <div className='space-y-6 p-5'>
                {plugin.runtimeError && (
                  <Alert variant='destructive'>
                    <IconAlertTriangle />
                    <AlertDescription className='break-all'>{plugin.runtimeError}</AlertDescription>
                  </Alert>
                )}

                <div className='grid grid-cols-2 gap-3'>
                  <Stat label={t('plugins.fields.priority')} value={plugin.priority} />
                  <Stat label={t('plugins.fields.overloaded')} value={plugin.overloaded ?? 0} />
                  <Stat label={t('plugins.fields.protectedDrops')} value={plugin.protectedDrops ?? 0} />
                  <Stat label={t('plugins.fields.overriddenByPassThrough')} value={plugin.overriddenByPassThrough ?? 0} />
                </div>

                <Section
                  title={t('plugins.detail.access')}
                  action={
                    canManage &&
                    plugin.enabled && (
                      <Button type='button' variant='ghost' size='sm' className='h-7' onClick={() => onEditPermissions(plugin)}>
                        <IconPencil className='size-3.5' />
                        {t('plugins.detail.edit')}
                      </Button>
                    )
                  }
                >
                  <div className='divide-y rounded-xl border'>
                    <ChipRow
                      label={t('plugins.fields.grantedPermissions')}
                      items={plugin.grantedPermissions}
                      total={plugin.manifestPermissions.length}
                    />
                    <ChipRow label={t('plugins.fields.grantedNetwork')} items={plugin.grantedNetwork} total={plugin.manifestNetwork.length} />
                  </div>
                </Section>

                <Section title={t('plugins.hooks.title')}>
                  <div className='divide-y rounded-xl border'>
                    <ChipRow
                      label={`${t('plugins.hooks.request')} · ${t(plugin.manifestHooks?.request?.enabled ? 'plugins.hooks.enabled' : 'plugins.hooks.disabled')}`}
                      items={plugin.manifestHooks?.request?.requestTypes ?? []}
                      total={0}
                    />
                    <ChipRow
                      label={`${t('plugins.hooks.attempt')} · ${t(plugin.manifestHooks?.attempt?.enabled ? 'plugins.hooks.enabled' : 'plugins.hooks.disabled')}`}
                      items={plugin.manifestHooks?.attempt?.apiFormats ?? []}
                      total={0}
                    />
                  </div>
                </Section>

                <Section title={t('plugins.config.title')}>
                  {canWrite ? (
                    <PluginConfigForm plugin={plugin} />
                  ) : (
                    <div className='divide-y rounded-xl border'>
                      {plugin.configFields.length === 0 && (
                        <p className='text-muted-foreground p-3 text-sm'>{t('plugins.config.empty')}</p>
                      )}
                      {plugin.configFields.map((field) => (
                        <div key={field.name} className='flex items-start justify-between gap-4 p-3 text-sm'>
                          <span className='text-muted-foreground'>{field.title || field.name}</span>
                          <span className='text-right break-all'>{formatConfigValue(plugin.config[field.name]) || t('plugins.none')}</span>
                        </div>
                      ))}
                    </div>
                  )}
                </Section>
              </div>
            </ScrollArea>
          </>
        )}
      </SheetContent>
    </Sheet>
  );
}

function Section({ title, action, children }: { title: string; action?: ReactNode; children: ReactNode }) {
  return (
    <section className='space-y-3'>
      <div className='flex h-7 items-center justify-between'>
        <h3 className='text-sm font-semibold'>{title}</h3>
        {action}
      </div>
      {children}
    </section>
  );
}

function Stat({ label, value }: { label: string; value: number }) {
  return (
    <div className='bg-muted/40 rounded-xl border p-3'>
      <div className='text-muted-foreground truncate text-xs' title={label}>
        {label}
      </div>
      <div className='mt-1 text-lg font-semibold tabular-nums'>{value}</div>
    </div>
  );
}

function ChipRow({ label, items, total }: { label: string; items: string[]; total: number }) {
  const { t } = useTranslation();

  return (
    <div className='space-y-2 p-3'>
      <div className='text-muted-foreground flex items-center justify-between text-xs'>
        <span>{label}</span>
        {total > 0 && (
          <span className='tabular-nums'>
            {items.length}/{total}
          </span>
        )}
      </div>
      {items.length === 0 ? (
        <div className='text-muted-foreground text-sm'>{t('plugins.none')}</div>
      ) : (
        <div className='flex flex-wrap gap-1.5'>
          {items.map((item) => (
            <Badge
              key={item}
              variant={item.startsWith('write_') ? 'destructive' : 'secondary'}
              className='max-w-full font-mono font-normal break-all whitespace-normal'
            >
              {item}
            </Badge>
          ))}
        </div>
      )}
    </div>
  );
}

function formatConfigValue(value: unknown) {
  if (value === undefined || value === null || value === '') {
    return '';
  }
  if (typeof value === 'string') {
    return value;
  }
  if (typeof value === 'boolean' || typeof value === 'number') {
    return String(value);
  }
  return JSON.stringify(value);
}
