import { memo, useEffect, useMemo } from 'react';
import { useFieldArray, useFormContext, useWatch, type Control, type FieldArrayPath, type FieldPath } from 'react-hook-form';
import { IconPlus, IconTrash } from '@tabler/icons-react';
import { useTranslation } from 'react-i18next';
import { Button } from '@/components/ui/button';
import { FormControl, FormField, FormItem, FormMessage } from '@/components/ui/form';
import { Label } from '@/components/ui/label';
import { Input } from '@/components/ui/input';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Separator } from '@/components/ui/separator';

const priceItemCodes = ['prompt_tokens', 'completion_tokens', 'prompt_cached_tokens', 'prompt_write_cached_tokens'] as const;
const promptWriteCacheVariantCodes = ['five_min', 'one_hour'] as const;
type PricingMode = 'flat_fee' | 'usage_per_unit' | 'usage_tiered';

type PriceEditorFormValues = {
  prices: Array<{
    modelId: string;
    price: {
      items: Array<{
        itemCode: (typeof priceItemCodes)[number];
        pricing: {
          mode: PricingMode;
          flatFee?: string | null;
          usagePerUnit?: string | null;
          usageTiered?: {
            tiers: Array<{
              upTo?: number | null;
              pricePerUnit: string;
            }>;
          } | null;
        };
        promptWriteCacheVariants?: Array<{
          variantCode: (typeof promptWriteCacheVariantCodes)[number];
          pricing: {
            mode: PricingMode;
            flatFee?: string | null;
            usagePerUnit?: string | null;
            usageTiered?: {
              tiers: Array<{
                upTo?: number | null;
                pricePerUnit: string;
              }>;
            } | null;
          };
        }> | null;
      }>;
    };
  }>;
};

function asFieldPath(path: string) {
  return path as unknown as FieldPath<PriceEditorFormValues>;
}

function asFieldArrayPath(path: string) {
  return path as unknown as FieldArrayPath<PriceEditorFormValues>;
}

function usePriceEditorWatch<TValue>(control: Control<PriceEditorFormValues>, name: string) {
  return useWatch({ control, name: asFieldPath(name) }) as unknown as TValue;
}

type PriceItem = PriceEditorFormValues['prices'][number]['price']['items'][number];
type PriceItemVariant = NonNullable<NonNullable<PriceItem['promptWriteCacheVariants']>[number]>;
type Tier = NonNullable<NonNullable<NonNullable<PriceItem['pricing']['usageTiered']>['tiers']>[number]>;

type ModelPriceEditorProps = {
  control: Control<PriceEditorFormValues>;
  priceIndex: number;
  currencyCode?: string;
  hideHeader?: boolean;
  onAddItem: (priceIndex: number) => void;
  onRemoveItem: (priceIndex: number, itemIndex: number) => void;
  onAddVariant: (priceIndex: number, itemIndex: number) => void;
  onRemoveVariant: (priceIndex: number, itemIndex: number, variantIndex: number) => void;
};

export const ModelPriceEditor = memo(function ChannelModelPriceEditor({
  control,
  priceIndex,
  currencyCode,
  hideHeader,
  onAddItem,
  onRemoveItem,
  onAddVariant,
  onRemoveVariant,
}: ModelPriceEditorProps) {
  const { t } = useTranslation();
  const { fields } = useFieldArray({
    control,
    name: asFieldArrayPath(`prices.${priceIndex}.price.items`),
  });

  return (
    <div className='min-w-0 space-y-4'>
      {!hideHeader && (
        <div className='flex h-8 items-center'>
          <Label className='text-sm font-medium'>{t('price.items')}</Label>
        </div>
      )}
      {/* <Separator /> */}
      {fields.map((field, itemIndex) => (
        <PriceItemRow
          key={field.id}
          control={control}
          priceIndex={priceIndex}
          itemIndex={itemIndex}
          itemCount={fields.length}
          currencyCode={currencyCode}
          onRemoveItem={onRemoveItem}
          onAddVariant={onAddVariant}
          onRemoveVariant={onRemoveVariant}
        />
      ))}
      <div className='grid grid-cols-1 sm:grid-cols-[minmax(0,1.2fr)_minmax(0,1fr)_minmax(0,1fr)_auto] sm:items-end'>
        <div className='sm:col-span-3'>
          <Button
            type='button'
            variant='outline'
            className='w-full'
            size='sm'
            onClick={() => onAddItem(priceIndex)}
            title={t('price.addItem')}
          >
            <IconPlus size={14} />
          </Button>
        </div>
      </div>
    </div>
  );
});

