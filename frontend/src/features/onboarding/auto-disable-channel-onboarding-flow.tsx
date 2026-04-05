'use client';

import { useCallback, useEffect, useRef, useState } from 'react';
import { useNavigate } from '@tanstack/react-router';
import { driver } from 'driver.js';
import 'driver.js/dist/driver.css';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { useCompleteAutoDisableChannelOnboarding } from '@/features/system/data/system';

interface AutoDisableChannelOnboardingFlowProps {
  onComplete?: () => void;
}

export function AutoDisableChannelOnboardingFlow({ onComplete }: AutoDisableChannelOnboardingFlowProps) {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const completeOnboarding = useCompleteAutoDisableChannelOnboarding();
  const [showPrompt, setShowPrompt] = useState(true);
  const completedRef = useRef(false);

  useEffect(() => {
    if (showPrompt) {
      document.body.style.overflow = 'hidden';
    } else {
      document.body.style.overflow = '';
    }

    return () => {
      document.body.style.overflow = '';
    };
  }, [showPrompt]);

  const markComplete = useCallback(() => {
    if (completedRef.current || completeOnboarding.isPending) return;
    completedRef.current = true;

    completeOnboarding.mutate(undefined, {
      onSuccess: () => {
        onComplete?.();
      },
      onError: () => {
        completedRef.current = false;
        toast.error(t('common.errors.onboardingFailed'));
      },
    });
  }, [completeOnboarding, onComplete, t]);

  const startOnboarding = useCallback(() => {
    setShowPrompt(false);

    navigate({ to: '/system', search: { tab: 'retry' } }).then(() => {
      setTimeout(() => {
        const driverObj = driver({
          showProgress: true,
          steps: [
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
          ],
          onDestroyStarted: () => {
            markComplete();
            driverObj.destroy();
          },
          onDestroyed: () => {
            markComplete();
          },
        });

        driverObj.drive();
      }, 500);
    });
  }, [navigate, t, markComplete]);

  const skipOnboarding = useCallback(() => {
    setShowPrompt(false);
    markComplete();
  }, [markComplete]);

  if (showPrompt === false) {
    return null;
  }

  return (
    <div className='fixed inset-0 z-50 flex items-center justify-center overflow-hidden bg-black/50'>
      <div className='absolute inset-0' onClick={() => {}} />
      <Card className='relative z-10 mx-4 w-full max-w-md'>
        <CardHeader className='text-center'>
          <CardTitle className='text-2xl'>{t('system.onboarding.autoDisableChannel.title')}</CardTitle>
          <CardDescription className='text-lg'>{t('system.onboarding.autoDisableChannel.description')}</CardDescription>
        </CardHeader>
        <CardContent className='space-y-4'>
          <div className='flex flex-col gap-3'>
            <Button onClick={startOnboarding} className='w-full' size='lg' data-testid='auto-disable-onboarding-start-tour'>
              {t('system.onboarding.autoDisableChannel.startTour')}
            </Button>
            <Button variant='outline' onClick={skipOnboarding} className='w-full' size='lg' data-testid='auto-disable-onboarding-skip-tour'>
              {t('system.onboarding.autoDisableChannel.skipTour')}
            </Button>
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
