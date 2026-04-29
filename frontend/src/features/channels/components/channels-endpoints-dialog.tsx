'use client';

import { useState, useMemo, useCallback } from 'react';
import { useTranslation } from 'react-i18next';
import { Plus, X } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Badge } from '@/components/ui/badge';
import { Channel, ChannelEndpoint, ApiFormat, channelEndpointSchema } from '../data/schema';
import { useSaveChannelEndpoints } from '../data/channels';
import { apiFormatSchema } from '../data/schema';
import { CHANNEL_TYPE_TO_DEFAULT_ENDPOINTS } from '../data/config_channels';

interface Props {
  channel: Channel;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

export function ChannelsEndpointsDialog({ channel, open, onOpenChange }: Props) {
  const { t } = useTranslation();
  const saveEndpoints = useSaveChannelEndpoints();

  const defaultEndpoints = useMemo(() => {
    return CHANNEL_TYPE_TO_DEFAULT_ENDPOINTS[channel.type as keyof typeof CHANNEL_TYPE_TO_DEFAULT_ENDPOINTS] || [];
  }, [channel.type]);

  const [endpoints, setEndpoints] = useState<ChannelEndpoint[]>(() =>
    channel.endpoints && channel.endpoints.length > 0 ? channel.endpoints : defaultEndpoints
  );
  const [newApiFormat, setNewApiFormat] = useState<string>('');
  const [newPath, setNewPath] = useState('');
  const [error, setError] = useState<string | null>(null);

  const usedApiFormats = useMemo(() => new Set(endpoints.map((ep) => ep.apiFormat)), [endpoints]);

  const defaultEndpointFormats = useMemo(() => new Set(defaultEndpoints.map((ep) => ep.apiFormat)), [defaultEndpoints]);

  const availableApiFormats = useMemo(() => {
    return apiFormatSchema.options.filter((f) => !usedApiFormats.has(f));
  }, [usedApiFormats]);

  const handleAddEndpoint = useCallback(() => {
    setError(null);
    if (!newApiFormat) return;

    if (usedApiFormats.has(newApiFormat)) {
      setError(t('channels.endpoints.duplicateError'));
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
  }, [newApiFormat, newPath, usedApiFormats, t]);

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
  }, [endpoints, channel.id, saveEndpoints, onOpenChange, t]);

  const handleKeyDown = useCallback((e: React.KeyboardEvent) => {
    if (e.key === 'Enter' && newApiFormat) {
      e.preventDefault();
      handleAddEndpoint();
    }
  }, [newApiFormat, handleAddEndpoint]);

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="flex h-[90vh] max-h-[700px] flex-col w-full max-w-full sm:max-w-2xl">
        <DialogHeader className="shrink-0">
          <DialogTitle>{t('channels.endpoints.title')}</DialogTitle>
          <DialogDescription>
            {channel.name}
          </DialogDescription>
        </DialogHeader>

        <div className="flex-1 overflow-y-auto py-4 space-y-5 min-h-0">
          {/* Existing endpoints */}
          <div className="space-y-3">
            <div className="flex items-center justify-between">
              <label className="text-sm font-medium">{t('channels.endpoints.currentEndpoints')}</label>
              <span className="text-xs text-muted-foreground">
                {endpoints.length > 0 && `${endpoints.length} configured`}
              </span>
            </div>
            {endpoints.length === 0 ? (
              <div className="rounded-lg border border-dashed p-4">
                <p className="text-sm text-muted-foreground text-center">{t('channels.endpoints.emptyHint')}</p>
              </div>
            ) : (
              <div className="rounded-lg border overflow-hidden">
                {/* Table Header */}
                <div className="grid grid-cols-[1fr_1fr_auto] gap-2 px-3 py-2 bg-muted/50 border-b text-xs font-medium text-muted-foreground">
                  <span>{t('channels.endpoints.apiFormat')}</span>
                  <span>{t('channels.endpoints.path')}</span>
                  <span className="w-8"></span>
                </div>
                {/* Table Body */}
                <div className="divide-y">
                  {endpoints.map((ep) => (
                    <div
                      key={ep.apiFormat}
                      className="grid grid-cols-[1fr_1fr_auto] gap-2 px-3 py-2.5 items-center text-sm hover:bg-muted/30 transition-colors"
                    >
                      <div className="flex items-center gap-2">
                        <Badge variant="secondary" className="font-mono text-xs w-fit">
                          {ep.apiFormat}
                        </Badge>
                        {defaultEndpointFormats.has(ep.apiFormat) && (
                          <Badge variant="outline" className="text-[10px]">
                            {t('channels.endpoints.defaultBadge')}
                          </Badge>
                        )}
                      </div>
                      <span className="text-muted-foreground text-xs font-mono truncate">
                        {ep.path || '-'}
                      </span>
                      <Button
                        type="button"
                        variant="ghost"
                        size="sm"
                        className="h-7 w-7 p-0 hover:text-destructive hover:bg-destructive/10"
                        onClick={() => handleRemoveEndpoint(ep.apiFormat)}
                        disabled={defaultEndpointFormats.has(ep.apiFormat)}
                        title={defaultEndpointFormats.has(ep.apiFormat) ? t('channels.endpoints.defaultLockedHint') : undefined}
                      >
                        <X className="h-3.5 w-3.5" />
                      </Button>
                    </div>
                  ))}
                </div>
              </div>
            )}
          </div>

          {/* Add new endpoint */}
          <div className="space-y-3">
            <label className="text-sm font-medium">{t('channels.endpoints.addEndpoint')}</label>
            <div className="flex gap-2 items-start">
              <Select value={newApiFormat} onValueChange={setNewApiFormat}>
                <SelectTrigger className="flex-1">
                  <SelectValue placeholder={t('channels.endpoints.apiFormat')} />
                </SelectTrigger>
                <SelectContent>
                  {availableApiFormats.length === 0 ? (
                    <div className="px-2 py-4 text-sm text-muted-foreground text-center">
                      {t('channels.endpoints.allFormatsUsed')}
                    </div>
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
                className="flex-1 disabled:opacity-50"
              />
              <Button
                type="button"
                variant="default"
                size="icon"
                onClick={handleAddEndpoint}
                disabled={!newApiFormat}
                className="shrink-0"
              >
                <Plus className="h-4 w-4" />
              </Button>
            </div>
          </div>

          {error && (
            <div className="flex items-center gap-2 text-sm text-destructive bg-destructive/10 rounded-md px-3 py-2">
              <span className="text-base">⚠</span>
              <span>{error}</span>
            </div>
          )}
        </div>

        <DialogFooter className="shrink-0 border-t pt-4">
          <Button variant="outline" onClick={() => onOpenChange(false)}>
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
