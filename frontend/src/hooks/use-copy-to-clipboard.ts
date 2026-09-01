import { useCallback, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';
import { copyTextToClipboard } from '@/lib/clipboard';

type UseCopyToClipboardProps = {
  text: string;
  copyMessage?: string;
};

export function useCopyToClipboard({ text, copyMessage }: UseCopyToClipboardProps) {
  const { t } = useTranslation();
  const [isCopied, setIsCopied] = useState(false);
  const timeoutRef = useRef<NodeJS.Timeout | null>(null);

  const handleCopy = useCallback(async () => {
    try {
      await copyTextToClipboard(text);
      toast.success(copyMessage ?? t('common.success.copiedToClipboard'));
      setIsCopied(true);
      if (timeoutRef.current) {
        clearTimeout(timeoutRef.current);
        timeoutRef.current = null;
      }
      timeoutRef.current = setTimeout(() => {
        setIsCopied(false);
      }, 2000);
    } catch {
      toast.error(t('common.errors.copyFailed'));
    }
  }, [text, copyMessage, t]);

  return { isCopied, handleCopy };
}
