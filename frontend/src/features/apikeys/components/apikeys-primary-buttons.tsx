import { Plus } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import { useApiKeysContext } from '../context/apikeys-context'

export function ApiKeysPrimaryButtons() {
  const { t } = useTranslation()
  const { openDialog } = useApiKeysContext()

  return (
    <div className='flex items-center space-x-2'>
      <Button onClick={() => openDialog('create')} size='sm'>
        <Plus className='mr-2 h-4 w-4' />
        {t('apikeys.createApiKey')}
      </Button>
    </div>
  )
}