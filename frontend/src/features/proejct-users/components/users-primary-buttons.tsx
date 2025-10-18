import { IconUserPlus } from '@tabler/icons-react'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import { PermissionGuard } from '@/components/permission-guard'
import { useUsers } from '../context/users-context'

export function UsersPrimaryButtons() {
  const { t } = useTranslation()
  const { setOpen } = useUsers()
  return (
    <div className='flex gap-2'>
      {/* Add User - requires system-level read_users and any-level write_users */}
      <PermissionGuard requiredSystemScope='read_users' requiredScope='write_users'>
        <Button className='space-x-1' onClick={() => setOpen('add')}>
          <span>{t('users.addUser')}</span> <IconUserPlus size={18} />
        </Button>
      </PermissionGuard>
    </div>
  )
}