const PriceItemRow = memo(function PriceItemRow({
  control,
  priceIndex,
  itemIndex,
  itemCount,
  currencyCode,
  onRemoveItem,
  onAddVariant,
  onRemoveVariant,
}: {
  control: Control<PriceEditorFormValues>;
  priceIndex: number;
  itemIndex: number;
  itemCount: number;
  currencyCode?: string;
  onRemoveItem: (priceIndex: number, itemIndex: number) => void;
  onAddVariant: (priceIndex: number, itemIndex: number) => void;
  onRemoveVariant: (priceIndex: number, itemIndex: number, variantIndex: number) => void;
}) {
  const { t } = useTranslation();
  const itemCode = usePriceEditorWatch<PriceItem['itemCode'] | undefined>(
    control,
    `prices.${priceIndex}.price.items.${itemIndex}.itemCode`
  );
  const pricingMode = usePriceEditorWatch<PriceItem['pricing']['mode'] | undefined>(
    control,
    `prices.${priceIndex}.price.items.${itemIndex}.pricing.mode`
  );
  const items = usePriceEditorWatch<PriceItem[] | undefined>(control, `prices.${priceIndex}.price.items`);
  const { fields: variantFields } = useFieldArray({
    control,
    name: asFieldArrayPath(`prices.${priceIndex}.price.items.${itemIndex}.promptWriteCacheVariants`),
  });
  const {
    fields: tierFields,
    append: appendTier,
    remove: removeTier,
  } = useFieldArray({
    control,
    name: asFieldArrayPath(`prices.${priceIndex}.price.items.${itemIndex}.pricing.usageTiered.tiers`),
  });
  const tiers = usePriceEditorWatch<Tier[] | undefined>(
    control,
    `prices.${priceIndex}.price.items.${itemIndex}.pricing.usageTiered.tiers`
  );
  const { setValue } = useFormContext<PriceEditorFormValues>();
  const requiredMessage = t('price.validation.priceRequired');

  useEffect(() => {
    if (pricingMode === 'usage_tiered' && tierFields.length === 0) {
      appendTier({ upTo: null, pricePerUnit: '' });
    }
  }, [appendTier, pricingMode, tierFields.length]);

  useEffect(() => {
    if (!tiers?.length) return;
    const lastIndex = tiers.length - 1;
    if (tiers[lastIndex]?.upTo !== null) {
      setValue(
        asFieldPath(
          `prices.${priceIndex}.price.items.${itemIndex}.pricing.usageTiered.tiers.${lastIndex}.upTo`
        ) as FieldPath<PriceEditorFormValues>,
        null,
        { shouldDirty: true, shouldValidate: true }
      );
    }
  }, [itemIndex, priceIndex, setValue, tiers]);

  const availableItemCodes = priceItemCodes.filter((code) => {
    if (code === itemCode) return true;
    const currentItems = items || [];
    const isUsedByOther = currentItems.some((item, i) => i !== itemIndex && item.itemCode === code);
    return !isUsedByOther;
  });

  const currencyPrefix = useMemo(() => {
    if (!currencyCode) return '';
    const formatted = t('currencies.format', { val: 0, currency: currencyCode, locale: 'en-US' });
    // Remove numbers, separators, spaces and any uppercase letters (country codes like HK, CN)
    return formatted.replace(/[0.,\s]+/g, '').replace(/[A-Z]/g, '');
  }, [currencyCode, t]);

  return (
    <div className='min-w-0 space-y-4'>
      <div className='grid grid-cols-1 gap-x-4 gap-y-2 sm:grid-cols-[minmax(0,1.2fr)_minmax(0,1fr)_minmax(0,1fr)_auto] sm:items-start'>
        <div className='min-w-0 pt-0 sm:pt-0'>
          <FormField
            control={control}
            name={asFieldPath(`prices.${priceIndex}.price.items.${itemIndex}.itemCode`)}
            render={({ field }) => (
              <FormItem>
                <Select onValueChange={field.onChange} value={field.value as unknown as string | undefined}>
                  <FormControl>
                    <SelectTrigger size='sm' className='h-8 w-full min-w-0'>
                      <SelectValue placeholder={t('price.itemCode')} className='truncate' />
                    </SelectTrigger>
                  </FormControl>
                  <SelectContent>
                    {availableItemCodes.map((code) => (
                      <SelectItem key={code} value={code}>
                        {t(`price.itemCodes.${code}`, { defaultValue: code })}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
                <FormMessage className='text-[10px]' />
              </FormItem>
            )}
          />
        </div>
        <div className='min-w-0'>
          <FormField
            control={control}
            name={asFieldPath(`prices.${priceIndex}.price.items.${itemIndex}.pricing.mode`)}
            render={({ field }) => (
              <FormItem>
                <Select onValueChange={field.onChange} value={field.value as unknown as string | undefined}>
                  <FormControl>
                    <SelectTrigger size='sm' className='h-8 w-full min-w-0'>
                      <SelectValue placeholder={t('price.mode')} className='truncate' />
                    </SelectTrigger>
                  </FormControl>
                  <SelectContent>
                    <SelectItem value='flat_fee'>{t('price.mode_flat_fee')}</SelectItem>
                    <SelectItem value='usage_per_unit'>{t('price.mode_usage_per_unit')}</SelectItem>
                    <SelectItem value='usage_tiered'>{t('price.mode_usage_tiered')}</SelectItem>
                  </SelectContent>
                </Select>
              </FormItem>
            )}
          />
        </div>
        <div className='min-w-0'>
          {pricingMode === 'usage_per_unit' && (
            <FormField
              control={control}
              name={asFieldPath(`prices.${priceIndex}.price.items.${itemIndex}.pricing.usagePerUnit`)}
              render={({ field }) => (
                <FormItem className='relative'>
                  <FormControl>
                    <div className='relative'>
                      {currencyPrefix && (
                        <span className='text-muted-foreground absolute top-1/2 left-2 -translate-y-1/2 text-xs'>{currencyPrefix}</span>
                      )}
                      <Input
                        {...field}
                        value={(field.value as unknown as string | null | undefined) || ''}
                        placeholder='0.00'
                        className={`h-8 text-right ${currencyPrefix ? 'pl-8' : ''}`}
                      />
                    </div>
                  </FormControl>
                  <FormMessage className='absolute -bottom-4 left-0 text-[10px]' />
                </FormItem>
              )}
              rules={{ required: requiredMessage }}
            />
          )}
          {pricingMode === 'flat_fee' && (
            <FormField
              control={control}
              name={asFieldPath(`prices.${priceIndex}.price.items.${itemIndex}.pricing.flatFee`)}
              render={({ field }) => (
                <FormItem className='relative'>
                  <FormControl>
                    <div className='relative'>
                      {currencyPrefix && (
                        <span className='text-muted-foreground absolute top-1/2 left-2 -translate-y-1/2 text-xs'>{currencyPrefix}</span>
                      )}
                      <Input
                        {...field}
                        value={(field.value as unknown as string | null | undefined) || ''}
                        placeholder='0.00'
                        className={`h-8 text-right ${currencyPrefix ? 'pl-8' : ''}`}
                      />
                    </div>
                  </FormControl>
                  <FormMessage className='absolute -bottom-4 left-0 text-[10px]' />
                </FormItem>
              )}
              rules={{ required: requiredMessage }}
            />
          )}
        </div>
        <div className='flex justify-end'>
          <Button
            type='button'
            variant='ghost'
            size='icon-sm'
            className='text-destructive'
            disabled={itemCount <= 1}
            onClick={() => onRemoveItem(priceIndex, itemIndex)}
          >
            <IconTrash size={14} />
          </Button>
        </div>

        {pricingMode === 'usage_tiered' && (
          <div className='col-span-full ml-4 mt-2 min-w-0 space-y-2 rounded-md border border-dashed p-3'>
            <div className='text-muted-foreground flex items-center justify-between text-xs'>
              <span>{t('price.tiers')}</span>
              <Button type='button' variant='outline' size='icon-sm' onClick={() => appendTier({ upTo: null, pricePerUnit: '0' })}>
                <IconPlus size={14} />
              </Button>
            </div>
            {tierFields.map((field, tierIndex) => (
              <div key={field.id} className='flex items-center gap-2'>
                <FormField
                  control={control}
                  name={asFieldPath(`prices.${priceIndex}.price.items.${itemIndex}.pricing.usageTiered.tiers.${tierIndex}.upTo`)}
                  render={({ field }) => {
                    const isLastTier = tierIndex === tierFields.length - 1;
                    return (
                      <FormItem className='flex-1'>
                        <FormControl>
                          <Input
                            type='number'
                            {...field}
                            value={isLastTier ? '' : (field.value as unknown as number | null | undefined) ?? ''}
                            onChange={(e) =>
                              isLastTier
                                ? field.onChange(null)
                                : field.onChange(e.target.value ? parseInt(e.target.value) : null)
                            }
                            placeholder={isLastTier ? '∞' : t('price.upTo')}
                            disabled={isLastTier}
                            className='h-7 text-xs'
                          />
                        </FormControl>
                        {!isLastTier && <FormMessage className='text-[10px]' />}
                      </FormItem>
                    );
                  }}
                  rules={{
                    validate: (val) => {
                      const isLastTier = tierIndex === tierFields.length - 1;
                      if (isLastTier) return true;
                      return typeof val === 'number' ? true : requiredMessage;
                    },
                  }}
                />
                <FormField
                  control={control}
                  name={asFieldPath(`prices.${priceIndex}.price.items.${itemIndex}.pricing.usageTiered.tiers.${tierIndex}.pricePerUnit`)}
                  render={({ field }) => (
                    <FormItem className='flex-1'>
                      <FormControl>
                        <div className='relative'>
                          {currencyPrefix && (
                            <span className='text-muted-foreground absolute top-1/2 left-2 -translate-y-1/2 text-[10px]'>
                              {currencyPrefix}
                            </span>
                          )}
                          <Input
                            {...field}
                            value={(field.value as unknown as string | null | undefined) || ''}
                            placeholder={t('price.pricePerUnit')}
                            className={`h-7 text-right text-xs ${currencyPrefix ? 'pl-7' : ''}`}
                          />
                        </div>
                      </FormControl>
                      <FormMessage className='text-[10px]' />
                    </FormItem>
                  )}
                  rules={{ required: requiredMessage }}
                />
                <Button type='button' variant='ghost' size='icon-sm' onClick={() => removeTier(tierIndex)}>
                  <IconTrash size={14} className='text-destructive' />
                </Button>
              </div>
            ))}
          </div>
        )}
      </div>

      {itemCode === 'prompt_write_cached_tokens' && (
        <div className='ml-4 min-w-0 space-y-2 overflow-hidden rounded-md border border-dashed p-3'>
          <div className='text-muted-foreground flex items-center justify-between text-xs'>
            <span>{t('price.promptWriteCacheVariants')}</span>
            <Button
              type='button'
              variant='outline'
              size='icon-sm'
              disabled={variantFields.length >= promptWriteCacheVariantCodes.length}
              onClick={() => onAddVariant(priceIndex, itemIndex)}
              title={t('price.addVariant')}
            >
              <IconPlus size={14} />
            </Button>
          </div>
          {variantFields.map((field, variantIndex) => (
            <PriceVariantRow
              key={field.id}
              control={control}
              priceIndex={priceIndex}
              itemIndex={itemIndex}
              variantIndex={variantIndex}
              currencyCode={currencyCode}
              onRemoveVariant={onRemoveVariant}
            />
          ))}
        </div>
      )}

      {itemIndex < itemCount - 1 && <Separator className='opacity-50' />}
    </div>
  );
});

const PriceVariantRow = memo(function PriceVariantRow({
  control,
  priceIndex,
  itemIndex,
  variantIndex,
  currencyCode,
  onRemoveVariant,
}: {
  control: Control<PriceEditorFormValues>;
  priceIndex: number;
  itemIndex: number;
  variantIndex: number;
  currencyCode?: string;
  onRemoveVariant: (priceIndex: number, itemIndex: number, variantIndex: number) => void;
}) {
  const { t } = useTranslation();
  const pricingMode = usePriceEditorWatch<PriceItemVariant['pricing']['mode'] | undefined>(
    control,
    `prices.${priceIndex}.price.items.${itemIndex}.promptWriteCacheVariants.${variantIndex}.pricing.mode`
  );
  const variantCode = usePriceEditorWatch<PriceItemVariant['variantCode'] | undefined>(
    control,
    `prices.${priceIndex}.price.items.${itemIndex}.promptWriteCacheVariants.${variantIndex}.variantCode`
  );
  const watchedVariants = usePriceEditorWatch<PriceItemVariant[] | null | undefined>(
    control,
    `prices.${priceIndex}.price.items.${itemIndex}.promptWriteCacheVariants`
  );
  const {
    fields: tierFields,
    append: appendTier,
    remove: removeTier,
  } = useFieldArray({
    control,
    name: asFieldArrayPath(
      `prices.${priceIndex}.price.items.${itemIndex}.promptWriteCacheVariants.${variantIndex}.pricing.usageTiered.tiers`
    ),
  });
  const tiers = usePriceEditorWatch<Tier[] | undefined>(
    control,
    `prices.${priceIndex}.price.items.${itemIndex}.promptWriteCacheVariants.${variantIndex}.pricing.usageTiered.tiers`
  );
  const { setValue } = useFormContext<PriceEditorFormValues>();
  const requiredMessage = t('price.validation.priceRequired');

  const availableVariantCodes = promptWriteCacheVariantCodes.filter((code) => {
    if (code === variantCode) return true;
    const variants = watchedVariants || [];
    const isUsedByOther = variants.some((variant, i) => i !== variantIndex && variant.variantCode === code);
    return !isUsedByOther;
  });

  const currencyPrefix = useMemo(() => {
    if (!currencyCode) return '';
    const formatted = t('currencies.format', { val: 0, currency: currencyCode, locale: 'en-US' });
    // Remove numbers, separators, spaces and any uppercase letters (country codes like HK, CN)
    return formatted.replace(/[0.,\s]+/g, '').replace(/[A-Z]/g, '');
  }, [currencyCode, t]);

  // Initialize tiers for usage_tiered mode
  useEffect(() => {
    if (pricingMode === 'usage_tiered' && tierFields.length === 0) {
      appendTier({ upTo: null, pricePerUnit: '' });
    }
  }, [appendTier, pricingMode, tierFields.length]);

  // Ensure last tier's upTo is always null
  useEffect(() => {
    if (!tiers?.length) return;
    const lastIndex = tiers.length - 1;
    if (tiers[lastIndex]?.upTo !== null) {
      setValue(
        asFieldPath(
          `prices.${priceIndex}.price.items.${itemIndex}.promptWriteCacheVariants.${variantIndex}.pricing.usageTiered.tiers.${lastIndex}.upTo`
        ) as FieldPath<PriceEditorFormValues>,
        null,
        { shouldDirty: true, shouldValidate: true }
      );
    }
  }, [itemIndex, priceIndex, setValue, tiers, variantIndex]);

  return (
    <div className='space-y-2'>
      <div className='flex items-center gap-2'>
        <FormField
          control={control}
          name={asFieldPath(`prices.${priceIndex}.price.items.${itemIndex}.promptWriteCacheVariants.${variantIndex}.variantCode`)}
          render={({ field }) => (
            <FormItem className='min-w-0 flex-1'>
              <Select onValueChange={field.onChange} value={field.value as unknown as string | undefined}>
                <FormControl>
                  <SelectTrigger size='sm' className='h-7 text-xs'>
                    <SelectValue placeholder={t('price.variantCode')} />
                  </SelectTrigger>
                </FormControl>
                <SelectContent>
                  {availableVariantCodes.map((code) => (
                    <SelectItem key={code} value={code}>
                      {t(`price.variantCodes.${code}`, { defaultValue: code })}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
              <FormMessage className='text-[10px]' />
            </FormItem>
          )}
        />
        <FormField
          control={control}
          name={asFieldPath(`prices.${priceIndex}.price.items.${itemIndex}.promptWriteCacheVariants.${variantIndex}.pricing.mode`)}
          render={({ field }) => (
            <FormItem className='min-w-0 flex-1'>
              <Select onValueChange={field.onChange} value={field.value as unknown as string | undefined}>
                <FormControl>
                  <SelectTrigger size='sm' className='h-7 text-xs'>
                    <SelectValue placeholder={t('price.mode')} />
                  </SelectTrigger>
                </FormControl>
                <SelectContent>
                  <SelectItem value='flat_fee'>{t('price.mode_flat_fee')}</SelectItem>
                  <SelectItem value='usage_per_unit'>{t('price.mode_usage_per_unit')}</SelectItem>
                  <SelectItem value='usage_tiered'>{t('price.mode_usage_tiered')}</SelectItem>
                </SelectContent>
              </Select>
            </FormItem>
          )}
        />
        {pricingMode === 'usage_per_unit' && (
          <FormField
            control={control}
            name={asFieldPath(
              `prices.${priceIndex}.price.items.${itemIndex}.promptWriteCacheVariants.${variantIndex}.pricing.usagePerUnit`
            )}
            render={({ field }) => (
              <FormItem className='min-w-0 flex-1'>
                <FormControl>
                  <div className='relative'>
                    {currencyPrefix && (
                      <span className='text-muted-foreground absolute top-1/2 left-2 -translate-y-1/2 text-[10px]'>{currencyPrefix}</span>
                    )}
                    <Input
                      {...field}
                      value={(field.value as unknown as string | null | undefined) || ''}
                      placeholder='0.00'
                      className={`h-7 text-right text-xs ${currencyPrefix ? 'pl-7' : ''}`}
                    />
                  </div>
                </FormControl>
                <FormMessage className='text-[10px]' />
              </FormItem>
            )}
            rules={{ required: requiredMessage }}
          />
        )}
        {pricingMode === 'flat_fee' && (
          <FormField
            control={control}
            name={asFieldPath(
              `prices.${priceIndex}.price.items.${itemIndex}.promptWriteCacheVariants.${variantIndex}.pricing.flatFee`
            )}
            render={({ field }) => (
              <FormItem className='min-w-0 flex-1'>
                <FormControl>
                  <div className='relative'>
                    {currencyPrefix && (
                      <span className='text-muted-foreground absolute top-1/2 left-2 -translate-y-1/2 text-[10px]'>{currencyPrefix}</span>
                    )}
                    <Input
                      {...field}
                      value={(field.value as unknown as string | null | undefined) || ''}
                      placeholder='0.00'
                      className={`h-7 text-right text-xs ${currencyPrefix ? 'pl-7' : ''}`}
                    />
                  </div>
                </FormControl>
                <FormMessage className='text-[10px]' />
              </FormItem>
            )}
            rules={{ required: requiredMessage }}
          />
        )}
        <Button type='button' variant='ghost' size='icon-sm' onClick={() => onRemoveVariant(priceIndex, itemIndex, variantIndex)}>
          <IconTrash size={14} className='text-destructive' />
        </Button>
      </div>

      {pricingMode === 'usage_tiered' && (
        <div className='ml-4 space-y-2 rounded-md border border-dashed p-2'>
          <div className='text-muted-foreground flex items-center justify-between text-[10px]'>
            <span>{t('price.tiers')}</span>
            <Button
              type='button'
              variant='outline'
              size='icon-sm'
              className='h-5 w-5'
              onClick={() => appendTier({ upTo: null, pricePerUnit: '0' })}
            >
              <IconPlus size={10} />
            </Button>
          </div>
          {tierFields.map((field, tierIndex) => (
            <div key={field.id} className='flex items-center gap-1'>
              <FormField
                control={control}
                name={asFieldPath(
                  `prices.${priceIndex}.price.items.${itemIndex}.promptWriteCacheVariants.${variantIndex}.pricing.usageTiered.tiers.${tierIndex}.upTo`
                )}
                render={({ field }) => {
                  const isLastTier = tierIndex === tierFields.length - 1;
                  return (
                    <FormItem className='flex-1'>
                      <FormControl>
                        <Input
                          type='number'
                          {...field}
                          value={isLastTier ? '' : (field.value as unknown as number | null | undefined) ?? ''}
                          onChange={(e) =>
                            isLastTier
                              ? field.onChange(null)
                              : field.onChange(e.target.value ? parseInt(e.target.value) : null)
                          }
                          placeholder={isLastTier ? '∞' : t('price.upTo')}
                          disabled={isLastTier}
                          className='h-6 text-[10px]'
                        />
                      </FormControl>
                      {!isLastTier && <FormMessage className='text-[10px]' />}
                    </FormItem>
                  );
                }}
                rules={{
                  validate: (val) => {
                    const isLastTier = tierIndex === tierFields.length - 1;
                    if (isLastTier) return true;
                    return typeof val === 'number' ? true : requiredMessage;
                  },
                }}
              />
              <FormField
                control={control}
                name={asFieldPath(
                  `prices.${priceIndex}.price.items.${itemIndex}.promptWriteCacheVariants.${variantIndex}.pricing.usageTiered.tiers.${tierIndex}.pricePerUnit`
                )}
                render={({ field }) => (
                  <FormItem className='flex-1'>
                    <FormControl>
                      <div className='relative'>
                        {currencyPrefix && (
                          <span className='text-muted-foreground absolute top-1/2 left-1.5 -translate-y-1/2 text-[9px]'>
                            {currencyPrefix}
                          </span>
                        )}
                        <Input
                          {...field}
                          value={(field.value as unknown as string | null | undefined) || ''}
                          placeholder={t('price.pricePerUnit')}
                          className={`h-6 text-right text-[10px] ${currencyPrefix ? 'pl-6' : ''}`}
                        />
                      </div>
                    </FormControl>
                    <FormMessage className='text-[10px]' />
                  </FormItem>
                )}
                rules={{ required: requiredMessage }}
              />
              <Button
                type='button'
                variant='ghost'
                size='icon-sm'
                onClick={() => removeTier(tierIndex)}
              >
                <IconTrash size={10} className='text-destructive' />
              </Button>
            </div>
          ))}
        </div>
      )}
    </div>
  );
});
