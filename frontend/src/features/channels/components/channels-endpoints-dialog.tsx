'use client';

import { useState, useMemo, useCallback, useEffect } from 'react';
import { AlertCircle, Plus, X } from 'lucide-react';
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

const ALLOWED_API_FORMATS = configurableChannelEndpointApiFormatSchema.options;

function EndpointTable({
  endpoints,
  readOnly,
  hideBaseURL,
  children,
}: {
  endpoints: ChannelEndpoint[];
  readOnly?: boolean;
  hideBaseURL?: boolean;
  children?: (ep: ChannelEndpoint, index: number) => React.ReactNode;
}) {
  const { t } = useTranslation();
  const gridCols = hideBaseURL ? 'grid-cols-[1fr_1fr_auto]' : 'grid-cols-[1fr_1fr_1fr_auto]';
  return (
    <div className='overflow-hidden rounded-lg border'>
      <div className={`bg-muted/50 text-muted-foreground ${gridCols} gap-2 border-b px-3 py-2 text-xs font-medium`}>
        <span>{t('channels.endpoints.apiFormat')}</span>
        {!hideBaseURL && <span>{t('channels.endpoints.baseURL')}</span>}
        <span>{t('channels.endpoints.path')}</span>
        <span className='w-8' />
      </div>
      <div className='divide-y'>
        {endpoints.map((ep, index) => (
          <div
            key={`${ep.apiFormat}-${index}`}
            className={`hover:bg-muted/30 ${gridCols} items-center gap-2 px-3 py-2.5 text-sm transition-colors`}
          >
            <div className='flex items-center gap-2'>
              <Badge variant='secondary' className='w-fit font-mono text-xs'>
                {ep.apiFormat}
              </Badge>
              {readOnly && index === 0 && (
                <Badge variant='outline' className='text-[10px]'>
                  {t('channels.endpoints.primaryBadge')}
                </Badge>
              )}
              {!readOnly && !ALLOWED_API_FORMATS.includes(ep.apiFormat as (typeof ALLOWED_API_FORMATS)[number]) && (
                <Badge variant='destructive' className='text-[10px]'>
                  {t('channels.endpoints.unsupportedBadge')}
                </Badge>
              )}
            </div>
            {!hideBaseURL && <span className='text-muted-foreground truncate font-mono text-xs'>{ep.baseURL || '-'}</span>}
            <span className='text-muted-foreground truncate font-mono text-xs'>{ep.path || '-'}</span>
            {readOnly ? (
              <span className='text-muted-foreground text-right text-[10px]'>{t('channels.endpoints.readOnly')}</span>
            ) : (
              children?.(ep, index)
            )}
          </div>
        ))}
      </div>
    </div>
  );
}

