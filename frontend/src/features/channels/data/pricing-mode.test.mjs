import assert from 'node:assert/strict';
import test from 'node:test';
import { loadChannelSchema } from './load-channel-schema.mjs';

const { channelModelPriceSchema } = await loadChannelSchema();

const usagePricing = { mode: 'usage_per_unit', usagePerUnit: '1' };
const fiveMinVariant = { variantCode: 'five_min', pricing: { mode: 'usage_per_unit', usagePerUnit: '2.5' } };
const oneHourVariant = { variantCode: 'one_hour', pricing: { mode: 'usage_per_unit', usagePerUnit: '3.5' } };

function priceWithWriteItem(writeItem) {
  return {
    id: '1',
    modelID: 'mixed-ttl',
    price: {
      items: [{ itemCode: 'prompt_tokens', pricing: usagePricing }, writeItem],
    },
  };
}

test('variant-only write-cache item parses with empty mode', () => {
  const price = priceWithWriteItem({
    itemCode: 'prompt_write_cached_tokens',
    pricing: { mode: '' },
    promptWriteCacheVariants: [fiveMinVariant, oneHourVariant],
  });

  const parsed = channelModelPriceSchema.parse(price);
  assert.equal(parsed.price.items[1].pricing.mode, '');
  assert.equal(parsed.price.items[1].promptWriteCacheVariants.length, 2);
});

test('variant-only write-cache item parses without mode key (undefined)', () => {
  const price = priceWithWriteItem({
    itemCode: 'prompt_write_cached_tokens',
    pricing: {},
    promptWriteCacheVariants: [fiveMinVariant],
  });

  channelModelPriceSchema.parse(price);
});

test('write-cache item with empty mode and no variants is rejected', () => {
  const price = priceWithWriteItem({
    itemCode: 'prompt_write_cached_tokens',
    pricing: { mode: '' },
  });

  assert.throws(() => channelModelPriceSchema.parse(price), /mode is required/);
});

test('non-write-cache item with empty mode is rejected', () => {
  const price = {
    id: '1',
    modelID: 'bad-model',
    price: { items: [{ itemCode: 'prompt_tokens', pricing: { mode: '' } }] },
  };

  assert.throws(() => channelModelPriceSchema.parse(price), /mode is required/);
});

test('empty mode on a variant pricing is rejected', () => {
  const price = priceWithWriteItem({
    itemCode: 'prompt_write_cached_tokens',
    pricing: { mode: '' },
    promptWriteCacheVariants: [{ variantCode: 'five_min', pricing: { mode: '' } }, oneHourVariant],
  });

  const result = channelModelPriceSchema.safeParse(price);
  assert.equal(result.success, false);
  const issuePaths = result.error.issues.map((issue) => issue.path.join('.'));
  assert.ok(
    issuePaths.some((p) => p.endsWith('promptWriteCacheVariants.0.pricing.mode')),
    JSON.stringify(issuePaths)
  );
});

test('normal priced items still parse and keep their mode', () => {
  const price = priceWithWriteItem({
    itemCode: 'prompt_write_cached_tokens',
    pricing: { mode: 'usage_per_unit', usagePerUnit: '4' },
    promptWriteCacheVariants: [fiveMinVariant],
  });

  const parsed = channelModelPriceSchema.parse(price);
  assert.equal(parsed.price.items[1].pricing.mode, 'usage_per_unit');
});
