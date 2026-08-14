import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Button } from '@/components/ui/button';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Label } from '@/components/ui/label';
import { Switch } from '@/components/ui/switch';
import { AutoCompleteSelect } from '@/components/auto-complete-select';
import { useModels } from '../context/models-context';
import { useUpdateModel, useVisionDelegationCandidates } from '../data/models';

export function ModelsVisionDelegationDialog() {
  const { t } = useTranslation();
  const { open, setOpen, currentRow } = useModels();
  const updateModel = useUpdateModel();
  const dialogContentRef = useRef<HTMLDivElement>(null);
  const [enabled, setEnabled] = useState(false);
  const [targetModelID, setTargetModelID] = useState('');
  const isOpen = open === 'visionDelegation';
  const { data: candidates = [], isLoading } = useVisionDelegationCandidates(currentRow?.modelID, isOpen);
  const supportsNativeVision = Boolean(
    currentRow?.modelCard?.vision || currentRow?.modelCard?.modalities?.input?.some((modality) => modality.toLowerCase() === 'image')
  );

  useEffect(() => {
    if (!isOpen || !currentRow) return;
    setEnabled(currentRow.settings?.visionDelegation?.enabled ?? false);
    setTargetModelID(currentRow.settings?.visionDelegation?.targetModelID ?? '');
  }, [currentRow, isOpen]);

  const candidateOptions = useMemo(() => {
    const options = candidates.map((candidate) => ({
      value: candidate.modelID,
      label: `${candidate.name} (${candidate.modelID})`,
    }));

    if (targetModelID && !options.some((option) => option.value === targetModelID)) {
      options.push({
        value: targetModelID,
        label: `${targetModelID} (${t('models.dialogs.visionDelegation.unavailable')})`,
      });
    }

    return options;
  }, [candidates, t, targetModelID]);

  const targetAvailable = candidates.some((candidate) => candidate.modelID === targetModelID);

  const handleClose = useCallback(() => {
    setOpen(null);
  }, [setOpen]);

  const handleSave = useCallback(async () => {
    if (!currentRow || (enabled && (supportsNativeVision || !targetModelID || !targetAvailable))) return;

    try {
      await updateModel.mutateAsync({
        id: currentRow.id,
        input: {
          settings: {
            ...currentRow.settings,
            visionDelegation: {
              enabled,
              targetModelID: targetModelID || null,
            },
          },
        },
      });
      handleClose();
    } catch (_error) {
      // Error is handled by the mutation.
    }
  }, [currentRow, enabled, handleClose, supportsNativeVision, targetAvailable, targetModelID, updateModel]);

  if (!currentRow) return null;

  return (
    <Dialog open={isOpen} onOpenChange={(nextOpen) => !nextOpen && handleClose()}>
      <DialogContent ref={dialogContentRef} className='sm:max-w-lg'>
        <DialogHeader>
          <DialogTitle>{t('models.dialogs.visionDelegation.title')}</DialogTitle>
          <DialogDescription>{t('models.dialogs.visionDelegation.description', { name: currentRow.name })}</DialogDescription>
        </DialogHeader>

        <div className='space-y-5 py-2'>
          <div className='flex items-center justify-between gap-4 border-b pb-4'>
            <Label htmlFor='vision-delegation-enabled' className='text-sm font-medium'>
              {t('models.dialogs.visionDelegation.enabled')}
            </Label>
            <Switch id='vision-delegation-enabled' checked={enabled} onCheckedChange={setEnabled} disabled={updateModel.isPending} />
          </div>

          {enabled && (
            <div className='space-y-2'>
              {supportsNativeVision && (
                <p role='alert' className='text-sm font-medium text-red-600 dark:text-red-400'>
                  {t('models.dialogs.visionDelegation.nativeVisionWarning')}
                </p>
              )}
              <Label>{t('models.dialogs.visionDelegation.target')}</Label>
              <AutoCompleteSelect
                selectedValue={targetModelID}
                onSelectedValueChange={setTargetModelID}
                items={candidateOptions}
                isLoading={isLoading}
                placeholder={t('models.dialogs.visionDelegation.placeholder')}
                emptyMessage={t('models.dialogs.visionDelegation.noCandidates')}
                portalContainer={dialogContentRef.current}
                inputClassName='h-9'
              />
              {(isLoading || targetModelID) && (
                <p
                  className={targetAvailable ? 'text-xs text-green-600 dark:text-green-400' : 'text-xs text-amber-600 dark:text-amber-400'}
                >
                  {isLoading
                    ? t('models.dialogs.visionDelegation.checking')
                    : targetAvailable
                      ? t('models.dialogs.visionDelegation.available')
                      : t('models.dialogs.visionDelegation.unavailable')}
                </p>
              )}
            </div>
          )}
        </div>

        <DialogFooter>
          <Button type='button' variant='outline' onClick={handleClose} disabled={updateModel.isPending}>
            {t('common.buttons.cancel')}
          </Button>
          <Button
            type='button'
            onClick={handleSave}
            disabled={updateModel.isPending || (enabled && (supportsNativeVision || isLoading || !targetModelID || !targetAvailable))}
          >
            {t('common.buttons.save')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