export function ChannelsEndpointsDialog({ channel, open, onOpenChange }: Props) {
  const { t } = useTranslation();
  const saveEndpoints = useSaveChannelEndpoints();

  const defaultEndpoints = channel.defaultEndpoints ?? [];

  const [endpoints, setEndpoints] = useState<ChannelEndpoint[]>(() => channel.endpoints ?? []);
  const [newApiFormat, setNewApiFormat] = useState('');
  const [newPath, setNewPath] = useState('');
  const [newBaseURL, setNewBaseURL] = useState('');
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (open) {
      setEndpoints(channel.endpoints ?? []);
      setNewApiFormat('');
      setNewPath('');
      setNewBaseURL('');
      setError(null);
    }
  }, [open, channel.endpoints]);

  const usedApiFormats = useMemo(() => new Set(endpoints.map((ep) => ep.apiFormat)), [endpoints]);

  const availableApiFormats = useMemo(() => configurableChannelEndpointApiFormats.filter((f) => !usedApiFormats.has(f)), [usedApiFormats]);

  const handleAddEndpoint = useCallback(() => {
    setError(null);
    if (!newApiFormat) return;

    if (usedApiFormats.has(newApiFormat)) {
      setError(t('channels.endpoints.duplicateError'));
      return;
    }

    if (!ALLOWED_API_FORMATS.includes(newApiFormat as (typeof ALLOWED_API_FORMATS)[number])) {
      setError(t('channels.endpoints.invalidApiFormat', 'Unsupported API format'));
      return;
    }

    const parsed = channelEndpointSchema.safeParse({
      apiFormat: newApiFormat,
      path: newPath || undefined,
      baseURL: newBaseURL || undefined,
    });
    if (!parsed.success) {
      const firstIssue = parsed.error.issues[0];
      if (firstIssue?.path[0] === 'baseURL') {
        setError(t('channels.endpoints.invalidBaseURL'));
      } else {
        setError(firstIssue?.message || 'Invalid endpoint');
      }
      return;
    }

    setEndpoints((prev) => [...prev, parsed.data]);
    setNewApiFormat('');
    setNewPath('');
    setNewBaseURL('');
  }, [newApiFormat, newPath, newBaseURL, usedApiFormats, t]);

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

    const invalidApiFormat = apiFormats.find((format) => !ALLOWED_API_FORMATS.includes(format as (typeof ALLOWED_API_FORMATS)[number]));
    if (invalidApiFormat) {
      setError(t('channels.endpoints.invalidApiFormat', 'Unsupported API format'));
      return;
    }

    try {
      await saveEndpoints.mutateAsync({
        channelID: channel.id,
        endpoints: endpoints.map((ep) => ({
          apiFormat: ep.apiFormat,
          path: ep.path || undefined,
          baseURL: ep.baseURL || undefined,
          transport: ep.transport || undefined,
        })),
      });
      onOpenChange(false);
    } catch {
      // error handled by hook
    }
  }, [channel.id, endpoints, onOpenChange, saveEndpoints, t]);

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
      <DialogContent className='flex h-[90vh] max-h-[700px] w-full max-w-full flex-col sm:max-w-4xl'>
        <DialogHeader className='shrink-0'>
          <DialogTitle>{t('channels.endpoints.title')}</DialogTitle>
          <DialogDescription>{channel.name}</DialogDescription>
        </DialogHeader>

        <div className='min-h-0 flex-1 space-y-6 overflow-y-auto py-4'>
          {/* Default endpoints */}
          <div className='space-y-3'>
            <div className='flex items-center justify-between'>
              <label className='text-sm font-medium'>{t('channels.endpoints.defaultEndpoints', 'Default endpoints')}</label>
              {defaultEndpoints.length > 0 && (
                <span className='text-muted-foreground text-xs'>
                  {t('channels.endpoints.resolvedCount', { count: defaultEndpoints.length })}
                </span>
              )}
            </div>
            {defaultEndpoints.length === 0 ? (
              <div className='text-muted-foreground rounded-lg border border-dashed p-4 text-center text-sm'>
                {t('channels.endpoints.noDefaultEndpoints', 'No default endpoints resolved for this channel type.')}
              </div>
            ) : (
              <EndpointTable endpoints={defaultEndpoints} readOnly hideBaseURL />
            )}
          </div>

          {/* Current configured endpoints */}
          <div className='space-y-3'>
            <div className='flex items-center justify-between'>
              <label className='text-sm font-medium'>{t('channels.endpoints.currentEndpoints')}</label>
              {endpoints.length > 0 && (
                <span className='text-muted-foreground text-xs'>
                  {t('channels.endpoints.configuredCount', { count: endpoints.length })}
                </span>
              )}
            </div>
            {endpoints.length === 0 ? (
              <div className='text-muted-foreground rounded-lg border border-dashed p-4 text-center text-sm'>
                {t('channels.endpoints.noOverridesHint', 'No custom endpoint overrides configured.')}
              </div>
            ) : (
              <EndpointTable endpoints={endpoints}>
                {(ep) => (
                  <Button
                    type='button'
                    variant='ghost'
                    size='sm'
                    className='hover:text-destructive hover:bg-destructive/10 h-7 w-7 p-0'
                    onClick={() => handleRemoveEndpoint(ep.apiFormat)}
                  >
                    <X className='h-3.5 w-3.5' />
                  </Button>
                )}
              </EndpointTable>
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
                placeholder={newApiFormat ? t('channels.endpoints.baseURLPlaceholder') : t('channels.endpoints.selectFormatFirst')}
                value={newBaseURL}
                onChange={(e) => setNewBaseURL(e.target.value)}
                onKeyDown={handleKeyDown}
                disabled={!newApiFormat}
                className='flex-1 disabled:opacity-50'
              />
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
              <AlertCircle className='h-4 w-4 shrink-0' />
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
