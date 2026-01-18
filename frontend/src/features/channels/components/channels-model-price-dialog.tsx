import { memo, useCallback, useEffect, useMemo, useState } from 'react';
import { z } from 'zod';
import { useFieldArray, useForm, useWatch, type Control } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { IconPlus, IconTrash, IconCopy } from '@tabler/icons-react';
import type { TFunction } from 'i18next';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';
import { AutoCompleteSelect } from '@/components/auto-complete-select';
import { Button } from '@/components/ui/button';
import { Card, CardContent } from '@/components/ui/card';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Form, FormControl, FormField, FormItem, FormLabel, FormMessage } from '@/components/ui/form';
import { Input } from '@/components/ui/input';
import { ScrollArea } from '@/components/ui/scroll-area';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { ModelPriceEditor } from '@/components/model-price-editor';
import { type ProviderModel, type ProvidersData } from '@/features/models/data/providers.schema';
import { useProvidersData } from '@/features/models/data/providers';
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
      type UsageTier = {
        upTo?: number | null;
        pricePerUnit?: string;
      };
      type PricingLike = {
        mode?: (typeof pricingModes)[number];
        flatFee?: string | null;
        usagePerUnit?: string | null;
        usageTiered?: {
          tiers: UsageTier[];
        } | null;
      };

      const validatePricing = (pricing: PricingLike | null | undefined, pathPrefix: Array<string | number>) => {
        const requiredMsg = t('price.validation.priceRequired');
        const { mode, flatFee, usagePerUnit, usageTiered } = pricing || {};
        if (mode === 'flat_fee' && !flatFee) {
          ctx.addIssue({
            code: z.ZodIssueCode.custom,
            message: requiredMsg,
            path: [...pathPrefix, 'flatFee'],
          });
        }
        if (mode === 'usage_per_unit' && !usagePerUnit) {
          ctx.addIssue({
            code: z.ZodIssueCode.custom,
            message: requiredMsg,
            path: [...pathPrefix, 'usagePerUnit'],
          });
        }
        if (mode === 'usage_tiered') {
          const tiers = usageTiered?.tiers || [];
          if (tiers.length === 0) {
            ctx.addIssue({
              code: z.ZodIssueCode.custom,
              message: requiredMsg,
              path: [...pathPrefix, 'usageTiered'],
            });
          }

          const lastTierIndex = tiers.length - 1;
          tiers.forEach((tier: UsageTier, tierIndex: number) => {
            if (!tier.pricePerUnit) {
              ctx.addIssue({
                code: z.ZodIssueCode.custom,
                message: requiredMsg,
                path: [...pathPrefix, 'usageTiered', 'tiers', tierIndex, 'pricePerUnit'],
              });
            }

            const isLastTier = tierIndex === lastTierIndex;
            if (isLastTier) {
              if (tier.upTo != null) {
                ctx.addIssue({
                  code: z.ZodIssueCode.custom,
                  message: requiredMsg,
                  path: [...pathPrefix, 'usageTiered', 'tiers', tierIndex, 'upTo'],
                });
              }
            } else {
              if (tier.upTo == null) {
                ctx.addIssue({
                  code: z.ZodIssueCode.custom,
                  message: requiredMsg,
                  path: [...pathPrefix, 'usageTiered', 'tiers', tierIndex, 'upTo'],
                });
              }
            }
          });
        }
      };

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
          validatePricing(item.pricing, ['prices', priceIndex, 'price', 'items', itemIndex, 'pricing']);

          // Validate variant pricing based on mode
          (item.promptWriteCacheVariants || []).forEach((variant, variantIndex) => {
            validatePricing(variant.pricing, [
              'prices',
              priceIndex,
              'price',
              'items',
              itemIndex,
              'promptWriteCacheVariants',
              variantIndex,
              'pricing',
            ]);
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

function normalizeProviderKeyFromChannelType(type?: string | null) {
  if (!type) return '';
  const first = type.split('_')[0] || '';
  return first;
}

function getProviderModelLabel(model: ProviderModel) {
  const name = model.display_name || model.name || '';
  if (!name || name === model.id) return model.id;
  return `${name} (${model.id})`;
}

function findProviderModelById(providersData: ProvidersData, modelId: string, providerId?: string) {
  const provider = providerId ? providersData.providers[providerId] : undefined;
  if (provider?.models?.length) {
    const found = provider.models.find((m) => m.id === modelId);
    if (found) return { providerId, model: found };
  }

  for (const [pid, p] of Object.entries(providersData.providers)) {
    const found = (p.models || []).find((m) => m.id === modelId);
    if (found) return { providerId: pid, model: found };
  }

  return null;
}

function buildItemsFromProviderModel(model: ProviderModel, multiplier: number = 1): PriceFormData['prices'][number]['price']['items'] {
  const items: PriceFormData['prices'][number]['price']['items'] = [];
  const cost = model.cost;

  const pushUsagePerUnit = (itemCode: (typeof priceItemCodes)[number], value: number) => {
    items.push({
      itemCode,
      pricing: {
        mode: 'usage_per_unit',
        usagePerUnit: String(value * multiplier),
      },
    });
  };

  if (cost?.input != null) pushUsagePerUnit('prompt_tokens', cost.input);
  if (cost?.output != null) pushUsagePerUnit('completion_tokens', cost.output);
  if (cost?.cache_read != null) pushUsagePerUnit('prompt_cached_tokens', cost.cache_read);
  if (cost?.cache_write != null) pushUsagePerUnit('prompt_write_cached_tokens', cost.cache_write);

  if (items.length === 0) {
    items.push({
      itemCode: 'prompt_tokens',
      pricing: { mode: 'usage_per_unit', usagePerUnit: '0' },
    });
  }

  return items;
}

function mergeItemsWithProviderCost(
  currentItems: PriceFormData['prices'][number]['price']['items'],
  model: ProviderModel,
  multiplier: number = 1
): PriceFormData['prices'][number]['price']['items'] {
  const byCode = new Map<(typeof priceItemCodes)[number], PriceFormData['prices'][number]['price']['items'][number]>();
  currentItems.forEach((item) => {
    byCode.set(item.itemCode, item);
  });

  const applyUsagePerUnit = (itemCode: (typeof priceItemCodes)[number], value: number) => {
    const existing = byCode.get(itemCode);
    if (existing) {
      byCode.set(itemCode, {
        ...existing,
        pricing: {
          mode: 'usage_per_unit',
          usagePerUnit: String(value * multiplier),
          flatFee: '',
          usageTiered: null,
        },
      });
      return;
    }
    byCode.set(itemCode, {
      itemCode,
      pricing: { mode: 'usage_per_unit', usagePerUnit: String(value * multiplier) },
    });
  };

  const cost = model.cost;
  if (cost?.input != null) applyUsagePerUnit('prompt_tokens', cost.input);
  if (cost?.output != null) applyUsagePerUnit('completion_tokens', cost.output);
  if (cost?.cache_read != null) applyUsagePerUnit('prompt_cached_tokens', cost.cache_read);
  if (cost?.cache_write != null) applyUsagePerUnit('prompt_write_cached_tokens', cost.cache_write);

  return Array.from(byCode.values());
}

const PriceCard = memo(function PriceCard({
  availableModels,
  control,
  t,
  priceIndex,
  currencyCode,
  onAddItem,
  onModelSelected,
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
  onModelSelected: (priceIndex: number, modelId: string) => void;
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
                  <Select
                    onValueChange={(value) => {
                      field.onChange(value);
                      onModelSelected(priceIndex, value);
                    }}
                    value={field.value}
                  >
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
  const [dialogContent, setDialogContent] = useState<HTMLDivElement | null>(null);

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

  const { data: providersData } = useProvidersData();

  const providerOptions = useMemo(
    () =>
      providersData
        ? Object.entries(providersData.providers).map(([id, p]) => ({
            value: id,
            label: p.display_name || p.name || id,
          }))
        : [],
    [providersData]
  );

  const defaultProviderId = useMemo(() => normalizeProviderKeyFromChannelType(currentRow?.type), [currentRow?.type]);
  const [selectedProviderId, setSelectedProviderId] = useState<string>('');
  const [selectedModelId, setSelectedModelId] = useState<string>('');
  const [multiplier, setMultiplier] = useState<number>(1);

  useEffect(() => {
    if (!isOpen || !providersData) return;
    const next = defaultProviderId && providersData.providers[defaultProviderId] ? defaultProviderId : '';
    setSelectedProviderId(next);
    setSelectedModelId('');
    setMultiplier(1);
  }, [defaultProviderId, isOpen, providersData]);

  const providerModels = useMemo(() => {
    if (!selectedProviderId || !providersData) return [];
    return providersData.providers[selectedProviderId]?.models || [];
  }, [providersData, selectedProviderId]);

  const providerModelOptions = useMemo(
    () =>
      providerModels.map((m) => ({
        value: m.id,
        label: getProviderModelLabel(m),
      })),
    [providerModels]
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

  const applyProviderModelToIndex = useCallback(
    (priceIndex: number, providerModel: ProviderModel) => {
      const currentItems = getValues(`prices.${priceIndex}.price.items`) || [];
      const merged = mergeItemsWithProviderCost(currentItems, providerModel, multiplier);
      setValue(`prices.${priceIndex}.price.items`, merged, { shouldDirty: true, shouldValidate: true });
    },
    [getValues, setValue, multiplier]
  );

  const applyProviderModelById = useCallback(
    (modelId: string, providerId?: string) => {
      if (!providersData) return;
      const found = findProviderModelById(providersData, modelId, providerId);
      if (!found) {
        toast.error(t('price.apply.notFound', { modelId }));
        return;
      }

      const prices = getValues('prices') || [];
      const existingIndex = prices.findIndex((p) => p?.modelId === modelId);
      if (existingIndex >= 0) {
        applyProviderModelToIndex(existingIndex, found.model);
        toast.success(t('price.apply.applied', { modelId }));
        return;
      }

      append({
        modelId,
        price: { items: buildItemsFromProviderModel(found.model, multiplier) },
      });
      toast.success(t('price.apply.added', { modelId }));
    },
    [append, applyProviderModelToIndex, getValues, providersData, t, multiplier]
  );

  const onModelSelected = useCallback(
    (priceIndex: number, modelId: string) => {
      if (!modelId || !providersData) return;
      const preferredProviderId =
        defaultProviderId && providersData.providers[defaultProviderId] ? defaultProviderId : selectedProviderId;
      const found = findProviderModelById(providersData, modelId, preferredProviderId);
      if (!found) return;
      applyProviderModelToIndex(priceIndex, found.model);
      toast.success(t('price.apply.applied', { modelId }));
    },
    [applyProviderModelToIndex, defaultProviderId, providersData, selectedProviderId, t]
  );

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
      <DialogContent
        ref={setDialogContent}
        className='flex h-[85vh] max-h-[800px] flex-col overflow-hidden sm:max-w-4xl'
      >
        <DialogHeader>
          <DialogTitle>{t('price.title')}</DialogTitle>
          <DialogDescription>{t('price.description', { name: currentRow?.name })}</DialogDescription>
        </DialogHeader>

        <Form {...form}>
          <form onSubmit={form.handleSubmit(onSubmit)} className='flex min-h-0 min-w-0 flex-1 flex-col overflow-hidden'>
            <Card className='mb-4 shrink-0'>
              <CardContent className='pt-4'>
                <div className='mb-3 text-xs text-muted-foreground'>
                  {t('price.apply.usdHint')}
                </div>
                <div className='grid grid-cols-1 gap-3 md:grid-cols-[minmax(0,1fr)_minmax(0,2fr)_80px_auto] md:items-end'>
                  <div className='min-w-0'>
                    <FormLabel className='text-sm'>{t('price.apply.provider')}</FormLabel>
                    <AutoCompleteSelect
                      selectedValue={selectedProviderId}
                      onSelectedValueChange={(value) => {
                        setSelectedProviderId(value);
                        setSelectedModelId('');
                      }}
                      items={providerOptions}
                      placeholder={t('price.apply.providerPlaceholder')}
                      emptyMessage={t('price.apply.empty')}
                      portalContainer={dialogContent}
                      inputClassName='h-8'
                    />
                  </div>
                  <div className='min-w-0'>
                    <FormLabel className='text-sm'>{t('price.apply.model')}</FormLabel>
                    <AutoCompleteSelect
                      selectedValue={selectedModelId}
                      onSelectedValueChange={(value) => {
                        setSelectedModelId(value);
                        if (value) applyProviderModelById(value, selectedProviderId);
                      }}
                      items={providerModelOptions}
                      placeholder={t('price.apply.modelPlaceholder')}
                      emptyMessage={t('price.apply.empty')}
                      portalContainer={dialogContent}
                      inputClassName='h-8'
                    />
                  </div>
                  <div className='min-w-0'>
                    <FormLabel className='text-sm'>{t('price.apply.multiplier')}</FormLabel>
                    <Input
                      type='number'
                      value={multiplier}
                      onChange={(e) => setMultiplier(parseFloat(e.target.value) || 0)}
                      className='h-8'
                      step='0.01'
                      min='0'
                    />
                  </div>
                  <div className='flex gap-2'>
                    <Button
                      type='button'
                      variant='outline'
                      onClick={() => {
                        if (!providersData) return;
                        const providerId = selectedProviderId || defaultProviderId;
                        const prices = getValues('prices') || [];
                        const existingModelIds = new Set(prices.map((p) => p?.modelId).filter(Boolean));

                        let applied = 0;
                        let added = 0;
                        let missed = 0;

                        supportedModels.forEach((modelId) => {
                          const found = findProviderModelById(providersData, modelId, providerId);
                          if (!found) {
                            missed += 1;
                            return;
                          }
                          const existingIndex = prices.findIndex((p) => p?.modelId === modelId);
                          if (existingIndex >= 0) {
                            applyProviderModelToIndex(existingIndex, found.model);
                            applied += 1;
                            return;
                          }
                          if (existingModelIds.has(modelId)) return;
                          append({
                            modelId,
                            price: { items: buildItemsFromProviderModel(found.model, multiplier) },
                          });
                          added += 1;
                        });

                        if (applied || added) {
                          toast.success(t('price.apply.bulkSuccess', { applied, added }));
                        }
                        if (missed) {
                          toast.warning(t('price.apply.bulkMissed', { missed }));
                        }
                      }}
                      disabled={supportedModels.length === 0}
                      title={t('price.apply.bulk')}
                    >
                      {t('price.apply.bulk')}
                    </Button>
                  </div>
                </div>
              </CardContent>
            </Card>
            <ScrollArea className='min-h-0 min-w-0 flex-1 [&>[data-slot=scroll-area-viewport]]:!overflow-x-hidden'>
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
                    onModelSelected={onModelSelected}
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
