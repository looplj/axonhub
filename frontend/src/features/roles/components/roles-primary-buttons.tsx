import { IconPlus } from '@tabler/icons-react'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import { useRolesContext } from '../context/roles-context'

export function RolesPrimaryButtons() {
  const { t } = useTranslation()
  const { setIsCreateDialogOpen } = useRolesContext()

  return (
    <div className='flex items-center space-x-2'>
      <Button onClick={() => setIsCreateDialogOpen(true)}>
        <IconPlus className='mr-2 h-4 w-4' />
        {t('roles.createRole')}
      </Button>
    </div>
  )
}