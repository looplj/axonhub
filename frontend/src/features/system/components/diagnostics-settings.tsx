'use client';

import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger,
} from '@/components/ui/alert-dialog';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { RefreshCw } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { useClearCache, useExportCacheDiagnostics } from '../data/system';

export function DiagnosticsSettings() {
  const { t } = useTranslation();
  const { mutate: exportDiagnostics, isPending: isExportingDiagnostics } = useExportCacheDiagnostics();
  const { mutate: clearCache, isPending: isClearingCache } = useClearCache();

  return (
    <div className='space-y-6'>
      <Card>
        <CardHeader>
          <CardTitle>{t('system.diagnostics.title')}</CardTitle>
          <CardDescription>{t('system.diagnostics.description')}</CardDescription>
        </CardHeader>
        <CardContent className='space-y-6'>
          <div className='rounded-lg border p-3 sm:p-4'>
            <div className='flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4'>
              <div className='space-y-1 min-w-0'>
                <h4 className='text-sm font-medium'>{t('system.diagnostics.cache.title')}</h4>
                <p className='text-muted-foreground text-sm'>{t('system.diagnostics.cache.description')}</p>
              </div>
              <div className='flex flex-col sm:flex-row gap-2'>
                <Button variant='outline' size='sm' onClick={() => exportDiagnostics()} disabled={isExportingDiagnostics} className='w-full sm:w-auto'>
                  <RefreshCw className={`mr-2 h-4 w-4 ${isExportingDiagnostics ? 'animate-spin' : ''}`} />
                  {t('system.diagnostics.cache.export')}
                </Button>
                <AlertDialog>
                  <AlertDialogTrigger asChild>
                    <Button variant='outline' size='sm' disabled={isClearingCache} className='w-full sm:w-auto'>
                      <RefreshCw className={`mr-2 h-4 w-4 ${isClearingCache ? 'animate-spin' : ''}`} />
                      {t('system.diagnostics.cache.clear')}
                    </Button>
                  </AlertDialogTrigger>
                  <AlertDialogContent className='sm:max-w-md mx-4'>
                    <AlertDialogHeader>
                      <AlertDialogTitle>{t('system.diagnostics.cache.clearConfirmTitle')}</AlertDialogTitle>
                      <AlertDialogDescription>{t('system.diagnostics.cache.clearConfirmDescription')}</AlertDialogDescription>
                    </AlertDialogHeader>
                    <AlertDialogFooter className='flex-col sm:flex-row gap-2'>
                      <AlertDialogCancel className='w-full sm:w-auto mt-0'>{t('system.diagnostics.cache.clearCancel')}</AlertDialogCancel>
                      <AlertDialogAction onClick={() => clearCache()} className='w-full sm:w-auto'>{t('system.diagnostics.cache.clearConfirm')}</AlertDialogAction>
                    </AlertDialogFooter>
                  </AlertDialogContent>
                </AlertDialog>
              </div>
            </div>
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
