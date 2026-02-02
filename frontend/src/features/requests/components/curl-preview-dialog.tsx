import { useState, useCallback } from 'react';
import { Terminal, Copy, Check, CopyX } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { Button } from '@/components/ui/button';
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip';

interface CurlPreviewDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  curlCommand: string;
  title?: string;
}

export function CurlPreviewDialog({ open, onOpenChange, curlCommand, title }: CurlPreviewDialogProps) {
  const { t } = useTranslation();
  const [copied, setCopied] = useState(false);
  const [nonStreamCopied, setNonStreamCopied] = useState(false);

  const handleCopy = useCallback(async () => {
    try {
      await navigator.clipboard.writeText(curlCommand);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch (err) {
      console.error('Failed to copy:', err);
    }
  }, [curlCommand]);

  const handleCopyNonStream = useCallback(async () => {
    try {
      // 移除 body 中的 stream 字段
      // 1. 移除 "stream": true/false 及其前后的逗号和空格
      let modified = curlCommand.replace(/"stream":\s*(true|false),?\s*/g, '');
      // 2. 清理对象末尾多余的逗号: {"a": 1, } -> {"a": 1}
      modified = modified.replace(/,\s*}/g, '}');
      // 3. 清理对象开头多余的逗号: { , "a": 1} -> {"a": 1}
      modified = modified.replace(/{\s*,/g, '{');
      
      await navigator.clipboard.writeText(modified);
      setNonStreamCopied(true);
      setTimeout(() => setNonStreamCopied(false), 2000);
    } catch (err) {
      console.error('Failed to copy non-stream curl:', err);
    }
  }, [curlCommand]);

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className='flex max-h-[80vh] flex-col sm:max-w-3xl'>
        <DialogHeader>
          <DialogTitle className='flex items-center gap-2'>
            <Terminal className='h-5 w-5' />
            {title || t('requests.dialogs.curlPreview.title')}
            <TooltipProvider>
              <Tooltip>
                <TooltipTrigger asChild>
                  <Button
                    variant='ghost'
                    size='icon'
                    className='h-8 w-8'
                    onClick={handleCopy}
                  >
                    {copied ? <Check className='h-4 w-4 text-green-500' /> : <Copy className='h-4 w-4' />}
                  </Button>
                </TooltipTrigger>
                <TooltipContent>
                  <p>{t('requests.actions.copy')}</p>
                </TooltipContent>
              </Tooltip>

              <Tooltip>
                <TooltipTrigger asChild>
                  <Button
                    variant='ghost'
                    size='icon'
                    className='h-8 w-8'
                    onClick={handleCopyNonStream}
                  >
                    {nonStreamCopied ? (
                      <Check className='h-4 w-4 text-green-500' />
                    ) : (
                      <CopyX className='h-4 w-4' />
                    )}
                  </Button>
                </TooltipTrigger>
                <TooltipContent>
                  <p>{t('requests.dialogs.curlPreview.copyNonStream')}</p>
                </TooltipContent>
              </Tooltip>
            </TooltipProvider>
          </DialogTitle>
        </DialogHeader>

        <div className='bg-muted/30 flex-1 overflow-auto rounded-lg border p-4'>
          <pre className='whitespace-pre-wrap break-all font-mono text-sm'>{curlCommand}</pre>
        </div>
      </DialogContent>
    </Dialog>
  );
}
