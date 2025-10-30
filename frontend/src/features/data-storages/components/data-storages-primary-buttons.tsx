'use client'

import { useNavigate } from '@tanstack/react-router'
import { Plus, Settings } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import { useDataStoragesContext } from '../context/data-storages-context'

export function DataStoragesPrimaryButtons() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const { setIsCreateDialogOpen } = useDataStoragesContext()

  return (
    <div className='flex flex-wrap items-center gap-2'>
      <Button variant='outline' onClick={() => navigate({ to: '/system', search: { tab: 'storage' } })}>
        <Settings className='mr-2 h-4 w-4' />
        {t('dataStorages.buttons.openStorageSettings')}
      </Button>
      <Button onClick={() => setIsCreateDialogOpen(true)}>
        <Plus className='mr-2 h-4 w-4' />
        {t('dataStorages.buttons.create')}
      </Button>
    </div>
  )
}
