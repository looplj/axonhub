import { useCallback, useEffect, useState } from 'react';
import { Link } from '@tanstack/react-router';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';
import { Button } from '@/components/ui/button';
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from '@/components/ui/collapsible';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Label } from '@/components/ui/label';
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group';
import { useRetryPolicy } from '@/features/system/data/system';
import {
  type ApiKeyAutoDisableRuleFormValue,
  availabilityPoliciesPayload,
  effectiveAutoDisableMode,
  serializeApiKeyAutoDisableRules,
  toApiKeyAutoDisableRuleFormValues,
} from '../data/auto-disable';
import { useUpdateChannel } from '../data/channels';
import { APIKeyAutoDisableMode, Channel } from '../data/schema';
import { ApiKeyAutoDisableRulesEditor, ApiKeyAutoDisableRulesPreview } from './api-key-auto-disable-rules-editor';

interface Props {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  currentRow: Channel;
}

export function ChannelsAvailabilityDialog({ open, onOpenChange, currentRow }: Props) {
  const { t } = useTranslation();
  const updateChannel = useUpdateChannel();
  const { data: retryPolicy } = useRetryPolicy();
  const currentPolicies = currentRow.policies ?? null;
  const [mode, setMode] = useState<APIKeyAutoDisableMode>(() => effectiveAutoDisableMode(currentRow.policies));
  const [rules, setRules] = useState<ApiKeyAutoDisableRuleFormValue[]>(() =>
    toApiKeyAutoDisableRuleFormValues(currentPolicies?.apiKeyAutoDisableRules)
  );

  useEffect(() => {
    if (open) {
      setMode(effectiveAutoDisableMode(currentRow.policies));
      setRules(toApiKeyAutoDisableRuleFormValues(currentRow.policies?.apiKeyAutoDisableRules));
    }
  }, [currentRow, open]);

  const handleRulesChange = useCallback((next: ApiKeyAutoDisableRuleFormValue[]) => {
    setRules((prev) => (JSON.stringify(prev) === JSON.stringify(next) ? prev : next));
  }, []);

  const globalRules = retryPolicy?.autoDisableChannel?.rules ?? [];
  const globalEnabled = retryPolicy?.autoDisableChannel?.enabled ?? false;

  const onSubmit = useCallback(async () => {
    const payload = availabilityPoliciesPayload({
      stream: currentPolicies?.stream,
      mode,
      rules,
    });
    if (payload.emptiedCustom) {
      toast.error(t('channels.dialogs.availability.custom.emptyPrompt'));
    }

    try {
      await updateChannel.mutateAsync({
        id: currentRow.id,
        input: {
          policies: {
            stream: payload.stream,
            apiKeyAutoDisableMode: payload.apiKeyAutoDisableMode,
            apiKeyAutoDisableRules: payload.apiKeyAutoDisableRules,
          },
        },
      });
      toast.success(t('channels.messages.updateSuccess'));
      onOpenChange(false);
    } catch {
      // useUpdateChannel reports the request error.
    }
  }, [currentPolicies?.stream, currentRow.id, mode, onOpenChange, rules, t, updateChannel]);

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className='max-h-[85vh] overflow-y-auto sm:max-w-3xl'>
        <DialogHeader className='text-left'>
          <DialogTitle>{t('channels.dialogs.availability.title')}</DialogTitle>
          <DialogDescription>{t('channels.dialogs.availability.description', { name: currentRow.name })}</DialogDescription>
        </DialogHeader>

        <div className='bg-muted/50 text-muted-foreground rounded-md border p-3 text-sm'>{t('channels.dialogs.availability.scope')}</div>

        <RadioGroup value={mode} onValueChange={(value) => setMode(value as APIKeyAutoDisableMode)} className='gap-2'>
          {(['inherit', 'custom', 'off'] as const).map((value) => (
            <div key={value} className='flex items-start gap-2 rounded-md border p-3'>
              <RadioGroupItem value={value} id={`auto-disable-mode-${value}`} className='mt-1' />
              <Label htmlFor={`auto-disable-mode-${value}`} className='flex flex-1 cursor-pointer flex-col gap-0.5 font-normal'>
                <span className='font-medium'>{t(`channels.dialogs.availability.modes.${value}`)}</span>
                <span className='text-muted-foreground text-sm'>{t(`channels.dialogs.availability.modes.${value}Description`)}</span>
              </Label>
            </div>
          ))}
        </RadioGroup>

        {mode === 'inherit' && (
          <div className='space-y-3'>
            {!globalEnabled && <p className='text-muted-foreground text-sm'>{t('channels.dialogs.availability.inherit.globalDisabled')}</p>}
            <ApiKeyAutoDisableRulesPreview rules={globalRules} emptyLabel={t('channels.dialogs.availability.inherit.emptyGlobal')} />
            <Button variant='link' className='h-auto px-0' asChild>
              <Link to='/system' search={{ tab: 'retry' }}>
                {t('channels.dialogs.availability.inherit.goToSettings')}
              </Link>
            </Button>
          </div>
        )}

        {mode === 'custom' && (
          <div className='space-y-4'>
            <ApiKeyAutoDisableRulesEditor rules={rules} onChange={handleRulesChange} allowDelete />
            <Collapsible>
              <CollapsibleTrigger className='text-sm underline-offset-4 hover:underline'>
                {t('channels.dialogs.availability.custom.fallbackTitle')}
              </CollapsibleTrigger>
              <CollapsibleContent className='mt-3'>
                <ApiKeyAutoDisableRulesPreview rules={globalRules} emptyLabel={t('channels.dialogs.availability.inherit.emptyGlobal')} />
              </CollapsibleContent>
            </Collapsible>
          </div>
        )}

        {mode === 'off' && (
          <div className='space-y-3'>
            <p className='text-muted-foreground text-sm'>{t('channels.dialogs.availability.off.description')}</p>
            <ApiKeyAutoDisableRulesPreview
              rules={serializeApiKeyAutoDisableRules(rules)}
              emptyLabel={t('channels.dialogs.availability.off.emptySaved')}
              inactiveHint={t('channels.dialogs.availability.off.savedRulesHint')}
            />
          </div>
        )}

        <DialogFooter>
          <Button type='button' variant='outline' onClick={() => onOpenChange(false)}>
            {t('common.buttons.cancel')}
          </Button>
          <Button type='button' disabled={updateChannel.isPending} onClick={onSubmit}>
            {updateChannel.isPending ? t('common.buttons.saving') : t('common.buttons.save')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
