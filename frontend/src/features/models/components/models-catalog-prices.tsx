// 职责: 展示模型各项报价、阶梯条件和已有价格来源，并构造明确的导入选择。
// 关联文件:
//   frontend/src/features/models/components/models-catalog-import.tsx
//   frontend/src/features/models/components/models-table.tsx
//   frontend/src/features/models/data/catalog-import.ts
//   frontend/src/features/models/data/schema.ts
//   frontend/src/locales/en.json
//   frontend/src/locales/zh.json
// 注意事项:
//   未绑定来源的历史价格默认保留，选择某档报价会保存为固定单价。
import { useTranslation } from 'react-i18next';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import type { ImportRow, LocalRate, Offer, PriceSelection, ImportSelection } from '../data/catalog-import';
import { modelTypeSchema, referencePriceFields, type ReferencePriceField } from '../data/schema';

export const selectStyle = 'h-9 max-w-full rounded-md border bg-background px-2 text-sm';
export const offerKey = (offer: { provider: string; modelID: string }) => JSON.stringify([offer.provider, offer.modelID]);
export function useReferencePriceFormat() {
  const { t, i18n } = useTranslation();
  return (value: number) =>
    t('currencies.format', { val: value, currency: 'USD', locale: i18n.language, minimumFractionDigits: 0, maximumFractionDigits: 12 });
}

function findOffer(row: ImportRow, source: Pick<Offer, 'provider' | 'modelID'> | null) {
  return row.offers.find((offer) => offer.provider === source?.provider && offer.modelID === source.modelID);
}

function needsRebind(local: LocalRate | undefined, offer: Offer) {
  return local?.follow && (local.provider !== offer.provider || local.modelID !== offer.modelID || local.canonical !== offer.canonical);
}

function defaultRequest(modelID: string, route: ImportRow['routes'][number]) {
  if (route.requests.includes(modelID)) return modelID;
  if (route.requests.includes(route.actual)) return route.actual;
  if (!route.requests.length) return '';
  return route.requests.length === 1 ? route.requests[0] : undefined;
}

export function defaultSelection(row: ImportRow, offer = findOffer(row, row.preferred) ?? null): ImportSelection {
  const prices: PriceSelection[] = [];
  for (const field of referencePriceFields) {
    const local = row.prices.find((p) => p.field === field);
    if (row.version && !local?.follow) continue;
    const source = local ? findOffer(row, local) : offer;
    const rate = source?.rates.find((r) => r.field === field);
    if (!source || rate?.base == null || needsRebind(local, source) || local?.value === rate.base) continue;
    prices.push({
      field,
      action: 'FOLLOW',
      expected: rate.base,
      identity: source.identity,
      provider: source.provider,
      modelID: source.modelID,
    });
  }
  return {
    type: offer?.type ? (modelTypeSchema.safeParse(offer.type).data ?? '') : 'chat',
    identity: offer?.identity ?? '',
    modelID: row.modelID,
    members: row.members,
    manualMerge: row.manualMerge,
    routes: row.version
      ? []
      : row.routes.map((route) => ({ channelID: route.channelID, actual: route.actual, request: defaultRequest(row.modelID, route) })),
    version: row.version,
    name: row.version ? row.name : offer?.name || row.modelID,
    developer: row.version ? '' : offer?.developer || '',
    provider: offer?.provider || '',
    sourceModelID: offer?.modelID || '',
    prices,
  };
}

