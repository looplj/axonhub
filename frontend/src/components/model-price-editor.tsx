import { memo } from 'react';
import { useFieldArray, useWatch, type Control, type FieldPath } from 'react-hook-form';
import { IconPlus, IconTrash } from '@tabler/icons-react';
import { useTranslation } from 'react-i18next';
import { Button } from '@/components/ui/button';
import { FormControl, FormField, FormItem, FormMessage } from '@/components/ui/form';
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
  return path as any;
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
  onAddItem: (priceIndex: number) => void;
  onRemoveItem: (priceIndex: number, itemIndex: number) => void;
  onAddVariant: (priceIndex: number, itemIndex: number) => void;
  onRemoveVariant: (priceIndex: number, itemIndex: number, variantIndex: number) => void;
  onAddTier: (priceIndex: number, itemIndex: number) => void;
  onRemoveTier: (priceIndex: number, itemIndex: number, tierIndex: number) => void;
  onAddVariantTier: (priceIndex: number, itemIndex: number, variantIndex: number) => void;
  onRemoveVariantTier: (priceIndex: number, itemIndex: number, variantIndex: number, tierIndex: number) => void;
};

export const ModelPriceEditor = memo(function ChannelModelPriceEditor({
  control,
  priceIndex,
  onAddItem,
  onRemoveItem,
  onAddVariant,
  onRemoveVariant,
  onAddTier,
  onRemoveTier,
  onAddVariantTier,
  onRemoveVariantTier,
}: ModelPriceEditorProps) {
  const { t } = useTranslation();
  const { fields } = useFieldArray({
    control,
    name: asFieldPath(`prices.${priceIndex}.price.items`),
  });

  return (
    <div className='space-y-4 md:col-span-3'>
      <div className='flex items-center justify-between'>
        <span className='text-sm font-medium'>{t('price.items')}</span>
        <Button type='button' variant='outline' size='icon-sm' onClick={() => onAddItem(priceIndex)} title={t('price.addItem')}>
          <IconPlus size={14} />
        </Button>
      </div>
      <Separator />
      {fields.map((field, itemIndex) => (
        <PriceItemRow
          key={field.id}
          control={control}
          priceIndex={priceIndex}
          itemIndex={itemIndex}
          itemCount={fields.length}
          onRemoveItem={onRemoveItem}
          onAddVariant={onAddVariant}
          onRemoveVariant={onRemoveVariant}
          onAddTier={onAddTier}
          onRemoveTier={onRemoveTier}
          onAddVariantTier={onAddVariantTier}
          onRemoveVariantTier={onRemoveVariantTier}
        />
      ))}
    </div>
  );
});

