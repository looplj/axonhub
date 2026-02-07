'use client';

import React, { useEffect, useState } from 'react';
import { useOnboardingInfo } from '@/features/system/data/system';
import { AutoDisableChannelOnboardingFlow } from './auto-disable-channel-onboarding-flow';
import { OnboardingFlow } from './onboarding-flow';

type OnboardingMode = 'none' | 'main' | 'autoDisableChannel';

interface OnboardingProviderProps {
  children: React.ReactNode;
  showOnboarding?: boolean;
  onComplete?: () => void;
}

export function OnboardingProvider({ children, showOnboarding = true, onComplete }: OnboardingProviderProps) {
  const { data: onboardingInfo, isLoading } = useOnboardingInfo();
  const [mode, setMode] = useState<OnboardingMode>('none');

  useEffect(() => {
    if (!isLoading && showOnboarding) {
      if (!onboardingInfo || !onboardingInfo.onboarded) {
        setMode('main');
      } else if (!onboardingInfo.autoDisableChannel?.onboarded) {
        setMode('autoDisableChannel');
      } else {
        setMode('none');
      }
    }
  }, [onboardingInfo, isLoading, showOnboarding]);

  const handleComplete = () => {
    setMode('none');
    onComplete?.();
  };

  return (
    <>
      {children}
      {mode === 'main' && <OnboardingFlow onComplete={handleComplete} />}
      {mode === 'autoDisableChannel' && <AutoDisableChannelOnboardingFlow onComplete={handleComplete} />}
    </>
  );
}
