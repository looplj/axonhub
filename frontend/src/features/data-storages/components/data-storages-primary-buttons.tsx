'use client'

import { Plus } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import { useDataStoragesContext } from '../context/data-storages-context'

export function DataStoragesPrimaryButtons() {
  const { t } = useTranslation()
  const { setIsCreateDialogOpen } = useDataStoragesContext()

  return (
    <Button onClick={() => setIsCreateDialogOpen(true)}>
      <Plus className='mr-2 h-4 w-4' />
      {t('dataStorages.buttons.create', '创建数据存储')}
    </Button>
  )
}
