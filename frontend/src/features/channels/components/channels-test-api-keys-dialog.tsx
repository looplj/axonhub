'use client';

import { useEffect, useMemo, useState } from 'react';
import { IconAlertTriangle, IconKey, IconLoader2, IconPlayerPlay, IconTrash } from '@tabler/icons-react';
import { useTranslation } from 'react-i18next';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Checkbox } from '@/components/ui/checkbox';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import { ScrollArea } from '@/components/ui/scroll-area';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { useChannels } from '../context/channels-context';
import { useDeleteDisabledChannelAPIKeys, useDisableChannelAPIKey, useTestChannelAPIKeys, useUpdateChannel } from '../data/channels';
import { TestAPIKeyResult } from '../data/schema';

interface ChannelsTestAPIKeysDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

export function ChannelsTestAPIKeysDialog({ open, onOpenChange }: ChannelsTestAPIKeysDialogProps) {
  const { t } = useTranslation();
  const { currentRow, setOpen } = useChannels();
  const [results, setResults] = useState<TestAPIKeyResult[]>([]);
  const [selectedKeys, setSelectedKeys] = useState<Set<number>>(new Set());
  const [confirmDeleteFailed, setConfirmDeleteFailed] = useState(false);

  const testAPIKeys = useTestChannelAPIKeys({ silent: true });
  const disableAPIKey = useDisableChannelAPIKey();
  const updateChannel = useUpdateChannel();
  const deleteDisabledAPIKeys = useDeleteDisabledChannelAPIKeys();

  useEffect(() => {
    if (open) {
      setResults([]);
      setSelectedKeys(new Set());
      setConfirmDeleteFailed(false);
    }
  }, [open, currentRow?.id]);

  const failedResultIndexes = useMemo(
    () => results.flatMap((result, index) => (!result.success ? [index] : [])),
    [results]
  );

  const isAllFailedSelected = failedResultIndexes.length > 0 && selectedKeys.size === failedResultIndexes.length;
  const isSomeFailedSelected = selectedKeys.size > 0 && selectedKeys.size < failedResultIndexes.length;

  const isPending = testAPIKeys.isPending || disableAPIKey.isPending || updateChannel.isPending || deleteDisabledAPIKeys.isPending;

  if (!currentRow) {
    return null;
  }

  const handleClose = () => {
    setOpen(null);
    onOpenChange(false);
    setResults([]);
    setSelectedKeys(new Set());
    setConfirmDeleteFailed(false);
  };

  const handleTestAll = async () => {
    try {
      const data = await testAPIKeys.mutateAsync({
        channelID: currentRow.id,
        modelID: currentRow.defaultTestModel || undefined,
      });
      setResults(data.results);
      setSelectedKeys(new Set(data.results.flatMap((result, index) => (!result.success ? [index] : []))));
    } catch {
      setResults([]);
    }
  };

  const handleSelectAllFailed = () => {
    if (isAllFailedSelected) {
      setSelectedKeys(new Set());
      return;
    }

    setSelectedKeys(new Set(failedResultIndexes));
  };

  const handleToggleFailed = (resultIndex: number, checked: boolean) => {
    setSelectedKeys((prev) => {
      const next = new Set(prev);
      if (checked) {
        next.add(resultIndex);
      } else {
        next.delete(resultIndex);
      }
      return next;
    });
  };

  const getSelectedAPIKeys = () => {
    const apiKeys = currentRow.credentials?.apiKeys ?? [];

    return [...selectedKeys]
      .sort((a, b) => a - b)
      .flatMap((index) => {
        const key = apiKeys[index];
        return key ? [key] : [];
      });
  };

  const handleDisableFailed = async () => {
    if (selectedKeys.size === 0) {
      return;
    }

    const keysToDisable = getSelectedAPIKeys();
    if (keysToDisable.length === 0) {
      return;
    }

    try {
      await Promise.all(keysToDisable.map((key) => disableAPIKey.mutateAsync({ channelID: currentRow.id, key })));
      handleClose();
    } catch {
      // handled by hook
    }
  };

  const handleDeleteFailed = async () => {
    if (selectedKeys.size === 0) {
      return;
    }

    const failedKeys = getSelectedAPIKeys();
    if (failedKeys.length === 0) {
      return;
    }

    try {
      const disabledKeys = failedKeys.filter((key) => currentRow.disabledAPIKeys?.some((item) => item.key === key));
      const activeKeys = failedKeys.filter((key) => !disabledKeys.includes(key));

      if (disabledKeys.length > 0) {
        await deleteDisabledAPIKeys.mutateAsync({ channelID: currentRow.id, keys: disabledKeys });
      }

      if (activeKeys.length > 0) {
        const remainingKeys = (currentRow.credentials?.apiKeys ?? []).filter((key) => !activeKeys.includes(key));
        await updateChannel.mutateAsync({
          id: currentRow.id,
          input: {
            credentials: {
              apiKeys: remainingKeys,
            },
          },
        });
      }

      handleClose();
    } catch {
      // handled by hooks
    }
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className='flex max-h-[90vh] flex-col sm:max-w-3xl'>
        <DialogHeader>
          <DialogTitle className='flex items-center gap-2'>
            <IconKey className='h-5 w-5' />
            {t('channels.dialogs.testAPIKeys.title')}
          </DialogTitle>
          <DialogDescription>{t('channels.dialogs.testAPIKeys.description', { name: currentRow.name })}</DialogDescription>
        </DialogHeader>

        <div className='min-h-0 flex-1 space-y-4'>
          {results.length > 0 && (
            <div className='flex items-center justify-between gap-4 rounded-md border bg-muted/40 px-4 py-3'>
              <div className='text-sm font-medium'>
                {t('channels.dialogs.testAPIKeys.successSummary', {
                  success: results.filter((result) => result.success).length,
                  total: results.length,
                })}
              </div>
              {failedResultIndexes.length > 0 && (
                <div className='flex items-center gap-2'>
                  <Checkbox
                    checked={isAllFailedSelected || (isSomeFailedSelected && 'indeterminate')}
                    onCheckedChange={handleSelectAllFailed}
                    aria-label={t('common.columns.selectAll')}
                  />
                  <span className='text-muted-foreground text-sm'>{t('common.columns.selectAll')}</span>
                </div>
              )}
            </div>
          )}

          <div className='min-h-0 flex-1 overflow-hidden rounded-lg border'>
            <ScrollArea className='h-[420px]'>
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead className='w-12'></TableHead>
                    <TableHead>{t('channels.dialogs.testAPIKeys.keyColumn')}</TableHead>
                    <TableHead className='w-32'>{t('channels.dialogs.testAPIKeys.statusColumn')}</TableHead>
                    <TableHead className='w-28'>{t('channels.dialogs.testAPIKeys.latencyColumn')}</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {results.length === 0 ? (
                    <TableRow>
                      <TableCell colSpan={4} className='h-32 text-center text-sm text-muted-foreground'>
                        {testAPIKeys.isPending ? t('channels.dialogs.testAPIKeys.testing') : t('channels.dialogs.testAPIKeys.empty')}
                      </TableCell>
                    </TableRow>
                  ) : (
                    results.map((result, index) => (
                      <TableRow key={index}>
                        <TableCell>
                          {!result.success ? (
                            <Checkbox
                              checked={selectedKeys.has(index)}
                              onCheckedChange={(checked) => handleToggleFailed(index, checked === true)}
                            />
                          ) : null}
                        </TableCell>
                        <TableCell className='font-medium'>
                          <div className='flex items-center gap-2'>
                            <code className='bg-muted rounded px-2 py-0.5 font-mono text-sm'>{result.keyPrefix}</code>
                            {result.disabled && <Badge variant='secondary'>{t('channels.dialogs.testAPIKeys.disabledBadge')}</Badge>}
                          </div>
                          {result.error && (
                            <div className='mt-1 flex items-start gap-1 text-xs text-destructive'>
                              <IconAlertTriangle className='mt-0.5 h-3 w-3 shrink-0' />
                              <span className='whitespace-normal break-all'>{result.error}</span>
                            </div>
                          )}
                        </TableCell>
                        <TableCell>
                          {result.success ? (
                            <Badge variant='default' className='border-green-200 bg-green-100 text-green-800'>
                              {t('channels.dialogs.testAPIKeys.success')}
                            </Badge>
                          ) : (
                            <Badge variant='destructive'>{t('channels.dialogs.testAPIKeys.failed')}</Badge>
                          )}
                        </TableCell>
                        <TableCell>{result.latency > 0 ? `${result.latency.toFixed(2)}s` : '-'}</TableCell>
                      </TableRow>
                    ))
                  )}
                </TableBody>
              </Table>
            </ScrollArea>
          </div>
        </div>

        <DialogFooter className='flex items-center justify-between sm:justify-between'>
          <div className='flex items-center gap-2'>
            {selectedKeys.size > 0 && (
              <>
                  <Button variant='outline' onClick={handleDisableFailed} disabled={isPending}>
                  {disableAPIKey.isPending ? (
                    <IconLoader2 className='mr-2 h-4 w-4 animate-spin' />
                  ) : (
                    <IconAlertTriangle className='mr-2 h-4 w-4' />
                  )}
                  {t('channels.dialogs.testAPIKeys.disableFailed')}
                </Button>
                <Popover open={confirmDeleteFailed} onOpenChange={setConfirmDeleteFailed}>
                  <PopoverTrigger asChild>
                    <Button variant='destructive' disabled={isPending}>
                      <IconTrash className='mr-2 h-4 w-4' />
                      {t('channels.dialogs.testAPIKeys.deleteFailed')}
                    </Button>
                  </PopoverTrigger>
                  <PopoverContent className='w-80'>
                    <div className='flex flex-col gap-3'>
                      <p className='text-sm'>
                        {t('channels.dialogs.testAPIKeys.confirmDeleteFailed', { count: selectedKeys.size })}
                      </p>
                      <div className='flex justify-end gap-2'>
                        <Button size='sm' variant='outline' onClick={() => setConfirmDeleteFailed(false)}>
                          {t('common.buttons.cancel')}
                        </Button>
                        <Button size='sm' variant='destructive' onClick={handleDeleteFailed} disabled={isPending}>
                          {t('common.buttons.confirm')}
                        </Button>
                      </div>
                    </div>
                  </PopoverContent>
                </Popover>
              </>
            )}
          </div>
          <div className='flex items-center gap-2'>
            <Button variant='outline' onClick={handleClose}>
              {t('common.buttons.close')}
            </Button>
            <Button onClick={handleTestAll} disabled={isPending}>
              {testAPIKeys.isPending ? (
                <IconLoader2 className='mr-2 h-4 w-4 animate-spin' />
              ) : (
                <IconPlayerPlay className='mr-2 h-4 w-4' />
              )}
              {testAPIKeys.isPending ? t('channels.dialogs.testAPIKeys.testing') : t('channels.dialogs.testAPIKeys.testAll')}
            </Button>
          </div>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
