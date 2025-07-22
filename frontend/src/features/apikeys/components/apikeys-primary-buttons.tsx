import { Plus } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { useApiKeysContext } from '../context/apikeys-context'

export function ApiKeysPrimaryButtons() {
  const { openDialog } = useApiKeysContext()

  return (
    <div className='flex items-center space-x-2'>
      <Button onClick={() => openDialog('create')} size='sm'>
        <Plus className='mr-2 h-4 w-4' />
        创建 API Key
      </Button>
    </div>
  )
}