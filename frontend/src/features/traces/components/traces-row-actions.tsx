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
import { useArchiveTrace, useUnarchiveTrace, useRetainTrace, useUnretainTrace } from '../data/traces';
import { Trace } from '../data/schema';

interface TracesRowActionsProps {
  row: Row<Trace>;
}

export function TracesRowActions({ row }: TracesRowActionsProps) {
  const { t } = useTranslation();
  const trace = row.original;
  const [open, setOpen] = React.useState(false);
  const [showArchiveDialog, setShowArchiveDialog] = React.useState(false);

  const archiveMutation = useArchiveTrace();
  const unarchiveMutation = useUnarchiveTrace();
  const retainMutation = useRetainTrace();
  const unretainMutation = useUnretainTrace();

  const handleArchive = () => {
    setOpen(false);
    setShowArchiveDialog(true);
  };

  const confirmArchive = () => {
    archiveMutation.mutate(trace.id);
    setShowArchiveDialog(false);
  };

  const handleUnarchive = () => {
    setOpen(false);
    unarchiveMutation.mutate(trace.id);
  };

  const handleRetain = () => {
    setOpen(false);
    retainMutation.mutate(trace.id);
  };

  const handleUnretain = () => {
    setOpen(false);
    unretainMutation.mutate(trace.id);
  };

  const status = trace.status ?? 'active';

  return (
    <>
      <DropdownMenu open={open} onOpenChange={setOpen}>
        <DropdownMenuTrigger asChild>
          <Button variant='ghost' className='data-[state=open]:bg-muted flex h-8 w-8 p-0' data-testid='trace-row-actions'>
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
            <AlertDialogTitle>{t('traces.dialogs.archiveTitle', 'Archive trace?')}</AlertDialogTitle>
            <AlertDialogDescription>
              {t('traces.dialogs.archiveDescription', 'This trace will be hidden from the default view. You can restore it later by filtering for archived traces.')}
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
