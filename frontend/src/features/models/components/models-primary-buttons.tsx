import { Plus } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { useModelsContext } from '../context/models-context'

export function ModelsPrimaryButtons() {
  const { openDialog } = useModelsContext()

  return (
    <div className='flex items-center space-x-2'>
      <Button onClick={() => openDialog('create')} size='sm'>
        <Plus className='mr-2 h-4 w-4' />
        创建模型
      </Button>
    </div>
  )
}