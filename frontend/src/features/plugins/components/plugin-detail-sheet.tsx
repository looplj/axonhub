'use client';

import { useTranslation } from 'react-i18next';
import { Badge } from '@/components/ui/badge';
import { ScrollArea } from '@/components/ui/scroll-area';
import { Separator } from '@/components/ui/separator';
import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle } from '@/components/ui/sheet';
import type { Plugin } from '../data/plugins';
import { PluginConfigForm } from './plugin-config-form';
import { runtimeStatusLabel } from './plugins-table';

interface PluginDetailSheetProps {
  plugin: Plugin | null;
  canWrite: boolean;
  onOpenChange: (open: boolean) => void;
}

export function PluginDetailSheet({ plugin, canWrite, onOpenChange }: PluginDetailSheetProps) {
  const { t } = useTranslation();

  return (
    <Sheet open={plugin !== null} onOpenChange={onOpenChange}>
      <SheetContent className='w-full gap-0 sm:max-w-lg'>
        {plugin && (
          <>
            <SheetHeader className='border-b pr-10'>
              <SheetTitle className='flex items-center gap-2'>
                <span className='truncate'>{plugin.name || plugin.pluginID}</span>
                {plugin.builtin && <Badge variant='outline'>{t('plugins.builtin')}</Badge>}
              </SheetTitle>
              {plugin.description && <SheetDescription>{plugin.description}</SheetDescription>}
            </SheetHeader>
            <ScrollArea className='min-h-0 flex-1'>
              <div className='space-y-6 p-4'>
                <div className='grid gap-3 sm:grid-cols-2'>
                  <Field label={t('plugins.fields.pluginID')} value={plugin.pluginID} />
                  <Field label={t('plugins.fields.version')} value={plugin.version} />
                  <Field label={t('plugins.fields.enabled')} value={plugin.enabled ? t('plugins.enabledYes') : t('plugins.enabledNo')} />
                  <Field label={t('plugins.fields.runtimeStatus')} value={runtimeStatusLabel(t, plugin.runtimeStatus)} />
                  <Field
                    label={t('plugins.fields.grantedPermissions')}
                    value={plugin.grantedPermissions.length > 0 ? plugin.grantedPermissions.join(', ') : t('plugins.none')}
                  />
                  <Field
                    label={t('plugins.fields.grantedNetwork')}
                    value={plugin.grantedNetwork.length > 0 ? plugin.grantedNetwork.join(', ') : t('plugins.none')}
                  />
                  <Field label={t('plugins.fields.runtimeError')} value={plugin.runtimeError || t('plugins.none')} />
                </div>
                <Separator />
                {canWrite ? (
                  <PluginConfigForm plugin={plugin} />
                ) : (
                  <div className='space-y-3'>
                    <div>
                      <h3 className='text-sm font-medium'>{t('plugins.config.title')}</h3>
                      <p className='text-muted-foreground text-sm'>{t('plugins.config.readonly')}</p>
                    </div>
                    <Field label={t('plugins.config.priority')} value={String(plugin.priority)} />
                    {plugin.configFields.length === 0 && <p className='text-muted-foreground text-sm'>{t('plugins.config.empty')}</p>}
                    {plugin.configFields.map((field) => (
                      <Field
                        key={field.name}
                        label={field.title || field.name}
                        value={formatConfigValue(plugin.config[field.name]) || t('plugins.none')}
                      />
                    ))}
                  </div>
                )}
              </div>
            </ScrollArea>
          </>
        )}
      </SheetContent>
    </Sheet>
  );
}

function Field({ label, value }: { label: string; value: string }) {
  return (
    <div className='space-y-1'>
      <div className='text-muted-foreground text-sm'>{label}</div>
      <div className='text-sm break-all'>{value}</div>
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
