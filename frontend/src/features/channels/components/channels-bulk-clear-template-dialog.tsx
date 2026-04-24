import { Loader2 } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { Button } from '@/components/ui/button';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { useChannels } from '../context/channels-context';
import { useClearChannelOverrideTemplates } from '../data/templates';

export function ChannelsBulkClearTemplateDialog() {
  const { t } = useTranslation();
  const { open, setOpen, selectedChannels, resetRowSelection, setSelectedChannels } = useChannels();
  const clearTemplates = useClearChannelOverrideTemplates();

  const isDialogOpen = open === 'bulkClearTemplate';

  const handleClear = async () => {
    try {
      await clearTemplates.mutateAsync({
        channelIDs: selectedChannels.map((ch) => ch.id),
      });
      resetRowSelection();
      setSelectedChannels([]);
      setOpen(null);
    } catch (error) {
      // Error already handled by mutation
    }
  };

  const handleClose = () => {
    setOpen(null);
  };

  return (
    <Dialog open={isDialogOpen} onOpenChange={handleClose}>
      <DialogContent className='sm:max-w-[500px]'>
        <DialogHeader>
          <DialogTitle>{t('channels.templates.bulkClear.title')}</DialogTitle>
          <DialogDescription>
            {t('channels.templates.bulkClear.description', { count: selectedChannels.length })}
          </DialogDescription>
        </DialogHeader>

        <div className='space-y-4 py-4'>
          <div className='bg-destructive/10 rounded-md border border-destructive/30 p-3'>
            <p className='text-destructive text-sm'>
              {t('channels.templates.bulkClear.warning')}
            </p>
          </div>
        </div>

        <DialogFooter>
          <Button variant='outline' onClick={handleClose} disabled={clearTemplates.isPending}>
            {t('common.buttons.cancel')}
          </Button>
          <Button variant='destructive' onClick={handleClear} disabled={clearTemplates.isPending}>
            {clearTemplates.isPending ? (
              <>
                <Loader2 className='mr-2 h-4 w-4 animate-spin' />
                {t('channels.templates.bulkClear.clearing')}
              </>
            ) : (
              t('channels.templates.bulkClear.clear', { count: selectedChannels.length })
            )}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
