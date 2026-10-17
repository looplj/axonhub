import type { TFunction } from 'i18next';

export type CurrencyAmount = {
  currencyCode: string;
  amount: number;
};

export function formatCurrencyAmount(
  amount: number,
  currencyCode: string | null | undefined,
  t: TFunction,
  locale: string,
  minimumFractionDigits = 4
): string {
  return t('currencies.format', {
    val: amount,
    currency: currencyCode || 'USD',
    locale,
    minimumFractionDigits,
  });
}

export function formatCurrencyAmounts(
  amounts: CurrencyAmount[] | null | undefined,
  t: TFunction,
  locale: string,
  options?: { minimumFractionDigits?: number; separator?: string }
): string {
  if (!amounts || amounts.length === 0) return '-';

  return amounts
    .map((item) => formatCurrencyAmount(item.amount, item.currencyCode, t, locale, options?.minimumFractionDigits ?? 4))
    .join(options?.separator ?? ' + ');
}

export function currencyAmount(amounts: CurrencyAmount[] | null | undefined, currencyCode: string): number {
  return amounts?.find((item) => item.currencyCode === currencyCode)?.amount ?? 0;
}

export function availableCurrencyCodes(amountLists: Array<CurrencyAmount[] | null | undefined>): string[] {
  const codes = new Set<string>();
  for (const list of amountLists) {
    for (const item of list ?? []) {
      codes.add(item.currencyCode);
    }
  }

  return [...codes].sort();
}

export function localeForLanguage(language: string): string {
  return language === 'zh' ? 'zh-CN' : 'en-US';
}
