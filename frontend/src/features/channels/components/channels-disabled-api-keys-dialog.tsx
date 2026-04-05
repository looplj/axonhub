import { useState, useMemo } from 'react';
import { format } from 'date-fns';
import { useTranslation } from 'react-i18next';
import { IconRefresh, IconRefreshOff, IconKey, IconAlertTriangle, IconTrash } from '@tabler/icons-react';
import { Button } from '@/components/ui/button';
import { Checkbox } from '@/components/ui/checkbox';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import { ScrollArea } from '@/components/ui/scroll-area';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import { useChannels } from '../context/channels-context';
import {
  useChannelDisabledAPIKeys,
  useEnableChannelAPIKey,
  useEnableAllChannelAPIKeys,
  useEnableSelectedChannelAPIKeys,
  useDeleteDisabledChannelAPIKeys,
} from '../data/channels';

interface ChannelsDisabledAPIKeysDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

export function ChannelsDisabledAPIKeysDialog({ open, onOpenChange }: ChannelsDisabledAPIKeysDialogProps) {
  const { t } = useTranslation();
  const { currentRow, setOpen } = useChannels();
  const [selectedKeys, setSelectedKeys] = useState<Set<string>>(new Set());
  const [confirmPopoverKey, setConfirmPopoverKey] = useState<string | null>(null);
  const [confirmDeletePopoverKey, setConfirmDeletePopoverKey] = useState<string | null>(null);
  const [confirmEnableAll, setConfirmEnableAll] = useState(false);
  const [confirmEnableSelected, setConfirmEnableSelected] = useState(false);
  const [confirmDeleteSelected, setConfirmDeleteSelected] = useState(false);

  const { data: disabledKeys = [], isLoading } = useChannelDisabledAPIKeys(currentRow?.id || '', {
    enabled: open && !!currentRow?.id,
  });

  const enableAPIKey = useEnableChannelAPIKey();
  const enableAllAPIKeys = useEnableAllChannelAPIKeys();
  const enableSelectedAPIKeys = useEnableSelectedChannelAPIKeys();
  const deleteDisabledAPIKeys = useDeleteDisabledChannelAPIKeys();

  const isPending =
    enableAPIKey.isPending ||
    enableAllAPIKeys.isPending ||
    enableSelectedAPIKeys.isPending ||
    deleteDisabledAPIKeys.isPending;

  const handleClose = () => {
    setOpen(null);
    onOpenChange(false);
    setSelectedKeys(new Set());
    setConfirmPopoverKey(null);
    setConfirmDeletePopoverKey(null);
    setConfirmEnableAll(false);
    setConfirmEnableSelected(false);
    setConfirmDeleteSelected(false);
  };

  const handleEnableKey = async (key: string) => {
    if (!currentRow) return;
    try {
      await enableAPIKey.mutateAsync({ channelID: currentRow.id, key });
      setConfirmPopoverKey(null);
      setSelectedKeys((prev) => {
        const next = new Set(prev);
        next.delete(key);
        return next;
      });
    } catch {
      // Error handled by mutation hook
    }
  };

  const handleDeleteKey = async (key: string) => {
    if (!currentRow) return;
    try {
      await deleteDisabledAPIKeys.mutateAsync({ channelID: currentRow.id, keys: [key] });
      setConfirmDeletePopoverKey(null);
      setSelectedKeys((prev) => {
        const next = new Set(prev);
        next.delete(key);
        return next;
      });
    } catch {
      // Error handled by mutation hook
    }
  };

  const handleEnableAll = async () => {
    if (!currentRow) return;
    try {
      await enableAllAPIKeys.mutateAsync({ channelID: currentRow.id });
      setConfirmEnableAll(false);
      handleClose();
    } catch {
      // Error handled by mutation hook
    }
  };

  const handleEnableSelected = async () => {
    if (!currentRow || selectedKeys.size === 0) return;
    try {
      await enableSelectedAPIKeys.mutateAsync({
        channelID: currentRow.id,
        keys: Array.from(selectedKeys),
      });
      setConfirmEnableSelected(false);
      setSelectedKeys(new Set());
    } catch {
      // Error handled by mutation hook
    }
  };

  const handleDeleteSelected = async () => {
    if (!currentRow || selectedKeys.size === 0) return;
    try {
      await deleteDisabledAPIKeys.mutateAsync({
        channelID: currentRow.id,
        keys: Array.from(selectedKeys),
      });
      setConfirmDeleteSelected(false);
      setSelectedKeys(new Set());
    } catch {
      // Error handled by mutation hook
    }
  };

  const handleSelectAll = () => {
    if (selectedKeys.size === disabledKeys.length) {
      setSelectedKeys(new Set());
    } else {
      setSelectedKeys(new Set(disabledKeys.map((dk) => dk.key)));
    }
  };

  const isAllSelected = useMemo(
    () => disabledKeys.length > 0 && selectedKeys.size === disabledKeys.length,
    [disabledKeys.length, selectedKeys.size]
  );

  const isSomeSelected = useMemo(
    () => selectedKeys.size > 0 && selectedKeys.size < disabledKeys.length,
    [disabledKeys.length, selectedKeys.size]
  );

  if (!currentRow) {
    return null;
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className='sm:max-w-[600px]'>
        <DialogHeader>
          <DialogTitle className='flex items-center gap-2'>
            <IconKey className='h-5 w-5' />
            {t('channels.dialogs.disabledAPIKeys.title')}
          </DialogTitle>
          <DialogDescription>
            {t('channels.dialogs.disabledAPIKeys.description', { name: currentRow.name })}
          </DialogDescription>
        </DialogHeader>

        <div className='py-4'>
          {isLoading ? (
            <div className='flex items-center justify-center py-8'>
              <div className='text-muted-foreground text-sm'>{t('common.loading')}</div>
            </div>
          ) : disabledKeys.length === 0 ? (
            <div className='flex flex-col items-center justify-center py-8'>
              <IconRefresh className='text-muted-foreground mb-2 h-12 w-12' />
              <p className='text-muted-foreground text-sm'>{t('channels.dialogs.disabledAPIKeys.noDisabledKeys')}</p>
            </div>
          ) : (
            <>
              {/* Batch actions header */}
              <div className='mb-3 flex items-center justify-between'>
                <div className='flex items-center gap-2'>
                  <Checkbox
                    checked={isAllSelected || (isSomeSelected && 'indeterminate')}
                    onCheckedChange={handleSelectAll}
                    aria-label={t('common.columns.selectAll')}
                  />
                  <span className='text-muted-foreground text-sm'>
                    {selectedKeys.size > 0
                      ? t('channels.dialogs.disabledAPIKeys.selectedCount', { count: selectedKeys.size })
                      : t('channels.dialogs.disabledAPIKeys.selectToEnable')}
                  </span>
                </div>
                {selectedKeys.size > 0 && (
                  <div className='flex gap-2'>
                    {/* Enable Selected */}
                    <Popover open={confirmEnableSelected} onOpenChange={setConfirmEnableSelected}>
                      <PopoverTrigger asChild>
                        <Button size='sm' variant='outline' disabled={isPending}>
                          <IconRefresh className='mr-1 h-4 w-4' />
                          {t('channels.dialogs.disabledAPIKeys.enableSelected', { count: selectedKeys.size })}
                        </Button>
                      </PopoverTrigger>
                      <PopoverContent className='w-80'>
                        <div className='flex flex-col gap-3'>
                          <p className='text-sm'>
                            {t('channels.dialogs.disabledAPIKeys.confirmEnableSelected', { count: selectedKeys.size })}
                          </p>
                          <div className='flex justify-end gap-2'>
                            <Button size='sm' variant='outline' onClick={() => setConfirmEnableSelected(false)}>
                              {t('common.buttons.cancel')}
                            </Button>
                            <Button size='sm' onClick={handleEnableSelected} disabled={isPending}>
                              {isPending ? t('common.buttons.processing') : t('common.buttons.confirm')}
                            </Button>
                          </div>
                        </div>
                      </PopoverContent>
                    </Popover>

                    {/* Delete Selected */}
                    <Popover open={confirmDeleteSelected} onOpenChange={setConfirmDeleteSelected}>
                      <PopoverTrigger asChild>
                        <Button size='sm' variant='outline' className='text-destructive' disabled={isPending}>
                          <IconTrash className='mr-1 h-4 w-4' />
                          {t('channels.dialogs.disabledAPIKeys.deleteSelected', { count: selectedKeys.size })}
                        </Button>
                      </PopoverTrigger>
                      <PopoverContent className='w-80'>
                        <div className='flex flex-col gap-3'>
                          <p className='text-sm'>
                            {t('channels.dialogs.disabledAPIKeys.confirmDeleteSelected', { count: selectedKeys.size })}
                          </p>
                          <div className='flex justify-end gap-2'>
                            <Button size='sm' variant='outline' onClick={() => setConfirmDeleteSelected(false)}>
                              {t('common.buttons.cancel')}
                            </Button>
                            <Button size='sm' variant='destructive' onClick={handleDeleteSelected} disabled={isPending}>
                              {isPending ? t('common.buttons.processing') : t('common.buttons.confirm')}
                            </Button>
                          </div>
                        </div>
                      </PopoverContent>
                    </Popover>
                  </div>
                )}
              </div>

              {/* Disabled keys list */}
              <ScrollArea className='h-[300px] rounded-md border'>
                <div className='divide-y'>
                  {disabledKeys.map((dk) => (
                    <div key={dk.key} className='flex items-center justify-between gap-3 px-4 py-3 hover:bg-muted/50'>
                      <div className='flex items-center gap-3'>
                        <Checkbox
                          checked={selectedKeys.has(dk.key)}
                          onCheckedChange={(checked) => {
                            setSelectedKeys((prev) => {
                              const next = new Set(prev);
                              if (checked) {
                                next.add(dk.key);
                              } else {
                                next.delete(dk.key);
                              }
                              return next;
                            });
                          }}
                        />
                        <div className='flex flex-col gap-1'>
                          <div className='flex items-center gap-2'>
                            <code className='bg-muted rounded px-2 py-0.5 font-mono text-sm'>****{dk.key.slice(-4)}</code>
                            <Tooltip>
                              <TooltipTrigger asChild>
                                <span className='text-destructive flex items-center gap-1 text-xs'>
                                  <IconAlertTriangle className='h-3 w-3' />
                                  {dk.errorCode}
                                </span>
                              </TooltipTrigger>
                              <TooltipContent>
                                {t('channels.dialogs.disabledAPIKeys.errorCodeTooltip', { code: dk.errorCode })}
                              </TooltipContent>
                            </Tooltip>
                          </div>
                          <div className='text-muted-foreground flex items-center gap-2 text-xs'>
                            <span>{format(new Date(dk.disabledAt), 'yyyy-MM-dd HH:mm:ss')}</span>
                            {dk.reason && (
                              <>
                                <span>•</span>
                                <span className='max-w-[200px] truncate'>{dk.reason}</span>
                              </>
                            )}
                          </div>
                        </div>
                      </div>

                      <div className='flex gap-1'>
                        {/* Enable single key */}
                        <Popover
                          open={confirmPopoverKey === dk.key}
                          onOpenChange={(isOpen) => setConfirmPopoverKey(isOpen ? dk.key : null)}
                        >
                          <PopoverTrigger asChild>
                            <Button size='sm' variant='ghost' disabled={isPending}>
                              <IconRefresh className='h-4 w-4' />
                            </Button>
                          </PopoverTrigger>
                          <PopoverContent className='w-64'>
                            <div className='flex flex-col gap-3'>
                              <p className='text-sm'>{t('channels.dialogs.disabledAPIKeys.confirmEnable')}</p>
                              <div className='flex justify-end gap-2'>
                                <Button size='sm' variant='outline' onClick={() => setConfirmPopoverKey(null)}>
                                  {t('common.buttons.cancel')}
                                </Button>
                                <Button size='sm' onClick={() => handleEnableKey(dk.key)} disabled={isPending}>
                                  {isPending ? t('common.buttons.processing') : t('common.buttons.confirm')}
                                </Button>
                              </div>
                            </div>
                          </PopoverContent>
                        </Popover>

                        {/* Delete single key */}
                        <Popover
                          open={confirmDeletePopoverKey === dk.key}
                          onOpenChange={(isOpen) => setConfirmDeletePopoverKey(isOpen ? dk.key : null)}
                        >
                          <PopoverTrigger asChild>
                            <Button size='sm' variant='ghost' className='text-destructive' disabled={isPending}>
                              <IconTrash className='h-4 w-4' />
                            </Button>
                          </PopoverTrigger>
                          <PopoverContent className='w-64'>
                            <div className='flex flex-col gap-3'>
                              <p className='text-sm'>{t('channels.dialogs.disabledAPIKeys.confirmDelete')}</p>
                              <div className='flex justify-end gap-2'>
                                <Button size='sm' variant='outline' onClick={() => setConfirmDeletePopoverKey(null)}>
                                  {t('common.buttons.cancel')}
                                </Button>
                                <Button
                                  size='sm'
                                  variant='destructive'
                                  onClick={() => handleDeleteKey(dk.key)}
                                  disabled={isPending}
                                >
                                  {isPending ? t('common.buttons.processing') : t('common.buttons.confirm')}
                                </Button>
                              </div>
                            </div>
                          </PopoverContent>
                        </Popover>
                      </div>
                    </div>
                  ))}
                </div>
              </ScrollArea>
            </>
          )}
        </div>

        <DialogFooter className='flex items-center justify-between sm:justify-between'>
          <div>
            {disabledKeys.length > 0 && (
              <Popover open={confirmEnableAll} onOpenChange={setConfirmEnableAll}>
                <PopoverTrigger asChild>
                  <Button variant='outline' disabled={isPending}>
                    <IconRefreshOff className='mr-2 h-4 w-4' />
                    {t('channels.dialogs.disabledAPIKeys.enableAll')}
                  </Button>
                </PopoverTrigger>
                <PopoverContent className='w-80'>
                  <div className='flex flex-col gap-3'>
                    <p className='text-sm'>
                      {t('channels.dialogs.disabledAPIKeys.confirmEnableAll', { count: disabledKeys.length })}
                    </p>
                    <div className='flex justify-end gap-2'>
                      <Button size='sm' variant='outline' onClick={() => setConfirmEnableAll(false)}>
                        {t('common.buttons.cancel')}
                      </Button>
                      <Button size='sm' onClick={handleEnableAll} disabled={isPending}>
                        {isPending ? t('common.buttons.processing') : t('common.buttons.confirm')}
                      </Button>
                    </div>
                  </div>
                </PopoverContent>
              </Popover>
            )}
          </div>
          <Button variant='outline' onClick={handleClose}>
            {t('common.buttons.close')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
