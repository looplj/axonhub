import { IconPlus, IconSettings } from '@tabler/icons-react'
import { Button } from '@/components/ui/button'
import { useChannels } from '../context/channels-context'

export function ChannelsPrimaryButtons() {
  const { setOpen } = useChannels()
  
  return (
    <div className='flex gap-2'>
      <Button
        variant='outline'
        className='space-x-1'
        onClick={() => setOpen('settings')}
      >
        <span>设置</span> <IconSettings size={18} />
      </Button>
      <Button className='space-x-1' onClick={() => setOpen('add')}>
        <span>添加 Channel</span> <IconPlus size={18} />
      </Button>
    </div>
  )
}