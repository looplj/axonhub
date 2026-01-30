import { memo } from 'react';
import { format } from 'date-fns';
import { useTranslation } from 'react-i18next';
import { Badge } from '@/components/ui/badge';
import { CHANNEL_CONFIGS } from '../data/config_channels';
import { Channel } from '../data/schema';

interface ChannelExpandedRowProps {
  channel: Channel;
  columnsLength: number;
  getApiFormatLabel: (apiFormat?: string) => string;
}

export const ChannelExpandedRow = memo(({ channel, columnsLength, getApiFormatLabel }: ChannelExpandedRowProps) => {
  const { t } = useTranslation();
  const config = CHANNEL_CONFIGS[channel.type];

  return (
    <div className='bg-muted/30 p-6 hover:bg-muted/50'>
      <div className='space-y-6'>
        <div className='grid grid-cols-1 gap-6 md:grid-cols-2'>
          <div className='space-y-3'>
            <h4 className='text-sm font-semibold'>{t('channels.expandedRow.basic')}</h4>
            <div className='space-y-2 text-sm'>
              <div className='flex items-start gap-2'>
                <span className='text-muted-foreground shrink-0'>{t('channels.columns.baseURL')}:</span>
                <span className='min-w-0 flex-1 text-right font-mono text-xs break-all'>{channel.baseURL}</span>
              </div>
              <div className='flex items-center justify-between'>
                <span className='text-muted-foreground'>{t('channels.columns.type')}:</span>
                <Badge variant='outline' className={config?.color}>
                  {t(`channels.types.${channel.type}`)}
                </Badge>
              </div>
              <div className='flex items-center justify-between'>
                <span className='text-muted-foreground'>{t('channels.expandedRow.apiFormat')}:</span>
                <span className='font-mono text-xs'>{getApiFormatLabel(config?.apiFormat)}</span>
              </div>
              <div className='flex justify-between'>
                <span className='text-muted-foreground'>{t('common.columns.createdAt')}:</span>
                <span>{format(channel.createdAt, 'yyyy-MM-dd HH:mm')}</span>
              </div>
              <div className='flex justify-between'>
                <span className='text-muted-foreground'>{t('common.columns.updatedAt')}:</span>
                <span>{format(channel.updatedAt, 'yyyy-MM-dd HH:mm')}</span>
              </div>
            </div>
          </div>

          <div className='space-y-6'>
            <div className='space-y-3'>
              <h4 className='text-sm font-semibold'>{t('channels.expandedRow.additional')}</h4>
              <div className='space-y-2 text-sm'>
                <div className='flex items-center justify-between'>
                  <span className='text-muted-foreground'>{t('channels.columns.orderingWeight')}:</span>
                  <span className='font-mono text-xs'>{channel.orderingWeight ?? 0}</span>
                </div>
                <div className='flex justify-between'>
                  <span className='text-muted-foreground'>{t('channels.expandedRow.remark')}:</span>
                  <span className='max-w-[200px] truncate text-right' title={channel.remark || undefined}>
                    {channel.remark || '-'}
                  </span>
                </div>
                <div className='flex items-start justify-between'>
                  <span className='text-muted-foreground shrink-0'>{t('channels.expandedRow.tags')}:</span>
                  <div className='flex max-w-[200px] flex-wrap justify-end gap-1'>
                    {channel.tags && channel.tags.length > 0 ? (
                      channel.tags.map((tag) => (
                        <Badge key={tag} variant='outline' className='text-xs'>
                          {tag}
                        </Badge>
                      ))
                    ) : (
                      <span>-</span>
                    )}
                  </div>
                </div>
              </div>
            </div>

          </div>
        </div>

        {channel.supportedModels && channel.supportedModels.length > 0 && (
          <div className='space-y-3'>
            <h4 className='text-sm font-semibold'>{t('channels.expandedRow.supportedModels')}</h4>
            <div className='flex flex-wrap gap-2'>
              {channel.supportedModels.slice(0, 5).map((model) => (
                <Badge key={model} variant='secondary' className='font-mono text-xs'>
                  {model}
                </Badge>
              ))}
              {channel.supportedModels.length > 5 && (
                <span className='text-muted-foreground flex items-center text-xs italic'>
                  {t('channels.expandedRow.moreModels', { count: channel.supportedModels.length - 5 })}
                </span>
              )}
            </div>
          </div>
        )}
      </div>
    </div>
  );
});

ChannelExpandedRow.displayName = 'ChannelExpandedRow';
