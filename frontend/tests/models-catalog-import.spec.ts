// 职责: 验证批量导入的分项报价来源、缺价提示和跨页选择在刷新后的行为。
// 关联文件:
//   frontend/src/features/models/components/models-catalog-import.tsx
//   frontend/src/features/models/components/models-catalog-prices.tsx
//   frontend/src/features/models/data/catalog-import.ts
// 注意事项:
//   导入提交使用隔离响应。
import { expect, test } from '@playwright/test';
import { gotoAndEnsureAuth } from './auth.utils';

test.beforeEach(async ({ page }) => {
  await page.route('**/admin/graphql', async (route) => {
    if (route.request().postDataJSON().operationName !== 'OnboardingInfo') return route.fallback();
    const onboardingInfo = { onboarded: true, systemModelSetting: { onboarded: true }, autoDisableChannel: { onboarded: true } };
    return route.fulfill({ json: { data: { onboardingInfo } } });
  });
});

test('a filtered batch keeps same-source quotes and resets when the catalog source changes', async ({ page }) => {
  let inputPrice = 1;
  let upstreamURL = 'https://a.test/catalog';
  let source = 'upstream';
  let submittedRevision: string | undefined;
  const provider = 'model-test';
  let reads = 0;
  type Selection = { prices: { field: string; action: string; expected?: number; tier?: string }[] };
  let applied: Selection[] = [];
  await page.route('**/admin/graphql', async (route) => {
    const body = route.request().postDataJSON();
    if (body.operationName === 'ApplyCatalogImport') {
      applied = body.variables.selections;
      submittedRevision = body.variables.revision;
      const outcomes = body.variables.selections.map(({ modelID }: { modelID: string }) => ({ modelID, status: 'catalogChanged' }));
      return route.fulfill({ json: { data: { applyCatalogImport: outcomes } } });
    }
    if (body.operationName !== 'CatalogImportPreview') return route.fallback();
    reads++;
    const rows = Array.from({ length: 28 }, (_, i) => {
      const modelID = i < 27 ? 'batch-' + i : 'outside';
      const offer = {
        provider, modelID, name: modelID, developer: provider, identity: 'original', type: 'chat',
        rates: [
          { field: 'input', base: inputPrice, tiers: [] },
          { field: 'cacheWrite', base: null, tiers: [{ label: 'context 256000', value: 2.5 }] },
        ],
      };
      return {
        modelID, name: modelID, members: [{id:modelID,identity:'original'}], routes: [], version: modelID === 'outside' ? 'saved' : '',
        mergeTarget: '', mergeTargetOccupied: false, changed: false, archived: false, offers: [offer], preferred: offer, prices: [],
      };
    });
    const preview = { providers: [provider], source, upstreamURL, revision: source + upstreamURL, rows };
    return route.fulfill({ json: { data: { catalogImportPreview: preview } } });
  });
  await gotoAndEnsureAuth(page, '/models');
  await page.getByTestId('catalog-import-open').click();
  const dialog = page.getByRole('dialog');
  await dialog.getByRole('textbox').fill('batch-');
  await expect(dialog.getByText(/Models: 27|共 27 个模型/)).toBeVisible();
  await dialog.getByTestId('catalog-import-select-all').click();
  await expect(dialog.getByTestId('catalog-import-apply')).toContainText('27');
  await dialog.getByTestId('catalog-import-expand-batch-0').click();
  await page.setViewportSize({ width: 390, height: 844 });
  const priceRow = dialog.getByTestId('catalog-price-row-cacheWrite');
  await priceRow.scrollIntoViewIfNeeded();
  const cache = priceRow.getByTestId('catalog-price-action-cacheWrite');
  const labelBox = await priceRow.getByRole('rowheader').boundingBox();
  const controlBox = await cache.boundingBox();
  const listBox = await dialog.getByTestId('catalog-import-rows').boundingBox();
  expect(labelBox!.x).toBeGreaterThanOrEqual(listBox!.x);
  expect(controlBox!.x + controlBox!.width).toBeLessThanOrEqual(listBox!.x + listBox!.width);
  const tier = await cache.locator('option').filter({ hasText: '2.5' }).getAttribute('value');
  expect(tier).toBeTruthy();
  await cache.selectOption(tier!);

  await page.clock.install();
  async function refresh() {
    const previousReads = reads;
    await page.clock.fastForward(61_000);
    await page.evaluate(() => window.dispatchEvent(new Event('offline')));
    await page.evaluate(() => window.dispatchEvent(new Event('online')));
    await expect.poll(() => reads).toBeGreaterThan(previousReads);
  }
  inputPrice = 2;
  await refresh();
  await dialog.getByTestId('catalog-import-apply').click();
  await expect.poll(() => applied.length).toBe(27);
  expect(applied.every((row) => row.prices.find(p => p.field === 'input')?.expected === 1)).toBe(true);
  expect(applied.flatMap((row) => row.prices).filter(price=>price.action === 'TIER')).toEqual([
    expect.objectContaining({ field: 'cacheWrite', tier: 'context 256000', expected: 2.5 }),
  ]);

  const firstRow = dialog.getByRole('checkbox', { name: 'batch-0', exact: true });
  for (const nextSource of ['upstream', 'fallback']) {
    await firstRow.check();
    await dialog.getByTestId('catalog-import-expand-batch-0').click();
    await cache.selectOption(tier!);
    upstreamURL = 'https://b.test/catalog';
    source = nextSource;
    await refresh();
    await expect(firstRow).not.toBeChecked();
    await expect(dialog.getByText(upstreamURL, { exact: false })).toBeVisible();
    await expect(dialog.getByTestId('catalog-import-apply')).toBeDisabled();
    await expect(cache).toHaveCount(0);
  }
  await expect(dialog.getByText(/Using the bundled snapshot|正在使用内置快照/)).toBeVisible();
  await dialog.getByRole('textbox').fill('outside');
  await expect(dialog.getByRole('checkbox', { name: 'outside' })).toBeDisabled();
  await dialog.getByRole('textbox').fill('');
  await firstRow.check();
  await dialog.getByTestId('catalog-import-apply').click();
  await expect.poll(() => submittedRevision).toBe(source + upstreamURL);
  expect(applied[0].prices).toEqual([expect.objectContaining({ field: 'input', expected: 2 })]);
});

