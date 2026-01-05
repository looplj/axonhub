import { useState } from 'react';
import { useAllScopes } from '@/gql/scopes';
import { Check, ChevronsUpDown } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { cn } from '@/lib/utils';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Command, CommandEmpty, CommandGroup, CommandInput, CommandItem } from '@/components/ui/command';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import { filterGrantableScopes } from '@/lib/permission-utils';
import { useAuthStore } from '@/stores/authStore';
import { useSelectedProjectId } from '@/stores/projectStore';

interface ScopesSelectProps {
  value: string[];
  onChange: (value: string[]) => void;
  portalContainer?: HTMLElement | null;
  level?: 'system' | 'project';
  enablePermissionFilter?: boolean;
}

export function ScopesSelect({ value, onChange, portalContainer, level = 'project', enablePermissionFilter = false }: ScopesSelectProps) {
  const { t } = useTranslation();
  const [open, setOpen] = useState(false);
  const currentUser = useAuthStore((state) => state.auth.user);
  const selectedProjectId = useSelectedProjectId();
  const { data: allScopes } = useAllScopes(level);

  let filteredScopes = allScopes || [];

  if (enablePermissionFilter && currentUser) {
    const allScopeValues = allScopes?.map((s) => s.scope) || [];
    const grantableScopes = filterGrantableScopes(currentUser, allScopeValues, selectedProjectId);
    filteredScopes = allScopes?.filter((s) => grantableScopes.includes(s.scope)) || [];
  }

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
              ? t('scopes.select.selectedCount', { count: value.length })
              : t('scopes.select.selectPlaceholder')}
            <ChevronsUpDown className='ml-2 h-4 w-4 shrink-0 opacity-50' />
          </Button>
        </PopoverTrigger>
        <PopoverContent className='w-full p-0' align='start' container={portalContainer}>
          <Command>
            <CommandInput placeholder={t('scopes.select.searchPlaceholder')} />
            <CommandEmpty>{t('scopes.select.noResults')}</CommandEmpty>
            <CommandGroup className='max-h-64 overflow-auto'>
              {filteredScopes.map((scope) => (
                <CommandItem key={scope.scope} value={scope.scope} onSelect={() => handleSelect(scope.scope)}>
                  <Check className={cn('mr-2 h-4 w-4', value.includes(scope.scope) ? 'opacity-100' : 'opacity-0')} />
                  <div className='flex flex-col'>
                    <span>{scope.scope}</span>
                    <span className='text-muted-foreground text-xs'>{t(`scopes.${scope.scope}`)}</span>
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
