'use client';

import { useEffect, useMemo, useRef, useState } from 'react';
import {
  IconAlertTriangle,
  IconCopy,
  IconKey,
  IconLoader2,
  IconPlayerPlay,
  IconRefresh,
  IconTrash,
  IconUpload,
} from '@tabler/icons-react';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Checkbox } from '@/components/ui/checkbox';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import { ScrollArea } from '@/components/ui/scroll-area';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { Textarea } from '@/components/ui/textarea';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import { useChannels } from '../context/channels-context';
import {
  useChannelDisabledAPIKeys,
  useDeleteDisabledChannelAPIKeys,
  useDisableChannelAPIKey,
  useEnableChannelAPIKey,
  useTestChannelAPIKey,
  useUpdateChannel,
} from '../data/channels';
import { TestAPIKeyResult } from '../data/schema';

interface ChannelsAPIKeyManagementDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

function maskAPIKey(key: string) {
  if (key.length <= 8) {
    return '****';
  }
  return `${key.slice(0, 4)}****${key.slice(-4)}`;
}

/**
 * 统一密钥管理弹窗：
 * - 始终可从行操作菜单进入（无论 1 个还是多个 key）
 * - 批量导入（多行文本，一行一个）
 * - 单个/批量：测试、复制、禁用/启用、删除
 * - 密钥状态（正常 / 已禁用 / 测试中 / 测试结果）实时动态更新
 */
