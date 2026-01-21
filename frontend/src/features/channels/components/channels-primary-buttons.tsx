import { IconPlus, IconUpload, IconArrowsSort, IconSettings, IconScale } from '@tabler/icons-react';
import { useTranslation } from 'react-i18next';
import { useNavigate } from '@tanstack/react-router';
import { Button } from '@/components/ui/button';
import { PermissionGuard } from '@/components/permission-guard';
import { useChannels } from '../context/channels-context';

export function ChannelsPrimaryButtons() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const { setOpen } = useChannels();

  return (
    <div className='flex gap-2'>
      <PermissionGuard requiredScope='read_system'>
        {/* Load Balancing Strategy - navigate to system retry configuration */}
        <Button
          variant='outline'
          className='space-x-1'
          onClick={() => navigate({ to: '/system', search: { tab: 'retry' } })}
        >
          <span>{t('channels.loadBalancingStrategy')}</span> <IconScale size={18} />
        </Button>
      </PermissionGuard>

      <PermissionGuard requiredScope='write_channels'>
        <>
          {/* Settings - requires write_channels permission */}
          <Button variant='outline' className='space-x-1' onClick={() => setOpen('channelSettings')}>
            <span>{t('channels.actions.settings')}</span> <IconSettings size={18} />
          </Button>

          {/* Bulk Import - requires write_channels permission */}
          <Button variant='outline' className='space-x-1' onClick={() => setOpen('bulkImport')}>
            <span>{t('channels.importChannels', '批量导入')}</span> <IconUpload size={18} />
          </Button>

          {/* Bulk Ordering - requires write_channels permission */}
          <Button variant='outline' className='space-x-1' onClick={() => setOpen('bulkOrdering')}>
            <span>{t('channels.orderChannels')}</span> <IconArrowsSort size={18} />
          </Button>

          {/* Add Channel - requires write_channels permission */}
          <Button className='space-x-1' onClick={() => setOpen('add')} data-testid='add-channel-button'>
            <span>{t('channels.addChannel')}</span> <IconPlus size={18} />
          </Button>
        </>
      </PermissionGuard>
    </div>
  );
}
