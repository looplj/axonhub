import { format } from 'date-fns';
import { useParams, useNavigate } from '@tanstack/react-router';
import { ArrowLeft, FileText } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { extractNumberID } from '@/lib/utils';
import { Button } from '@/components/ui/button';
import { Separator } from '@/components/ui/separator';
import { Header } from '@/components/layout/header';
import { Main } from '@/components/layout/main';
import { useRequest } from '../data';
import { RequestDetailContent } from './request-detail-content';

export default function RequestDetailGlobalPage() {
  const { t } = useTranslation();
  const { requestId } = useParams({ from: '/_authenticated/requests/$requestId' });
  const navigate = useNavigate();
  const { data: request } = useRequest(requestId, { projectId: null });

  return (
    <div className='flex h-screen flex-col'>
      <Header className='bg-background/95 supports-[backdrop-filter]:bg-background/60 border-b backdrop-blur'>
        <div className='flex items-center space-x-4'>
          <Button variant='ghost' size='sm' onClick={() => navigate({ to: '/channels' })} className='hover:bg-accent'>
            <ArrowLeft className='mr-2 h-4 w-4' />
            {t('common.back')}
          </Button>
          <Separator orientation='vertical' className='h-6' />
          <div className='flex items-center space-x-3'>
            <div className='bg-primary/10 flex h-8 w-8 items-center justify-center rounded-lg'>
              <FileText className='text-primary h-4 w-4' />
            </div>
            <div>
              <h1 className='text-lg leading-none font-semibold'>
                {t('requests.detail.title')} #{request ? extractNumberID(request.id) || request.id : extractNumberID(requestId) || requestId}
              </h1>
              {request && (
                <div className='mt-1 flex items-center gap-2'>
                  <p className='text-muted-foreground text-sm'>{request.modelID || t('requests.columns.unknown')}</p>
                  <span className='text-muted-foreground text-xs'>•</span>
                  <p className='text-muted-foreground text-xs'>{format(new Date(request.createdAt), 'yyyy-MM-dd HH:mm:ss')}</p>
                </div>
              )}
            </div>
          </div>
        </div>
      </Header>

      <Main className='flex-1 overflow-auto'>
        <div className='container mx-auto max-w-7xl p-6'>
          <RequestDetailContent requestId={requestId} projectId={null} />
        </div>
      </Main>
    </div>
  );
}
