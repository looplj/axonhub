import { useState } from 'react';
import { Loader2 } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { useAuthStore } from '@/stores/authStore';
import { usePermissions } from '@/hooks/usePermissions';
import { Alert, AlertDescription } from '@/components/ui/alert';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Switch } from '@/components/ui/switch';
import { Textarea } from '@/components/ui/textarea';
import { parseFallbackModels, safeguardsValidation, type ClaudeCodeSafeguardsConfig } from '../data/claude-code-safeguards';
import { useClaudeCodeSafeguardsSettings, useSaveClaudeCodeSafeguardsSettings } from '../data/claude-code-safeguards-api';

export function ClaudeCodeSettings() {
  const userID = useAuthStore((state) => state.auth.user?.id ?? 'signed-out');
  return <ClaudeCodeSettingsForm key={userID} />;
}

function ClaudeCodeSettingsForm() {
  const { t } = useTranslation();
  const { hasSystemScope } = usePermissions();
  const query = useClaudeCodeSafeguardsSettings();
  const save = useSaveClaudeCodeSafeguardsSettings();
  const [draft, setDraft] = useState<Partial<ClaudeCodeSafeguardsConfig>>({});
  const [fallbackText, setFallbackText] = useState<string | null>(null);
  const canWrite = hasSystemScope('write_settings');
  if (!hasSystemScope('read_settings')) {
    return (
      <Alert>
        <AlertDescription>{t('common.routeGuard.noPermission')}</AlertDescription>
      </Alert>
    );
  }
  if (query.isPending) {
    return (
      <div className='flex items-center gap-2 p-6'>
        <Loader2 className='h-5 w-5 animate-spin' />
        {t('common.loading')}
      </div>
    );
  }
  if (query.isError || !query.data) {
    return (
      <Alert variant='destructive'>
        <AlertDescription>
          {t('system.claudeCode.loadFailed')}
          <Button variant='outline' onClick={() => void query.refetch()}>
            {t('system.claudeCode.retry')}
          </Button>
        </AlertDescription>
      </Alert>
    );
  }
  const saved = query.data;
  const config = { ...saved.config, ...draft };
  const fallbacks = fallbackText ?? config.fallbackModels.join('\n');
  const input = { ...config, classifierModel: config.classifierModel.trim(), fallbackModels: parseFallbackModels(fallbacks) };
  const issue = safeguardsValidation(input);
  const disabled = !canWrite || save.isPending;
  const dirty = Object.keys(draft).length > 0 || fallbackText !== null;
  const clearDraft = () => {
    setDraft({});
    setFallbackText(null);
  };
  const limits = [
    ['attemptTimeoutSeconds', 0, 120, 'any'],
    ['timeoutSeconds', 0.000000001, 120, 'any'],
    ['maxTokens', 1, 16384, 1],
    ['transcriptCharBudget', 1, 1000000, 1],
  ] as const;

  return (
    <Card data-testid='claude-code-safeguards'>
      <CardHeader>
        <CardTitle>{t('system.claudeCode.title')}</CardTitle>
        <CardDescription>{t('system.claudeCode.description')}</CardDescription>
      </CardHeader>
      <CardContent>
        <form
          className='space-y-6'
          onSubmit={(event) => {
            event.preventDefault();
            if (!disabled && !issue) save.mutate(input, { onSuccess: clearDraft });
          }}
        >
          <p className='text-muted-foreground text-sm'>
            {t(saved.usingServerDefaults ? 'system.claudeCode.serverDefaults' : 'system.claudeCode.savedOverride')}
          </p>
          {!canWrite && (
            <Alert>
              <AlertDescription>{t('system.claudeCode.readOnly')}</AlertDescription>
            </Alert>
          )}
          <fieldset disabled={disabled} className='space-y-6'>
            <div className='space-y-2'>
              <Label htmlFor='claude-reviewer'>{t('system.claudeCode.classifierModel')}</Label>
              <Input
                data-testid='claude-reviewer'
                id='claude-reviewer'
                value={config.classifierModel}
                onChange={(event) => setDraft({ ...draft, classifierModel: event.target.value })}
              />
              <p className='text-muted-foreground text-sm'>{t('system.claudeCode.modelHelp')}</p>
            </div>
            <div className='space-y-2'>
              <Label htmlFor='claude-fallbacks'>{t('system.claudeCode.fallbackModels')}</Label>
              <Textarea
                data-testid='claude-fallbacks'
                id='claude-fallbacks'
                value={fallbacks}
                onChange={(event) => setFallbackText(event.target.value)}
                rows={3}
              />
              <p className='text-muted-foreground text-sm'>{t('system.claudeCode.fallbackHelp')}</p>
            </div>
            <div className='space-y-2'>
              <Label htmlFor='claude-policy'>{t('system.claudeCode.policy')}</Label>
              <Textarea
                data-testid='claude-policy'
                id='claude-policy'
                value={config.policy}
                onChange={(event) => setDraft({ ...draft, policy: event.target.value })}
                rows={12}
              />
              <p className='text-muted-foreground text-sm'>{t('system.claudeCode.policyHelp')}</p>
              <div className='flex flex-wrap gap-2'>
                <Button type='button' variant='outline' onClick={() => setDraft({ ...draft, policy: saved.defaultPolicy })}>
                  {t('system.claudeCode.copyDefault')}
                </Button>
                <Button type='button' variant='outline' onClick={() => setDraft({ ...draft, policy: '' })}>
                  {t('system.claudeCode.useDefault')}
                </Button>
              </div>
              <details className='text-sm'>
                <summary className='cursor-pointer'>{t('system.claudeCode.viewDefault')}</summary>
                <p className='text-muted-foreground mt-2 whitespace-pre-wrap'>{saved.defaultPolicy}</p>
              </details>
            </div>
            <details>
              <summary className='cursor-pointer font-medium'>{t('system.claudeCode.advanced')}</summary>
              <div className='mt-4 space-y-4'>
                <div className='flex items-center justify-between gap-4'>
                  <div>
                    <Label htmlFor='claude-escalate'>{t('system.claudeCode.escalateOnDenial')}</Label>
                    <p className='text-muted-foreground text-sm'>{t('system.claudeCode.escalateHelp')}</p>
                  </div>
                  <Switch
                    id='claude-escalate'
                    checked={config.escalateOnDenial}
                    onCheckedChange={(checked) => setDraft({ ...draft, escalateOnDenial: checked })}
                  />
                </div>
                <div className='grid gap-4 sm:grid-cols-2'>
                  {limits.map(([field, min, max, step]) => (
                    <div key={field} className='space-y-2'>
                      <Label htmlFor={`claude-${field}`}>{t(`system.claudeCode.${field}`)}</Label>
                      <Input
                        id={`claude-${field}`}
                        type='number'
                        required
                        min={min}
                        max={max}
                        step={step}
                        value={Number.isNaN(config[field]) ? '' : config[field]}
                        onChange={(event) => setDraft({ ...draft, [field]: event.target.valueAsNumber })}
                      />
                    </div>
                  ))}
                </div>
                <p className='text-muted-foreground text-sm'>{t('system.claudeCode.limitsHelp')}</p>
              </div>
            </details>
          </fieldset>
          {issue && (
            <p role='alert' className='text-destructive text-sm'>
              {t(`system.claudeCode.validation.${issue}`)}
            </p>
          )}
          <div className='flex flex-wrap justify-end gap-2'>
            <Button
              type='button'
              variant='outline'
              disabled={disabled || saved.usingServerDefaults}
              data-testid='claude-reset'
              onClick={() => save.mutate(null, { onSuccess: clearDraft })}
            >
              {t('system.claudeCode.reset')}
            </Button>
            <Button type='button' variant='outline' disabled={disabled || !dirty} onClick={clearDraft}>
              {t('system.claudeCode.discard')}
            </Button>
            <Button data-testid='claude-save' type='submit' disabled={disabled || !!issue || !dirty}>
              {save.isPending && <Loader2 className='mr-2 h-4 w-4 animate-spin' />}
              {t(save.isPending ? 'system.buttons.saving' : 'system.buttons.save')}
            </Button>
          </div>
        </form>
      </CardContent>
    </Card>
  );
}
