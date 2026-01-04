import { useTranslation } from 'react-i18next';
import { Button } from '@/components/ui/button';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { useChannels } from '../context/channels-context';
import { useClearChannelErrorMessage } from '../data/channels';

interface ChannelsErrorResolvedDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

export function ChannelsErrorResolvedDialog({ open, onOpenChange }: ChannelsErrorResolvedDialogProps) {
  const { t } = useTranslation();
  const { currentRow, setOpen } = useChannels();
  const updateChannelErrorMessage = useClearChannelErrorMessage();

  const handleConfirm = async () => {
    if (!currentRow) return;

    try {
      await updateChannelErrorMessage.mutateAsync({
        id: currentRow.id,
      });
      setOpen(null);
      onOpenChange(false);
    } catch (error) {
      // Error is handled by the mutation hook
    }
  };

  const handleCancel = () => {
    setOpen(null);
    onOpenChange(false);
  };

  if (!currentRow || !currentRow.errorMessage) {
    return null;
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className='sm:max-w-[425px]'>
        <DialogHeader>
          <DialogTitle>{t('channels.dialogs.errorResolved.title')}</DialogTitle>
          <DialogDescription>
            {t('channels.dialogs.errorResolved.description', {
              channelName: currentRow.name,
              errorMessage: t(`channels.messages.${currentRow.errorMessage}`),
            })}
          </DialogDescription>
        </DialogHeader>

        <div className='py-4'>
          <div className='bg-muted rounded-md p-3'>
            <p className='text-sm font-medium'>{t('channels.dialogs.errorResolved.currentError')}</p>
            <p className='text-muted-foreground mt-1 text-sm'>{t(`channels.messages.${currentRow.errorMessage}`)}</p>
          </div>
        </div>

        <DialogFooter>
          <Button variant='outline' onClick={handleCancel} disabled={updateChannelErrorMessage.isPending}>
            {t('common.buttons.cancel')}
          </Button>
          <Button onClick={handleConfirm} disabled={updateChannelErrorMessage.isPending}>
            {updateChannelErrorMessage.isPending ? t('common.buttons.processing') : t('channels.actions.errorResolved')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
