import { useState } from 'react';
import { useAllScopes } from '@/gql/scopes';
import { Check, ChevronsUpDown } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { cn } from '@/lib/utils';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Command, CommandEmpty, CommandGroup, CommandInput, CommandItem } from '@/components/ui/command';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';

interface ScopesSelectProps {
  value: string[];
  onChange: (value: string[]) => void;
  portalContainer?: HTMLElement | null;
}

export function ScopesSelect({ value, onChange, portalContainer }: ScopesSelectProps) {
  const { t } = useTranslation();
  const [open, setOpen] = useState(false);
  const { data: allScopes } = useAllScopes('project');

  const handleSelect = (scopeValue: string) => {
    const newValue = value.includes(scopeValue) ? value.filter((v) => v !== scopeValue) : [...value, scopeValue];
    onChange(newValue);
  };

  const handleRemove = (scopeValue: string) => {
    onChange(value.filter((v) => v !== scopeValue));
  };

  return (
    <div className='space-y-2'>
      <Popover open={open} onOpenChange={setOpen}>
        <PopoverTrigger asChild>
          <Button variant='outline' role='combobox' aria-expanded={open} className='w-full justify-between'>
            {value.length > 0
              ? t('apikeys.dialogs.fields.scopes.selectedCount', { count: value.length })
              : t('apikeys.dialogs.fields.scopes.selectPlaceholder')}
            <ChevronsUpDown className='ml-2 h-4 w-4 shrink-0 opacity-50' />
          </Button>
        </PopoverTrigger>
        <PopoverContent className='w-full p-0' align='start' container={portalContainer}>
          <Command>
            <CommandInput placeholder={t('apikeys.dialogs.fields.scopes.searchPlaceholder')} />
            <CommandEmpty>{t('apikeys.dialogs.fields.scopes.noResults')}</CommandEmpty>
            <CommandGroup className='max-h-64 overflow-auto'>
              {allScopes?.map((scope) => (
                <CommandItem key={scope.scope} value={scope.scope} onSelect={() => handleSelect(scope.scope)}>
                  <Check className={cn('mr-2 h-4 w-4', value.includes(scope.scope) ? 'opacity-100' : 'opacity-0')} />
                  <div className='flex flex-col'>
                    <span>{scope.scope}</span>
                    <span className='text-muted-foreground text-xs'>{scope.description}</span>
                  </div>
                </CommandItem>
              ))}
            </CommandGroup>
          </Command>
        </PopoverContent>
      </Popover>

      {value.length > 0 && (
        <div className='flex flex-wrap gap-2'>
          {value.map((scopeValue) => {
            const scopeInfo = allScopes?.find((s) => s.scope === scopeValue);
            return (
              <Badge key={scopeValue} variant='secondary' className='cursor-pointer' onClick={() => handleRemove(scopeValue)}>
                {scopeInfo?.scope || scopeValue}
                <span className='ml-1 text-xs'>×</span>
              </Badge>
            );
          })}
        </div>
      )}
    </div>
  );
}
