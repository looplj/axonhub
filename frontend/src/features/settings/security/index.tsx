import { useTranslation } from 'react-i18next';
import { Separator } from '@/components/ui/separator';
import SecurityForm from './security-form';

export default function SettingsSecurity() {
  const { t } = useTranslation();

  return (
    <div className='lg:max-w-2xl'>
      <div>
        <h3 className='text-lg font-medium'>{t('security.title')}</h3>
        <p className='text-muted-foreground text-sm'>{t('security.description')}</p>
      </div>
      <Separator className='my-4' />
      <SecurityForm />
    </div>
  );
}
