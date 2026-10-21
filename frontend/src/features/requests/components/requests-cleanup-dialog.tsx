'use client';

import { useCallback, useEffect, useMemo, useState } from 'react';
import { Loader2, Trash2 } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
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
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import {
  previewGcCleanup,
  useTriggerGcCleanup,
  type GcCleanupPreviewItem,
  type TriggerGcCleanupInput,
} from '@/features/system/data/system';
import { useDebounce } from '@/hooks/use-debounce';

const RETENTION_PRESETS = [1, 7, 30, 90] as const;

type TargetKey = 'requests' | 'requestBodies' | 'responseBodies' | 'responseChunks' | 'usageLogs';

const DEFAULT_TARGETS: Record<TargetKey, boolean> = {
  requests: true,
  requestBodies: true,
  responseBodies: true,
  responseChunks: true,
  usageLogs: false,
};

const RESOURCE_LABEL_KEYS: Record<string, string> = {
  requests: 'requests.cleanup.resources.requests',
  request_bodies: 'requests.cleanup.resources.requestBodies',
  response_bodies: 'requests.cleanup.resources.responseBodies',
  response_chunks: 'requests.cleanup.resources.responseChunks',
  usage_logs: 'requests.cleanup.resources.usageLogs',
};

const TARGET_LABELS: [TargetKey, string][] = [
  ['requests', 'requests'],
  ['requestBodies', 'requestBodies'],
  ['responseBodies', 'responseBodies'],
  ['responseChunks', 'responseChunks'],
  ['usageLogs', 'usageLogs'],
];

function clampRetentionDays(days: number): number {
  if (!Number.isFinite(days)) {
    return 1;
  }

  return Math.min(3650, Math.max(1, Math.trunc(days)));
}

// The GC mutation treats 0 as "this resource is not selected".
export function buildCleanupInput(retentionDays: number, targets: Record<TargetKey, boolean>): TriggerGcCleanupInput {
  const days = clampRetentionDays(retentionDays);

  return {
    requestsCleanupDays: targets.requests ? days : 0,
    usageLogsCleanupDays: targets.usageLogs ? days : 0,
    requestBodiesCleanupDays: targets.requestBodies ? days : 0,
    responseBodiesCleanupDays: targets.responseBodies ? days : 0,
    responseChunksCleanupDays: targets.responseChunks ? days : 0,
  };
}

interface Props {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onCleaned?: () => void;
}

