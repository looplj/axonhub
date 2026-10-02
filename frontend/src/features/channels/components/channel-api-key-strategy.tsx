import { useTranslation } from 'react-i18next';
import { FormLabel } from '@/components/ui/form';
import { Input } from '@/components/ui/input';
import { SelectDropdown } from '@/components/select-dropdown';

export type APIKeyStrategyValue = 'sticky' | 'random' | 'round_robin' | 'round_robin_success' | 'priority' | 'fixed';

// GraphQL `Int` is a signed 32-bit value, so the switch-after count must stay
// within this range or the save would be rejected by the server.
const GRAPHQL_INT_MAX = 2147483647;

interface ApiKeyStrategyFieldsProps {
  value: APIKeyStrategyValue;
  switchAfter: number;
  onChange: (value: APIKeyStrategyValue) => void;
  onSwitchAfterChange: (value: number) => void;
}

/**
 * Multi-key selection policy editor. Extracted from
 * channels-action-dialog.tsx so the dialog stays close to its
 * own baseline and only carries a single mount point here.
 */
export function ApiKeyStrategyFields({ value, switchAfter, onChange, onSwitchAfterChange }: ApiKeyStrategyFieldsProps) {
  const { t } = useTranslation();

  return (
    <div className='grid grid-cols-1 items-start gap-x-6 gap-y-2 md:grid-cols-8'>
      <FormLabel className='pt-2 font-medium md:col-span-2 md:text-right'>{t('channels.dialogs.fields.apiKeyStrategy.label')}</FormLabel>
      <div className='space-y-1 md:col-span-6'>
        <SelectDropdown
          defaultValue={value}
          onValueChange={(next) => onChange(next as APIKeyStrategyValue)}
          data-testid='channel-api-key-strategy-select'
          isControlled={true}
          items={[
            { value: 'sticky', label: t('channels.dialogs.fields.apiKeyStrategy.options.sticky') },
            { value: 'random', label: t('channels.dialogs.fields.apiKeyStrategy.options.random') },
            { value: 'round_robin', label: t('channels.dialogs.fields.apiKeyStrategy.options.roundRobin') },
            {
              value: 'round_robin_success',
              label: t('channels.dialogs.fields.apiKeyStrategy.options.roundRobinSuccess'),
            },
            { value: 'priority', label: t('channels.dialogs.fields.apiKeyStrategy.options.priority') },
            { value: 'fixed', label: t('channels.dialogs.fields.apiKeyStrategy.options.fixed') },
          ]}
        />
        <p className='text-muted-foreground text-xs'>{t('channels.dialogs.fields.apiKeyStrategy.tooltip')}</p>
        <div className='flex items-center gap-2'>
          <Input
            type='number'
            min={1}
            max={GRAPHQL_INT_MAX}
            value={switchAfter}
            aria-label={t('channels.dialogs.fields.apiKeyRoundRobinSwitchAfter.label')}
            disabled={value !== 'round_robin' && value !== 'round_robin_success'}
            onChange={(e) => {
              const n = parseInt(e.target.value, 10);
              onSwitchAfterChange(Number.isNaN(n) || n < 1 ? 1 : Math.min(n, GRAPHQL_INT_MAX));
            }}
            className='w-24'
            data-testid='channel-api-key-round-robin-input'
          />
          <span className='text-muted-foreground text-xs'>{t('channels.dialogs.fields.apiKeyRoundRobinSwitchAfter.label')}</span>
        </div>
      </div>
    </div>
  );
}
