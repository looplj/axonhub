'use client';

import { useCallback, useEffect, useRef, useState } from 'react';
import { useNavigate } from '@tanstack/react-router';
import { driver } from 'driver.js';
import 'driver.js/dist/driver.css';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { useCompleteOnboarding } from '@/features/system/data/system';
import { createTourNavigation, type TourStep } from './onboarding-tour';

interface OnboardingFlowProps {
  onComplete?: () => void;
}

export function OnboardingFlow({ onComplete }: OnboardingFlowProps) {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const completeOnboarding = useCompleteOnboarding();
  const [showPrompt, setShowPrompt] = useState(true);
  const completedRef = useRef(false);
  const cleanupTourRef = useRef<(() => void) | undefined>(undefined);

  useEffect(() => () => cleanupTourRef.current?.(), []);

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

  const markComplete = useCallback(
    (onSuccess?: () => void) => {
      if (completedRef.current || completeOnboarding.isPending) return;
      completedRef.current = true;

      completeOnboarding.mutate(undefined, {
        onSuccess: () => {
          toast.success(t('system.onboarding.completeTour'));
          onSuccess?.();
        },
        onError: () => {
          completedRef.current = false;
          setShowPrompt(true);
          toast.error(t('common.errors.onboardingFailed'));
        },
      });
    },
    [completeOnboarding, t],
  );

  const startOnboarding = useCallback(() => {
    setShowPrompt(false);

    cleanupTourRef.current?.();
    const steps: TourStep[] = [
      {
        destination: 'general',
        element: '#system-title',
        popover: {
          title: t('system.onboarding.steps.welcome.title'),
          description: t('system.onboarding.steps.welcome.description'),
          side: 'bottom',
          align: 'start',
        },
      },
      {
        destination: 'brand',
        element: '#brand-name',
        popover: {
          title: t('system.onboarding.steps.brandName.title'),
          description: t('system.onboarding.steps.brandName.description'),
          side: 'bottom',
          align: 'start',
        },
      },
      {
        destination: 'brand',
        element: '#brand-logo-upload',
        popover: {
          title: t('system.onboarding.steps.brandLogo.title'),
          description: t('system.onboarding.steps.brandLogo.description'),
          side: 'bottom',
          align: 'start',
        },
      },
      {
        destination: 'retry',
        element: '[data-value="retry"]',
        popover: {
          title: t('system.onboarding.steps.retryPolicy.title'),
          description: t('system.onboarding.steps.retryPolicy.description'),
          side: 'bottom',
          align: 'center',
        },
      },
      {
        destination: 'retry',
        element: '#retry-enabled-switch',
        popover: {
          title: t('system.onboarding.steps.retryEnabled.title'),
          description: t('system.onboarding.steps.retryEnabled.description'),
          side: 'right',
          align: 'start',
        },
      },
      {
        requiresRetryEnabled: true,
        destination: 'retry',
        element: '#retry-max-retries',
        popover: {
          title: t('system.onboarding.steps.retryMaxRetries.title'),
          description: t('system.onboarding.steps.retryMaxRetries.description'),
          side: 'right',
          align: 'start',
        },
      },
      {
        requiresRetryEnabled: true,
        destination: 'retry',
        element: '#max-single-channel-retries',
        popover: {
          title: t('system.onboarding.steps.retrySingleChannel.title'),
          description: t('system.onboarding.steps.retrySingleChannel.description'),
          side: 'right',
          align: 'start',
        },
      },
      {
        requiresRetryEnabled: true,
        destination: 'retry',
        element: '#retry-delay',
        popover: {
          title: t('system.onboarding.steps.retryDelay.title'),
          description: t('system.onboarding.steps.retryDelay.description'),
          side: 'right',
          align: 'start',
        },
      },
      {
        requiresRetryEnabled: true,
        destination: 'retry',
        popover: {
          title: t('system.onboarding.steps.autoDisableIntro.title'),
          description: t('system.onboarding.steps.autoDisableIntro.description'),
        },
      },
      {
        requiresRetryEnabled: true,
        destination: 'retry',
        element: '#auto-disable-channel',
        popover: {
          title: t('system.onboarding.steps.autoDisableToggle.title'),
          description: t('system.onboarding.steps.autoDisableToggle.description'),
          side: 'right',
          align: 'start',
        },
      },
      {
        requiresRetryEnabled: true,
        destination: 'retry',
        popover: {
          title: t('system.onboarding.steps.autoDisableComplete.title'),
          description: t('system.onboarding.steps.autoDisableComplete.description'),
        },
      },
      {
        destination: 'storage',
        element: '[data-value="storage"]',
        popover: {
          title: t('system.onboarding.steps.dataStorage.title'),
          description: t('system.onboarding.steps.dataStorage.description'),
          side: 'bottom',
          align: 'center',
        },
      },
      {
        destination: 'storage',
        element: '#default-data-storage',
        popover: {
          title: t('system.onboarding.steps.dataStorageSelection.title'),
          description: t('system.onboarding.steps.dataStorageSelection.description'),
          side: 'right',
          align: 'start',
        },
      },
      {
        destination: 'storage',
        element: '#storage-policy-store-chunks',
        popover: {
          title: t('system.onboarding.steps.storageChunks.title'),
          description: t('system.onboarding.steps.storageChunks.description'),
          side: 'right',
          align: 'start',
        },
      },
      {
        destination: 'storage',
        element: '#storage-policy-store-request-body',
        popover: {
          title: t('system.onboarding.steps.storageRequestBody.title'),
          description: t('system.onboarding.steps.storageRequestBody.description'),
          side: 'right',
          align: 'start',
        },
      },
      {
        destination: 'storage',
        element: '#storage-policy-store-response-body',
        popover: {
          title: t('system.onboarding.steps.storageResponseBody.title'),
          description: t('system.onboarding.steps.storageResponseBody.description'),
          side: 'right',
          align: 'start',
        },
      },
      {
        destination: 'storage',
        element: '#storage-cleanup-option-requests',
        popover: {
          title: t('system.onboarding.steps.storageCleanupRequests.title'),
          description: t('system.onboarding.steps.storageCleanupRequests.description'),
          side: 'right',
          align: 'start',
        },
      },
      {
        destination: 'storage',
        element: '#storage-cleanup-option-usage_logs',
        popover: {
          title: t('system.onboarding.steps.storageCleanupUsageLogs.title'),
          description: t('system.onboarding.steps.storageCleanupUsageLogs.description'),
          side: 'right',
          align: 'start',
        },
      },
      {
        destination: 'storage',
        popover: {
          title: t('system.onboarding.steps.complete.title'),
          description: t('system.onboarding.steps.complete.description'),
          side: 'bottom',
          align: 'start',
        },
        onHighlighted: () => {
          markComplete(() => {
            onComplete?.();
            navigate({ to: '/data-storages' });
          });
        },
      },
    ];
    const driverObj = driver({
      showProgress: true,
      steps,
      onNextClick: () => {
        const index = driverObj.getActiveIndex() ?? 0;
        if (index === steps.length - 1) {
          tour.cancel();
          markComplete(onComplete);
          driverObj.destroy();
        } else {
          void tour.moveTo(index + 1);
        }
      },
      onPrevClick: () => void tour.moveTo((driverObj.getActiveIndex() ?? 0) - 1, -1),
      onDestroyStarted: () => {
        tour.cancel();
        markComplete(onComplete);
        driverObj.destroy();
      },
      onDestroyed: () => tour.cancel(),
    });
    const tour = createTourNavigation({
      steps,
      driver: driverObj,
      navigate: (destination) => {
        if (destination === 'general' || destination === 'brand') {
          return navigate({ to: '/system', search: { tab: destination } });
        }
        return navigate({
          to: '/system/$section',
          params: { section: destination === 'retry' ? 'requests' : 'operations' },
          search: { tab: destination },
        });
      },
      onError: () => {
        tour.cancel();
        driverObj.destroy();
        setShowPrompt(true);
        toast.error(t('common.errors.onboardingFailed'));
      },
    });
    cleanupTourRef.current = () => {
      tour.cancel();
      driverObj.destroy();
    };
    void tour.moveTo(0);
  }, [markComplete, navigate, onComplete, t]);

  const skipOnboarding = useCallback(() => {
    setShowPrompt(false);
    markComplete(onComplete);
  }, [markComplete, onComplete]);

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