export function ChannelsAPIKeyManagementDialog({ open, onOpenChange }: ChannelsAPIKeyManagementDialogProps) {
  const { t } = useTranslation();
  const { currentRow, setOpen } = useChannels();
  const [bulkImportText, setBulkImportText] = useState('');
  const [selectedKeys, setSelectedKeys] = useState<Set<string>>(new Set());
  const [testingKey, setTestingKey] = useState<string | null>(null);
  const [testResults, setTestResults] = useState<Record<string, TestAPIKeyResult>>({});
  const [confirmDisableKey, setConfirmDisableKey] = useState<string | null>(null);
  const [confirmEnableKey, setConfirmEnableKey] = useState<string | null>(null);
  const [confirmDeleteKey, setConfirmDeleteKey] = useState<string | null>(null);
  const [confirmDeleteSelected, setConfirmDeleteSelected] = useState(false);
  // 本弹窗独立的 key 列表：基于 currentRow 快照初始化，导入/删除后本地更新，
  // 避免依赖不会随 query 刷新而更新的 currentRow 快照。
  const [localKeys, setLocalKeys] = useState<string[]>([]);
  const abortRef = useRef(false);

  const testSingleKey = useTestChannelAPIKey();
  const disableAPIKey = useDisableChannelAPIKey();
  const enableAPIKey = useEnableChannelAPIKey();
  const updateChannel = useUpdateChannel();
  const deleteDisabledAPIKeys = useDeleteDisabledChannelAPIKeys();

  const validKeys = useMemo(() => localKeys.filter((key) => key.trim().length > 0), [localKeys]);
  // 服务端实时禁用的 key 集合（所有 disable/enable mutation 都会 invalidate 此 query）
  const { data: disabledKeys = [], isLoading: disabledLoading } = useChannelDisabledAPIKeys(currentRow?.id || '', {
    enabled: open && !!currentRow?.id,
  });
  const disabledKeySet = useMemo(() => new Set(disabledKeys.map((item) => item.key)), [disabledKeys]);

  // 本弹窗内测试后得到的"已禁用"结果（测试接口会动态反馈真实状态）
  const effectivelyDisabledSet = useMemo(() => {
    const set = new Set(disabledKeySet);
    Object.entries(testResults).forEach(([key, result]) => {
      if (result.disabled) {
        set.add(key);
      }
    });
    return set;
  }, [disabledKeySet, testResults]);

  // 当前可用的 key 数量（未被禁用的），与编辑弹窗的 mustKeepOneEnabled 保护一致
  const enabledKeysCount = useMemo(
    () => validKeys.filter((key) => !effectivelyDisabledSet.has(key)).length,
    [validKeys, effectivelyDisabledSet]
  );
  const isLastKey = validKeys.length <= 1;

  const isPending =
    testingKey !== null || disableAPIKey.isPending || enableAPIKey.isPending || updateChannel.isPending || deleteDisabledAPIKeys.isPending;

  useEffect(() => {
    if (open) {
      setBulkImportText('');
      setLocalKeys(currentRow?.credentials?.apiKeys ?? []);
      setSelectedKeys(new Set());
      setTestingKey(null);
      setTestResults({});
      setConfirmDisableKey(null);
      setConfirmEnableKey(null);
      setConfirmDeleteKey(null);
      setConfirmDeleteSelected(false);
      abortRef.current = false;
    }
    // 仅在弹窗打开或切换渠道时初始化一次 localKeys；
    // 依赖 currentRow.credentials.apiKeys 会导致导入/删除后的本地更新被覆盖。
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, currentRow?.id]);

  if (!currentRow) {
    return null;
  }

  const handleClose = () => {
    abortRef.current = true;
    setOpen(null);
    onOpenChange(false);
    setBulkImportText('');
    setSelectedKeys(new Set());
    setTestingKey(null);
    setTestResults({});
    setConfirmDisableKey(null);
    setConfirmEnableKey(null);
    setConfirmDeleteKey(null);
    setConfirmDeleteSelected(false);
  };

  /** 批量导入：多行文本，一行一个 key，去重、去空白，追加到渠道 apiKeys */
  const handleBulkImport = async () => {
    const newKeys = bulkImportText
      .split('\n')
      .map((k) => k.trim())
      .filter((k) => k.length > 0);
    if (newKeys.length === 0) {
      return;
    }

    const existing = new Set(validKeys);
    // 先对输入自身去重（同一 key 粘贴多行时只保留一个）
    const deduped = [...new Set(newKeys)].filter((k) => !existing.has(k));
    if (deduped.length === 0) {
      setBulkImportText('');
      return;
    }

    try {
      await updateChannel.mutateAsync({
        id: currentRow.id,
        input: {
          credentials: {
            apiKeys: [...validKeys, ...deduped],
          },
        },
      });
      // 本地同步更新，避免依赖不会刷新的 currentRow 快照
      setLocalKeys((prev) => [...prev, ...deduped]);
      setBulkImportText('');
    } catch {
      // handled by hook
    }
  };

  /** 删除 key（活动 key 从 credentials 移除；已禁用 key 走 deleteDisabledAPIKeys） */
  const deleteKeys = async (keysToDelete: string[]) => {
    if (keysToDelete.length === 0) {
      return;
    }

    const disabled = keysToDelete.filter((key) => effectivelyDisabledSet.has(key));
    const active = keysToDelete.filter((key) => !effectivelyDisabledSet.has(key));

    if (disabled.length > 0) {
      await deleteDisabledAPIKeys.mutateAsync({ channelID: currentRow.id, keys: disabled });
    }
    // 注意：remainingKeys 必须同时过滤 active 与 disabled，
    // 避免把上面 deleteDisabledAPIKeys 已从 credentials 移除的 disabled key 写回去。
    const removed = new Set([...active, ...disabled]);
    const remainingKeys = validKeys.filter((key) => !removed.has(key));
    if (active.length > 0) {
      await updateChannel.mutateAsync({
        id: currentRow.id,
        input: { credentials: { apiKeys: remainingKeys } },
      });
    }
    // 本地同步更新（disabled 删除走 deleteDisabledAPIKeys 不触发 updateChannel，
    // 但同样要从弹窗列表里移除）
    setLocalKeys(remainingKeys);
  };

  const handleDeleteSingle = async (key: string) => {
    try {
      await deleteKeys([key]);
      setConfirmDeleteKey(null);
    } catch {
      // handled by hooks
    }
  };

  const handleDeleteSelected = async () => {
    if (selectedKeys.size === 0 || selectedKeys.size >= validKeys.length) {
      return;
    }
    try {
      await deleteKeys(Array.from(selectedKeys));
      setConfirmDeleteSelected(false);
      setSelectedKeys(new Set());
    } catch {
      // handled by hooks
    }
  };

  const handleTestSingle = async (key: string) => {
    if (testingKey) {
      return;
    }
    setTestingKey(key);
    try {
      const result = await testSingleKey.mutateAsync({
        channelID: currentRow.id,
        key,
        modelID: currentRow.defaultTestModel || undefined,
      });
      if (!abortRef.current) {
        setTestResults((prev) => ({ ...prev, [key]: result }));
      }
    } catch {
      if (!abortRef.current) {
        setTestResults((prev) => ({
          ...prev,
          [key]: { keyPrefix: maskAPIKey(key), success: false, latency: 0, error: t('channels.dialogs.testAPIKeys.requestFailed'), disabled: false },
        }));
      }
    } finally {
      if (!abortRef.current) {
        setTestingKey(null);
      }
    }
  };

  const handleTestSelected = async () => {
    const keysToTest = validKeys.filter((key) => selectedKeys.has(key));
    if (keysToTest.length === 0 || testingKey) {
      return;
    }

    abortRef.current = false;
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
        if (!abortRef.current) {
          setTestResults((prev) => ({ ...prev, [key]: result }));
        }
      } catch {
        if (!abortRef.current) {
          setTestResults((prev) => ({
            ...prev,
            [key]: { keyPrefix: maskAPIKey(key), success: false, latency: 0, error: t('channels.dialogs.testAPIKeys.requestFailed'), disabled: false },
          }));
        }
      }
      if (!abortRef.current) {
        setTestingKey(null);
      }
    }
    setTestingKey(null);
  };

  const handleToggleDisable = async (key: string) => {
    try {
      await disableAPIKey.mutateAsync({ channelID: currentRow.id, key });
      setConfirmDisableKey(null);
    } catch {
      // handled by hooks
    }
  };

  const handleToggleEnable = async (key: string) => {
    try {
      await enableAPIKey.mutateAsync({ channelID: currentRow.id, key });
      setConfirmEnableKey(null);
    } catch {
      // handled by hooks
    }
  };

  const handleSelectAll = () => {
    if (selectedKeys.size === validKeys.length) {
      setSelectedKeys(new Set());
    } else {
      setSelectedKeys(new Set(validKeys));
    }
  };

  const isAllSelected = validKeys.length > 0 && selectedKeys.size === validKeys.length;
  const isSomeSelected = selectedKeys.size > 0 && selectedKeys.size < validKeys.length;

  const getKeyResult = (key: string): TestAPIKeyResult | undefined => testResults[key];
  const isKeyDisabled = (key: string) => effectivelyDisabledSet.has(key);

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className='flex max-h-[90vh] flex-col sm:max-w-4xl'>
        <DialogHeader>
          <DialogTitle className='flex items-center gap-2'>
            <IconKey className='h-5 w-5' />
            {t('channels.dialogs.keyManagement.title')}
          </DialogTitle>
          <DialogDescription>{t('channels.dialogs.keyManagement.description', { name: currentRow.name })}</DialogDescription>
        </DialogHeader>

        <div className='flex min-h-0 flex-1 flex-col gap-4'>
          {/* 批量导入区 */}
          <div className='rounded-md border bg-muted/40 p-3'>
            <div className='flex items-center justify-between gap-2'>
              <div className='flex items-center gap-2 text-sm font-medium'>
                <IconUpload className='h-4 w-4' />
                {t('channels.dialogs.keyManagement.importTitle')}
              </div>
              <span className='text-muted-foreground text-xs'>{t('channels.dialogs.keyManagement.importHint')}</span>
            </div>
            <div className='mt-2 flex items-start gap-2'>
              <Textarea
                value={bulkImportText}
                onChange={(e) => setBulkImportText(e.target.value)}
                placeholder={t('channels.dialogs.keyManagement.importPlaceholder')}
                className='min-h-[64px] flex-1 font-mono text-xs'
              />
              <Button onClick={handleBulkImport} disabled={isPending || bulkImportText.trim().length === 0}>
                {updateChannel.isPending ? <IconLoader2 className='mr-2 h-4 w-4 animate-spin' /> : <IconUpload className='mr-2 h-4 w-4' />}
                {t('channels.dialogs.keyManagement.importButton')}
              </Button>
            </div>
          </div>

          {/* 表格 */}
          <div className='flex min-h-0 flex-1 flex-col overflow-hidden rounded-lg border'>
            {/* 批量操作头 */}
            <div className='flex items-center justify-between gap-4 rounded-t-lg border-b bg-muted/40 px-4 py-2'>
              <div className='flex items-center gap-2'>
                <Checkbox
                  checked={isAllSelected || (isSomeSelected && 'indeterminate')}
                  onCheckedChange={handleSelectAll}
                  aria-label={t('common.columns.selectAll')}
                />
                <span className='text-muted-foreground text-sm'>
                  {selectedKeys.size > 0
                    ? t('channels.dialogs.keyManagement.selectedCount', { count: selectedKeys.size })
                    : t('channels.dialogs.keyManagement.totalCount', { count: validKeys.length })}
                </span>
              </div>
              <div className='flex items-center gap-1.5'>
                <Button size='sm' variant='outline' onClick={handleTestSelected} disabled={isPending || selectedKeys.size === 0}>
                  <IconPlayerPlay className='mr-1 h-3.5 w-3.5' />
                  {t('channels.dialogs.keyManagement.testSelected', { count: selectedKeys.size })}
                </Button>
                <Popover open={confirmDeleteSelected} onOpenChange={setConfirmDeleteSelected}>
                  <PopoverTrigger asChild>
                    <Button
                      size='sm'
                      variant='outline'
                      className='text-destructive'
                      disabled={isPending || selectedKeys.size === 0 || selectedKeys.size >= validKeys.length}
                    >
                      <IconTrash className='mr-1 h-3.5 w-3.5' />
                      {t('channels.dialogs.keyManagement.deleteSelected', { count: selectedKeys.size })}
                    </Button>
                  </PopoverTrigger>
                  <PopoverContent className='w-80'>
                    <div className='flex flex-col gap-3'>
                      <p className='text-sm'>
                        {t('channels.dialogs.keyManagement.confirmDeleteSelected', { count: selectedKeys.size })}
                      </p>
                      <div className='flex justify-end gap-2'>
                        <Button size='sm' variant='outline' onClick={() => setConfirmDeleteSelected(false)}>
                          {t('common.buttons.cancel')}
                        </Button>
                        <Button size='sm' variant='destructive' onClick={handleDeleteSelected} disabled={isPending}>
                          {t('common.buttons.confirm')}
                        </Button>
                      </div>
                    </div>
                  </PopoverContent>
                </Popover>
              </div>
            </div>

            <ScrollArea className='min-h-0 flex-1'>
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead className='w-12'></TableHead>
                    <TableHead>{t('channels.dialogs.keyManagement.keyColumn')}</TableHead>
                    <TableHead className='w-40'>{t('channels.dialogs.keyManagement.statusColumn')}</TableHead>
                    <TableHead className='w-28'>{t('channels.dialogs.keyManagement.latencyColumn')}</TableHead>
                    <TableHead className='w-36'>{t('channels.dialogs.keyManagement.actionsColumn')}</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {disabledLoading && validKeys.length === 0 ? (
                    <TableRow>
                      <TableCell colSpan={5} className='h-24 text-center'>
                        <IconLoader2 className='text-muted-foreground mx-auto h-5 w-5 animate-spin' />
                      </TableCell>
                    </TableRow>
                  ) : validKeys.length === 0 ? (
                    <TableRow>
                      <TableCell colSpan={5} className='h-32 text-center text-sm text-muted-foreground'>
                        {t('channels.dialogs.keyManagement.noKeys')}
                      </TableCell>
                    </TableRow>
                  ) : (
                    validKeys.map((key) => {
                      const isTesting = testingKey === key;
                      const isDisabled = isKeyDisabled(key);
                      const result = getKeyResult(key);
                      const handleCopy = async () => {
                        try {
                          await navigator.clipboard.writeText(key);
                          toast.success(t('channels.dialogs.keyManagement.copySuccess'));
                        } catch {
                          toast.error(t('common.errors.copyFailed'));
                        }
                      };
                      return (
                        <TableRow key={key}>
                          <TableCell>
                            <Checkbox
                              checked={selectedKeys.has(key)}
                              onCheckedChange={(checked) => {
                                setSelectedKeys((prev) => {
                                  const next = new Set(prev);
                                  if (checked) {
                                    next.add(key);
                                  } else {
                                    next.delete(key);
                                  }
                                  return next;
                                });
                              }}
                            />
                          </TableCell>
                          <TableCell className='font-medium'>
                            <div className='flex items-center gap-2'>
                              {isTesting && <IconLoader2 className='h-3 w-3 animate-spin text-muted-foreground' />}
                              <code className='bg-muted rounded px-2 py-0.5 font-mono text-sm'>
                                {result ? result.keyPrefix : maskAPIKey(key)}
                              </code>
                            </div>
                            {result?.error && (
                              <div className='mt-1 flex items-start gap-1 text-xs text-destructive'>
                                <IconAlertTriangle className='mt-0.5 h-3 w-3 shrink-0' />
                                <span className='whitespace-normal break-all'>{result.error}</span>
                              </div>
                            )}
                          </TableCell>
                          <TableCell>
                            {isDisabled ? (
                              <Badge variant='secondary'>{t('channels.dialogs.keyManagement.disabled')}</Badge>
                            ) : result ? (
                              result.success ? (
                                <Badge variant='default' className='border-green-200 bg-green-100 text-green-800'>
                                  {t('channels.dialogs.keyManagement.success')}
                                </Badge>
                              ) : (
                                <Badge variant='destructive'>{t('channels.dialogs.keyManagement.failed')}</Badge>
                              )
                            ) : (
                              <span className='text-muted-foreground'>-</span>
                            )}
                          </TableCell>
                          <TableCell>{result && result.latency > 0 ? `${result.latency.toFixed(2)}s` : '-'}</TableCell>
                          <TableCell>
                            <div className='flex items-center gap-0.5'>
                              <Tooltip>
                                <TooltipTrigger asChild>
                                  <Button size='sm' variant='ghost' className='h-7 w-7 p-0' onClick={() => handleTestSingle(key)} disabled={isPending}>
                                    <IconPlayerPlay className='h-4 w-4' />
                                  </Button>
                                </TooltipTrigger>
                                <TooltipContent>{t('channels.dialogs.keyManagement.test')}</TooltipContent>
                              </Tooltip>
                              <Tooltip>
                                <TooltipTrigger asChild>
                                  <Button size='sm' variant='ghost' className='h-7 w-7 p-0' onClick={handleCopy}>
                                    <IconCopy className='h-4 w-4' />
                                  </Button>
                                </TooltipTrigger>
                                <TooltipContent>{t('channels.dialogs.keyManagement.copy')}</TooltipContent>
                              </Tooltip>

                              {isDisabled ? (
                                <Popover open={confirmEnableKey === key} onOpenChange={(o) => setConfirmEnableKey(o ? key : null)}>
                                  <PopoverTrigger asChild>
                                    <Button size='sm' variant='ghost' className='h-7 w-7 p-0' disabled={isPending}>
                                      <IconRefresh className='h-4 w-4' />
                                    </Button>
                                  </PopoverTrigger>
                                  <PopoverContent className='w-64'>
                                    <div className='flex flex-col gap-3'>
                                      <p className='text-sm'>{t('channels.dialogs.keyManagement.confirmEnable')}</p>
                                      <div className='flex justify-end gap-2'>
                                        <Button size='sm' variant='outline' onClick={() => setConfirmEnableKey(null)}>
                                          {t('common.buttons.cancel')}
                                        </Button>
                                        <Button size='sm' onClick={() => handleToggleEnable(key)} disabled={isPending}>
                                          {t('common.buttons.confirm')}
                                        </Button>
                                      </div>
                                    </div>
                                  </PopoverContent>
                                </Popover>
                              ) : (
                                <Popover open={confirmDisableKey === key} onOpenChange={(o) => setConfirmDisableKey(o ? key : null)}>
                                  <PopoverTrigger asChild>
                                    <Button size='sm' variant='ghost' className='h-7 w-7 p-0' disabled={isPending || enabledKeysCount <= 1}>
                                      <IconAlertTriangle className='h-4 w-4' />
                                    </Button>
                                  </PopoverTrigger>
                                  <PopoverContent className='w-64'>
                                    <div className='flex flex-col gap-3'>
                                      <p className='text-sm'>{t('channels.dialogs.keyManagement.confirmDisable')}</p>
                                      <div className='flex justify-end gap-2'>
                                        <Button size='sm' variant='outline' onClick={() => setConfirmDisableKey(null)}>
                                          {t('common.buttons.cancel')}
                                        </Button>
                                        <Button size='sm' onClick={() => handleToggleDisable(key)} disabled={isPending}>
                                          {t('common.buttons.confirm')}
                                        </Button>
                                      </div>
                                    </div>
                                  </PopoverContent>
                                </Popover>
                              )}

                              <Popover open={confirmDeleteKey === key} onOpenChange={(o) => setConfirmDeleteKey(o ? key : null)}>
                                <PopoverTrigger asChild>
                                  <Button size='sm' variant='ghost' className='text-destructive h-7 w-7 p-0' disabled={isPending || isLastKey}>
                                    <IconTrash className='h-4 w-4' />
                                  </Button>
                                </PopoverTrigger>
                                <PopoverContent className='w-64'>
                                  <div className='flex flex-col gap-3'>
                                    <p className='text-sm'>{t('channels.dialogs.keyManagement.confirmDelete')}</p>
                                    <div className='flex justify-end gap-2'>
                                      <Button size='sm' variant='outline' onClick={() => setConfirmDeleteKey(null)}>
                                        {t('common.buttons.cancel')}
                                      </Button>
                                      <Button size='sm' variant='destructive' onClick={() => handleDeleteSingle(key)} disabled={isPending}>
                                        {t('common.buttons.confirm')}
                                      </Button>
                                    </div>
                                  </div>
                                </PopoverContent>
                              </Popover>
                            </div>
                          </TableCell>
                        </TableRow>
                      );
                    })
                  )}
                </TableBody>
              </Table>
            </ScrollArea>
          </div>
        </div>

        <DialogFooter className='flex items-center justify-between sm:justify-between'>
          <div className='text-muted-foreground text-xs'>
            {isPending ? t('channels.dialogs.keyManagement.processing') : t('channels.dialogs.keyManagement.footerHint')}
          </div>
          <Button variant='outline' onClick={handleClose}>
            {t('common.buttons.close')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
