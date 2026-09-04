export function shouldNotifyChannelQueryError(error: unknown, hasData: boolean, isPlaceholderData: boolean): boolean {
  return Boolean(error && (!hasData || isPlaceholderData));
}
