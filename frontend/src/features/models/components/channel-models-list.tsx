import type { ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import { IconSnowflake } from '@tabler/icons-react';
import { Badge } from '@/components/ui/badge';

interface ChannelModel {
  requestModel: string;
}

interface Channel {
  id: string | number;
  name: string;
  type: string;
  status: string;
}

interface KeyGroupRef {
  index: number;
  name: string;
}

interface ChannelModelsListProps {
  channels: Array<{
    channel: Channel;
    keyGroup?: KeyGroupRef | null;
    models: ChannelModel[];
  }>;
  emptyMessage?: string;
}

// A single preview card. Exported separately from the list so the association
// dialog can wrap each card in its own sortable container and keep this
// component purely presentational.
export function ChannelModelsCard({
  connection,
  dragHandle,
  frozenModels,
  freezeMenu,
}: {
  connection: { channel: Channel; keyGroup?: KeyGroupRef | null; models: ChannelModel[] };
  dragHandle?: ReactNode;
  // Models of this card that are currently frozen (channel x model cooldown).
  frozenModels?: string[];
  // Optional freeze/unfreeze menu rendered on the card header.
  freezeMenu?: ReactNode;
}) {
  const frozenSet = new Set(frozenModels ?? []);
  const { t } = useTranslation();

  const getStatusColor = (status: string) => {
    switch (status) {
      case 'enabled':
        return 'bg-emerald-50 text-emerald-700 border-emerald-200 dark:bg-emerald-950 dark:text-emerald-400 dark:border-emerald-800';
      case 'disabled':
        return 'bg-gray-50 text-gray-600 border-gray-200 dark:bg-gray-900 dark:text-gray-400 dark:border-gray-700';
      case 'archived':
        return 'bg-amber-50 text-amber-700 border-amber-200 dark:bg-amber-950 dark:text-amber-400 dark:border-amber-800';
      default:
        return 'bg-gray-50 text-gray-600 border-gray-200 dark:bg-gray-900 dark:text-gray-400 dark:border-gray-700';
    }
  };

  const getTypeColor = (type: string) => {
    const colors = {
      openai: 'bg-blue-50 text-blue-700 border-blue-200 dark:bg-blue-950 dark:text-blue-400',
      anthropic: 'bg-purple-50 text-purple-700 border-purple-200 dark:bg-purple-950 dark:text-purple-400',
      deepseek: 'bg-indigo-50 text-indigo-700 border-indigo-200 dark:bg-indigo-950 dark:text-indigo-400',
      doubao: 'bg-orange-50 text-orange-700 border-orange-200 dark:bg-orange-950 dark:text-orange-400',
      kimi: 'bg-pink-50 text-pink-700 border-pink-200 dark:bg-pink-950 dark:text-pink-400',
    };
    return colors[type as keyof typeof colors] || 'bg-gray-50 text-gray-700 border-gray-200 dark:bg-gray-900 dark:text-gray-400';
  };

  return (
    <div className='rounded-lg border p-3'>
      <div className='mb-2 flex items-start gap-1.5'>
        {dragHandle}
        <div className='flex flex-wrap items-center gap-1.5'>
          <span className='text-sm font-medium'>{connection.channel.name}</span>
          <Badge variant='outline' className={`h-5 px-1.5 text-[10px] font-normal ${getTypeColor(connection.channel.type)}`}>
            {t(`channels.types.${connection.channel.type}`, connection.channel.type)}
          </Badge>
          <Badge variant='outline' className={`h-5 px-1.5 text-[10px] font-normal ${getStatusColor(connection.channel.status)}`}>
            {t(`channels.status.${connection.channel.status}`)}
          </Badge>
          {connection.keyGroup && (
            <Badge variant='secondary' className='h-5 px-1.5 text-[10px] font-normal'>
              {connection.keyGroup.name}
            </Badge>
          )}
          {frozenSet.size > 0 && (
            <Badge variant='outline' className='h-5 px-1.5 text-[10px] font-normal text-cyan-600 dark:text-cyan-400'>
              <IconSnowflake className='mr-0.5 h-3 w-3' />
              {t('models.dialogs.association.cooldown.frozenBadge')}
            </Badge>
          )}
        </div>
        {freezeMenu && <div className='ml-auto shrink-0'>{freezeMenu}</div>}
      </div>
      <div className='space-y-1'>
        {connection.models.length === 0 ? (
          <p className='text-muted-foreground text-xs'>{t('models.dialogs.association.previewNoModels')}</p>
        ) : (
          connection.models.map((model) => (
            <div key={model.requestModel} className='bg-muted flex items-center gap-1 rounded px-2 py-1 text-xs'>
              {model.requestModel}
              {frozenSet.has(model.requestModel) && (
                <IconSnowflake className='h-3 w-3 shrink-0 text-cyan-600 dark:text-cyan-400' />
              )}
            </div>
          ))
        )}
      </div>
    </div>
  );
}

export function ChannelModelsList({ channels, emptyMessage }: ChannelModelsListProps) {
  const { t } = useTranslation();

  if (channels.length === 0) {
    return (
      <p className='text-muted-foreground py-8 text-center text-sm'>{emptyMessage || t('models.dialogs.association.noConnections')}</p>
    );
  }

  return (
    <div className='space-y-3'>
      {channels.map((conn) => (
        <ChannelModelsCard key={conn.channel.id} connection={conn} />
      ))}
    </div>
  );
}
