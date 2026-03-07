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
import { useUpdateMessageChannel } from '../data/message-channels';
import type { MessageChannel } from '../data/schema';

interface MessageChannelsStatusDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  currentRow: MessageChannel;
}

export function MessageChannelsStatusDialog({
  open,
  onOpenChange,
  currentRow,
}: MessageChannelsStatusDialogProps) {
  const { t } = useTranslation();
  const updateMessageChannel = useUpdateMessageChannel();

  const isEnabled = currentRow.status === 'enabled';
  const newStatus = isEnabled ? 'disabled' : 'enabled';

  const handleConfirm = async () => {
    await updateMessageChannel.mutateAsync({
      id: currentRow.id,
      input: { status: newStatus },
    });
    onOpenChange(false);
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>
            {isEnabled
              ? t('messageChannels.dialogs.disable.title')
              : t('messageChannels.dialogs.enable.title')}
          </DialogTitle>
          <DialogDescription>
            {isEnabled
              ? t('messageChannels.dialogs.disable.description', { name: currentRow.name })
              : t('messageChannels.dialogs.enable.description', { name: currentRow.name })}
          </DialogDescription>
        </DialogHeader>
        <DialogFooter>
          <Button variant='outline' onClick={() => onOpenChange(false)}>
            {t('common.buttons.cancel')}
          </Button>
          <Button
            onClick={handleConfirm}
            disabled={updateMessageChannel.isPending}
            variant={isEnabled ? 'destructive' : 'default'}
          >
            {updateMessageChannel.isPending
              ? t('common.buttons.processing')
              : isEnabled
                ? t('common.buttons.disable')
                : t('common.buttons.enable')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
