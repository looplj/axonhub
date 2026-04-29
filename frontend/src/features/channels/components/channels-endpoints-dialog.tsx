'use client';

import { useState, useMemo, useCallback } from 'react';
import { Plus, X } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { useSaveChannelEndpoints } from '../data/channels';
import {
  Channel,
  ChannelEndpoint,
  channelEndpointSchema,
  configurableChannelEndpointApiFormats,
  configurableChannelEndpointApiFormatSchema,
} from '../data/schema';

interface Props {
  channel: Channel;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

export function ChannelsEndpointsDialog({ channel, open, onOpenChange }: Props) {
  const { t } = useTranslation();
  const saveEndpoints = useSaveChannelEndpoints();

  const defaultEndpoints = useMemo(() => {
    return channel.defaultEndpoints ?? [];
  }, [channel.defaultEndpoints]);

  const [endpoints, setEndpoints] = useState<ChannelEndpoint[]>(() =>
    channel.endpoints && channel.endpoints.length > 0 ? channel.endpoints : []
  );
  const [newApiFormat, setNewApiFormat] = useState<string>('');
  const [newPath, setNewPath] = useState('');
  const [error, setError] = useState<string | null>(null);

  const usedApiFormats = useMemo(() => new Set(endpoints.map((ep) => ep.apiFormat)), [endpoints]);
  const defaultApiFormats = useMemo(() => new Set(defaultEndpoints.map((ep) => ep.apiFormat)), [defaultEndpoints]);
  const allowedApiFormats = configurableChannelEndpointApiFormatSchema.options;

  const availableApiFormats = useMemo(() => {
    return configurableChannelEndpointApiFormats.filter((f) => !usedApiFormats.has(f) && !defaultApiFormats.has(f));
  }, [defaultApiFormats, usedApiFormats]);

  const handleAddEndpoint = useCallback(() => {
    setError(null);
    if (!newApiFormat) return;

    if (usedApiFormats.has(newApiFormat)) {
      setError(t('channels.endpoints.duplicateError'));
      return;
    }

    if (!allowedApiFormats.includes(newApiFormat as (typeof allowedApiFormats)[number])) {
      setError(t('channels.endpoints.invalidApiFormat', 'Unsupported API format'));
      return;
    }

    if (defaultApiFormats.has(newApiFormat)) {
      setError(t('channels.endpoints.defaultConflictError', 'Cannot override a default endpoint'));
      return;
    }

    const parsed = channelEndpointSchema.safeParse({ apiFormat: newApiFormat, path: newPath || undefined });
    if (!parsed.success) {
      setError(parsed.error.errors[0]?.message || 'Invalid endpoint');
      return;
    }

    setEndpoints((prev) => [...prev, parsed.data]);
    setNewApiFormat('');
    setNewPath('');
  }, [allowedApiFormats, defaultApiFormats, newApiFormat, newPath, usedApiFormats, t]);

  const handleRemoveEndpoint = useCallback((apiFormat: string) => {
    setEndpoints((prev) => prev.filter((ep) => ep.apiFormat !== apiFormat));
    setError(null);
  }, []);

  const handleSave = useCallback(async () => {
    setError(null);

    const apiFormats = endpoints.map((ep) => ep.apiFormat);
    const duplicates = apiFormats.filter((f, i) => apiFormats.indexOf(f) !== i);
    if (duplicates.length > 0) {
      setError(t('channels.endpoints.duplicateError'));
      return;
    }

    const invalidApiFormat = apiFormats.find((format) => !allowedApiFormats.includes(format as (typeof allowedApiFormats)[number]));
    if (invalidApiFormat) {
      setError(t('channels.endpoints.invalidApiFormat', 'Unsupported API format'));
      return;
    }

    const conflictingDefaultApiFormat = apiFormats.find((format) => defaultApiFormats.has(format));
    if (conflictingDefaultApiFormat) {
      setError(t('channels.endpoints.defaultConflictError', 'Cannot override a default endpoint'));
      return;
    }

    try {
      await saveEndpoints.mutateAsync({
        channelID: channel.id,
        endpoints: endpoints.map((ep) => ({
          apiFormat: ep.apiFormat,
          path: ep.path || undefined,
        })),
      });
      onOpenChange(false);
    } catch {
      // error handled by hook
    }
  }, [allowedApiFormats, channel.id, defaultApiFormats, endpoints, onOpenChange, saveEndpoints, t]);

  const handleKeyDown = useCallback(
    (e: React.KeyboardEvent) => {
      if (e.key === 'Enter' && newApiFormat) {
        e.preventDefault();
        handleAddEndpoint();
      }
    },
    [newApiFormat, handleAddEndpoint]
  );

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className='flex h-[90vh] max-h-[700px] w-full max-w-full flex-col sm:max-w-2xl'>
        <DialogHeader className='shrink-0'>
          <DialogTitle>{t('channels.endpoints.title')}</DialogTitle>
          <DialogDescription>{channel.name}</DialogDescription>
        </DialogHeader>

        <div className='min-h-0 flex-1 space-y-5 overflow-y-auto py-4'>
          {/* Default endpoints */}
          <div className='space-y-3'>
            <div className='flex items-center justify-between'>
              <label className='text-sm font-medium'>{t('channels.endpoints.defaultEndpoints', 'Default endpoints')}</label>
              <span className='text-muted-foreground text-xs'>{defaultEndpoints.length > 0 && `${defaultEndpoints.length} resolved`}</span>
            </div>
            {defaultEndpoints.length === 0 ? (
              <div className='rounded-lg border border-dashed p-4'>
                <p className='text-muted-foreground text-center text-sm'>
                  {t('channels.endpoints.noDefaultEndpoints', 'No default endpoints resolved for this channel type.')}
                </p>
              </div>
            ) : (
              <div className='overflow-hidden rounded-lg border'>
                {/* Table Header */}
                <div className='bg-muted/50 text-muted-foreground grid grid-cols-[1fr_1fr_auto] gap-2 border-b px-3 py-2 text-xs font-medium'>
                  <span>{t('channels.endpoints.apiFormat')}</span>
                  <span>{t('channels.endpoints.path')}</span>
                  <span className='w-8'></span>
                </div>
                {/* Table Body */}
                <div className='divide-y'>
                  {defaultEndpoints.map((ep, index) => (
                    <div
                      key={`${ep.apiFormat}-${index}`}
                      className='hover:bg-muted/30 grid grid-cols-[1fr_1fr_auto] items-center gap-2 px-3 py-2.5 text-sm transition-colors'
                    >
                      <div className='flex items-center gap-2'>
                        <Badge variant='secondary' className='w-fit font-mono text-xs'>
                          {ep.apiFormat}
                        </Badge>
                        {index === 0 && (
                          <Badge variant='outline' className='text-[10px]'>
                            {t('channels.endpoints.primaryBadge', 'Primary')}
                          </Badge>
                        )}
                      </div>
                      <span className='text-muted-foreground truncate font-mono text-xs'>{ep.path || '-'}</span>
                      <span className='text-muted-foreground text-right text-[10px]'>{t('channels.endpoints.readOnly', 'Read-only')}</span>
                    </div>
                  ))}
                </div>
              </div>
            )}
          </div>

          {/* Current configured endpoints */}
          <div className='space-y-3'>
            <div className='flex items-center justify-between'>
              <label className='text-sm font-medium'>{t('channels.endpoints.currentEndpoints')}</label>
              <span className='text-muted-foreground text-xs'>{endpoints.length > 0 && `${endpoints.length} configured`}</span>
            </div>
            {endpoints.length === 0 ? (
              <div className='rounded-lg border border-dashed p-4'>
                <p className='text-muted-foreground text-center text-sm'>
                  {t('channels.endpoints.noOverridesHint', 'No custom endpoint overrides configured.')}
                </p>
              </div>
            ) : (
              <div className='overflow-hidden rounded-lg border'>
                <div className='bg-muted/50 text-muted-foreground grid grid-cols-[1fr_1fr_auto] gap-2 border-b px-3 py-2 text-xs font-medium'>
                  <span>{t('channels.endpoints.apiFormat')}</span>
                  <span>{t('channels.endpoints.path')}</span>
                  <span className='w-8'></span>
                </div>
                <div className='divide-y'>
                  {endpoints.map((ep) => (
                    <div
                      key={ep.apiFormat}
                      className='hover:bg-muted/30 grid grid-cols-[1fr_1fr_auto] items-center gap-2 px-3 py-2.5 text-sm transition-colors'
                    >
                      <div className='flex items-center gap-2'>
                        <Badge variant='secondary' className='w-fit font-mono text-xs'>
                          {ep.apiFormat}
                        </Badge>
                        {!allowedApiFormats.includes(ep.apiFormat as (typeof allowedApiFormats)[number]) && (
                          <Badge variant='destructive' className='text-[10px]'>
                            {t('channels.endpoints.unsupportedBadge', 'Unsupported')}
                          </Badge>
                        )}
                      </div>
                      <span className='text-muted-foreground truncate font-mono text-xs'>{ep.path || '-'}</span>
                      <Button
                        type='button'
                        variant='ghost'
                        size='sm'
                        className='hover:text-destructive hover:bg-destructive/10 h-7 w-7 p-0'
                        onClick={() => handleRemoveEndpoint(ep.apiFormat)}
                      >
                        <X className='h-3.5 w-3.5' />
                      </Button>
                    </div>
                  ))}
                </div>
              </div>
            )}
          </div>

          {/* Add new endpoint */}
          <div className='space-y-3'>
            <label className='text-sm font-medium'>{t('channels.endpoints.addEndpoint')}</label>
            <div className='flex items-start gap-2'>
              <Select value={newApiFormat} onValueChange={setNewApiFormat}>
                <SelectTrigger className='flex-1'>
                  <SelectValue placeholder={t('channels.endpoints.apiFormat')} />
                </SelectTrigger>
                <SelectContent>
                  {availableApiFormats.length === 0 ? (
                    <div className='text-muted-foreground px-2 py-4 text-center text-sm'>{t('channels.endpoints.allFormatsUsed')}</div>
                  ) : (
                    availableApiFormats.map((format) => (
                      <SelectItem key={format} value={format}>
                        {format}
                      </SelectItem>
                    ))
                  )}
                </SelectContent>
              </Select>
              <Input
                placeholder={newApiFormat ? t('channels.endpoints.pathPlaceholder') : t('channels.endpoints.selectFormatFirst')}
                value={newPath}
                onChange={(e) => setNewPath(e.target.value)}
                onKeyDown={handleKeyDown}
                disabled={!newApiFormat}
                className='flex-1 disabled:opacity-50'
              />
              <Button type='button' variant='default' size='icon' onClick={handleAddEndpoint} disabled={!newApiFormat} className='shrink-0'>
                <Plus className='h-4 w-4' />
              </Button>
            </div>
          </div>

          {error && (
            <div className='text-destructive bg-destructive/10 flex items-center gap-2 rounded-md px-3 py-2 text-sm'>
              <span className='text-base'>⚠</span>
              <span>{error}</span>
            </div>
          )}
        </div>

        <DialogFooter className='shrink-0 border-t pt-4'>
          <Button variant='outline' onClick={() => onOpenChange(false)}>
            {t('common.buttons.cancel')}
          </Button>
          <Button onClick={handleSave} disabled={saveEndpoints.isPending}>
            {saveEndpoints.isPending ? t('common.buttons.saving') : t('common.buttons.save')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
