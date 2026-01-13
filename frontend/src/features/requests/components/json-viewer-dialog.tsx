import { useState } from 'react';
import { Copy, Check, Download } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { Button } from '@/components/ui/button';
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { ScrollArea } from '@/components/ui/scroll-area';

interface JsonViewerDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  title: string;
  jsonData: any;
}

export function JsonViewerDialog({ open, onOpenChange, title, jsonData }: JsonViewerDialogProps) {
  const { t } = useTranslation();
  const [copied, setCopied] = useState(false);

  const formatJson = (data: any) => {
    try {
      if (typeof data === 'string') {
        return JSON.stringify(JSON.parse(data), null, 2);
      }
      return JSON.stringify(data, null, 2);
    } catch {
      return typeof data === 'string' ? data : JSON.stringify(data);
    }
  };

  const copyToClipboard = (text: string) => {
    navigator.clipboard.writeText(text);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  const downloadFile = (content: string, filename: string) => {
    const blob = new Blob([content], { type: 'application/json' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = filename;
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
    URL.revokeObjectURL(url);
  };

  const formattedJson = formatJson(jsonData);

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className='max-h-[90vh] w-[90vw] max-w-[90vw]'>
        <DialogHeader className='pb-4'>
          <DialogTitle className='start flex items-center'>
            {title}
            <Button className={'ml-4'} variant='outline' size='sm' onClick={() => copyToClipboard(formattedJson)}>
              {copied ? <Check className='mr-2 h-4 w-4' /> : <Copy className='mr-2 h-4 w-4' />}
              {copied ? t('requests.dialogs.jsonViewer.copied') : t('requests.dialogs.jsonViewer.copy')}
            </Button>
            <Button className={'ml-2'} variant='outline' size='sm' onClick={() => downloadFile(formattedJson, `${title.replace(/\s+/g, '-').toLowerCase()}-${Date.now()}.json`)}>
              <Download className='mr-2 h-4 w-4' />
              {t('requests.dialogs.jsonViewer.download')}
            </Button>
          </DialogTitle>
        </DialogHeader>
        <ScrollArea className='h-[72vh] w-full rounded-xs border p-6'>
          <pre className='font-mono text-xs whitespace-pre-wrap'>{formattedJson}</pre>
        </ScrollArea>
      </DialogContent>
    </Dialog>
  );
}
