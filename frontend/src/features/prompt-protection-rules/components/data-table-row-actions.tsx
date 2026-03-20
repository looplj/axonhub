import { Row } from '@tanstack/react-table';
import { IconDotsVertical, IconEdit, IconTrash } from '@tabler/icons-react';
import { useTranslation } from 'react-i18next';
import { Button } from '@/components/ui/button';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import { PermissionGuard } from '@/components/permission-guard';
import { usePromptProtectionRules } from '../context/rules-context';
import { PromptProtectionRule } from '../data/schema';

interface DataTableRowActionsProps {
  row: Row<PromptProtectionRule>;
}

export function DataTableRowActions({ row }: DataTableRowActionsProps) {
  const { t } = useTranslation();
  const { setOpen, setCurrentRow } = usePromptProtectionRules();
  const rule = row.original;

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button variant='ghost' className='flex h-8 w-8 p-0 data-[state=open]:bg-muted'>
          <IconDotsVertical className='h-4 w-4' />
          <span className='sr-only'>{t('common.buttons.openMenu')}</span>
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align='end' className='w-[160px]'>
        <PermissionGuard requiredScope='write_channels'>
          <>
            <DropdownMenuItem
              onClick={() => {
                setCurrentRow(rule);
                setOpen('edit');
              }}
            >
              <IconEdit className='mr-2 h-4 w-4' />
              {t('common.buttons.edit')}
            </DropdownMenuItem>
            <DropdownMenuSeparator />
            <DropdownMenuItem
              onClick={() => {
                setCurrentRow(rule);
                setOpen('delete');
              }}
              className='text-destructive focus:text-destructive'
            >
              <IconTrash className='mr-2 h-4 w-4' />
              {t('common.buttons.delete')}
            </DropdownMenuItem>
          </>
        </PermissionGuard>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
