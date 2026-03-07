import { useTranslation } from 'react-i18next';
import { Button } from '@/components/ui/button';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { useDeleteMessageChannel } from '../data/message-channels';
import type { MessageChannel } from '../data/schema';

interface MessageChannelsDeleteDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  currentRow: MessageChannel;
}

export function MessageChannelsDeleteDialog({
  open,
  onOpenChange,
  currentRow,
}: MessageChannelsDeleteDialogProps) {
  const { t } = useTranslation();
  const deleteMessageChannel = useDeleteMessageChannel();

  const handleConfirm = async () => {
    await deleteMessageChannel.mutateAsync(currentRow.id);
    onOpenChange(false);
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t('messageChannels.dialogs.delete.title')}</DialogTitle>
          <DialogDescription>
            {t('messageChannels.dialogs.delete.description', { name: currentRow.name })}
          </DialogDescription>
        </DialogHeader>
        <DialogFooter>
          <Button variant='outline' onClick={() => onOpenChange(false)}>
            {t('common.buttons.cancel')}
          </Button>
          <Button onClick={handleConfirm} disabled={deleteMessageChannel.isPending} variant='destructive'>
            {deleteMessageChannel.isPending ? t('common.buttons.processing') : t('common.buttons.delete')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