export function RequestsCleanupDialog({ open, onOpenChange, onCleaned }: Props) {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const triggerGcCleanup = useTriggerGcCleanup();

  const [retention, setRetention] = useState<string>('30');
  const [customDays, setCustomDays] = useState(30);
  const [targets, setTargets] = useState<Record<TargetKey, boolean>>(DEFAULT_TARGETS);
  const [previewItems, setPreviewItems] = useState<GcCleanupPreviewItem[]>([]);
  const [previewing, setPreviewing] = useState(false);
  const [previewError, setPreviewError] = useState<string | null>(null);

  const retentionDays = retention === 'custom' ? clampRetentionDays(customDays) : Number(retention);
  const input = useMemo(() => buildCleanupInput(retentionDays, targets), [retentionDays, targets]);
  const debouncedInput = useDebounce(JSON.stringify(input), 400);
  const hasTarget = TARGET_LABELS.some(([key]) => targets[key] === true);

  useEffect(() => {
    if (!open) {
      setPreviewItems([]);
      setPreviewError(null);
      return;
    }

    let cancelled = false;
    const parsed = JSON.parse(debouncedInput) as TriggerGcCleanupInput;
    if (!Object.values(parsed).some((value) => value > 0)) {
      setPreviewItems([]);
      return;
    }

    setPreviewing(true);
    previewGcCleanup(parsed)
      .then((items) => {
        if (cancelled) return;
        setPreviewItems(items);
        setPreviewError(null);
      })
      .catch((error) => {
        if (cancelled) return;
        setPreviewItems([]);
        setPreviewError(error instanceof Error ? error.message : String(error));
      })
      .finally(() => {
        if (!cancelled) setPreviewing(false);
      });

    return () => {
      cancelled = true;
    };
  }, [debouncedInput, open]);

  const handleToggleTarget = useCallback((key: TargetKey, checked: boolean) => {
    setTargets((prev) => ({ ...prev, [key]: checked }));
  }, []);

  const handleCleanup = useCallback(async () => {
    if (!hasTarget) {
      toast.error(t('requests.cleanup.errorNoTarget'));
      return;
    }

    try {
      await triggerGcCleanup.mutateAsync(input);
      toast.success(t('requests.cleanup.success'));
      await queryClient.invalidateQueries({ queryKey: ['requests'] });
      onCleaned?.();
      onOpenChange(false);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t('requests.cleanup.failed'));
    }
  }, [hasTarget, input, onCleaned, onOpenChange, queryClient, t, triggerGcCleanup]);

  const totalEstimated = previewItems.reduce((sum, item) => sum + item.estimatedCount, 0);

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className='sm:max-w-lg' data-testid='requests-cleanup-dialog'>
        <DialogHeader>
          <DialogTitle>{t('requests.cleanup.title')}</DialogTitle>
          <DialogDescription>{t('requests.cleanup.description')}</DialogDescription>
        </DialogHeader>

        <div className='space-y-5'>
          <div className='space-y-2'>
            <Label htmlFor='requests-cleanup-retention'>{t('requests.cleanup.retention.label')}</Label>
            <div className='flex flex-wrap items-center gap-2'>
              <Select value={retention} onValueChange={(value) => value && setRetention(value)}>
                <SelectTrigger id='requests-cleanup-retention' className='w-40'>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {RETENTION_PRESETS.map((days) => (
                    <SelectItem key={days} value={String(days)}>
                      {t('requests.cleanup.retention.options.' + days)}
                    </SelectItem>
                  ))}
                  <SelectItem value='custom'>{t('requests.cleanup.retention.custom')}</SelectItem>
                </SelectContent>
              </Select>
              {retention === 'custom' && (
                <div className='flex items-center gap-2'>
                  <Input
                    id='requests-cleanup-retention-days'
                    type='number'
                    min='1'
                    max='3650'
                    value={customDays}
                    onChange={(event) => {
                      const raw = event.target.value;
                      if (raw.trim() === '') return;
                      setCustomDays(clampRetentionDays(parseInt(raw, 10)));
                    }}
                    className='w-24'
                  />
                  <span className='text-muted-foreground text-sm'>{t('requests.cleanup.retention.days')}</span>
                </div>
              )}
            </div>
            <p className='text-muted-foreground text-xs'>
              {t('requests.cleanup.retention.hint', { days: retentionDays })}
            </p>
          </div>

          <div className='space-y-2'>
            <Label>{t('requests.cleanup.targets.label')}</Label>
            <div className='grid gap-2'>
              {TARGET_LABELS.map(([key, labelKey]) => (
                <label
                  key={key}
                  className='flex items-start gap-2 text-sm'
                  htmlFor={'requests-cleanup-target-' + key}
                >
                  <Checkbox
                    id={'requests-cleanup-target-' + key}
                    checked={targets[key]}
                    onCheckedChange={(checked) => handleToggleTarget(key, checked === true)}
                    className='mt-0.5'
                  />
                  <span>
                    {t('requests.cleanup.targets.' + labelKey)}
                    {key === 'requests' && (
                      <span className='text-muted-foreground block text-xs'>
                        {t('requests.cleanup.targets.requestsHint')}
                      </span>
                    )}
                  </span>
                </label>
              ))}
            </div>
          </div>

          <div className='bg-muted/30 space-y-1 rounded-md border p-3'>
            <div className='flex items-center justify-between text-sm'>
              <span className='font-medium'>{t('requests.cleanup.preview.label')}</span>
              {previewing && <Loader2 className='h-3.5 w-3.5 animate-spin' />}
            </div>
            {previewError && <p className='text-destructive text-xs'>{previewError}</p>}
            {!previewError && previewItems.length === 0 && (
              <p className='text-muted-foreground text-xs'>
                {previewing ? t('requests.cleanup.preview.loading') : t('requests.cleanup.preview.empty')}
              </p>
            )}
            {previewItems.map((item) => (
              <div key={item.resourceType} className='flex items-center justify-between text-xs'>
                <span>{t(RESOURCE_LABEL_KEYS[item.resourceType] || item.resourceType)}</span>
                <span className='font-mono'>{item.estimatedCount}</span>
              </div>
            ))}
            {previewItems.length > 0 && (
              <div className='flex items-center justify-between border-t pt-1 text-xs font-medium'>
                <span>{t('requests.cleanup.preview.total')}</span>
                <span className='font-mono'>{totalEstimated}</span>
              </div>
            )}
          </div>

          <p className='text-muted-foreground text-xs'>{t('requests.cleanup.warning')}</p>
        </div>

        <DialogFooter>
          <Button
            type='button'
            variant='outline'
            onClick={() => onOpenChange(false)}
            disabled={triggerGcCleanup.isPending}
          >
            {t('common.buttons.cancel')}
          </Button>
          <Button
            type='button'
            variant='destructive'
            onClick={handleCleanup}
            disabled={triggerGcCleanup.isPending || !hasTarget}
            data-testid='requests-cleanup-confirm'
          >
            {triggerGcCleanup.isPending ? (
              <Loader2 className='mr-2 h-4 w-4 animate-spin' />
            ) : (
              <Trash2 className='mr-2 h-4 w-4' />
            )}
            {t('requests.cleanup.confirm')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
