'use client';

import React, { useEffect, useState } from 'react';
import { useOnboardingInfo } from '@/features/system/data/system';
import { OnboardingFlow } from './onboarding-flow';

interface OnboardingProviderProps {
  children: React.ReactNode;
  showOnboarding?: boolean;
  onComplete?: () => void;
}

export function OnboardingProvider({ children, showOnboarding = true, onComplete }: OnboardingProviderProps) {
  const { data: onboardingInfo, isLoading } = useOnboardingInfo();
  const [shouldShowOnboarding, setShouldShowOnboarding] = useState(false);

  useEffect(() => {
    if (!isLoading && showOnboarding) {
      // If onboardingInfo is null or not onboarded, show onboarding
      if (!onboardingInfo || !onboardingInfo.onboarded) {
        setShouldShowOnboarding(true);
      } else {
        setShouldShowOnboarding(false);
      }
    }
  }, [onboardingInfo, isLoading, showOnboarding]);

  return (
    <>
      {children}
      {shouldShowOnboarding && <OnboardingFlow onComplete={onComplete} />}
    </>
  );
}
