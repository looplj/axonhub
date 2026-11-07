'use client';

import { useEffect, useRef, useState, type ReactNode } from 'react';
import { IconAlertTriangle, IconArrowRight } from '@tabler/icons-react';
import { useTranslation } from 'react-i18next';
import { Alert, AlertDescription } from '@/components/ui/alert';
import { Badge } from '@/components/ui/badge';
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
import { cn } from '@/lib/utils';
import type { Plugin, PluginUpgradePreview } from '../data/plugins';
import type { PluginGrantMode } from '../hooks/use-plugin-grant-flow';
import { PluginAvatar } from './plugin-visuals';

interface PluginGrantDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  mode: PluginGrantMode;
  pluginID: string;
  pluginName: string;
  manifestHooks?: Plugin['manifestHooks'];
  permissions: string[];
  network: string[];
  initialGranted: string[];
  initialGrantedNetwork: string[];
  preview?: PluginUpgradePreview | null;
  pending?: boolean;
  onConfirm: (grantedPermissions: string[], grantedNetwork: string[]) => void;
}

const TITLE_KEYS: Record<PluginGrantMode, string> = {
  install: 'plugins.grant.installTitle',
  upgrade: 'plugins.grant.upgradeTitle',
  edit: 'plugins.grant.editTitle',
};

const DESCRIPTION_KEYS: Record<PluginGrantMode, string> = {
  install: 'plugins.grant.installDescription',
  upgrade: 'plugins.grant.upgradeDescription',
  edit: 'plugins.grant.editDescription',
};

const CONFIRM_KEYS: Record<PluginGrantMode, string> = {
  install: 'plugins.grant.confirmInstall',
  upgrade: 'plugins.grant.confirmUpgrade',
  edit: 'plugins.grant.confirmEdit',
};

