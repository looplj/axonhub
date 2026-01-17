import { memo, useCallback, useEffect, useMemo } from 'react';
import { z } from 'zod';
import { useFieldArray, useForm, useWatch, type Control } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { IconPlus, IconTrash, IconCopy } from '@tabler/icons-react';
import type { TFunction } from 'i18next';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';
import { Button } from '@/components/ui/button';
import { Card, CardContent } from '@/components/ui/card';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Form, FormControl, FormField, FormItem, FormLabel, FormMessage } from '@/components/ui/form';
import { ScrollArea } from '@/components/ui/scroll-area';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { ModelPriceEditor } from '@/components/model-price-editor';
import { useGeneralSettings } from '@/features/system/data/system';
import { useChannels } from '../context/channels-context';
import { useChannelModelPrices, useSaveChannelModelPrices } from '../data/channels';
import { PricingMode, PriceItemCode } from '../data/schema';

const priceItemCodes = ['prompt_tokens', 'completion_tokens', 'prompt_cached_tokens', 'prompt_write_cached_tokens'] as const;
const pricingModes = ['flat_fee', 'usage_per_unit', 'usage_tiered'] as const;
const promptWriteCacheVariantCodes = ['five_min', 'one_hour'] as const;

const createPriceFormSchema = (t: (key: string) => string) =>
  z
    .object({
      prices: z.array(
        z.object({
          modelId: z.string().min(1, { message: t('price.validation.modelRequired') }),
          price: z.object({
            items: z.array(
              z.object({
                itemCode: z.enum(priceItemCodes),
                pricing: z.object({
                  mode: z.enum(pricingModes),
                  flatFee: z.string().optional().nullable(),
                  usagePerUnit: z.string().optional().nullable(),
                  usageTiered: z
                    .object({
                      tiers: z.array(
                        z.object({
                          upTo: z.number().nullable().optional(),
                          pricePerUnit: z.string(),
                        })
                      ),
                    })
                    .optional()
                    .nullable(),
                }),
                promptWriteCacheVariants: z
                  .array(
                    z.object({
                      variantCode: z.enum(promptWriteCacheVariantCodes),
                      pricing: z.object({
                        mode: z.enum(pricingModes),
                        flatFee: z.string().optional().nullable(),
                        usagePerUnit: z.string().optional().nullable(),
                        usageTiered: z
                          .object({
                            tiers: z.array(
                              z.object({
                                upTo: z.number().nullable().optional(),
                                pricePerUnit: z.string(),
                              })
                            ),
                          })
                          .optional()
                          .nullable(),
                      }),
                    })
                  )
                  .optional()
                  .nullable(),
              })
            ),
          }),
        })
      ),
    })
    .superRefine((data, ctx) => {
      data.prices.forEach((price, priceIndex) => {
        // Check for duplicate item codes
        const itemCodes = new Map<string, number[]>();
        price.price.items.forEach((item, itemIndex) => {
          const code = item.itemCode;
          if (!itemCodes.has(code)) {
            itemCodes.set(code, []);
          }
          itemCodes.get(code)!.push(itemIndex);
        });

        itemCodes.forEach((indexes, _code) => {
          if (indexes.length > 1) {
            indexes.forEach((index) => {
              ctx.addIssue({
                code: z.ZodIssueCode.custom,
                message: t('price.duplicateItemCode'),
                path: ['prices', priceIndex, 'price', 'items', index, 'itemCode'],
              });
            });
          }
        });

        // Check for duplicate variant codes and validate pricing fields
        price.price.items.forEach((item, itemIndex) => {
          const variantCodes = new Map<string, number[]>();
          (item.promptWriteCacheVariants || []).forEach((variant, variantIndex) => {
            const code = variant.variantCode;
            if (!variantCodes.has(code)) {
              variantCodes.set(code, []);
            }
            variantCodes.get(code)!.push(variantIndex);
          });

          variantCodes.forEach((indexes, _code) => {
            if (indexes.length > 1) {
              indexes.forEach((index) => {
                ctx.addIssue({
                  code: z.ZodIssueCode.custom,
                  message: t('price.duplicateVariantCode'),
                  path: ['prices', priceIndex, 'price', 'items', itemIndex, 'promptWriteCacheVariants', index, 'variantCode'],
                });
              });
            }
          });

          // Validate item pricing based on mode
          const { mode, flatFee, usagePerUnit, usageTiered } = item.pricing;
          const requiredMsg = t('price.validation.priceRequired');
          if (mode === 'flat_fee' && !flatFee) {
            ctx.addIssue({
              code: z.ZodIssueCode.custom,
              message: requiredMsg,
              path: ['prices', priceIndex, 'price', 'items', itemIndex, 'pricing', 'flatFee'],
            });
          }
          if (mode === 'usage_per_unit' && !usagePerUnit) {
            ctx.addIssue({
              code: z.ZodIssueCode.custom,
              message: requiredMsg,
              path: ['prices', priceIndex, 'price', 'items', itemIndex, 'pricing', 'usagePerUnit'],
            });
          }
          if (mode === 'usage_tiered') {
            const tiers = usageTiered?.tiers || [];
            if (tiers.length === 0) {
              ctx.addIssue({
                code: z.ZodIssueCode.custom,
                message: requiredMsg,
                path: ['prices', priceIndex, 'price', 'items', itemIndex, 'pricing', 'usageTiered'],
              });
            }

            const lastTierIndex = tiers.length - 1;
            tiers.forEach((tier, tierIndex) => {
              if (!tier.pricePerUnit) {
                ctx.addIssue({
                  code: z.ZodIssueCode.custom,
                  message: requiredMsg,
                  path: ['prices', priceIndex, 'price', 'items', itemIndex, 'pricing', 'usageTiered', 'tiers', tierIndex, 'pricePerUnit'],
                });
              }

              const isLastTier = tierIndex === lastTierIndex;
              if (isLastTier) {
                if (tier.upTo != null) {
                  ctx.addIssue({
                    code: z.ZodIssueCode.custom,
                    message: t('price.validation.priceRequired'),
                    path: ['prices', priceIndex, 'price', 'items', itemIndex, 'pricing', 'usageTiered', 'tiers', tierIndex, 'upTo'],
                  });
                }
              } else {
                if (tier.upTo == null) {
                  ctx.addIssue({
                    code: z.ZodIssueCode.custom,
                    message: requiredMsg,
                    path: ['prices', priceIndex, 'price', 'items', itemIndex, 'pricing', 'usageTiered', 'tiers', tierIndex, 'upTo'],
                  });
                }
              }
            });
          }

          // Validate variant pricing based on mode
          (item.promptWriteCacheVariants || []).forEach((variant, variantIndex) => {
            const { mode: vMode, flatFee: vFlatFee, usagePerUnit: vUsagePerUnit, usageTiered: vUsageTiered } = variant.pricing;
            if (vMode === 'flat_fee' && !vFlatFee) {
              ctx.addIssue({
                code: z.ZodIssueCode.custom,
                message: requiredMsg,
                path: ['prices', priceIndex, 'price', 'items', itemIndex, 'promptWriteCacheVariants', variantIndex, 'pricing', 'flatFee'],
              });
            }
            if (vMode === 'usage_per_unit' && !vUsagePerUnit) {
              ctx.addIssue({
                code: z.ZodIssueCode.custom,
                message: requiredMsg,
                path: ['prices', priceIndex, 'price', 'items', itemIndex, 'promptWriteCacheVariants', variantIndex, 'pricing', 'usagePerUnit'],
              });
            }
            if (vMode === 'usage_tiered') {
              const vTiers = vUsageTiered?.tiers || [];
              if (vTiers.length === 0) {
                ctx.addIssue({
                  code: z.ZodIssueCode.custom,
                  message: requiredMsg,
                  path: ['prices', priceIndex, 'price', 'items', itemIndex, 'promptWriteCacheVariants', variantIndex, 'pricing', 'usageTiered'],
                });
              }

              const lastTierIndex = vTiers.length - 1;
              vTiers.forEach((tier, tierIndex) => {
                if (!tier.pricePerUnit) {
                  ctx.addIssue({
                    code: z.ZodIssueCode.custom,
                    message: requiredMsg,
                    path: ['prices', priceIndex, 'price', 'items', itemIndex, 'promptWriteCacheVariants', variantIndex, 'pricing', 'usageTiered', 'tiers', tierIndex, 'pricePerUnit'],
                  });
                }

                const isLastTier = tierIndex === lastTierIndex;
                if (isLastTier) {
                  if (tier.upTo != null) {
                    ctx.addIssue({
                      code: z.ZodIssueCode.custom,
                      message: t('price.validation.priceRequired'),
                      path: ['prices', priceIndex, 'price', 'items', itemIndex, 'promptWriteCacheVariants', variantIndex, 'pricing', 'usageTiered', 'tiers', tierIndex, 'upTo'],
                    });
                  }
                } else {
                  if (tier.upTo == null) {
                    ctx.addIssue({
                      code: z.ZodIssueCode.custom,
                      message: requiredMsg,
                      path: ['prices', priceIndex, 'price', 'items', itemIndex, 'promptWriteCacheVariants', variantIndex, 'pricing', 'usageTiered', 'tiers', tierIndex, 'upTo'],
                    });
                  }
                }
              });
            }
          });
        });
      });
    });
