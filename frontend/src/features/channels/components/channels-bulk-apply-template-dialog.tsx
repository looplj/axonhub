import { useState } from 'react';
import { Loader2 } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { Button } from '@/components/ui/button';
import { Command, CommandEmpty, CommandGroup, CommandInput, CommandItem, CommandList } from '@/components/ui/command';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Label } from '@/components/ui/label';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import { Channel } from '../data/schema';
import { useChannelOverrideTemplates, useApplyChannelOverrideTemplate } from '../data/templates';

interface Props {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  selectedChannels: Channel[];
}

export function ChannelsBulkApplyTemplateDialog({ open, onOpenChange, selectedChannels }: Props) {
  const { t } = useTranslation();
  const applyTemplate = useApplyChannelOverrideTemplate();
  const [selectedTemplateId, setSelectedTemplateId] = useState<string | null>(null);
  const [templateSearchOpen, setTemplateSearchOpen] = useState(false);
  const [templateSearchValue, setTemplateSearchValue] = useState('');

  const { data: templatesData, isLoading } = useChannelOverrideTemplates(
    {
      search: templateSearchValue,
      first: 50,
    },
    {
      enabled: open,
    }
  );

  const templates = templatesData?.edges?.map((edge) => edge.node) || [];

  const handleApply = async () => {
    if (!selectedTemplateId) return;

    try {
      await applyTemplate.mutateAsync({
        templateID: selectedTemplateId,
        channelIDs: selectedChannels.map((ch) => ch.id),
        mode: 'MERGE',
      });
      onOpenChange(false);
      setSelectedTemplateId(null);
    } catch (error) {
      // Error already handled by mutation
    }
  };

  const handleClose = () => {
    onOpenChange(false);
    setSelectedTemplateId(null);
    setTemplateSearchValue('');
  };

  return (
    <Dialog open={open} onOpenChange={handleClose}>
      <DialogContent className='sm:max-w-[600px]'>
        <DialogHeader>
          <DialogTitle>{t('channels.templates.bulk.title')}</DialogTitle>
          <DialogDescription>{t('channels.templates.bulk.description', { count: selectedChannels.length })}</DialogDescription>
        </DialogHeader>

        <div className='space-y-4 py-4'>
          {/* Template Selector */}
          <div className='space-y-2'>
            <Label>{t('channels.templates.selectTemplate')}</Label>
            <Popover open={templateSearchOpen} onOpenChange={setTemplateSearchOpen}>
              <PopoverTrigger asChild>
                <Button
                  variant='outline'
                  role='combobox'
                  aria-expanded={templateSearchOpen}
                  className='w-full justify-between'
                  disabled={isLoading}
                >
                  {isLoading ? (
                    <>
                      <Loader2 className='mr-2 h-4 w-4 animate-spin' />
                      {t('common.loading')}
                    </>
                  ) : selectedTemplateId ? (
                    templates.find((t) => t.id === selectedTemplateId)?.name
                  ) : (
                    t('channels.templates.selectTemplate')
                  )}
                </Button>
              </PopoverTrigger>
              <PopoverContent className='w-[550px] p-0'>
                <Command>
                  <CommandInput
                    placeholder={t('channels.templates.searchPlaceholder')}
                    value={templateSearchValue}
                    onValueChange={setTemplateSearchValue}
                  />
                  <CommandList>
                    <CommandEmpty>{t('channels.templates.noTemplates')}</CommandEmpty>
                    <CommandGroup>
                      {templates.map((template) => (
                        <CommandItem
                          key={template.id}
                          value={template.id}
                          onSelect={() => {
                            setSelectedTemplateId(template.id);
                            setTemplateSearchOpen(false);
                          }}
                        >
                          <div className='flex flex-col'>
                            <span className='font-medium'>{template.name}</span>
                            {template.description && <span className='text-muted-foreground text-xs'>{template.description}</span>}
                          </div>
                        </CommandItem>
                      ))}
                    </CommandGroup>
                  </CommandList>
                </Command>
              </PopoverContent>
            </Popover>
          </div>

          {/* Info Message */}
          {selectedTemplateId && (
            <div className='bg-muted/50 rounded-md border p-3'>
              <p className='text-muted-foreground text-sm'>
                {t('channels.templates.bulk.applyInfo')}
              </p>
            </div>
          )}
        </div>

        <DialogFooter>
          <Button variant='outline' onClick={handleClose} disabled={applyTemplate.isPending}>
            {t('common.buttons.cancel')}
          </Button>
          <Button onClick={handleApply} disabled={!selectedTemplateId || applyTemplate.isPending}>
            {applyTemplate.isPending ? (
              <>
                <Loader2 className='mr-2 h-4 w-4 animate-spin' />
                {t('channels.templates.bulk.applying')}
              </>
            ) : (
              t('channels.templates.bulk.apply', { count: selectedChannels.length })
            )}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