export function PriceEditor({
  row,
  draft,
  offer,
  onChange,
}: {
  row: ImportRow;
  draft: ImportSelection;
  offer: Offer | undefined;
  onChange: (value: ImportSelection) => void;
}) {
  const { t } = useTranslation();
  const formatPrice = useReferencePriceFormat();
  function selectOffer(next: Offer | null) {
    const defaults = defaultSelection(row, next);
    const previous = defaultSelection(row, offer ?? null);
    for (const field of ['name', 'developer', 'type'] as const) {
      if (draft[field] !== previous[field]) defaults[field] = draft[field];
    }
    onChange({ ...defaults, routes: draft.routes });
  }
  return (
    <div className='space-y-3 rounded-md border p-3'>
      {!row.version && (
        <div className='grid gap-2 sm:grid-cols-3'>
          <label className='text-sm'>
            {t('models.fields.name')}
            <Input value={draft.name} onChange={(e) => onChange({ ...draft, name: e.target.value })} />
          </label>
          <label className='text-sm'>
            {t('models.fields.developer')}
            <Input value={draft.developer} onChange={(e) => onChange({ ...draft, developer: e.target.value })} />
          </label>
          <label className='grid text-sm'>
            {t('models.fields.type')}
            <select className={selectStyle} value={draft.type} onChange={(e) => onChange({ ...draft, type: e.target.value })}>
              {!draft.type && <option value=''>{t('models.import.pickType')}</option>}
              {modelTypeSchema.options.map((type) => (
                <option key={type} value={type}>
                  {t('models.types.' + type)}
                </option>
              ))}
            </select>
          </label>
        </div>
      )}
      {row.members.length > 1 && (
        <p className='text-xs break-words'>
          {t('models.import.aliases')}: {row.members.map((m) => m.id).join(', ')}
        </p>
      )}
      {!row.version &&
        row.routes.map((route, index) => {
          const key = route.channelID + '/' + route.actual;
          const label = t('models.import.route', { channel: route.channelName, model: route.actual });
          if (defaultRequest(row.modelID, route))
            return (
              <p key={key} className='text-muted-foreground text-xs break-all'>
                {label} · {draft.routes[index].request}
              </p>
            );
          if (!route.requests.length)
            return (
              <p key={key} className='text-muted-foreground text-xs'>
                {t('models.import.hiddenRoute', { channel: route.channelName, model: route.actual })}
              </p>
            );
          return (
            <label key={key} className='grid gap-1 text-sm'>
              {label}
              <select
                className={selectStyle}
                value={draft.routes[index].request ?? ''}
                onChange={(e) =>
                  onChange({
                    ...draft,
                    routes: draft.routes.map((r, i) => (i === index ? { ...r, request: e.target.value || undefined } : r)),
                  })
                }
              >
                <option value=''>{t('models.import.pickRequest')}</option>
                {route.requests.map((name) => (
                  <option key={name}>{name}</option>
                ))}
              </select>
            </label>
          );
        })}
      {!row.version && (
        <label className='flex flex-wrap items-center gap-2 text-sm'>
          {t('models.import.priceSource')}
          <select
            className={selectStyle}
            value={offer ? offerKey(offer) : ''}
            onChange={(e) => selectOffer(row.offers.find((o) => offerKey(o) === e.target.value) ?? null)}
          >
            <option value=''>{t('models.import.pickSource')}</option>
            {row.offers.map((o) => (
              <option key={offerKey(o)} value={offerKey(o)}>
                {o.provider} · {o.modelID}
              </option>
            ))}
          </select>
        </label>
      )}
      {!row.version && draft.prices.length > 0 && (
        <Button type='button' size='sm' variant='outline' data-testid='catalog-details-only' onClick={() => onChange({ ...draft, prices: [] })}>
          {t('models.import.noSource')}
        </Button>
      )}
      <p className='text-muted-foreground text-xs'>{t('models.import.unit')}</p>
      <div className='min-w-0'>
        <table className='block w-full text-left text-sm sm:table'>
          <thead className='hidden sm:table-header-group'>
            <tr className='text-muted-foreground'>
              <th>{t('models.modelCard.cost')}</th>
              <th>{t('models.import.currentPrice')}</th>
              <th>{t('models.import.referencePrice')}</th>
              <th>{t('models.import.onApply')}</th>
            </tr>
          </thead>
          <tbody className='block sm:table-row-group'>
            {referencePriceFields.map((field) => {
              const local = row.prices.find((r) => r.field === field);
              const action = draft.prices.find((r) => r.field === field);
              const binding = action?.provider ? action : local;
              const source = binding?.provider ? findOffer(row, binding) : offer;
              const rate = source?.rates.find((r) => r.field === field);
              const bound = local?.follow ? findOffer(row, local) : undefined;
              return (
                <tr
                  key={field}
                  data-testid={'catalog-price-row-' + field}
                  className='grid grid-cols-2 gap-x-3 gap-y-2 border-t py-3 sm:table-row'
                >
                  <th scope='row' className='col-span-2 font-medium sm:py-2 sm:pr-2'>
                    {t('models.modelCard.' + field)}
                  </th>
                  <td className='min-w-0 break-words tabular-nums sm:pr-2'>
                    <span className='text-muted-foreground block text-xs sm:hidden'>{t('models.import.currentPrice')}</span>
                    {local?.value == null ? '—' : formatPrice(local.value)}
                    {local && (
                      <div className='text-muted-foreground text-xs'>
                        {local.follow
                          ? t('models.import.following', { provider: local.provider, model: local.modelID })
                          : t('models.import.manual')}
                      </div>
                    )}
                  </td>
                  <td className='min-w-0 break-words tabular-nums sm:pr-2'>
                    <span className='text-muted-foreground block text-xs sm:hidden'>{t('models.import.referencePrice')}</span>
                    {rate?.base == null ? '—' : formatPrice(rate.base)}
                    {source && <div className='text-muted-foreground text-xs'>{source.provider} · {source.modelID}</div>}
                    {bound && local && needsRebind(local, bound) && (
                      <div className='text-xs'>{t('models.import.identityChanged', { from: local.canonical || '—', to: bound.canonical || '—' })}</div>
                    )}
                    {local?.follow && (!bound || bound.rates.find((r) => r.field === field)?.base == null) && (
                      <div className='text-muted-foreground text-xs'>{t('models.import.sourceMissingPrice')}</div>
                    )}
                    {rate?.tiers.length ? (
                      <details>
                        <summary className='cursor-pointer text-xs'>{t('models.import.tiers')}</summary>
                        {rate.tiers.map((tier, i) => (
                          <div key={i} className='text-xs'>
                            {tier.label} · {formatPrice(tier.value)}
                          </div>
                        ))}
                      </details>
                    ) : null}
                  </td>
                  <td className='col-span-2 min-w-0'>
                    <span className='text-muted-foreground block text-xs sm:hidden'>{t('models.import.onApply')}</span>
                    <PriceActionSelect
                      row={row}
                      field={field}
                      value={action}
                      onChange={(choice) => {
                        const prices = draft.prices.filter((p) => p.field !== field);
                        onChange({ ...draft, prices: choice ? [...prices, choice] : prices });
                      }}
                    />
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>
      {row.offers.some((o) => o.rates.some((rate) => rate.tiers.length > 0)) && (
        <p className='text-muted-foreground text-xs'>{t('models.import.flat')}</p>
      )}
    </div>
  );
}

function PriceActionSelect({
  row,
  field,
  value,
  onChange,
}: {
  row: ImportRow;
  field: ReferencePriceField;
  value?: PriceSelection;
  onChange: (value?: PriceSelection) => void;
}) {
  const { t } = useTranslation();
  const format = useReferencePriceFormat();
  const choices: PriceSelection[] = [];
  function option(label: string, choice: PriceSelection) {
    const index = choices.push(choice) - 1;
    return (
      <option key={index} value={index}>
        {label}
      </option>
    );
  }
  const local = row.prices.find((p) => p.field === field);
  const fixed =
    local?.follow &&
    option(t('models.import.fixCurrent'), { field, action: 'FIX', provider: '', modelID: '' });
  const groups = row.offers.map((o) => {
    const rate = o.rates.find((r) => r.field === field);
    if (!rate || (rate.base == null && !rate.tiers.length)) return null;
    const rebind = needsRebind(local, o);
    const source = { field, provider: o.provider, modelID: o.modelID, identity: o.identity };
    return (
      <optgroup key={offerKey(o)} label={o.provider + ' · ' + o.modelID}>
        {rate.base != null &&
          option(t(rebind ? 'models.import.rebind' : 'models.import.follow') + ' · ' + format(rate.base), { ...source, action: rebind ? 'REBIND' : 'FOLLOW', expected: rate.base })}
        {rate.base != null &&
          option(t('models.import.fixBase') + ' · ' + format(rate.base), { ...source, action: 'FIX_BASE', expected: rate.base })}
        {rate.tiers.map((tier) =>
          option(t('models.import.fixTier', { tier: tier.label, price: format(tier.value) }), {
            ...source,
            action: 'TIER',
            tier: tier.label,
            expected: tier.value,
          })
        )}
      </optgroup>
    );
  });
  const selected = choices.findIndex(
    (c) => c.action === value?.action && c.provider === value.provider && c.modelID === value.modelID && c.tier === value.tier
  );
  return (
    <select
      data-testid={'catalog-price-action-' + field}
      aria-label={t('models.modelCard.' + field)}
      className={selectStyle + ' w-full sm:w-auto'}
      value={selected}
      onChange={(e) => onChange(choices[Number(e.target.value)])}
    >
      <option value={-1}>{t('models.import.keep')}</option>
      {fixed}
      {groups}
    </select>
  );
}