type PriceFormData = z.infer<ReturnType<typeof createPriceFormSchema>>;

type ChannelModelPrices = NonNullable<ReturnType<typeof useChannelModelPrices>['data']>;

function buildAvailableModelsByIndex(prices: Array<PriceFormData['prices'][number] | undefined>, supportedModels: string[]) {
  return prices.map((p, currentIndex) => {
    const selectedModels = new Set(prices.map((p, i) => (i !== currentIndex ? p?.modelId : null)).filter(Boolean));

    const available = supportedModels.filter((model) => !selectedModels.has(model));
    if (p?.modelId && !available.includes(p.modelId)) {
      available.push(p.modelId);
    }

    return available;
  });
}

function mapServerPricesToFormData(currentPrices: ChannelModelPrices): PriceFormData {
  return {
    prices: currentPrices.map((p) => ({
      modelId: p.modelID,
      price: {
        items: p.price.items.map((item) => ({
          itemCode: item.itemCode,
          pricing: {
            mode: item.pricing.mode,
            flatFee: item.pricing.flatFee?.toString() || '',
            usagePerUnit: item.pricing.usagePerUnit?.toString() || '',
            usageTiered: item.pricing.usageTiered
              ? {
                  tiers: item.pricing.usageTiered.tiers.map((t) => ({
                    upTo: t.upTo,
                    pricePerUnit: t.pricePerUnit.toString(),
                  })),
                }
              : null,
          },
          promptWriteCacheVariants:
            item.promptWriteCacheVariants?.map((v) => ({
              variantCode: v.variantCode,
              pricing: {
                mode: v.pricing.mode,
                flatFee: v.pricing.flatFee?.toString() || '',
                usagePerUnit: v.pricing.usagePerUnit?.toString() || '',
                usageTiered: v.pricing.usageTiered
                  ? {
                      tiers: v.pricing.usageTiered.tiers.map((t) => ({
                        upTo: t.upTo,
                        pricePerUnit: t.pricePerUnit.toString(),
                      })),
                    }
                  : null,
              },
            })) || [],
        })),
      },
    })),
  };
}

