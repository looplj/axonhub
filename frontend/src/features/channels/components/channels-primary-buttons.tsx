import { IconPlus, IconUpload, IconArrowsSort } from '@tabler/icons-react'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import { useChannels } from '../context/channels-context'

export function ChannelsPrimaryButtons() {
  const { t } = useTranslation()
  const { setOpen } = useChannels()
  
  return (
    <div className='flex gap-2'>
      <Button
        variant='outline'
        className='space-x-1'
        onClick={() => setOpen('bulkImport')}
      >
        <span>{t('channels.importChannels', '批量导入')}</span> <IconUpload size={18} />
      </Button>
      
      <Button
        variant='outline'
        className='space-x-1'
        onClick={() => setOpen('bulkOrdering')}
      >
        <span>{t('channels.orderChannels')}</span> <IconArrowsSort size={18} />
      </Button>
      
      {/* <Button
        variant='outline'
        className='space-x-1'
        onClick={() => setOpen('settings')}
      >
        <span>{t('channels.settings')}</span> <IconSettings size={18} />
      </Button> */}
      <Button className='space-x-1' onClick={() => setOpen('add')}>
        <span>{t('channels.addChannel')}</span> <IconPlus size={18} />
      </Button>
    </div>
  )
}