const PriceItemRow = memo(function PriceItemRow({
  control,
  priceIndex,
  itemIndex,
  itemCount,
  onRemoveItem,
  onAddVariant,
  onRemoveVariant,
  onAddTier,
  onRemoveTier,
  onAddVariantTier,
  onRemoveVariantTier,
}: {
  control: Control<PriceEditorFormValues>;
  priceIndex: number;
  itemIndex: number;
  itemCount: number;
  onRemoveItem: (priceIndex: number, itemIndex: number) => void;
  onAddVariant: (priceIndex: number, itemIndex: number) => void;
  onRemoveVariant: (priceIndex: number, itemIndex: number, variantIndex: number) => void;
  onAddTier: (priceIndex: number, itemIndex: number) => void;
  onRemoveTier: (priceIndex: number, itemIndex: number, tierIndex: number) => void;
  onAddVariantTier: (priceIndex: number, itemIndex: number, variantIndex: number) => void;
  onRemoveVariantTier: (priceIndex: number, itemIndex: number, variantIndex: number, tierIndex: number) => void;
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
  const { fields: variantFields } = useFieldArray({
    control,
    name: asFieldPath(`prices.${priceIndex}.price.items.${itemIndex}.promptWriteCacheVariants`),
  });
  const { fields: tierFields } = useFieldArray({
    control,
    name: asFieldPath(`prices.${priceIndex}.price.items.${itemIndex}.pricing.usageTiered.tiers`),
  });

  const availableItemCodes = priceItemCodes.filter((code) => {
    if (code === itemCode) return true;
    const items = (control._formValues as PriceEditorFormValues).prices[priceIndex].price.items;
    const isUsedByOther = items.some((item, i) => i !== itemIndex && item.itemCode === code);
    return !isUsedByOther;
  });

  return (
    <div className='space-y-4'>
      <div className='grid grid-cols-1 items-end gap-4 sm:grid-cols-4'>
        <div className='sm:col-span-1'>
          <FormField
            control={control}
            name={asFieldPath(`prices.${priceIndex}.price.items.${itemIndex}.itemCode`)}
            render={({ field }) => (
              <FormItem>
                <Select onValueChange={field.onChange} value={field.value as unknown as string | undefined}>
                  <FormControl>
                    <SelectTrigger size='sm' className='h-8'>
                      <SelectValue placeholder={t('price.itemCode')} />
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
        <div className='sm:col-span-1'>
          <FormField
            control={control}
            name={asFieldPath(`prices.${priceIndex}.price.items.${itemIndex}.pricing.mode`)}
            render={({ field }) => (
              <FormItem>
                <Select onValueChange={field.onChange} value={field.value as unknown as string | undefined}>
                  <FormControl>
                    <SelectTrigger size='sm' className='h-8'>
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
        </div>
        <div className='sm:col-span-1'>
          {pricingMode === 'usage_per_unit' && (
            <FormField
              control={control}
              name={asFieldPath(`prices.${priceIndex}.price.items.${itemIndex}.pricing.usagePerUnit`)}
              render={({ field }) => (
                <FormItem>
                  <FormControl>
                    <Input
                      {...field}
                      value={(field.value as unknown as string | null | undefined) || ''}
                      placeholder='0.00'
                      className='h-8'
                    />
                  </FormControl>
                </FormItem>
              )}
            />
          )}
          {pricingMode === 'flat_fee' && (
            <FormField
              control={control}
              name={asFieldPath(`prices.${priceIndex}.price.items.${itemIndex}.pricing.flatFee`)}
              render={({ field }) => (
                <FormItem>
                  <FormControl>
                    <Input
                      {...field}
                      value={(field.value as unknown as string | null | undefined) || ''}
                      placeholder='0.00'
                      className='h-8'
                    />
                  </FormControl>
                </FormItem>
              )}
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
          <div className='col-span-4 mt-2 space-y-2 rounded-md border border-dashed p-3'>
            <div className='text-muted-foreground flex items-center justify-between text-xs'>
              <span>{t('price.tiers')}</span>
              <Button type='button' variant='outline' size='icon-sm' onClick={() => onAddTier(priceIndex, itemIndex)}>
                <IconPlus size={14} />
              </Button>
            </div>
            {tierFields.map((field, tierIndex) => (
              <div key={field.id} className='flex items-center gap-2'>
                <FormField
                  control={control}
                  name={asFieldPath(`prices.${priceIndex}.price.items.${itemIndex}.pricing.usageTiered.tiers.${tierIndex}.upTo`)}
                  render={({ field }) => (
                    <FormItem className='flex-1'>
                      <FormControl>
                        <Input
                          type='number'
                          {...field}
                          value={(field.value as unknown as number | null | undefined) ?? ''}
                          onChange={(e) => field.onChange(e.target.value ? parseInt(e.target.value) : null)}
                          placeholder={t('price.upTo')}
                          className='h-7 text-xs'
                        />
                      </FormControl>
                    </FormItem>
                  )}
                />
                <FormField
                  control={control}
                  name={asFieldPath(`prices.${priceIndex}.price.items.${itemIndex}.pricing.usageTiered.tiers.${tierIndex}.pricePerUnit`)}
                  render={({ field }) => (
                    <FormItem className='flex-1'>
                      <FormControl>
                        <Input
                          {...field}
                          value={(field.value as unknown as string | null | undefined) || ''}
                          placeholder={t('price.pricePerUnit')}
                          className='h-7 text-xs'
                        />
                      </FormControl>
                    </FormItem>
                  )}
                />
                <Button type='button' variant='ghost' size='icon-sm' onClick={() => onRemoveTier(priceIndex, itemIndex, tierIndex)}>
                  <IconTrash size={14} className='text-destructive' />
                </Button>
              </div>
            ))}
          </div>
        )}
      </div>

      {itemCode === 'prompt_write_cached_tokens' && (
        <div className='ml-4 space-y-2 rounded-md border border-dashed p-3'>
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
              onRemoveVariant={onRemoveVariant}
              onAddTier={onAddVariantTier}
              onRemoveTier={onRemoveVariantTier}
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
  onRemoveVariant,
  onAddTier,
  onRemoveTier,
}: {
  control: Control<PriceEditorFormValues>;
  priceIndex: number;
  itemIndex: number;
  variantIndex: number;
  onRemoveVariant: (priceIndex: number, itemIndex: number, variantIndex: number) => void;
  onAddTier: (priceIndex: number, itemIndex: number, variantIndex: number) => void;
  onRemoveTier: (priceIndex: number, itemIndex: number, variantIndex: number, tierIndex: number) => void;
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
  const { fields: tierFields } = useFieldArray({
    control,
    name: asFieldPath(`prices.${priceIndex}.price.items.${itemIndex}.promptWriteCacheVariants.${variantIndex}.pricing.usageTiered.tiers`),
  });

  const availableVariantCodes = promptWriteCacheVariantCodes.filter((code) => {
    if (code === variantCode) return true;
    const variants = (control._formValues as PriceEditorFormValues).prices[priceIndex].price.items[itemIndex].promptWriteCacheVariants || [];
    const isUsedByOther = variants.some((variant, i) => i !== variantIndex && variant.variantCode === code);
    return !isUsedByOther;
  });

  return (
    <div className='space-y-2 rounded-md border p-2'>
      <div className='flex items-center gap-2'>
        <FormField
          control={control}
          name={asFieldPath(`prices.${priceIndex}.price.items.${itemIndex}.promptWriteCacheVariants.${variantIndex}.variantCode`)}
          render={({ field }) => (
            <FormItem className='flex-1'>
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
            <FormItem className='w-32'>
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
        <Button type='button' variant='ghost' size='icon-sm' onClick={() => onRemoveVariant(priceIndex, itemIndex, variantIndex)}>
          <IconTrash size={14} className='text-destructive' />
        </Button>
      </div>
      <div className='flex items-center gap-2'>
        {pricingMode === 'usage_per_unit' && (
          <FormField
            control={control}
            name={asFieldPath(
              `prices.${priceIndex}.price.items.${itemIndex}.promptWriteCacheVariants.${variantIndex}.pricing.usagePerUnit`
            )}
            render={({ field }) => (
              <FormItem className='flex-1'>
                <FormControl>
                  <Input
                    {...field}
                    value={(field.value as unknown as string | null | undefined) || ''}
                    placeholder='0.00'
                    className='h-7 text-xs'
                  />
                </FormControl>
              </FormItem>
            )}
          />
        )}
        {pricingMode === 'flat_fee' && (
          <FormField
            control={control}
            name={asFieldPath(`prices.${priceIndex}.price.items.${itemIndex}.promptWriteCacheVariants.${variantIndex}.pricing.flatFee`)}
            render={({ field }) => (
              <FormItem className='flex-1'>
                <FormControl>
                  <Input
                    {...field}
                    value={(field.value as unknown as string | null | undefined) || ''}
                    placeholder='0.00'
                    className='h-7 text-xs'
                  />
                </FormControl>
              </FormItem>
            )}
          />
        )}
      </div>

      {pricingMode === 'usage_tiered' && (
        <div className='mt-2 space-y-2 rounded-md border border-dashed p-2'>
          <div className='text-muted-foreground flex items-center justify-between text-[10px]'>
            <span>{t('price.tiers')}</span>
            <Button type='button' variant='outline' size='icon-sm' className="h-5 w-5" onClick={() => onAddTier(priceIndex, itemIndex, variantIndex)}>
              <IconPlus size={10} />
            </Button>
          </div>
          {tierFields.map((field, tierIndex) => (
            <div key={field.id} className='flex items-center gap-1'>
              <FormField
                control={control}
                name={asFieldPath(`prices.${priceIndex}.price.items.${itemIndex}.promptWriteCacheVariants.${variantIndex}.pricing.usageTiered.tiers.${tierIndex}.upTo`)}
                render={({ field }) => (
                  <FormItem className='flex-1'>
                    <FormControl>
                      <Input
                        type='number'
                        {...field}
                        value={(field.value as unknown as number | null | undefined) ?? ''}
                        onChange={(e) => field.onChange(e.target.value ? parseInt(e.target.value) : null)}
                        placeholder={t('price.upTo')}
                        className='h-6 text-[10px]'
                      />
                    </FormControl>
                  </FormItem>
                )}
              />
              <FormField
                control={control}
                name={asFieldPath(`prices.${priceIndex}.price.items.${itemIndex}.promptWriteCacheVariants.${variantIndex}.pricing.usageTiered.tiers.${tierIndex}.pricePerUnit`)}
                render={({ field }) => (
                  <FormItem className='flex-1'>
                    <FormControl>
                      <Input
                        {...field}
                        value={(field.value as unknown as string | null | undefined) || ''}
                        placeholder={t('price.pricePerUnit')}
                        className='h-6 text-[10px]'
                      />
                    </FormControl>
                  </FormItem>
                )}
              />
              <Button type='button' variant='ghost' size='icon-sm' className="h-5 w-5" onClick={() => onRemoveTier(priceIndex, itemIndex, variantIndex, tierIndex)}>
                <IconTrash size={10} className='text-destructive' />
              </Button>
            </div>
          ))}
        </div>
      )}
    </div>
  );
});
