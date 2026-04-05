import { useTranslation } from 'react-i18next';
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Alert, AlertDescription } from '@/components/ui/alert';
import { CopyButton } from '@/components/ui/copy-button';
import { useAgents } from '../context/agents-context';

export function AgentsKeyDialog() {
  const { t } = useTranslation();
  const { open, setOpen, createdApiKey, setCreatedApiKey } = useAgents();

  const isOpen = open === 'viewKey';

  return (
    <Dialog
      open={isOpen}
      onOpenChange={(next) => {
        if (!next) {
          setOpen(null);
          setCreatedApiKey(null);
        }
      }}
    >
      <DialogContent className='sm:max-w-xl'>
        <DialogHeader>
          <DialogTitle>{t('agents.dialogs.key.title')}</DialogTitle>
          <DialogDescription>{t('agents.dialogs.key.description')}</DialogDescription>
        </DialogHeader>

        <Alert className='border-orange-200 bg-orange-50 dark:border-orange-800 dark:bg-orange-950'>
          <AlertDescription className='text-orange-800 dark:text-orange-200'>{t('agents.dialogs.key.warning')}</AlertDescription>
        </Alert>

        <div className='space-y-2'>
          <div className='text-sm font-medium'>{t('agents.dialogs.key.name')}</div>
          <div className='bg-muted flex items-center justify-between gap-2 rounded-md p-3'>
            <div className='truncate text-sm'>{createdApiKey?.name || '-'}</div>
          </div>
        </div>

        <div className='space-y-2'>
          <div className='text-sm font-medium'>{t('agents.dialogs.key.value')}</div>
          <div className='bg-muted flex items-center justify-between gap-2 rounded-md p-3'>
            <code className='break-all font-mono text-sm'>{createdApiKey?.key || '-'}</code>
            {createdApiKey?.key && <CopyButton content={createdApiKey.key} copyMessage={t('agents.messages.keyCopied')} />}
          </div>
        </div>
      </DialogContent>
    </Dialog>
  );
}