export function PluginGrantDialog({
  open,
  onOpenChange,
  mode,
  pluginID,
  pluginName,
  manifestHooks,
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

  const showPreview = mode === 'upgrade' && preview;
  // Only items the upgrade adds are flagged; carried-over grants were already reviewed once.
  const newPermissions = new Set(showPreview ? (preview.permissions ?? []) : []);
  const newNetwork = new Set(showPreview ? (preview.network ?? []) : []);
  const hasWrite = permissions.some((permission) => permission.startsWith('write_'));
  const otherAdditions = showPreview
    ? [
        { label: t('plugins.grant.addedEvents'), items: preview.events ?? [] },
        { label: t('plugins.grant.addedHooks'), items: preview.hooks ?? [] },
        { label: t('plugins.grant.addedPages'), items: preview.pages ?? [] },
        { label: t('plugins.grant.addedActions'), items: preview.actions ?? [] },
      ].filter((group) => group.items.length > 0)
    : [];

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader className='flex-row items-center gap-3 space-y-0 text-left'>
          <PluginAvatar id={pluginID} name={pluginName} />
          <div className='min-w-0 space-y-1'>
            <DialogTitle>{t(TITLE_KEYS[mode])}</DialogTitle>
            <DialogDescription>{t(DESCRIPTION_KEYS[mode], { name: pluginName })}</DialogDescription>
          </div>
        </DialogHeader>
        <div className='space-y-5'>
          {!showPreview && manifestHooks && (manifestHooks.request || manifestHooks.attempt) && (
            <div className='space-y-2 rounded-lg border p-3 text-sm'>
              <p className='font-medium'>{t('plugins.hooks.title')}</p>
              {[
                { label: t('plugins.hooks.request'), hook: manifestHooks.request, scope: manifestHooks.request?.requestTypes },
                { label: t('plugins.hooks.attempt'), hook: manifestHooks.attempt, scope: manifestHooks.attempt?.apiFormats },
              ].filter((entry) => entry.hook).map((entry) => (
                <div key={entry.label}>
                  <span>{entry.label} · {t(entry.hook?.enabled ? 'plugins.hooks.enabled' : 'plugins.hooks.disabled')}</span>
                  <p className='text-muted-foreground break-words'>{entry.scope?.join(', ') || t('plugins.none')}</p>
                </div>
              ))}
            </div>
          )}
          {showPreview && (
            <div className='bg-muted/40 space-y-3 rounded-lg border p-3 text-sm'>
              <div className='flex items-center gap-2'>
                <Badge variant='outline'>{preview.currentVersion}</Badge>
                <IconArrowRight className='text-muted-foreground size-4' />
                <Badge>{preview.newVersion}</Badge>
              </div>
              {otherAdditions.map((group) => (
                <div key={group.label} className='space-y-1'>
                  <div className='text-muted-foreground text-xs'>{group.label}</div>
                  <div className='flex flex-wrap gap-1'>
                    {group.items.map((item) => (
                      <Badge key={item} variant='secondary' className='font-mono font-normal'>
                        {item}
                      </Badge>
                    ))}
                  </div>
                </div>
              ))}
              {otherAdditions.length === 0 && newPermissions.size === 0 && newNetwork.size === 0 && (
                <p className='text-muted-foreground text-xs'>{t('plugins.grant.noAdditions')}</p>
              )}
            </div>
          )}

          {(hasWrite || network.length > 0) && (
            <Alert variant='destructive'>
              <IconAlertTriangle />
              <AlertDescription>
                {hasWrite && <p>{t('plugins.grant.writeWarning')}</p>}
                {network.length > 0 && <p>{t('plugins.grant.networkWarning')}</p>}
              </AlertDescription>
            </Alert>
          )}

          <GrantSection
            title={t('plugins.grant.permissions')}
            items={permissions}
            selected={selected}
            pending={pending}
            onSelectedChange={setSelected}
          >
            {permissions.map((permission) => {
              const writePermission = permission.startsWith('write_');
              const inputID = `plugin-grant-${permission}`;
              return (
                <GrantItem
                  key={permission}
                  id={inputID}
                  checked={selected.includes(permission)}
                  disabled={pending}
                  danger={writePermission}
                  isNew={newPermissions.has(permission)}
                  onCheckedChange={(checked) => setSelected((current) => toggle(current, permission, checked))}
                  label={permission}
                  hint={writePermission ? t('plugins.grant.writeAccess') : undefined}
                />
              );
            })}
          </GrantSection>

          <GrantSection
            title={t('plugins.grant.network')}
            items={network}
            selected={selectedNetwork}
            pending={pending}
            onSelectedChange={setSelectedNetwork}
          >
            {network.map((rule, index) => (
              <GrantItem
                key={`${index}:${rule}`}
                id={`plugin-grant-network-${index}`}
                checked={selectedNetwork.includes(rule)}
                disabled={pending}
                isNew={newNetwork.has(rule)}
                onCheckedChange={(checked) => setSelectedNetwork((current) => toggle(current, rule, checked))}
                label={rule}
              />
            ))}
          </GrantSection>
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
            {pending ? t('plugins.grant.pending') : t(CONFIRM_KEYS[mode])}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

function GrantSection({
  title,
  items,
  selected,
  pending,
  onSelectedChange,
  children,
}: {
  title: string;
  items: string[];
  selected: string[];
  pending: boolean;
  onSelectedChange: (next: string[]) => void;
  children: ReactNode;
}) {
  const { t } = useTranslation();
  const allSelected = items.length > 0 && items.every((item) => selected.includes(item));

  return (
    <div className='space-y-2'>
      <div className='flex items-center justify-between gap-2'>
        <p className='text-sm font-medium'>
          {title}
          {items.length > 0 && (
            <span className='text-muted-foreground ml-2 font-normal'>
              {selected.length}/{items.length}
            </span>
          )}
        </p>
        {items.length > 1 && (
          <Button
            type='button'
            variant='link'
            size='sm'
            className='h-auto p-0'
            disabled={pending}
            onClick={() => onSelectedChange(allSelected ? [] : [...items])}
          >
            {allSelected ? t('plugins.grant.clearAll') : t('plugins.grant.selectAll')}
          </Button>
        )}
      </div>
      {items.length === 0 ? (
        <p className='text-muted-foreground rounded-xl border border-dashed px-3 py-2.5 text-sm'>{t('plugins.grant.none')}</p>
      ) : (
        <div className='divide-y overflow-hidden rounded-xl border'>{children}</div>
      )}
    </div>
  );
}

function GrantItem({
  id,
  label,
  hint,
  checked,
  disabled,
  danger = false,
  isNew,
  onCheckedChange,
}: {
  id: string;
  label: string;
  hint?: string;
  checked: boolean;
  disabled: boolean;
  danger?: boolean;
  isNew: boolean;
  onCheckedChange: (checked: boolean) => void;
}) {
  const { t } = useTranslation();

  return (
    <label
      htmlFor={id}
      className={cn(
        'hover:bg-muted/40 flex cursor-pointer items-center gap-3 px-3 py-2.5 transition-colors',
        checked && 'bg-muted/30',
        disabled && 'cursor-not-allowed opacity-60'
      )}
    >
      <Checkbox id={id} checked={checked} onCheckedChange={(value) => onCheckedChange(value === true)} disabled={disabled} />
      <span className={cn('min-w-0 flex-1 font-mono text-sm break-all', danger && 'text-destructive')}>{label}</span>
      {isNew && <Badge className='h-5'>{t('plugins.grant.new')}</Badge>}
      {hint && (
        <Badge variant='outline' className='border-destructive/40 text-destructive h-5'>
          {hint}
        </Badge>
      )}
    </label>
  );
}
