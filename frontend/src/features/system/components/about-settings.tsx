'use client';

import { ExternalLink, RefreshCw, CheckCircle, AlertCircle } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Skeleton } from '@/components/ui/skeleton';
import { useSystemVersion, useCheckForUpdate } from '../data/system';

export function AboutSettings() {
  const { t } = useTranslation();
  const { data: version, isLoading: versionLoading } = useSystemVersion();
  const { data: updateCheck, isFetching: isCheckingForUpdate, refetch: checkUpdate } = useCheckForUpdate();

  if (versionLoading) {
    return (
      <div className='space-y-6'>
        <Card>
          <CardHeader>
            <Skeleton className='h-6 w-48' />
            <Skeleton className='h-4 w-72' />
          </CardHeader>
          <CardContent className='space-y-4'>
            {[1, 2, 3, 4, 5].map((i) => (
              <Skeleton key={i} className='h-4 w-full' />
            ))}
          </CardContent>
        </Card>
      </div>
    );
  }

  return (
    <div className='space-y-6'>
      <Card>
        <CardHeader>
          <CardTitle>{t('system.about.title')}</CardTitle>
          <CardDescription>{t('system.about.description')}</CardDescription>
        </CardHeader>
        <CardContent className='space-y-6'>
          {/* Version Info */}
          <div className='space-y-4'>
            <div className='flex items-center justify-between'>
              <span className='text-muted-foreground text-sm'>{t('system.about.version')}</span>
              <Badge variant='secondary' className='font-mono'>
                {version?.version || '-'}
              </Badge>
            </div>

            {version?.commit && (
              <div className='flex items-center justify-between'>
                <span className='text-muted-foreground text-sm'>{t('system.about.commit')}</span>
                <span className='font-mono text-sm'>{version.commit.substring(0, 7)}</span>
              </div>
            )}

            {version?.buildTime && (
              <div className='flex items-center justify-between'>
                <span className='text-muted-foreground text-sm'>{t('system.about.buildTime')}</span>
                <span className='text-sm'>{version.buildTime}</span>
              </div>
            )}

            <div className='flex items-center justify-between'>
              <span className='text-muted-foreground text-sm'>{t('system.about.goVersion')}</span>
              <span className='text-sm'>{version?.goVersion || '-'}</span>
            </div>

            <div className='flex items-center justify-between'>
              <span className='text-muted-foreground text-sm'>{t('system.about.platform')}</span>
              <span className='text-sm'>{version?.platform || '-'}</span>
            </div>

            <div className='flex items-center justify-between'>
              <span className='text-muted-foreground text-sm'>{t('system.about.uptime')}</span>
              <span className='text-sm'>{version?.uptime || '-'}</span>
            </div>
          </div>

          {/* Update Check */}
          <div className='border-t pt-6'>
            <div className='flex items-center justify-between'>
              <div className='space-y-1'>
                <h4 className='text-sm font-medium'>{t('system.about.updateCheck.title')}</h4>
                <p className='text-muted-foreground text-sm'>{t('system.about.updateCheck.description')}</p>
              </div>
              <Button variant='outline' size='sm' onClick={() => checkUpdate()} disabled={isCheckingForUpdate}>
                <RefreshCw className={`mr-2 h-4 w-4 ${isCheckingForUpdate ? 'animate-spin' : ''}`} />
                {t('system.about.updateCheck.button')}
              </Button>
            </div>

            {updateCheck && !isCheckingForUpdate && (
              <div className='mt-4 rounded-lg border p-4'>
                {updateCheck.hasUpdate ? (
                  <div className='flex items-start gap-3'>
                    <AlertCircle className='mt-0.5 h-5 w-5 text-amber-500' />
                    <div className='flex-1 space-y-2'>
                      <p className='text-sm font-medium'>{t('system.about.updateCheck.newVersionAvailable')}</p>
                      <p className='text-muted-foreground text-sm'>
                        {t('system.about.updateCheck.currentVersion')}: {updateCheck.currentVersion} →{' '}
                        {t('system.about.updateCheck.latestVersion')}: {updateCheck.latestVersion}
                      </p>
                      <Button variant='link' size='sm' className='h-auto p-0' asChild>
                        <a href={updateCheck.releaseUrl} target='_blank' rel='noopener noreferrer'>
                          {t('system.about.updateCheck.viewRelease')}
                          <ExternalLink className='ml-1 h-3 w-3' />
                        </a>
                      </Button>
                    </div>
                  </div>
                ) : (
                  <div className='flex items-center gap-3'>
                    <CheckCircle className='h-5 w-5 text-green-500' />
                    <p className='text-sm'>{t('system.about.updateCheck.upToDate')}</p>
                  </div>
                )}
              </div>
            )}
          </div>

          {/* Links */}
          <div className='border-t pt-6'>
            <h4 className='mb-4 text-sm font-medium'>{t('system.about.links.title')}</h4>
            <div className='flex flex-wrap gap-4'>
              <Button variant='outline' size='sm' asChild>
                <a href='https://github.com/looplj/axonhub' target='_blank' rel='noopener noreferrer'>
                  GitHub
                  <ExternalLink className='ml-1 h-3 w-3' />
                </a>
              </Button>
              <Button variant='outline' size='sm' asChild>
                <a href='https://github.com/looplj/axonhub/releases' target='_blank' rel='noopener noreferrer'>
                  {t('system.about.links.releases')}
                  <ExternalLink className='ml-1 h-3 w-3' />
                </a>
              </Button>
              <Button variant='outline' size='sm' asChild>
                <a href='https://github.com/looplj/axonhub/issues' target='_blank' rel='noopener noreferrer'>
                  {t('system.about.links.issues')}
                  <ExternalLink className='ml-1 h-3 w-3' />
                </a>
              </Button>
            </div>
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
