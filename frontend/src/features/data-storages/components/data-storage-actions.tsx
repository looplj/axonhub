'use client'

import { MoreHorizontal, Pencil } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { DataStorage } from '../data/data-storages'
import { useDataStoragesContext } from '../context/data-storages-context'

interface DataStorageActionsProps {
  dataStorage: DataStorage
}

export function DataStorageActions({ dataStorage }: DataStorageActionsProps) {
  const { t } = useTranslation()
  const { setEditingDataStorage, setIsEditDialogOpen } = useDataStoragesContext()

  const handleEdit = () => {
    setEditingDataStorage(dataStorage)
    setIsEditDialogOpen(true)
  }

  // Primary data storage cannot be edited
  if (dataStorage.primary) {
    return null
  }

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button variant='ghost' className='h-8 w-8 p-0'>
          <span className='sr-only'>{t('common.openMenu')}</span>
          <MoreHorizontal className='h-4 w-4' />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align='end'>
        <DropdownMenuItem onClick={handleEdit}>
          <Pencil className='mr-2 h-4 w-4' />
          {t('common.buttons.edit')}
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  )
}
