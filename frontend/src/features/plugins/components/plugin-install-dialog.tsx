'use client';

import { useState } from 'react';
import { IconFileZip, IconLoader2, IconUpload } from '@tabler/icons-react';
import { useTranslation } from 'react-i18next';
import { cn } from '@/lib/utils';
import { Button } from '@/components/ui/button';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';

interface PluginInstallDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  file: File | null;
  onFileChange: (file: File | null) => void;
  pending: boolean;
  onSubmit: () => void;
}

export function PluginInstallDialog({ open, onOpenChange, file, onFileChange, pending, onSubmit }: PluginInstallDialogProps) {
  const { t } = useTranslation();
  const [dragging, setDragging] = useState(false);

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t('plugins.upload.title')}</DialogTitle>
          <DialogDescription>{t('plugins.upload.description')}</DialogDescription>
        </DialogHeader>
        <form
          id='plugin-install-form'
          onSubmit={(event) => {
            event.preventDefault();
            if (file && !pending) {
              onSubmit();
            }
          }}
        >
          <label
            htmlFor='plugin-package'
            onDragOver={(event) => {
              event.preventDefault();
              setDragging(true);
            }}
            onDragLeave={() => setDragging(false)}
            onDrop={(event) => {
              event.preventDefault();
              setDragging(false);
              const dropped = event.dataTransfer.files?.[0];
              if (!pending && dropped) {
                onFileChange(dropped);
              }
            }}
            className={cn(
              'flex cursor-pointer flex-col items-center justify-center gap-3 rounded-2xl border-2 border-dashed px-6 py-10 text-center transition-colors',
              'hover:border-primary/50 hover:bg-muted/40',
              'focus-within:border-primary focus-within:ring-ring/50 focus-within:ring-[3px]',
              dragging && 'border-primary bg-primary/5',
              file && 'border-solid',
              pending && 'pointer-events-none opacity-60'
            )}
          >
            <div
              className={cn(
                'flex size-12 items-center justify-center rounded-2xl',
                file ? 'bg-primary/10 text-primary' : 'bg-muted text-muted-foreground'
              )}
            >
              {file ? <IconFileZip className='size-6' /> : <IconUpload className='size-6' />}
            </div>
            {file ? (
              <div className='min-w-0 space-y-0.5'>
                <p className='truncate text-sm font-medium'>{file.name}</p>
                <p className='text-muted-foreground text-xs'>
                  {formatSize(file.size)} · {t('plugins.upload.change')}
                </p>
              </div>
            ) : (
              <div className='space-y-0.5'>
                <p className='text-sm font-medium'>{t('plugins.upload.dropHint')}</p>
                <p className='text-muted-foreground text-xs'>.axplugin</p>
              </div>
            )}
            <input
              id='plugin-package'
              type='file'
              accept='.axplugin,application/zip'
              className='sr-only'
              disabled={pending}
              onChange={(event) => onFileChange(event.target.files?.[0] ?? null)}
            />
          </label>
        </form>
        <DialogFooter>
          <Button type='button' variant='outline' onClick={() => onOpenChange(false)} disabled={pending}>
            {t('plugins.cancel')}
          </Button>
          <Button type='submit' form='plugin-install-form' disabled={!file || pending}>
            {pending && <IconLoader2 className='size-4 animate-spin' />}
            {pending ? t('plugins.upload.uploading') : t('plugins.upload.button')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

function formatSize(bytes: number) {
  if (bytes < 1024) {
    return `${bytes} B`;
  }
  if (bytes < 1024 * 1024) {
    return `${(bytes / 1024).toFixed(1)} KB`;
  }
  return `${(bytes / 1024 / 1024).toFixed(1)} MB`;
}
