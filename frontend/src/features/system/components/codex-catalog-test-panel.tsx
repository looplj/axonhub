'use client';

import { CheckCircle2, FlaskConical, Loader2, XCircle } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { Button } from '@/components/ui/button';
import type { CodexTestState } from '../data/codex-compatibility';

interface CodexCatalogTestPanelProps {
  state: CodexTestState;
  canTest: boolean;
  onTest: () => void;
  onCancel: () => void;
}

function TestOutcome({ state }: { state: CodexTestState }) {
  const { t } = useTranslation();

  switch (state.phase) {
    case 'idle':
      return null;
    case 'running':
      return (
        <span className='text-muted-foreground inline-flex items-center gap-2'>
          <Loader2 className='h-4 w-4 animate-spin' />
          {t('system.codex.test.running')}
        </span>
      );
    case 'succeeded':
      return (
        <span className='inline-flex items-center gap-2 text-emerald-600 dark:text-emerald-400' data-testid='codex-compat-test-success'>
          <CheckCircle2 className='h-4 w-4 shrink-0' />
          {t('system.codex.test.success', { count: state.modelCount })}
        </span>
      );
    case 'failed':
      return (
        <div className='text-destructive space-y-1' data-testid='codex-compat-test-failure'>
          <span className='inline-flex items-center gap-2 font-medium'>
            <XCircle className='h-4 w-4 shrink-0' />
            {t('system.codex.test.failure')}
          </span>
          {state.upstreamStatus !== null ? (
            <div>{t('system.codex.test.upstreamStatus', { status: state.upstreamStatus })}</div>
          ) : (
            state.message && (
              <div className='text-muted-foreground break-words'>{t('system.codex.test.detail', { message: state.message })}</div>
            )
          )}
        </div>
      );
  }
}

export function CodexCatalogTestPanel({ state, canTest, onTest, onCancel }: CodexCatalogTestPanelProps) {
  const { t } = useTranslation();

  return (
    <div className='space-y-3 rounded-lg border p-4'>
      <div className='flex flex-wrap items-center gap-2'>
        <Button type='button' variant='outline' onClick={onTest} disabled={!canTest} data-testid='codex-compat-test-button'>
          <FlaskConical className='h-4 w-4' />
          {t('system.codex.test.button')}
        </Button>
        {state.phase === 'running' && (
          <Button type='button' variant='ghost' onClick={onCancel} data-testid='codex-compat-test-cancel'>
            {t('common.buttons.cancel')}
          </Button>
        )}
      </div>
      <div role='status' aria-live='polite' className='text-sm' data-testid='codex-compat-test-result'>
        <TestOutcome state={state} />
      </div>
      <p className='text-muted-foreground text-xs'>{t('system.codex.test.hint')}</p>
    </div>
  );
}
