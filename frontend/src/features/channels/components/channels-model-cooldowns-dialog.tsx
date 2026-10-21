import { useEffect, useState } from 'react';
import { formatDistanceToNow } from 'date-fns';
import { IconSnowflake, IconSnowflakeOff } from '@tabler/icons-react';
import { enUS, zhCN } from 'date-fns/locale';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { ScrollArea } from '@/components/ui/scroll-area';
import { useChannels } from '../context/channels-context';
import { useChannelModelCooldowns, useClearChannelModelCooldown } from '../data/channels';

interface ChannelsModelCooldownsDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

function formatRemaining(expiresAt: string, language: string): string {
  return formatDistanceToNow(new Date(expiresAt), {
    addSuffix: false,
    locale: language.startsWith('zh') ? zhCN : enUS,
  });
}

export function ChannelsModelCooldownsDialog({ open, onOpenChange }: ChannelsModelCooldownsDialogProps) {
  const { t, i18n } = useTranslation();
  const { currentRow, setOpen } = useChannels();
  const [confirmModel, setConfirmModel] = useState<string | null>(null);
  const [now, setNow] = useState(() => Date.now());

  useEffect(() => {
    if (!open) return;

    setNow(Date.now());
    const timer = window.setInterval(() => setNow(Date.now()), 30_000);
    return () => window.clearInterval(timer);
  }, [open]);

  const channelId = currentRow?.id || '';
  const { data: cooldowns = [], isLoading } = useChannelModelCooldowns(channelId, {
    enabled: open && !!channelId,
  });

  const clearCooldown = useClearChannelModelCooldown();

  const handleClose = () => {
    setOpen(null);
    onOpenChange(false);
    setConfirmModel(null);
  };

  const handleClear = async (model: string) => {
    if (!currentRow) return;
    try {
      await clearCooldown.mutateAsync({ channelID: currentRow.id, model });
      setConfirmModel(null);
      toast.success(t('channels.dialogs.modelCooldowns.messages.cleared'));
    } catch {
      // Error is handled by the hook.
    }
  };

  return (
    <Dialog open={open} onOpenChange={handleClose}>
      <DialogContent className='sm:max-w-lg'>
        <DialogHeader className='text-left'>
          <DialogTitle className='flex items-center gap-2'>
            <IconSnowflake className='h-5 w-5' />
            {t('channels.dialogs.modelCooldowns.title')}
          </DialogTitle>
          <DialogDescription className='text-muted-foreground text-sm'>
            {t('channels.dialogs.modelCooldowns.description', { channel: currentRow?.name ?? '' })}
          </DialogDescription>
        </DialogHeader>

        <ScrollArea className='max-h-80'>
          <div className='space-y-2 pr-3'>
            {isLoading ? (
              <p className='text-muted-foreground py-6 text-center text-sm'>{t('common.loading', 'Loading')}...</p>
            ) : cooldowns.length === 0 ? (
              <p className='text-muted-foreground py-6 text-center text-sm'>
                {t('channels.dialogs.modelCooldowns.empty')}
              </p>
            ) : (
              cooldowns.map((cooldown) => (
                <div key={cooldown.model} className='rounded-lg border p-3'>
                  <div className='flex items-start justify-between gap-3'>
                    <div className='min-w-0 space-y-1'>
                      <div className='flex flex-wrap items-center gap-1.5'>
                        <span className='truncate text-sm font-medium'>{cooldown.model}</span>
                        {cooldown.expiresAt && new Date(cooldown.expiresAt).getTime() > now ? (
                          <Badge variant='outline' className='h-5 px-1.5 text-[10px] font-normal text-cyan-600 dark:text-cyan-400'>
                            {t('channels.dialogs.modelCooldowns.remaining', {
                              duration: formatRemaining(cooldown.expiresAt, i18n.language),
                            })}
                          </Badge>
                        ) : (
                          <Badge variant='outline' className='h-5 px-1.5 text-[10px] font-normal'>
                            {t('channels.dialogs.modelCooldowns.expired')}
                          </Badge>
                        )}
                      </div>
                      {cooldown.reason && (
                        <p className='text-muted-foreground line-clamp-2 text-xs'>{cooldown.reason}</p>
                      )}
                    </div>
                    <Button
                      size='sm'
                      variant='outline'
                      disabled={clearCooldown.isPending}
                      onClick={() => setConfirmModel(cooldown.model)}
                    >
                      <IconSnowflakeOff size={14} className='mr-1' />
                      {t('channels.dialogs.modelCooldowns.unfreeze')}
                    </Button>
                  </div>
                </div>
              ))
            )}
          </div>
        </ScrollArea>

        <DialogFooter>
          <Button variant='outline' onClick={handleClose}>
            {t('common.buttons.close', 'Close')}
          </Button>
        </DialogFooter>

        {confirmModel && (
          <Dialog open onOpenChange={(isOpen) => !isOpen && setConfirmModel(null)}>
            <DialogContent className='sm:max-w-sm'>
              <DialogHeader className='text-left'>
                <DialogTitle>{t('channels.dialogs.modelCooldowns.confirmTitle')}</DialogTitle>
                <DialogDescription className='text-muted-foreground text-sm'>
                  {t('channels.dialogs.modelCooldowns.confirmDescription', { model: confirmModel })}
                </DialogDescription>
              </DialogHeader>
              <DialogFooter>
                <Button variant='outline' onClick={() => setConfirmModel(null)}>
                  {t('common.buttons.cancel')}
                </Button>
                <Button onClick={() => handleClear(confirmModel)}>{t('channels.dialogs.modelCooldowns.unfreeze')}</Button>
              </DialogFooter>
            </DialogContent>
          </Dialog>
        )}
      </DialogContent>
    </Dialog>
  );
}
