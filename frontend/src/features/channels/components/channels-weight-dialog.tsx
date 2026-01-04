import { useState, useCallback } from 'react';
import { IconWeight } from '@tabler/icons-react';
import { useTranslation } from 'react-i18next';
import { Button } from '@/components/ui/button';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { useChannels } from '../context/channels-context';
import { useUpdateChannel } from '../data/channels';

const WEIGHT_PRECISION = 4;
const MIN_WEIGHT = 0;
const MAX_WEIGHT = 100;

const formatWeight = (value: number) => Number(value.toFixed(WEIGHT_PRECISION));
const clampWeight = (value: number) => formatWeight(Math.min(MAX_WEIGHT, Math.max(MIN_WEIGHT, value)));

interface ChannelsWeightDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

export function ChannelsWeightDialog({ open, onOpenChange }: ChannelsWeightDialogProps) {
  const { t } = useTranslation();
  const { currentRow } = useChannels();
  const updateChannel = useUpdateChannel();

  const [weight, setWeight] = useState<string>(currentRow?.orderingWeight?.toString() || clampWeight(1).toString());

  // Reset weight when channel changes
  const handleOpenChange = useCallback(
    (isOpen: boolean) => {
      if (isOpen && currentRow) {
        setWeight(currentRow.orderingWeight?.toString() || clampWeight(1).toString());
      }
      onOpenChange(isOpen);
    },
    [currentRow, onOpenChange]
  );

  const handleSave = useCallback(async () => {
    if (!currentRow) return;

    try {
      const weightValue = clampWeight(Number(weight));
      await updateChannel.mutateAsync({
        id: currentRow.id,
        input: { orderingWeight: weightValue },
      });
      onOpenChange(false);
    } catch (_error) {
      // Error is handled by the mutation hook
    }
  }, [currentRow, weight, updateChannel, onOpenChange]);

  const handleWeightChange = useCallback((value: string) => {
    setWeight(value);
  }, []);

  if (!currentRow) return null;

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogContent className='sm:max-w-[425px]'>
        <DialogHeader>
          <DialogTitle className='flex items-center gap-2'>
            <IconWeight className='h-5 w-5' />
            {t('channels.dialogs.weight.title')}
          </DialogTitle>
          <DialogDescription>{t('channels.dialogs.weight.description', { name: currentRow.name })}</DialogDescription>
        </DialogHeader>

        <div className='grid gap-4 py-4'>
          <div className='grid grid-cols-4 items-center gap-4'>
            <Label htmlFor='weight' className='text-right'>
              {t('channels.columns.orderingWeight')}
            </Label>
            <Input
              id='weight'
              type='number'
              inputMode='decimal'
              step='any'
              min={MIN_WEIGHT}
              max={MAX_WEIGHT}
              value={weight}
              onChange={(e) => handleWeightChange(e.target.value)}
              className='col-span-3'
            />
          </div>
          <div className='text-muted-foreground text-sm'>
            {t('channels.dialogs.weight.rangeHint', { min: MIN_WEIGHT, max: MAX_WEIGHT })}
          </div>
        </div>

        <DialogFooter>
          <Button variant='outline' onClick={() => onOpenChange(false)}>
            {t('common.buttons.cancel')}
          </Button>
          <Button onClick={handleSave} disabled={updateChannel.isPending}>
            {updateChannel.isPending ? (
              <div className='flex items-center gap-2'>
                <div className='h-4 w-4 animate-spin rounded-full border-b-2 border-white'></div>
                {t('common.buttons.saving')}
              </div>
            ) : (
              t('common.buttons.save')
            )}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