test('reference prices show the bound source and distinguish missing from unselected quotes', async ({ page }) => {
  const details = { name: 'Model', developer: 'dev', identity: 'v1', canonical: 'dev/current', type: 'chat' };
  const rate = (field: string, base: number | null) => ({ field, base, tiers: [] });
  const providerA = { ...details, provider: 'provider-a', modelID: 'mixed', rates: [rate('input', 1)] };
  const providerB = { ...details, provider: 'provider-b', modelID: 'mixed', rates: [
    rate('input', 9), { field: 'output', base: 20, tiers: [{ label: 'context 256000', value: 25 }] }, rate('cacheRead', 5),
  ] };
  const row = (id: string) => ({
    name: details.name, modelID: id, members: [{ id, identity: '' }], routes: [], version: '',
    mergeTarget: '', mergeTargetOccupied: false, changed: false, archived: false,
  });
  const previews = [
    {
      ...row('mixed'), version: 'saved', changed: true, offers: [providerA, providerB], preferred: providerB,
      prices: [
        { field: 'input', value: 1, follow: true, provider: 'provider-a', modelID: 'mixed', canonical: 'dev/previous' },
        { field: 'output', value: 15, follow: true, provider: 'provider-b', modelID: 'mixed', canonical: 'dev/current' },
        { field: 'cacheRead', value: 3, follow: true, provider: 'removed', modelID: 'mixed' },
      ],
    },
    ...[
      { modelID: 'no-rates', rates: [rate('input', null)] },
      { modelID: 'zero', rates: [rate('input', 0)] },
      { modelID: 'tiers-only', rates: [{ field: 'input', base: null, tiers: [{ label: 'context 256000', value: 2.5 }] }] },
    ].map(({ modelID, rates }) => {
      const offer = { ...providerA, modelID, rates, type: 'embedding', name: 'Vector embedding' };
      if (modelID === 'zero') {
        const unpriced = { ...offer, provider: 'provider-b', rates: [] };
        const route = { channelID: 12, channelName: 'Test channel', actual: modelID, requests: ['public-zero'] };
        return { ...row(modelID), offers: [offer, unpriced], preferred: offer, routes: [route], prices: [] };
      }
      return { ...row(modelID), offers: [offer], preferred: offer, prices: [] };
    }),
  ];
  let applied: unknown[] = [];
  await page.route('**/admin/graphql', (route) => {
    const body = route.request().postDataJSON();
    if (body.operationName === 'ApplyCatalogImport') {
      applied = body.variables.selections;
      const outcomes = body.variables.selections.map(({ modelID }: { modelID: string }) => ({ modelID, status: 'applied' }));
      return route.fulfill({ json: { data: { applyCatalogImport: outcomes } } });
    }
    if (body.operationName !== 'CatalogImportPreview') return route.fallback();
    const preview = { rows: previews, providers: [], source: 'upstream', upstreamURL: 'https://example.test', revision: 'same' };
    return route.fulfill({ json: { data: { catalogImportPreview: preview } } });
  });
  await gotoAndEnsureAuth(page, '/models');
  await page.getByTestId('catalog-import-open').click();
  const expand = (id: string) => page.getByTestId('catalog-import-expand-' + id);
  await expand('mixed').click();
  const source = page.getByRole('combobox', { name: /Price source|报价来源/ });
  await expect(source).toHaveCount(0);
  const quote = (field: string) => page.getByTestId('catalog-price-row-' + field).getByRole('cell').nth(1);
  await expect(quote('input')).toContainText('dev/previous');
  await expect(quote('input')).toContainText('dev/current');
  await expect(quote('input')).toContainText('$1');
  await expect(quote('output')).toContainText('$20');
  await expect(quote('cacheRead')).toContainText('—');
  await expect(quote('cacheRead')).toContainText(/Source has no current base quote|来源暂无基础报价/);
  const inputAction = page.getByTestId('catalog-price-action-input');
  const fromB = inputAction.locator('optgroup[label="provider-b · mixed"] option').first();
  await expect(inputAction).toHaveValue('-1');
  await expect(fromB).toContainText(/Confirm source|确认来源/);
  await inputAction.selectOption(await fromB.getAttribute('value') as string);
  await expect(quote('input')).toContainText('$9');
  const outputAction = page.getByTestId('catalog-price-action-output');
  const outputTier = outputAction.locator('option').filter({ hasText: 'context 256000' });
  await outputAction.selectOption({ label: await outputTier.innerText() });
  const cacheAction = page.getByTestId('catalog-price-action-cacheRead');
  await expect(cacheAction).toHaveValue('-1');
  const fixCurrent = cacheAction.getByRole('option', { name: /Fix the current price|固定当前价格/ });
  await cacheAction.selectOption({ label: await fixCurrent.innerText() });
  await inputAction.selectOption('-1');
  await page.getByRole('checkbox', { name: 'mixed', exact: true }).check();
  await expect(page.getByTestId('catalog-import-apply')).toBeEnabled();
  await expect(expand('no-rates')).toContainText(/No reference prices available|暂无参考价/);
  await expect(expand('tiers-only')).toContainText(/No reference prices selected|未选用参考价/);
  await expect(expand('zero')).toContainText('$0');
  await expand('zero').click();
  await expect(page.getByText(/Test channel.*zero.*public-zero|zero.*Test channel.*public-zero/)).toBeVisible();
  await source.selectOption('');
  await expect(expand('zero')).toContainText(/Choose a price source|请选择报价来源/);
  await expect(expand('zero')).not.toContainText(/No reference prices|暂无参考价|未选用参考价/);
  await source.selectOption({ label: 'provider-b · zero' });
  await expect(expand('zero')).toContainText(/No reference prices available|暂无参考价/);
  await source.selectOption({ label: 'provider-a · zero' });
  await expect(expand('zero')).toContainText('$0');
  await page.getByTestId('catalog-details-only').click();
  await expect(expand('zero')).toContainText(/No reference prices selected|未选用参考价/);
  await page.getByRole('checkbox', { name: 'zero', exact: true }).click();
  const developer = page.getByRole('dialog').getByRole('textbox', { name: /Developer|开发者/i });
  await expect(developer).toHaveValue('dev');
  await developer.fill('');
  await expect(page.getByTestId('catalog-import-apply')).toBeDisabled();
  await developer.fill('custom');
  await page.getByTestId('catalog-import-apply').click();
  await expect.poll(() => applied).toEqual([expect.objectContaining({
    modelID: 'mixed', developer: '', prices: [
      expect.objectContaining({ field: 'output', provider: 'provider-b', action: 'TIER', expected: 25 }),
      expect.objectContaining({ field: 'cacheRead', action: 'FIX' }),
    ],
  }), expect.objectContaining({
    modelID: 'zero', provider: 'provider-a', sourceModelID: 'zero', identity: 'v1',
    type: 'embedding', name: 'Vector embedding', developer: 'custom', prices: [],
    routes: [{ channelID: 12, actual: 'zero', request: 'public-zero' }],
  })]);
});

