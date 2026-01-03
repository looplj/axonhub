import { memo } from 'react';
import { AlertCircle, Filter } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { Alert, AlertDescription } from '@/components/ui/alert';
import { Button } from '@/components/ui/button';

interface ChannelsErrorBannerProps {
  errorCount: number;
  onFilterErrorChannels: () => void;
  showErrorOnly?: boolean;
  onExitErrorOnlyMode?: () => void;
}

export const ChannelsErrorBanner = memo(function ChannelsErrorBanner({ errorCount, onFilterErrorChannels, showErrorOnly, onExitErrorOnlyMode }: ChannelsErrorBannerProps) {
  const { t } = useTranslation();

  if (errorCount === 0) {
    return null;
  }

  return (
    <Alert className='mb-4 items-center border-orange-200 bg-orange-50 text-orange-800 dark:border-orange-800 dark:bg-orange-950 dark:text-orange-200 [&>svg]:translate-y-0'>
      <AlertCircle className='h-4 w-4' />
      <AlertDescription className='flex items-center justify-between'>
        <div>
          {showErrorOnly ? (
            <span>{t('channels.errorBanner.errorOnlyMode')}</span>
          ) : (
            <span>{t('channels.errorBanner.message', { count: errorCount })}</span>
          )}
        </div>
        <div className='flex items-center space-x-2'>
          {showErrorOnly && onExitErrorOnlyMode && (
            <Button
              variant='outline'
              size='sm'
              onClick={onExitErrorOnlyMode}
              className='border-orange-300 bg-orange-100 hover:bg-orange-200 dark:border-orange-700 dark:bg-orange-900 dark:hover:bg-orange-800'
            >
              {t('channels.errorBanner.exitErrorOnlyButton')}
            </Button>
          )}
          {!showErrorOnly && (
            <Button
              variant='outline'
              size='sm'
              onClick={onFilterErrorChannels}
              className='border-orange-300 bg-orange-100 hover:bg-orange-200 dark:border-orange-700 dark:bg-orange-900 dark:hover:bg-orange-800'
            >
              <Filter className='mr-2 h-4 w-4' />
              {t('channels.errorBanner.filterButton')}
            </Button>
          )}
        </div>
      </AlertDescription>
    </Alert>
  );
});
