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
  network: string[];
  initialGranted: string[];
  initialGrantedNetwork: string[];
  preview?: PluginUpgradePreview | null;
  pending?: boolean;
  onConfirm: (grantedPermissions: string[], grantedNetwork: string[]) => void;
}

export function PluginGrantDialog({
  open,
  onOpenChange,
  mode,
  permissions,
  network,
  initialGranted,
  initialGrantedNetwork,
  preview,
  pending = false,
  onConfirm,
}: PluginGrantDialogProps) {
  const { t } = useTranslation();
  const [selected, setSelected] = useState<string[]>([]);
  const [selectedNetwork, setSelectedNetwork] = useState<string[]>([]);
  const wasOpen = useRef(false);

  useEffect(() => {
    if (open && !wasOpen.current) {
      const allowed = new Set(permissions);
      setSelected(initialGranted.filter((permission) => allowed.has(permission)));
      const allowedNetwork = new Set(network);
      setSelectedNetwork(initialGrantedNetwork.filter((rule) => allowedNetwork.has(rule)));
    }
    wasOpen.current = open;
  }, [open, permissions, network, initialGranted, initialGrantedNetwork]);

  const toggle = (current: string[], value: string, checked: boolean) => {
    if (checked) {
      return current.includes(value) ? current : [...current, value];
    }
    return current.filter((item) => item !== value);
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
              <AddedList label={t('plugins.grant.addedNetwork')} items={added(preview.network)} empty={t('plugins.grant.none')} />
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
                    onCheckedChange={(value) => setSelected((current) => toggle(current, permission, value === true))}
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
          <div className='space-y-2'>
            <p className='text-sm font-medium'>{t('plugins.grant.network')}</p>
            {network.length > 0 && <p className='text-destructive text-sm'>{t('plugins.grant.networkWarning')}</p>}
            {network.length === 0 && <p className='text-muted-foreground text-sm'>{t('plugins.grant.none')}</p>}
            {network.map((rule, index) => {
              const inputID = `plugin-grant-network-${index}`;
              return (
                <div key={`${index}:${rule}`} className='flex items-start gap-3 rounded-md border p-3'>
                  <Checkbox
                    id={inputID}
                    checked={selectedNetwork.includes(rule)}
                    onCheckedChange={(value) => setSelectedNetwork((current) => toggle(current, rule, value === true))}
                    disabled={pending}
                  />
                  <Label htmlFor={inputID} className='font-normal break-all'>
                    {rule}
                  </Label>
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
            onClick={() =>
              onConfirm(
                permissions.filter((permission) => selected.includes(permission)),
                network.filter((rule) => selectedNetwork.includes(rule))
              )
            }
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
