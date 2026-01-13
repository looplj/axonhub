'use client';

import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { IconChevronDown, IconChevronUp } from '@tabler/icons-react';

interface ScopesCellProps {
  scopes: string[];
}

export function ScopesCell({ scopes }: ScopesCellProps) {
  const { t } = useTranslation();
  const [isExpanded, setIsExpanded] = useState(false);

  if (scopes.length === 0) {
    return <div className='text-muted-foreground text-xs'>-</div>;
  }

  const displayScopes = isExpanded ? scopes : scopes.slice(0, 3);
  const showExpandButton = scopes.length > 3;

  return (
    <div className='flex max-w-[300px] flex-col gap-1'>
      <div className='flex flex-wrap gap-1'>
        {displayScopes.map((scope) => (
          <Badge key={scope} variant='secondary' className='text-xs'>
            {scope}
          </Badge>
        ))}
      </div>
      {showExpandButton && (
        <Button
          variant='ghost'
          size='sm'
          onClick={() => setIsExpanded(!isExpanded)}
          className='h-6 px-2 text-xs text-muted-foreground hover:text-foreground'
        >
          {isExpanded ? (
            <>
              <IconChevronUp className='mr-1 h-3 w-3' />
              {t('roles.columns.showLess')}
            </>
          ) : (
            <>
              <IconChevronDown className='mr-1 h-3 w-3' />
              +{scopes.length - 3} {t('roles.columns.moreScopes')}
            </>
          )}
        </Button>
      )}
    </div>
  );
}
