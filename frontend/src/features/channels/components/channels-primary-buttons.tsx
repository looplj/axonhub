import { IconPlus, IconUpload, IconArrowsSort } from '@tabler/icons-react'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import { useChannels } from '../context/channels-context'
import { PermissionGuard } from '@/components/permission-guard'

export function ChannelsPrimaryButtons() {
  const { t } = useTranslation()
  const { setOpen } = useChannels()
  
  return (
    <div className='flex gap-2'>
      {/* Bulk Import - requires write_channels permission */}
      <PermissionGuard requiredScope='write_channels'>
        <Button
          variant='outline'
          className='space-x-1'
          onClick={() => setOpen('bulkImport')}
        >
          <span>{t('channels.importChannels', '批量导入')}</span> <IconUpload size={18} />
        </Button>
      </PermissionGuard>
      
      {/* Bulk Ordering - requires write_channels permission */}
      <PermissionGuard requiredScope='write_channels'>
        <Button
          variant='outline'
          className='space-x-1'
          onClick={() => setOpen('bulkOrdering')}
        >
          <span>{t('channels.orderChannels')}</span> <IconArrowsSort size={18} />
        </Button>
      </PermissionGuard>
      
      {/* <Button
        variant='outline'
        className='space-x-1'
        onClick={() => setOpen('settings')}
      >
        <span>{t('channels.settings')}</span> <IconSettings size={18} />
      </Button> */}
      
      {/* Add Channel - requires write_channels permission */}
      <PermissionGuard requiredScope='write_channels'>
        <Button className='space-x-1' onClick={() => setOpen('add')}>
          <span>{t('channels.addChannel')}</span> <IconPlus size={18} />
        </Button>
      </PermissionGuard>
    </div>
  )
}