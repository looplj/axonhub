import React from 'react';
import { DotsHorizontalIcon } from '@radix-ui/react-icons';
import { Row } from '@tanstack/react-table';
import { IconArchive, IconPin, IconRotate } from '@tabler/icons-react';
import { useTranslation } from 'react-i18next';
import { Button } from '@/components/ui/button';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog';
import { useArchiveThread, useUnarchiveThread, useRetainThread, useUnretainThread } from '../data/threads';
import { Thread } from '../data/schema';

interface ThreadsRowActionsProps {
  row: Row<Thread>;
}

export function ThreadsRowActions({ row }: ThreadsRowActionsProps) {
  const { t } = useTranslation();
  const thread = row.original;
  const [open, setOpen] = React.useState(false);
  const [showArchiveDialog, setShowArchiveDialog] = React.useState(false);

  const archiveMutation = useArchiveThread();
  const unarchiveMutation = useUnarchiveThread();
  const retainMutation = useRetainThread();
  const unretainMutation = useUnretainThread();

  const handleArchive = () => {
    setOpen(false);
    setShowArchiveDialog(true);
  };

  const confirmArchive = () => {
    archiveMutation.mutate(thread.id);
    setShowArchiveDialog(false);
  };

  const handleUnarchive = () => {
    setOpen(false);
    unarchiveMutation.mutate(thread.id);
  };

  const handleRetain = () => {
    setOpen(false);
    retainMutation.mutate(thread.id);
  };

  const handleUnretain = () => {
    setOpen(false);
    unretainMutation.mutate(thread.id);
  };

  const status = thread.status ?? 'active';

  return (
    <>
      <DropdownMenu open={open} onOpenChange={setOpen}>
        <DropdownMenuTrigger asChild>
          <Button variant='ghost' className='data-[state=open]:bg-muted flex h-8 w-8 p-0' data-testid='thread-row-actions'>
            <DotsHorizontalIcon className='h-4 w-4' />
            <span className='sr-only'>{t('common.actions.openMenu')}</span>
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align='end' className='w-[180px]'>
          {status === 'active' && (
            <>
              <DropdownMenuItem onClick={handleArchive}>
                <IconArchive className='mr-2 h-4 w-4' />
                {t('common.actions.archive', 'Archive')}
              </DropdownMenuItem>
              <DropdownMenuSeparator />
              <DropdownMenuItem onClick={handleRetain}>
                <IconPin className='mr-2 h-4 w-4' />
                {t('common.actions.retain', 'Retain')}
              </DropdownMenuItem>
            </>
          )}
          {status === 'archived' && (
            <DropdownMenuItem onClick={handleUnarchive}>
              <IconRotate className='mr-2 h-4 w-4' />
              {t('common.actions.unarchive', 'Restore')}
            </DropdownMenuItem>
          )}
          {status === 'retained' && (
            <DropdownMenuItem onClick={handleUnretain}>
              <IconRotate className='mr-2 h-4 w-4' />
              {t('common.actions.unretain', 'Stop retaining')}
            </DropdownMenuItem>
          )}
        </DropdownMenuContent>
      </DropdownMenu>

      <AlertDialog open={showArchiveDialog} onOpenChange={setShowArchiveDialog}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('threads.dialogs.archiveTitle', 'Archive thread?')}</AlertDialogTitle>
            <AlertDialogDescription>
              {t('threads.dialogs.archiveDescription', 'This thread and all its traces will be hidden from the default view. You can restore it later by filtering for archived threads.')}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t('common.actions.cancel', 'Cancel')}</AlertDialogCancel>
            <AlertDialogAction onClick={confirmArchive}>{t('common.actions.archive', 'Archive')}</AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  );
}
