'use client';

import { useEffect, useMemo, useRef, useState } from 'react';
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
import { useDeleteDisabledChannelAPIKeys, useDisableChannelAPIKey, useTestChannelAPIKey, useUpdateChannel } from '../data/channels';
import { TestAPIKeyResult } from '../data/schema';

interface ChannelsTestAPIKeysDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

function maskAPIKey(key: string) {
  if (key.length <= 8) {
    return '****';
  }
  return `${key.slice(0, 4)}****${key.slice(-4)}`;
}

export function ChannelsTestAPIKeysDialog({ open, onOpenChange }: ChannelsTestAPIKeysDialogProps) {
  const { t } = useTranslation();
  const { currentRow, setOpen } = useChannels();
  const [results, setResults] = useState<TestAPIKeyResult[]>([]);
  const [testedKeys, setTestedKeys] = useState<string[]>([]);
  const [selectedKeys, setSelectedKeys] = useState<Set<string>>(new Set());
  const [confirmDeleteFailed, setConfirmDeleteFailed] = useState(false);
  const [testingKey, setTestingKey] = useState<string | null>(null);
  const abortRef = useRef(false);

  const testSingleKey = useTestChannelAPIKey();
  const disableAPIKey = useDisableChannelAPIKey();
  const updateChannel = useUpdateChannel();
  const deleteDisabledAPIKeys = useDeleteDisabledChannelAPIKeys();

  const allKeys = useMemo(() => currentRow?.credentials?.apiKeys ?? [], [currentRow?.credentials?.apiKeys]);
  const disabledKeySet = useMemo(
    () => new Set(currentRow?.disabledAPIKeys?.map((item) => item.key) ?? []),
    [currentRow?.disabledAPIKeys]
  );

  const isTested = results.length > 0;
  const isTesting = testingKey !== null;

  useEffect(() => {
    if (open) {
      setResults([]);
      setTestedKeys([]);
      setSelectedKeys(new Set(allKeys));
      setConfirmDeleteFailed(false);
      setTestingKey(null);
      abortRef.current = false;
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, currentRow?.id]);

  const failedKeys = useMemo(() => {
    if (!isTested) {
      return new Set<string>();
    }
    const failed = new Set<string>();
    results.forEach((result, index) => {
      if (!result.success) {
        const key = testedKeys[index];
        if (key) {
          failed.add(key);
        }
      }
    });
    return failed;
  }, [isTested, results, testedKeys]);

  const selectableKeys = useMemo(() => {
    if (isTested) {
      return failedKeys;
    }
    return new Set(allKeys);
  }, [isTested, failedKeys, allKeys]);

  const isAllSelected = selectableKeys.size > 0 && [...selectableKeys].every((key) => selectedKeys.has(key));
  const isSomeSelected = [...selectableKeys].some((key) => selectedKeys.has(key)) && !isAllSelected;

  const isPending = isTesting || disableAPIKey.isPending || updateChannel.isPending || deleteDisabledAPIKeys.isPending;

  if (!currentRow) {
    return null;
  }

  const handleClose = () => {
    abortRef.current = true;
    setOpen(null);
    onOpenChange(false);
    setResults([]);
    setTestedKeys([]);
    setSelectedKeys(new Set());
    setConfirmDeleteFailed(false);
    setTestingKey(null);
  };

  const handleSelectAll = () => {
    if (isAllSelected) {
      setSelectedKeys(new Set());
      return;
    }
    setSelectedKeys(new Set(selectableKeys));
  };

  const handleToggle = (key: string, checked: boolean) => {
    setSelectedKeys((prev) => {
      const next = new Set(prev);
      if (checked) {
        next.add(key);
      } else {
        next.delete(key);
      }
      return next;
    });
  };

  const handleTest = async () => {
    if (selectedKeys.size === 0) {
      return;
    }

    const keysToTest = allKeys.filter((key) => selectedKeys.has(key));
    if (keysToTest.length === 0) {
      return;
    }

    abortRef.current = false;
    const newResults: TestAPIKeyResult[] = [];
    setResults([]);
    setTestedKeys(keysToTest);

    for (const key of keysToTest) {
      if (abortRef.current) {
        break;
      }

      setTestingKey(key);

      try {
        const result = await testSingleKey.mutateAsync({
          channelID: currentRow.id,
          key,
          modelID: currentRow.defaultTestModel || undefined,
        });
        newResults.push(result);
        setResults([...newResults]);
      } catch {
        newResults.push({
          keyPrefix: maskAPIKey(key),
          success: false,
          latency: 0,
          error: t('channels.dialogs.testAPIKeys.requestFailed'),
          disabled: disabledKeySet.has(key),
        });
        setResults([...newResults]);
      }

      setTestingKey(null);
    }

    if (!abortRef.current) {
      const nextSelected = new Set<string>();
      newResults.forEach((result, index) => {
        if (!result.success) {
          const key = keysToTest[index];
          if (key) {
            nextSelected.add(key);
          }
        }
      });
      setSelectedKeys(nextSelected);
    }

    setTestingKey(null);
  };

  const getSelectedFailedKeys = () => {
    return [...selectedKeys].filter((key) => failedKeys.has(key));
  };

  const handleDisableFailed = async () => {
    const keysToDisable = getSelectedFailedKeys();
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
    const failedKeysToDelete = getSelectedFailedKeys();
    if (failedKeysToDelete.length === 0) {
      return;
    }

    try {
      const disabledKeys = failedKeysToDelete.filter((key) => disabledKeySet.has(key));
      const activeKeys = failedKeysToDelete.filter((key) => !disabledKeys.includes(key));

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

  const renderKeyCell = (key: string, result?: TestAPIKeyResult) => (
    <TableCell className='font-medium'>
      <div className='flex items-center gap-2'>
        {isTesting && testingKey === key && (
          <IconLoader2 className='h-3 w-3 animate-spin text-muted-foreground' />
        )}
        <code className='bg-muted rounded px-2 py-0.5 font-mono text-sm'>
          {result ? result.keyPrefix : maskAPIKey(key)}
        </code>
        {(result ? result.disabled : disabledKeySet.has(key)) && (
          <Badge variant='secondary'>{t('channels.dialogs.testAPIKeys.disabledBadge')}</Badge>
        )}
      </div>
      {result?.error && (
        <div className='mt-1 flex items-start gap-1 text-xs text-destructive'>
          <IconAlertTriangle className='mt-0.5 h-3 w-3 shrink-0' />
          <span className='whitespace-normal break-all'>{result.error}</span>
        </div>
      )}
    </TableCell>
  );

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

        <div className='flex min-h-0 flex-1 flex-col gap-4'>
          {allKeys.length > 0 && (
            <div className='flex items-center justify-between gap-4 rounded-md border bg-muted/40 px-4 py-3'>
              <div className='text-sm font-medium'>
                {isTested
                  ? t('channels.dialogs.testAPIKeys.successSummary', {
                      success: results.filter((result) => result.success).length,
                      total: results.length,
                    })
                  : t('channels.dialogs.testAPIKeys.selectedCount', { count: selectedKeys.size })}
              </div>
              <div className='flex items-center gap-2'>
                <Checkbox
                  checked={isAllSelected || (isSomeSelected && 'indeterminate')}
                  onCheckedChange={handleSelectAll}
                  aria-label={t('common.columns.selectAll')}
                />
                <span className='text-muted-foreground text-sm'>
                  {isTested ? t('channels.dialogs.testAPIKeys.selectAllFailed') : t('common.columns.selectAll')}
                </span>
              </div>
            </div>
          )}

          <div className='flex min-h-0 flex-1 flex-col overflow-hidden rounded-lg border'>
            <ScrollArea className='min-h-0 flex-1'>
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
                  {allKeys.length === 0 ? (
                    <TableRow>
                      <TableCell colSpan={4} className='h-32 text-center text-sm text-muted-foreground'>
                        {t('channels.dialogs.testAPIKeys.noKeys')}
                      </TableCell>
                    </TableRow>
                  ) : isTested ? (
                    testedKeys.map((key, index) => {
                      const result = results[index];
                      const isCurrentlyTesting = isTesting && testingKey === key;
                      return (
                        <TableRow key={index}>
                          <TableCell>
                            {result && !result.success && key && (
                              <Checkbox
                                checked={selectedKeys.has(key)}
                                onCheckedChange={(checked) => handleToggle(key, checked === true)}
                              />
                            )}
                          </TableCell>
                          {renderKeyCell(key, result)}
                          <TableCell>
                            {isCurrentlyTesting ? (
                              <IconLoader2 className='h-4 w-4 animate-spin text-muted-foreground' />
                            ) : result ? (
                              result.success ? (
                                <Badge variant='default' className='border-green-200 bg-green-100 text-green-800'>
                                  {t('channels.dialogs.testAPIKeys.success')}
                                </Badge>
                              ) : (
                                <Badge variant='destructive'>{t('channels.dialogs.testAPIKeys.failed')}</Badge>
                              )
                            ) : (
                              <span className='text-muted-foreground'>-</span>
                            )}
                          </TableCell>
                          <TableCell>{result && result.latency > 0 ? `${result.latency.toFixed(2)}s` : '-'}</TableCell>
                        </TableRow>
                      );
                    })
                  ) : (
                    allKeys.map((key, index) => (
                      <TableRow key={index}>
                        <TableCell>
                          <Checkbox
                            checked={selectedKeys.has(key)}
                            onCheckedChange={(checked) => handleToggle(key, checked === true)}
                          />
                        </TableCell>
                        {renderKeyCell(key)}
                        <TableCell className='text-muted-foreground'>-</TableCell>
                        <TableCell className='text-muted-foreground'>-</TableCell>
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
            {isTested && selectedKeys.size > 0 && !isTesting && (
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
            <Button onClick={handleTest} disabled={isPending || selectedKeys.size === 0}>
              {isTesting ? (
                <IconLoader2 className='mr-2 h-4 w-4 animate-spin' />
              ) : (
                <IconPlayerPlay className='mr-2 h-4 w-4' />
              )}
              {isTesting
                ? t('channels.dialogs.testAPIKeys.testing')
                : t('channels.dialogs.testAPIKeys.testSelected', { count: selectedKeys.size })}
            </Button>
          </div>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}