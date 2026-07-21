'use client';

import { useCallback } from 'react';
import { format } from 'date-fns';
import { ColumnDef } from '@tanstack/react-table';
import { zhCN, enUS } from 'date-fns/locale';
import { FileText } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { extractNumberID } from '@/lib/utils';
import { usePaginationSearch } from '@/hooks/use-pagination-search';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { DataTableColumnHeader } from '@/components/data-table-column-header';
import type { Thread } from '../data/schema';
import { useArchiveThread, useUnarchiveThread, useRetainThread, useUnretainThread } from '../data/threads';
import { IconArchive, IconPin, IconRotate } from '@tabler/icons-react';
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
import React from 'react';

function ThreadActionButtons({ thread }: { thread: Thread }) {
  const { t } = useTranslation();
  const [showArchiveDialog, setShowArchiveDialog] = React.useState(false);
  const archiveMutation = useArchiveThread();
  const unarchiveMutation = useUnarchiveThread();
  const retainMutation = useRetainThread();
  const unretainMutation = useUnretainThread();
  const status = thread.status ?? 'active';

  return (
    <>
      <div className='flex items-center gap-1'>
        {status === 'active' && (
          <>
            <Button variant='ghost' size='sm' onClick={() => setShowArchiveDialog(true)} title={t('common.actions.archive')}>
              <IconArchive className='h-4 w-4' />
            </Button>
            <Button variant='ghost' size='sm' onClick={() => retainMutation.mutate(thread.id)} title={t('common.actions.retain')}>
              <IconPin className='h-4 w-4' />
            </Button>
          </>
        )}
        {status === 'archived' && (
          <Button variant='ghost' size='sm' onClick={() => unarchiveMutation.mutate(thread.id)} title={t('common.actions.unarchive')}>
            <IconRotate className='h-4 w-4' />
          </Button>
        )}
        {status === 'retained' && (
          <Button variant='ghost' size='sm' onClick={() => unretainMutation.mutate(thread.id)} title={t('common.actions.unretain')}>
            <IconRotate className='h-4 w-4' />
          </Button>
        )}
      </div>
      <AlertDialog open={showArchiveDialog} onOpenChange={setShowArchiveDialog}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('threads.dialogs.archiveTitle')}</AlertDialogTitle>
            <AlertDialogDescription>{t('threads.dialogs.archiveDescription')}</AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t('common.actions.cancel')}</AlertDialogCancel>
            <AlertDialogAction onClick={() => { archiveMutation.mutate(thread.id); setShowArchiveDialog(false); }}>
              {t('common.actions.archive')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  );
}

export function useThreadsColumns(): ColumnDef<Thread>[] {
  const { t, i18n } = useTranslation();
  const locale = i18n.language === 'zh' ? zhCN : enUS;
  const { navigateWithSearch } = usePaginationSearch({ defaultPageSize: 20 });

  const columns: ColumnDef<Thread>[] = [
    {
      accessorKey: 'id',
      header: ({ column }) => <DataTableColumnHeader column={column} title={t('common.columns.id')} />,
      cell: ({ row }) => {
        const handleClick = useCallback(() => {
          navigateWithSearch({
            to: '/project/threads/$threadId',
            params: { threadId: row.original.id },
          });
        }, [row.original.id, navigateWithSearch]);

        return (
          <button onClick={handleClick} className='text-primary cursor-pointer font-mono text-xs hover:underline'>
            #{extractNumberID(row.getValue('id'))}
          </button>
        );
      },
      enableSorting: true,
      enableHiding: false,
    },
    {
      accessorKey: 'threadID',
      header: ({ column }) => <DataTableColumnHeader column={column} title={t('threads.columns.threadId')} />,
      cell: ({ row }) => {
        const threadID = row.getValue('threadID') as string;
        return (
          <div className='max-w-64 truncate font-mono text-xs' title={threadID}>
            {threadID}
          </div>
        );
      },
      enableSorting: false,
    },
    {
      accessorKey: 'firstUserQuery',
      header: ({ column }) => <DataTableColumnHeader column={column} title={t('threads.columns.firstUserQuery')} />,
      cell: ({ row }) => {
        const query = row.getValue('firstUserQuery') as string | null | undefined;
        return (
          <div className='max-w-96 truncate text-xs' title={query || ''}>
            {query || '-'}
          </div>
        );
      },
      enableSorting: false,
    },
    // {
    //   id: 'project',
    //   header: ({ column }) => <DataTableColumnHeader column={column} title={t('threads.columns.project')} />,
    //   cell: ({ row }) => {
    //     const project = row.original.project
    //     return (
    //       <div className='max-w-48 truncate text-xs' title={project?.name || ''}>
    //         {project?.name || t('threads.columns.unknownProject')}
    //       </div>
    //     )
    //   },
    //   enableSorting: false,
    // },
    {
      id: 'traceCount',
      header: ({ column }) => <DataTableColumnHeader column={column} title={t('threads.columns.traceCount')} />,
      cell: ({ row }) => {
        const count = row.original.tracesSummary?.totalCount ?? 0;
        const archivedCount = row.original.archivedTracesCount ?? 0;
        return (
          <Badge variant='secondary' className='font-mono text-xs'>
            {archivedCount > 0 ? `${count} (+${archivedCount})` : `${count}`}
          </Badge>
        );
      },
      enableSorting: false,
    },

    {
      accessorKey: 'createdAt',
      header: ({ column }) => <DataTableColumnHeader column={column} title={t('common.columns.createdAt')} />,
      cell: ({ row }) => {
        const date = new Date(row.getValue('createdAt'));
        return <div className='text-xs'>{format(date, 'yyyy-MM-dd HH:mm:ss', { locale })}</div>;
      },
    },
    {
      id: 'details',
      header: ({ column }) => <DataTableColumnHeader column={column} title={t('threads.columns.details')} />,
      cell: ({ row }) => {
        const handleViewDetails = () => {
          navigateWithSearch({
            to: '/project/threads/$threadId',
            params: { threadId: row.original.id },
          });
        };

        return (
          <Button variant='outline' size='sm' onClick={handleViewDetails}>
            <FileText className='mr-2 h-4 w-4' />
            {t('threads.actions.viewDetails')}
          </Button>
        );
      },
    },
    {
      id: 'actions',
      header: ({ column }) => <DataTableColumnHeader column={column} title={t('common.columns.actions')} />,
      enableHiding: false,
      meta: { className: 'w-[100px]' },
      cell: ({ row }) => <ThreadActionButtons thread={row.original} />,
    },
  ];

  return columns;
}
