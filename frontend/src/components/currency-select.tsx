import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { availableCurrencyCodes, type CurrencyAmount } from '@/utils/currency';

export function CurrencySelect({
  codes,
  value,
  onChange,
  className,
}: {
  codes: string[];
  value: string;
  onChange: (value: string) => void;
  className?: string;
}) {
  const { t } = useTranslation();

  if (codes.length <= 1) return null;

  return (
    <Select value={value} onValueChange={onChange}>
      <SelectTrigger className={className ?? 'h-8 w-[180px]'} size='sm'>
        <SelectValue />
      </SelectTrigger>
      <SelectContent>
        {codes.map((code) => (
          <SelectItem key={code} value={code}>
            {t(`currencies.${code}`)}
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  );
}

// Tracks a currency that always exists in the current data set, preferring the
// configured system currency when it is present.
export function useSelectedCurrency(amountLists: Array<CurrencyAmount[] | null | undefined>, preferred: string) {
  const codes = availableCurrencyCodes(amountLists);
  const [currencyCode, setCurrencyCode] = useState(preferred);

  useEffect(() => {
    if (codes.length === 0) return;
    if (!codes.includes(currencyCode)) {
      setCurrencyCode(codes.includes(preferred) ? preferred : codes[0]);
    }
  }, [codes, currencyCode, preferred]);

  return { codes, currencyCode, setCurrencyCode };
}
