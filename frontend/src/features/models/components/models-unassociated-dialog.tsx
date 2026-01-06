import { useCallback, useEffect, useMemo, useState } from 'react';
import { IconAlertCircle, IconSearch } from '@tabler/icons-react';
import { useTranslation } from 'react-i18next';
import { useDebounce } from '@/hooks/use-debounce';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { ScrollArea } from '@/components/ui/scroll-area';
import { useModels } from '../context/models-context';
import { useQueryUnassociatedChannels } from '../data/models';
import { ChannelModelsList } from './channel-models-list';

export function ModelsUnassociatedDialog() {
  const { t } = useTranslation();
  const { open, setOpen } = useModels();
  const { data, refetch, isLoading, isFetching } = useQueryUnassociatedChannels();
  const [searchQuery, setSearchQuery] = useState('');
  const debouncedSearchQuery = useDebounce(searchQuery, 300);

  const isOpen = open === 'unassociated';

  useEffect(() => {
    if (isOpen) {
      refetch();
      setSearchQuery('');
    }
  }, [isOpen, refetch]);

  const handleClose = useCallback(() => {
    setOpen(null);
  }, [setOpen]);

  const filteredData = useMemo(() => {
    if (!data || !debouncedSearchQuery.trim()) return data;

    const query = debouncedSearchQuery.toLowerCase();
    return data
      .map((info) => ({
        ...info,
        models: info.models.filter((model) => model.toLowerCase().includes(query)),
      }))
      .filter((info) => info.models.length > 0);
  }, [data, debouncedSearchQuery]);

  const channelsForList = useMemo(() => {
    if (!filteredData) return [];
    return filteredData.map((info) => ({
      channel: info.channel,
      models: info.models.map((model) => ({ requestModel: model })),
    }));
  }, [filteredData]);

  return (
    <Dialog open={isOpen} onOpenChange={handleClose}>
      <DialogContent className='flex max-h-[90vh] flex-col overflow-hidden sm:max-w-6xl'>
        <DialogHeader className='flex-shrink-0'>
          <DialogTitle>{t('models.unassociated.title')}</DialogTitle>
          <DialogDescription>{t('models.unassociated.description')}</DialogDescription>
        </DialogHeader>

        <div className='flex-1 space-y-4 overflow-y-auto'>
          {isLoading || isFetching ? (
            <div className='flex items-center justify-center py-8'>
              <div className='text-muted-foreground text-sm'>{t('common.loading')}</div>
            </div>
          ) : data && data.length > 0 ? (
            <>
              <div className='flex items-center gap-2 rounded-lg border border-yellow-200 bg-yellow-50 p-3 dark:border-yellow-900 dark:bg-yellow-950'>
                <IconAlertCircle className='h-5 w-5 text-yellow-600 dark:text-yellow-500' />
                <div className='text-sm text-yellow-800 dark:text-yellow-200'>
                  {t('models.unassociated.summary', {
                    channelCount: data.length,
                    modelCount: data.reduce((sum, info) => sum + info.models.length, 0),
                  })}
                </div>
              </div>

              <div className='relative'>
                <IconSearch className='text-muted-foreground absolute top-1/2 left-3 h-4 w-4 -translate-y-1/2' />
                <Input
                  placeholder={t('models.unassociated.searchPlaceholder')}
                  value={searchQuery}
                  onChange={(e) => setSearchQuery(e.target.value)}
                  className='pl-9'
                />
              </div>

              <ScrollArea className='h-[400px] rounded-md border p-4'>
                <ChannelModelsList
                  channels={channelsForList}
                  emptyMessage={
                    debouncedSearchQuery.trim()
                      ? t('models.unassociated.noSearchResults')
                      : t('models.unassociated.noUnassociated')
                  }
                />
              </ScrollArea>
            </>
          ) : (
            <div className='flex flex-col items-center justify-center py-8 text-center'>
              <div className='text-muted-foreground text-sm'>{t('models.unassociated.noUnassociated')}</div>
            </div>
          )}
        </div>

        <div className='flex flex-shrink-0 justify-end border-t pt-4'>
          <Button onClick={handleClose}>{t('common.close')}</Button>
        </div>
      </DialogContent>
    </Dialog>
  );
}
