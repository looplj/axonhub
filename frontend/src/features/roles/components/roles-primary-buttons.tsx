import { IconPlus } from '@tabler/icons-react'
import { Button } from '@/components/ui/button'
import { useRolesContext } from '../context/roles-context'

export function RolesPrimaryButtons() {
  const { setIsCreateDialogOpen } = useRolesContext()

  return (
    <div className='flex items-center space-x-2'>
      <Button onClick={() => setIsCreateDialogOpen(true)}>
        <IconPlus className='mr-2 h-4 w-4' />
        新建角色
      </Button>
    </div>
  )
}