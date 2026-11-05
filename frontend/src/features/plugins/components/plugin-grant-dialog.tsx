'use client';

import { useEffect, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Button } from '@/components/ui/button';
import { Checkbox } from '@/components/ui/checkbox';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { Label } from '@/components/ui/label';
import { cn } from '@/lib/utils';
import type { PluginUpgradePreview } from '../data/plugins';

interface PluginGrantDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  mode: 'install' | 'upgrade';
  permissions: string[];
  initialGranted: string[];
  preview?: PluginUpgradePreview | null;
  pending?: boolean;
  onConfirm: (granted: string[]) => void;
}

export function PluginGrantDialog({
  open,
  onOpenChange,
  mode,
  permissions,
  initialGranted,
  preview,
  pending = false,
  onConfirm,
}: PluginGrantDialogProps) {
  const { t } = useTranslation();
  const [selected, setSelected] = useState<string[]>([]);
  const wasOpen = useRef(false);

  useEffect(() => {
    if (open && !wasOpen.current) {
      const allowed = new Set(permissions);
      setSelected(initialGranted.filter((permission) => allowed.has(permission)));
    }
    wasOpen.current = open;
  }, [open, permissions, initialGranted]);

  const toggle = (permission: string, checked: boolean) => {
    setSelected((current) => {
      if (checked) {
        return current.includes(permission) ? current : [...current, permission];
      }
      return current.filter((item) => item !== permission);
    });
  };

  const added = (items: string[] | undefined) => items ?? [];

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{mode === 'upgrade' ? t('plugins.grant.upgradeTitle') : t('plugins.grant.installTitle')}</DialogTitle>
          <DialogDescription>
            {mode === 'upgrade' ? t('plugins.grant.upgradeDescription') : t('plugins.grant.installDescription')}
          </DialogDescription>
        </DialogHeader>
        <div className='space-y-4'>
          <p className='text-sm'>{t('plugins.grant.writeWarning')}</p>
          <p className='text-sm'>{t('plugins.grant.writeChannelsWarning')}</p>
          {mode === 'upgrade' && preview && (
            <div className='space-y-2 text-sm'>
              <p>{t('plugins.grant.versionChange', { current: preview.currentVersion, next: preview.newVersion })}</p>
              <AddedList label={t('plugins.grant.addedPermissions')} items={added(preview.permissions)} empty={t('plugins.grant.none')} />
              <AddedList label={t('plugins.grant.addedEvents')} items={added(preview.events)} empty={t('plugins.grant.none')} />
              <AddedList label={t('plugins.grant.addedHooks')} items={added(preview.hooks)} empty={t('plugins.grant.none')} />
              <AddedList label={t('plugins.grant.addedPages')} items={added(preview.pages)} empty={t('plugins.grant.none')} />
            </div>
          )}
          <div className='space-y-2'>
            <p className='text-sm font-medium'>{t('plugins.grant.permissions')}</p>
            {permissions.length === 0 && <p className='text-muted-foreground text-sm'>{t('plugins.grant.none')}</p>}
            {permissions.map((permission) => {
              const writePermission = permission.startsWith('write_');
              const inputID = `plugin-grant-${permission}`;
              return (
                <div
                  key={permission}
                  className={cn(
                    'flex items-start gap-3 rounded-md border p-3',
                    writePermission && 'border-destructive/60 bg-destructive/10'
                  )}
                >
                  <Checkbox
                    id={inputID}
                    checked={selected.includes(permission)}
                    onCheckedChange={(value) => toggle(permission, value === true)}
                    disabled={pending}
                  />
                  <div className='space-y-1'>
                    <Label htmlFor={inputID} className='font-normal'>
                      {permission}
                    </Label>
                    {writePermission && <p className='text-destructive text-xs'>{t('plugins.grant.writeAccess')}</p>}
                  </div>
                </div>
              );
            })}
          </div>
        </div>
        <DialogFooter>
          <Button type='button' variant='outline' onClick={() => onOpenChange(false)} disabled={pending}>
            {t('plugins.cancel')}
          </Button>
          <Button
            type='button'
            onClick={() => onConfirm(permissions.filter((permission) => selected.includes(permission)))}
            disabled={pending}
          >
            {pending
              ? t('plugins.grant.pending')
              : mode === 'upgrade'
                ? t('plugins.grant.confirmUpgrade')
                : t('plugins.grant.confirmInstall')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

function AddedList({ label, items, empty }: { label: string; items: string[]; empty: string }) {
  return (
    <p>
      <span className='text-muted-foreground'>{label}: </span>
      {items.length > 0 ? items.join(', ') : empty}
    </p>
  );
}
