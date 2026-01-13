import { useState } from 'react';
import { Copy, Download } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';
import { JsonViewer } from '@/components/json-tree-view';
import { Button } from '@/components/ui/button';

interface ChunkItemProps {
  chunk: any;
  index: number;
}

export function ChunkItem({ chunk, index }: ChunkItemProps) {
  const { t } = useTranslation();
  const [copied, setCopied] = useState(false);

  const formatJson = (data: any) => {
    if (!data) return '';
    try {
      return JSON.stringify(data, null, 2);
    } catch {
      return String(data);
    }
  };

  const copyToClipboard = () => {
    navigator.clipboard.writeText(formatJson(chunk));
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
    toast.success(t('requests.actions.copy'));
  };

  const downloadChunk = () => {
    const blob = new Blob([formatJson(chunk)], { type: 'application/json' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = `chunk-${index + 1}.json`;
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
    URL.revokeObjectURL(url);
    toast.success(t('requests.actions.download'));
  };

  return (
    <div className='bg-background rounded-lg border p-4'>
      <div className='flex items-start gap-4'>
        <div className='flex-shrink-0'>
          <span className='text-muted-foreground text-sm font-medium'>Chunk {index + 1}</span>
        </div>
        <div className='min-w-0 flex-1'>
          <JsonViewer data={chunk} rootName='' defaultExpanded={false} className='text-sm' />
        </div>
        <div className='flex gap-2'>
          <Button
            variant='ghost'
            size='icon'
            className='h-8 w-8'
            onClick={copyToClipboard}
          >
            <Copy className='h-4 w-4' />
          </Button>
          <Button
            variant='ghost'
            size='icon'
            className='h-8 w-8'
            onClick={downloadChunk}
          >
            <Download className='h-4 w-4' />
          </Button>
        </div>
      </div>
    </div>
  );
}
