import { useEffect, useMemo, useState } from 'react';
import { Loader2 } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';
import { Button } from '@/components/ui/button';
import { Command, CommandEmpty, CommandGroup, CommandInput, CommandItem, CommandList } from '@/components/ui/command';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Label } from '@/components/ui/label';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group';
import { useChannels } from '../context/channels-context';
import {
  type ApiKeyAutoDisableRuleFormValue,
  type BulkAutoDisableAction,
  buildBulkAutoDisableInput,
  effectiveAutoDisableMode,
  toApiKeyAutoDisableRuleFormValues,
} from '../data/auto-disable';
import { useBulkUpdateChannelAutoDisable, useChannelAutoDisableCopySources } from '../data/channels';
import { Channel } from '../data/schema';
import { ApiKeyAutoDisableRulesEditor } from './api-key-auto-disable-rules-editor';

interface Props {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  selectedChannels: Channel[];
}

export function ChannelsBulkAutoDisableDialog({ open, onOpenChange, selectedChannels }: Props) {
  const { t } = useTranslation();
  const { resetRowSelection } = useChannels();
  const mutation = useBulkUpdateChannelAutoDisable();
  const [action, setAction] = useState<BulkAutoDisableAction>('write_rules');
  const [rules, setRules] = useState<ApiKeyAutoDisableRuleFormValue[]>([]);
  const [copySearchOpen, setCopySearchOpen] = useState(false);
  const [copySearchValue, setCopySearchValue] = useState('');

  const { data: copySources = [], isLoading: copySourcesLoading } = useChannelAutoDisableCopySources({
    enabled: open,
  });

  useEffect(() => {
    if (open) {
      setAction('write_rules');
      setRules([]);
      setCopySearchValue('');
      setCopySearchOpen(false);
    }
  }, [open]);

  const overwriteTargets = useMemo(() => {
    return selectedChannels.filter((channel) => {
      const source = copySources.find((item) => item.id === channel.id);
      return effectiveAutoDisableMode(source?.policies ?? channel.policies) === 'custom';
    });
  }, [copySources, selectedChannels]);

  const handleCopyFrom = (channelId: string) => {
    const source = copySources.find((item) => item.id === channelId);
    const copied = toApiKeyAutoDisableRuleFormValues(source?.policies?.apiKeyAutoDisableRules);
    if (copied.length === 0) {
      toast.error(t('channels.bulkAutoDisable.copyEmpty'));
      return;
    }

    setRules(copied);
    setCopySearchOpen(false);
  };

  const handleClose = () => {
    onOpenChange(false);
    setAction('write_rules');
    setRules([]);
    setCopySearchValue('');
  };

  const handleSubmit = async () => {
    const built = buildBulkAutoDisableInput({
      channelIDs: selectedChannels.map((channel) => channel.id),
      action,
      rules,
    });
    if (!built.ok) {
      toast.error(t('channels.bulkAutoDisable.emptyRules'));
      return;
    }

    try {
      await mutation.mutateAsync(built.input);
      resetRowSelection();
      handleClose();
    } catch {
      // useBulkUpdateChannelAutoDisable reports the request error.
    }
  };

  return (
    <Dialog open={open} onOpenChange={(isOpen) => (isOpen ? onOpenChange(true) : handleClose())}>
      <DialogContent className='max-h-[90vh] overflow-y-auto sm:max-w-[720px]'>
        <DialogHeader>
          <DialogTitle>{t('channels.bulkAutoDisable.title')}</DialogTitle>
          <DialogDescription>{t('channels.bulkAutoDisable.description', { count: selectedChannels.length })}</DialogDescription>
        </DialogHeader>

        <div className='space-y-4 py-2'>
          <RadioGroup value={action} onValueChange={(value) => setAction(value as BulkAutoDisableAction)} className='space-y-3'>
            {(['write_rules', 'inherit', 'off'] as const).map((value) => (
              <div key={value} className='flex items-start space-x-2'>
                <RadioGroupItem value={value} id={`bulk-auto-disable-${value}`} className='mt-1' />
                <div className='space-y-0.5'>
                  <Label htmlFor={`bulk-auto-disable-${value}`} className='cursor-pointer'>
                    {t(`channels.bulkAutoDisable.actions.${value === 'write_rules' ? 'writeRules' : value}`)}
                  </Label>
                  <p className='text-muted-foreground text-sm'>
                    {t(`channels.bulkAutoDisable.actions.${value === 'write_rules' ? 'writeRulesDescription' : `${value}Description`}`)}
                  </p>
                </div>
              </div>
            ))}
          </RadioGroup>

          {action === 'write_rules' ? (
            <div className='space-y-3'>
              <div className='space-y-2'>
                <Label>{t('channels.bulkAutoDisable.copyFrom')}</Label>
                <Popover open={copySearchOpen} onOpenChange={setCopySearchOpen}>
                  <PopoverTrigger asChild>
                    <Button variant='outline' role='combobox' aria-expanded={copySearchOpen} className='w-full justify-between'>
                      {copySourcesLoading ? (
                        <>
                          <Loader2 className='mr-2 h-4 w-4 animate-spin' />
                          {t('common.loading')}
                        </>
                      ) : (
                        t('channels.bulkAutoDisable.copyFromPlaceholder')
                      )}
                    </Button>
                  </PopoverTrigger>
                  <PopoverContent className='w-[650px] p-0'>
                    <Command>
                      <CommandInput
                        placeholder={t('channels.bulkAutoDisable.copyFromPlaceholder')}
                        value={copySearchValue}
                        onValueChange={setCopySearchValue}
                      />
                      <CommandList>
                        <CommandEmpty>{t('common.noData')}</CommandEmpty>
                        <CommandGroup>
                          {copySources.map((channel) => (
                            <CommandItem
                              key={channel.id}
                              value={`${channel.name} ${channel.id}`}
                              onSelect={() => handleCopyFrom(channel.id)}
                            >
                              <span className='font-medium'>{channel.name}</span>
                            </CommandItem>
                          ))}
                        </CommandGroup>
                      </CommandList>
                    </Command>
                  </PopoverContent>
                </Popover>
              </div>

              <ApiKeyAutoDisableRulesEditor rules={rules} onChange={setRules} allowDelete={true} />

              {overwriteTargets.length > 0 && (
                <p className='text-sm text-amber-700 dark:text-amber-400'>
                  {t('channels.bulkAutoDisable.overwriteWarning', {
                    count: overwriteTargets.length,
                    names: overwriteTargets.map((channel) => channel.name).join(', '),
                  })}
                </p>
              )}
            </div>
          ) : (
            <div className='space-y-2'>
              <Label>{t('channels.bulkAutoDisable.affectedChannels')}</Label>
              <ul className='text-muted-foreground list-inside list-disc text-sm'>
                {selectedChannels.map((channel) => (
                  <li key={channel.id}>{channel.name}</li>
                ))}
              </ul>
            </div>
          )}
        </div>

        <DialogFooter>
          <Button variant='outline' onClick={handleClose} disabled={mutation.isPending}>
            {t('common.buttons.cancel')}
          </Button>
          <Button
            onClick={handleSubmit}
            disabled={selectedChannels.length === 0 || mutation.isPending || (action === 'write_rules' && copySourcesLoading)}
          >
            {mutation.isPending ? (
              <>
                <Loader2 className='mr-2 h-4 w-4 animate-spin' />
                {t('channels.bulkAutoDisable.applying')}
              </>
            ) : (
              t('channels.bulkAutoDisable.apply', { count: selectedChannels.length })
            )}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
