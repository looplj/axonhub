'use client';

import { useTranslation } from 'react-i18next';
import { Button } from '@/components/ui/button';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';

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

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t('plugins.upload.title')}</DialogTitle>
          <DialogDescription>{t('plugins.upload.description')}</DialogDescription>
        </DialogHeader>
        <div className='space-y-2'>
          <Label htmlFor='plugin-package'>{t('plugins.upload.choose')}</Label>
          <Input
            id='plugin-package'
            type='file'
            accept='.axplugin,application/zip'
            onChange={(event) => onFileChange(event.target.files?.[0] ?? null)}
          />
        </div>
        <DialogFooter>
          <Button type='button' variant='outline' onClick={() => onOpenChange(false)}>
            {t('plugins.cancel')}
          </Button>
          <Button type='button' onClick={onSubmit} disabled={!file || pending}>
            {pending ? t('plugins.upload.uploading') : t('plugins.upload.button')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
