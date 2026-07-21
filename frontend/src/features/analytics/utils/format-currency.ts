const CURRENCY_SYMBOLS: Record<string, string> = {
  USD: '$',
  EUR: '€',
  GBP: '£',
  JPY: '¥',
  CNY: '¥',
};

export function formatCurrencySimple(val: number, currencyCode: string): string {
  const symbol = CURRENCY_SYMBOLS[currencyCode] || currencyCode + ' ';
  return `${symbol}${val.toFixed(4)}`;
}

export function formatCurrencyTick(value: number | string, currencyCode: string): string {
  const symbol = CURRENCY_SYMBOLS[currencyCode] || currencyCode + ' ';
  return `${symbol}${Number(value).toFixed(0)}`;
}
