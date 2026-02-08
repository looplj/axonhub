'use client';

import { useCallback, useEffect, useState } from 'react';
import { useNavigate } from '@tanstack/react-router';
import { driver } from 'driver.js';
import 'driver.js/dist/driver.css';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { useCompleteOnboarding } from '@/features/system/data/system';

interface OnboardingFlowProps {
  onComplete?: () => void;
}

export function OnboardingFlow({ onComplete }: OnboardingFlowProps) {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const completeOnboarding = useCompleteOnboarding();
  const [showPrompt, setShowPrompt] = useState(true);

  useEffect(() => {
    // Prevent body scroll when modal is open
    if (showPrompt) {
      document.body.style.overflow = 'hidden';
    } else {
      document.body.style.overflow = '';
    }

    // Cleanup on unmount
    return () => {
      document.body.style.overflow = '';
    };
  }, [showPrompt]);

  const startOnboarding = useCallback(() => {
    setShowPrompt(false);

    // Navigate to system page first
    navigate({ to: '/system' }).then(() => {
      // Start the tour after navigation
      setTimeout(() => {
        const driverObj = driver({
          showProgress: true,
          steps: [
            {
              element: '#system-title',
              popover: {
                title: t('system.onboarding.steps.welcome.title'),
                description: t('system.onboarding.steps.welcome.description'),
                side: 'bottom',
                align: 'start',
              },
            },
            {
              element: '#brand-name',
              popover: {
                title: t('system.onboarding.steps.brandName.title'),
                description: t('system.onboarding.steps.brandName.description'),
                side: 'bottom',
                align: 'start',
              },
            },
            {
              element: '#brand-logo-upload',
              popover: {
                title: t('system.onboarding.steps.brandLogo.title'),
                description: t('system.onboarding.steps.brandLogo.description'),
                side: 'bottom',
                align: 'start',
              },
            },
            {
              element: '[data-value="retry"]',
              popover: {
                title: t('system.onboarding.steps.retryPolicy.title'),
                description: t('system.onboarding.steps.retryPolicy.description'),
                side: 'bottom',
                align: 'center',
              },
              onHighlighted: () => {
                // Switch to retry tab using URL navigation for more reliable tab switching
                setTimeout(() => {
                  navigate({ to: '/system', search: { tab: 'retry' } });
                }, 300);
              },
            },
            {
              element: '#retry-enabled-switch',
              popover: {
                title: t('system.onboarding.steps.retryEnabled.title'),
                description: t('system.onboarding.steps.retryEnabled.description'),
                side: 'right',
                align: 'start',
              },
            },
            {
              element: '#retry-max-retries',
              popover: {
                title: t('system.onboarding.steps.retryMaxRetries.title'),
                description: t('system.onboarding.steps.retryMaxRetries.description'),
                side: 'right',
                align: 'start',
              },
            },
            {
              element: '#max-single-channel-retries',
              popover: {
                title: t('system.onboarding.steps.retrySingleChannel.title'),
                description: t('system.onboarding.steps.retrySingleChannel.description'),
                side: 'right',
                align: 'start',
              },
            },
            {
              element: '#retry-delay',
              popover: {
                title: t('system.onboarding.steps.retryDelay.title'),
                description: t('system.onboarding.steps.retryDelay.description'),
                side: 'right',
                align: 'start',
              },
            },
            {
              popover: {
                title: t('system.onboarding.steps.autoDisableIntro.title'),
                description: t('system.onboarding.steps.autoDisableIntro.description'),
              },
            },
            {
              element: '#auto-disable-channel',
              popover: {
                title: t('system.onboarding.steps.autoDisableToggle.title'),
                description: t('system.onboarding.steps.autoDisableToggle.description'),
                side: 'right',
                align: 'start',
              },
            },
            {
              popover: {
                title: t('system.onboarding.steps.autoDisableComplete.title'),
                description: t('system.onboarding.steps.autoDisableComplete.description'),
              },
            },
            {
              element: '[data-value="storage"]',
              popover: {
                title: t('system.onboarding.steps.dataStorage.title'),
                description: t('system.onboarding.steps.dataStorage.description'),
                side: 'bottom',
                align: 'center',
              },
              onHighlighted: () => {
                // Switch to storage tab using URL navigation for more reliable tab switching
                setTimeout(() => {
                  navigate({ to: '/system', search: { tab: 'storage' } });
                }, 300);
              },
            },
            {
              element: '#default-data-storage',
              popover: {
                title: t('system.onboarding.steps.dataStorageSelection.title'),
                description: t('system.onboarding.steps.dataStorageSelection.description'),
                side: 'right',
                align: 'start',
              },
            },
            // {
            //   element: '#storage-enabled-switch',
            //   popover: {
            //     title: t('system.onboarding.steps.storageEnabled.title'),
            //     description: t('system.onboarding.steps.storageEnabled.description'),
            //     side: 'right',
            //     align: 'start',
            //   },
            // },
            {
              element: '#storage-policy-store-chunks',
              popover: {
                title: t('system.onboarding.steps.storageChunks.title'),
                description: t('system.onboarding.steps.storageChunks.description'),
                side: 'right',
                align: 'start',
              },
            },
            {
              element: '#storage-policy-store-request-body',
              popover: {
                title: t('system.onboarding.steps.storageRequestBody.title'),
                description: t('system.onboarding.steps.storageRequestBody.description'),
                side: 'right',
                align: 'start',
              },
            },
            {
              element: '#storage-policy-store-response-body',
              popover: {
                title: t('system.onboarding.steps.storageResponseBody.title'),
                description: t('system.onboarding.steps.storageResponseBody.description'),
                side: 'right',
                align: 'start',
              },
            },
            {
              element: '#storage-cleanup-option-requests',
              popover: {
                title: t('system.onboarding.steps.storageCleanupRequests.title'),
                description: t('system.onboarding.steps.storageCleanupRequests.description'),
                side: 'right',
                align: 'start',
              },
            },
            {
              element: '#storage-cleanup-option-usage_logs',
              popover: {
                title: t('system.onboarding.steps.storageCleanupUsageLogs.title'),
                description: t('system.onboarding.steps.storageCleanupUsageLogs.description'),
                side: 'right',
                align: 'start',
              },
            },
            {
              element: '#data-storages-link',
              popover: {
                title: t('system.onboarding.steps.complete.title'),
                description: t('system.onboarding.steps.complete.description'),
                side: 'bottom',
                align: 'start',
              },
              onHighlighted: () => {
                // Mark onboarding as completed
                completeOnboarding.mutate(undefined, {
                  onSuccess: () => {
                    toast.success(t('system.onboarding.completeTour'));
                    onComplete?.();
                    // Navigate to data storages page
                    navigate({ to: '/data-storages' });
                  },
                });
              },
            },
          ],
          onDestroyStarted: () => {
            // Complete onboarding when user closes the tour
            completeOnboarding.mutate(undefined, {
              onSuccess: () => {
                toast.success(t('system.onboarding.completeTour'));
                onComplete?.();
              },
            });
            driverObj.destroy();
          },
        });

        driverObj.drive();
      }, 500); // Wait a bit for the page to render
    });
  }, [completeOnboarding, navigate, t]);

  const skipOnboarding = useCallback(() => {
    setShowPrompt(false);
    completeOnboarding.mutate(undefined, {
      onSuccess: () => {
        onComplete?.();
      },
    });
  }, [completeOnboarding, onComplete]);

  if (showPrompt === false) {
    return null;
  }

  return (
    <div className='fixed inset-0 z-50 flex items-center justify-center overflow-hidden bg-black/50'>
      <div className='absolute inset-0' onClick={() => {}} />
      <Card className='relative z-10 mx-4 w-full max-w-md'>
        <CardHeader className='text-center'>
          <CardTitle className='text-2xl'>{t('system.onboarding.title')}</CardTitle>
          <CardDescription className='text-lg'>{t('system.onboarding.description')}</CardDescription>
        </CardHeader>
        <CardContent className='space-y-4'>
          <div className='flex flex-col gap-3'>
            <Button onClick={startOnboarding} className='w-full' size='lg' data-testid='onboarding-start-tour'>
              {t('system.onboarding.startTour')}
            </Button>
            <Button variant='outline' onClick={skipOnboarding} className='w-full' size='lg' data-testid='onboarding-skip-tour'>
              {t('system.onboarding.skipTour')}
            </Button>
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
