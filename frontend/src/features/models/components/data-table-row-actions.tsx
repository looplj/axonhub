import { DotsHorizontalIcon } from '@radix-ui/react-icons';
import { Row } from '@tanstack/react-table';
import { IconEdit, IconArchive, IconTrash, IconNote } from '@tabler/icons-react';
import { useTranslation } from 'react-i18next';
import { usePermissions } from '@/hooks/usePermissions';
import { Button } from '@/components/ui/button';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import { PermissionGuard } from '@/components/permission-guard';
import { useModels } from '../context/models-context';
import { Model } from '../data/schema';

interface DataTableRowActionsProps {
  row: Row<Model>;
}

export function DataTableRowActions({ row }: DataTableRowActionsProps) {
  const { t } = useTranslation();
  const { setOpen, setCurrentRow } = useModels();
  const { channelPermissions } = usePermissions();
  const model = row.original;

  if (!channelPermissions.canWrite) {
    return null;
  }

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button variant='ghost' className='data-[state=open]:bg-muted flex h-8 w-8 p-0' data-testid='row-actions'>
          <DotsHorizontalIcon className='h-4 w-4' />
          <span className='sr-only'>{t('common.actions.openMenu')}</span>
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align='end' className='w-[160px]'>
        <PermissionGuard requiredScope='write_channels'>
          <>
            <DropdownMenuItem
              onClick={() => {
                setCurrentRow(row.original);
                setOpen('edit');
              }}
            >
              <IconEdit size={16} className='mr-2' />
              {t('common.actions.edit')}
            </DropdownMenuItem>

            <DropdownMenuItem
              onClick={() => {
                setCurrentRow(row.original);
                setOpen('association');
              }}
            >
              <IconNote size={16} className='mr-2' />
              {t('models.actions.manageAssociation')}
            </DropdownMenuItem>

            {channelPermissions.canRead && <DropdownMenuSeparator />}

            {model.status !== 'archived' && (
              <DropdownMenuItem
                onClick={() => {
                  setCurrentRow(row.original);
                  setOpen('archive');
                }}
                className='text-orange-500!'
              >
                <IconArchive size={16} className='mr-2' />
                {t('common.buttons.archive')}
              </DropdownMenuItem>
            )}

            <DropdownMenuItem
              onClick={() => {
                setCurrentRow(row.original);
                setOpen('delete');
              }}
              className='text-red-500!'
            >
              <IconTrash size={16} className='mr-2' />
              {t('common.buttons.delete')}
            </DropdownMenuItem>
          </>
        </PermissionGuard>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