const PriceCard = memo(function PriceCard({
  availableModels,
  control,
  t,
  priceIndex,
  currencyCode,
  onAddItem,
  onDuplicatePrice,
  onRemoveItem,
  onRemovePrice,
  onAddVariant,
  onRemoveVariant,
}: {
  availableModels: string[];
  control: Control<PriceFormData>;
  t: TFunction;
  priceIndex: number;
  currencyCode?: string;
  onAddItem: (priceIndex: number) => void;
  onDuplicatePrice: (priceIndex: number) => void;
  onRemoveItem: (priceIndex: number, itemIndex: number) => void;
  onRemovePrice: (priceIndex: number) => void;
  onAddVariant: (priceIndex: number, itemIndex: number) => void;
  onRemoveVariant: (priceIndex: number, itemIndex: number, variantIndex: number) => void;
}) {
  return (
    <Card className='overflow-hidden'>
      <CardContent className='pt-6'>
        <div className='grid grid-cols-[1fr_auto] gap-x-4 gap-y-3 md:grid-cols-[minmax(0,1fr)_minmax(0,3fr)_auto]'>
          <div className='flex h-8 min-w-0 items-center justify-between'>
            <FormLabel className='truncate pr-2'>{t('price.model')}</FormLabel>
            <Button
              type='button'
              variant='ghost'
              size='icon-sm'
              onClick={() => onDuplicatePrice(priceIndex)}
              title={t('common.actions.duplicate')}
            >
              <IconCopy size={14} />
            </Button>
          </div>

          <div className='flex h-8 min-w-0 items-center'>
            <FormLabel className='truncate'>{t('price.items')}</FormLabel>
          </div>

          <div className='flex items-start justify-end'>
            <Button
              type='button'
              variant='ghost'
              size='icon-sm'
              className='text-destructive'
              onClick={() => onRemovePrice(priceIndex)}
            >
              <IconTrash size={16} />
            </Button>
          </div>

          <div className='min-w-0'>
            <FormField
              control={control}
              name={`prices.${priceIndex}.modelId`}
              render={({ field }) => (
                <FormItem>
                  <Select onValueChange={field.onChange} value={field.value}>
                    <FormControl>
                      <SelectTrigger size='sm' className='h-8 w-full min-w-0' title={field.value || ''}>
                        <SelectValue placeholder={t('price.model')} className='truncate' />
                      </SelectTrigger>
                    </FormControl>
                    <SelectContent>
                      {availableModels.map((model) => (
                        <SelectItem key={model} value={model} title={model}>
                          {model}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                  <FormMessage />
                </FormItem>
              )}
            />
          </div>

          <div className='col-span-1 min-w-0 md:col-span-1'>
            <ModelPriceEditor
              control={control}
              priceIndex={priceIndex}
              currencyCode={currencyCode}
              hideHeader
              onAddItem={onAddItem}
              onRemoveItem={onRemoveItem}
              onAddVariant={onAddVariant}
              onRemoveVariant={onRemoveVariant}
            />
          </div>

          <div className='hidden md:block' />
        </div>
      </CardContent>
    </Card>
  );
});

export function ChannelsModelPriceDialog() {
  const { t } = useTranslation();
  const { open, setOpen, currentRow } = useChannels();
  const { data: settings } = useGeneralSettings();
  const isOpen = open === 'price';
  const { data: currentPrices, isLoading } = useChannelModelPrices(currentRow?.id || '');
  const savePrices = useSaveChannelModelPrices();

  const formSchema = useMemo(() => createPriceFormSchema(t), [t]);
  const form = useForm<PriceFormData>({
    resolver: zodResolver(formSchema),
    mode: 'onChange',
    defaultValues: {
      prices: [],
    },
  });

  const { control, getValues, reset, setValue, clearErrors } = form;

  const { fields, append, remove } = useFieldArray({
    control,
    name: 'prices',
  });

  const supportedModels = useMemo(() => currentRow?.supportedModels || [], [currentRow?.supportedModels]);
  const watchedPrices = useWatch({ control, name: 'prices' });
  const availableModelsByIndex = useMemo(
    () => buildAvailableModelsByIndex(watchedPrices || [], supportedModels),
    [supportedModels, watchedPrices]
  );

  useEffect(() => {
    if (isOpen && currentPrices) {
      reset(mapServerPricesToFormData(currentPrices));
    }
  }, [isOpen, currentPrices, reset]);

  const handleClose = useCallback(() => {
    setOpen(null);
    reset();
  }, [setOpen, reset]);

  const onSubmit = useCallback(
    async (data: PriceFormData) => {
      if (!currentRow) return;

      try {
        const input = data.prices.map((p) => ({
          modelId: p.modelId,
          price: {
            items: p.price.items.map((item) => ({
              itemCode: item.itemCode as PriceItemCode,
              pricing: {
                mode: item.pricing.mode as PricingMode,
                flatFee: item.pricing.flatFee || null,
                usagePerUnit: item.pricing.usagePerUnit || null,
                usageTiered: item.pricing.usageTiered
                  ? {
                      tiers: item.pricing.usageTiered.tiers.map((t) => ({
                        upTo: t.upTo,
                        pricePerUnit: t.pricePerUnit,
                      })),
                    }
                  : null,
              },
              promptWriteCacheVariants:
                item.promptWriteCacheVariants?.map((v) => ({
                  variantCode: v.variantCode,
                  pricing: {
                    mode: v.pricing.mode as PricingMode,
                    flatFee: v.pricing.flatFee || null,
                    usagePerUnit: v.pricing.usagePerUnit || null,
                    usageTiered: v.pricing.usageTiered
                      ? {
                          tiers: v.pricing.usageTiered.tiers.map((t) => ({
                            upTo: t.upTo,
                            pricePerUnit: t.pricePerUnit,
                          })),
                        }
                      : null,
                  },
                })) || [],
            })),
          },
        }));

        await savePrices.mutateAsync({
          channelId: currentRow.id,
          input,
        });
        handleClose();
      } catch (_error) {
        // Error handled by mutation
      }
    },
    [currentRow, handleClose, savePrices]
  );

  const addPrice = useCallback(() => {
    append({
      modelId: '',
      price: {
        items: [
          {
            itemCode: 'prompt_tokens',
            pricing: { mode: 'usage_per_unit', usagePerUnit: '0' },
          },
        ],
      },
    });
  }, [append]);

  const removePrice = useCallback((index: number) => remove(index), [remove]);

  const addItem = useCallback(
    (index: number) => {
      const currentItems = getValues(`prices.${index}.price.items`);
      const existingCodes = new Set(currentItems.map((item) => item.itemCode));
      const nextCode = priceItemCodes.find((code) => !existingCodes.has(code));

      if (nextCode) {
        setValue(`prices.${index}.price.items`, [
          ...currentItems,
          {
            itemCode: nextCode,
            pricing: { mode: 'usage_per_unit', usagePerUnit: '0' },
          },
        ]);
      }
    },
    [getValues, setValue]
  );

  const removeItem = useCallback(
    (priceIndex: number, itemIndex: number) => {
      const currentItems = getValues(`prices.${priceIndex}.price.items`);
      if (currentItems.length > 1) {
        // Clear all itemCode errors for this price before removal to avoid stale index errors
        currentItems.forEach((_, i) => {
          clearErrors(`prices.${priceIndex}.price.items.${i}.itemCode`);
        });
        setValue(
          `prices.${priceIndex}.price.items`,
          currentItems.filter((_, i) => i !== itemIndex)
        );
      }
    },
    [clearErrors, getValues, setValue]
  );

  const addVariant = useCallback(
    (priceIndex: number, itemIndex: number) => {
      const currentVariants = getValues(`prices.${priceIndex}.price.items.${itemIndex}.promptWriteCacheVariants`) || [];

      const existingCodes = new Set((currentVariants as Array<{ variantCode?: string }>).map((v) => v.variantCode).filter(Boolean));
      const nextCode = promptWriteCacheVariantCodes.find((code) => !existingCodes.has(code));
      if (!nextCode) return;

      setValue(`prices.${priceIndex}.price.items.${itemIndex}.promptWriteCacheVariants`, [
        ...currentVariants,
        {
          variantCode: nextCode,
          pricing: { mode: 'usage_per_unit', usagePerUnit: '0' },
        },
      ]);
    },
    [getValues, setValue]
  );

  const removeVariant = useCallback(
    (priceIndex: number, itemIndex: number, variantIndex: number) => {
      const currentVariants = getValues(`prices.${priceIndex}.price.items.${itemIndex}.promptWriteCacheVariants`) || [];
      // Clear all variantCode errors for this item before removal to avoid stale index errors
      currentVariants.forEach((_, i) => {
        clearErrors(`prices.${priceIndex}.price.items.${itemIndex}.promptWriteCacheVariants.${i}.variantCode`);
      });
      setValue(
        `prices.${priceIndex}.price.items.${itemIndex}.promptWriteCacheVariants`,
        currentVariants.filter((_, i) => i !== variantIndex)
      );
    },
    [clearErrors, getValues, setValue]
  );

  const duplicatePrice = useCallback(
    (index: number) => {
      const priceData = getValues(`prices.${index}.price`);
      append({
        modelId: '',
        price: structuredClone(priceData),
      });
      toast.success(t('common.success.duplicated'));
    },
    [getValues, append, t]
  );

  return (
    <Dialog open={isOpen} onOpenChange={handleClose}>
      <DialogContent className='flex h-[85vh] max-h-[800px] flex-col overflow-hidden sm:max-w-4xl'>
        <DialogHeader>
          <DialogTitle>{t('price.title')}</DialogTitle>
          <DialogDescription>{t('price.description', { name: currentRow?.name })}</DialogDescription>
        </DialogHeader>

        <Form {...form}>
          <form onSubmit={form.handleSubmit(onSubmit)} className='flex min-h-0 min-w-0 flex-1 flex-col overflow-hidden'>
            <ScrollArea className='min-h-0 min-w-0 flex-1 overflow-x-hidden'>
              <div className='space-y-4 py-4 pr-4'>
                {fields.map((field, index) => (
                  <PriceCard
                    key={field.id}
                    availableModels={availableModelsByIndex[index] || supportedModels}
                    control={control}
                    t={t}
                    priceIndex={index}
                    currencyCode={settings?.currencyCode}
                    onAddItem={addItem}
                    onDuplicatePrice={duplicatePrice}
                    onRemoveItem={removeItem}
                    onRemovePrice={removePrice}
                    onAddVariant={addVariant}
                    onRemoveVariant={removeVariant}
                  />
                ))}

                {fields.length === 0 && !isLoading && (
                  <div className='text-muted-foreground flex flex-col items-center justify-center py-12'>
                    <p>{t('price.noPrices')}</p>
                  </div>
                )}
              </div>
            </ScrollArea>

            <DialogFooter className='mt-6 shrink-0 gap-2 sm:justify-between'>
              <Button type='button' variant='outline' onClick={addPrice}>
                <IconPlus className='mr-2 h-4 w-4' />
                {t('price.addPrice')}
              </Button>
              <div className='flex gap-2'>
                <Button type='button' variant='ghost' onClick={handleClose}>
                  {t('common.buttons.cancel')}
                </Button>
                <Button type='submit' disabled={savePrices.isPending}>
                  {t('common.buttons.save')}
                </Button>
              </div>
            </DialogFooter>
          </form>
        </Form>
      </DialogContent>
    </Dialog>
  );
}
