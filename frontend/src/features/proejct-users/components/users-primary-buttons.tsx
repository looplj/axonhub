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
      {/* Invite User - commented out for now
      // Note: Add IconMailPlus import when enabling this feature
      <PermissionGuard requiredScope='write_users'>
        <Button
          variant='outline'
          className='space-x-1'
          onClick={() => setOpen('invite')}
        >
          <span>{t('users.inviteUser')}</span> <IconMailPlus size={18} />
        </Button>
      </PermissionGuard>
      */}
      
      {/* Add User - requires write_users permission */}
      <PermissionGuard requiredScope='write_users'>
        <Button className='space-x-1' onClick={() => setOpen('add')}>
          <span>{t('users.addUser')}</span> <IconUserPlus size={18} />
        </Button>
      </PermissionGuard>
    </div>
  )
}