test('a lost apply response requires a refreshed preview before retrying', async ({ page }) => {
  let online = true;
  let submissions = 0;
  await page.route('**/admin/graphql', async (route) => {
    const { operationName } = route.request().postDataJSON();
    if (operationName === 'ApplyCatalogImport') {
      submissions++;
      online = false;
      return route.abort();
    }
    if (operationName !== 'CatalogImportPreview') return route.fallback();
    if (!online) return route.fulfill({ json: { errors: [{ message: 'Preview temporarily unavailable' }] } });
    const offer = { provider: 'dev', modelID: 'item', name: 'Item', developer: 'dev', type: 'chat', canonical: '', identity: '', rates: [] };
    const row = { modelID: offer.modelID, name: offer.name, version: '', members: [{ id: 'item', identity: '' }], routes: [], offers: [offer], preferred: offer, prices: [] };
    const preview = { rows: submissions ? [] : [row], providers: ['dev'], source: 'upstream', upstreamURL: 'example', revision: 'mirror' };
    return route.fulfill({ json: { data: { catalogImportPreview: preview } } });
  });
  await gotoAndEnsureAuth(page, '/models');
  await page.getByTestId('catalog-import-open').click();
  await page.getByTestId('catalog-import-select-all').click();
  await page.getByTestId('catalog-import-apply').click();
  const dialog = page.getByRole('dialog');
  await expect(dialog.getByText(/result could not be confirmed|无法确认提交结果/)).toBeVisible();
  await expect(dialog.getByText('Preview temporarily unavailable', { exact: false })).toBeVisible();
  await expect(dialog.getByTestId('catalog-import-apply')).toHaveCount(0);
  online = true;
  await dialog.getByTestId('catalog-import-reload').click();
  await expect(dialog.getByTestId('catalog-import-apply')).toBeDisabled();
  expect(submissions).toBe(1);
});
