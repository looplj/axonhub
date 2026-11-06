import type { DriveStep, Driver } from 'driver.js';

export type TourDestination = 'general' | 'brand' | 'retry' | 'storage';

export interface TourStep extends Omit<DriveStep, 'element'> {
  destination: TourDestination;
  element?: string;
  requiresRetryEnabled?: boolean;
}

const readyTargets: Record<TourDestination, string> = {
  general: '#system-title',
  brand: '#brand-name',
  retry: '#retry-enabled-switch',
  storage: '#default-data-storage',
};

export function waitForVisibleElement(selector: string, signal: AbortSignal): Promise<void> {
  return new Promise((resolve, reject) => {
    let timer: ReturnType<typeof setTimeout>;
    const deadline = Date.now() + 10000;
    const finish = (error?: Error) => {
      clearTimeout(timer);
      signal.removeEventListener('abort', abort);
      if (error) reject(error);
      else resolve();
    };
    const abort = () => finish(new Error('Tour cancelled'));
    const check = () => {
      if (signal.aborted) return abort();
      const element = document.querySelector(selector);
      if (element?.checkVisibility({ checkVisibilityCSS: true, checkOpacity: true })) return finish();
      if (Date.now() >= deadline) return finish(new Error(`Tour target unavailable: ${selector}`));
      timer = setTimeout(check, 50);
    };
    signal.addEventListener('abort', abort, { once: true });
    check();
  });
}

// Both directions go through the same preparation, before Driver resolves its target.
export function createTourNavigation({ steps, driver, navigate, onError }: {
  steps: TourStep[];
  driver: Pick<Driver, 'drive'>;
  navigate: (destination: TourDestination) => Promise<unknown>;
  onError: () => void;
}) {
  const controller = new AbortController();
  const { signal } = controller;
  let pending = false;

  return {
    cancel: () => controller.abort(),
    async moveTo(index: number, direction = 1) {
      if (pending || signal.aborted || !steps[index]) return;
      pending = true;
      try {
        const step = steps[index];
        await navigate(step.destination);
        if (signal.aborted) return;
        await waitForVisibleElement(readyTargets[step.destination], signal);
        // Retry details are intentionally absent when retries are disabled.
        while (steps[index]?.requiresRetryEnabled && document.querySelector('#retry-enabled')?.getAttribute('aria-checked') !== 'true') {
          index += direction;
        }
        const target = steps[index];
        if (!target || signal.aborted) return;
        if (target.destination !== step.destination) {
          await navigate(target.destination);
          if (signal.aborted) return;
          await waitForVisibleElement(readyTargets[target.destination], signal);
        }
        if (target.element) await waitForVisibleElement(target.element, signal);
        if (!signal.aborted) driver.drive(index);
      } catch {
        if (!signal.aborted) onError();
      } finally {
        pending = false;
      }
    },
  };
}
