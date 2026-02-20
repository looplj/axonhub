import { useCallback, useEffect, useMemo } from 'react';
import { z } from 'zod';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { useTranslation } from 'react-i18next';
import { Button } from '@/components/ui/button';
import { Card, CardContent } from '@/components/ui/card';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Form, FormControl, FormField, FormItem, FormLabel, FormMessage } from '@/components/ui/form';
import { Input } from '@/components/ui/input';
import { useGeneralSettings } from '@/features/system/data/system';
import { useChannels } from '../context/channels-context';
import { useChannelModelPrices, useSaveChannelModelPrices } from '../data/channels';
import { PricingMode, PriceItemCode } from '../data/schema';

const createSearchPriceFormSchema = (t: (key: string) => string) =>
  z.object({
    flatFee: z.string().min(1, { message: t('price.validation.priceRequired') }),
  });

type SearchPriceFormData = z.infer<ReturnType<typeof createSearchPriceFormSchema>>;

type ChannelModelPrices = NonNullable<ReturnType<typeof useChannelModelPrices>['data']>;

function extractFlatFeeFromPrices(currentPrices: ChannelModelPrices): string {
  const firstPrice = currentPrices[0];
  if (!firstPrice) return '';
  const firstItem = firstPrice.price.items[0];
  return firstItem?.pricing.flatFee?.toString() || '';
}

export function SearchChannelPriceDialog() {
  const { t } = useTranslation();
  const { open, setOpen, currentRow } = useChannels();
  const { data: settings } = useGeneralSettings();
  const isOpen = open === 'searchPrice';
  const { data: currentPrices } = useChannelModelPrices(currentRow?.id || '');
  const savePrices = useSaveChannelModelPrices();

  const formSchema = useMemo(() => createSearchPriceFormSchema(t), [t]);
  const form = useForm<SearchPriceFormData>({
    resolver: zodResolver(formSchema),
    mode: 'onChange',
    defaultValues: { flatFee: '' },
  });

  const { reset } = form;

  useEffect(() => {
    if (isOpen && currentPrices) {
      reset({ flatFee: extractFlatFeeFromPrices(currentPrices) });
    }
  }, [isOpen, currentPrices, reset]);

  const handleClose = useCallback(() => {
    setOpen(null);
    reset();
  }, [setOpen, reset]);

  const onSubmit = useCallback(
    async (data: SearchPriceFormData) => {
      if (!currentRow) return;

      try {
        const models = (currentRow.supportedModels || []).filter((m) => m.startsWith('__') && m.endsWith('_search'));
        // Always include __search
        if (!models.includes('__search')) {
          models.unshift('__search');
        }

        const input = models.map((modelId) => ({
          modelId,
          price: {
            items: [
              {
                itemCode: 'requests' as PriceItemCode,
                pricing: {
                  mode: 'flat_fee' as PricingMode,
                  flatFee: data.flatFee || null,
                  usagePerUnit: null,
                  usageTiered: null,
                },
                promptWriteCacheVariants: [],
              },
            ],
          },
        }));

        await savePrices.mutateAsync({
          channelId: currentRow.id,
          input,
        });
        handleClose();
      } catch (_error) {}
    },
    [currentRow, handleClose, savePrices]
  );

  const currencyCode = settings?.currencyCode || 'USD';

  return (
    <Dialog open={isOpen} onOpenChange={handleClose}>
      <DialogContent className='sm:max-w-md'>
        <DialogHeader>
          <DialogTitle>{t('price.searchTitle')}</DialogTitle>
          <DialogDescription>{t('price.searchDescription', { name: currentRow?.name })}</DialogDescription>
        </DialogHeader>

        <Form {...form}>
          <form onSubmit={form.handleSubmit(onSubmit)} className='space-y-4'>
            <Card>
              <CardContent className='pt-4'>
                <div className='text-xs text-muted-foreground'>{t('price.searchHint')}</div>
              </CardContent>
            </Card>

            <FormField
              control={form.control}
              name='flatFee'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('price.pricePerRequest')}</FormLabel>
                  <FormControl>
                    <div className='relative'>
                      <span className='absolute left-3 top-1/2 -translate-y-1/2 text-sm text-muted-foreground'>{currencyCode} $</span>
                      <Input {...field} type='number' step='0.0001' min='0' placeholder='0.0000' className='pl-14' />
                    </div>
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />

            <DialogFooter className='gap-2'>
              <Button type='button' variant='ghost' onClick={handleClose}>
                {t('common.buttons.cancel')}
              </Button>
              <Button type='submit' disabled={savePrices.isPending}>
                {t('common.buttons.save')}
              </Button>
            </DialogFooter>
          </form>
        </Form>
      </DialogContent>
    </Dialog>
  );
}
