'use client';

import { useEffect, useRef } from 'react';
import { driver } from 'driver.js';
import 'driver.js/dist/driver.css';
import { useTranslation } from 'react-i18next';
import { useCompleteSystemModelSettingOnboarding } from '@/features/system/data/system';

interface ModelsOnboardingFlowProps {
  onComplete?: () => void;
}

export function ModelsOnboardingFlow({ onComplete }: ModelsOnboardingFlowProps) {
  const { t } = useTranslation();
  const completeOnboarding = useCompleteSystemModelSettingOnboarding();
  const hasStartedRef = useRef(false);

  useEffect(() => {
    if (hasStartedRef.current) {
      return;
    }

    const settingsButton = document.querySelector('[data-settings-button]') as HTMLButtonElement;
    if (!settingsButton) {
      return;
    }

    hasStartedRef.current = true;

    let driverObj: ReturnType<typeof driver> | null = null;
    let clickHandlerAdded = false;

    setTimeout(() => {
      driverObj = driver({
        showProgress: false,
        showButtons: [],
        allowClose: false,
        steps: [
          {
            element: '[data-settings-button]',
            popover: {
              title: t('models.onboarding.steps.settingsButton.title'),
              description: t('models.onboarding.steps.settingsButton.description'),
              side: 'bottom',
              align: 'end',
              showButtons: [],
            },
            onHighlighted: () => {
              if (clickHandlerAdded) return;
              clickHandlerAdded = true;

              const highlightedElement = document.querySelector('[data-settings-button]') as HTMLButtonElement;
              if (!highlightedElement) return;

              const handleClick = () => {
                if (driverObj) {
                  driverObj.destroy();
                  driverObj = null;
                }
                completeOnboarding.mutate(undefined, {
                  onSuccess: () => {
                    onComplete?.();
                  },
                });
              };

              highlightedElement.addEventListener('click', handleClick, { once: true });
            },
          },
        ],
      });
      driverObj.drive();
    }, 500);

    return () => {
      if (driverObj) {
        driverObj.destroy();
      }
    };
  }, [completeOnboarding, onComplete, t]);

  return null;
}
