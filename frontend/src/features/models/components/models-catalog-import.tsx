// 职责: 预览渠道或服务商的目录差异并批量提交勾选的模型和参考价。
// 关联文件:
//   frontend/src/features/models/data/catalog-import.ts
//   frontend/src/features/models/components/models-dialogs.tsx
//   frontend/src/locales/en.json
//   frontend/src/locales/zh.json
// 注意事项:
//   只有主动选择阶梯才将其保存为固定单价，手工价格默认保留。
//   同一目录来源刷新时保留已审阅的草稿，来源改变后重新勾选。
import { useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useDebounce } from '@/hooks/use-debounce';
import { Button } from '@/components/ui/button';
import { Checkbox } from '@/components/ui/checkbox';
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { useModels } from '../context/models-context';
import {
  useCatalogPreview,
  useCatalogApply,
  type ImportScope,
  type ImportRow,
  type ImportSelection,
  type CatalogPreview,
} from '../data/catalog-import';
import { PriceEditor, defaultSelection, selectStyle, offerKey, useReferencePriceFormat } from './models-catalog-prices';

const pageSize = 25;

function requiresSource(row: ImportRow) {
  return row.offers.length > 1 && row.offers.some((offer) =>
    row.members.some(({ id }) => id === offer.modelID || id === offer.canonical)
  );
}

function PreviewRows({
  preview: latest,
  applying,
  onApply,
}: {
  preview: CatalogPreview;
  applying: boolean;
  onApply: (revision: string, selections: ImportSelection[]) => void;
}) {
  const { t } = useTranslation();
  const formatPrice = useReferencePriceFormat();
  const [search, setSearch] = useState('');
  const query = useDebounce(search, 200);
  const [filter, setFilter] = useState('all');
  const [page, setPage] = useState(0);
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [edits, setEdits] = useState<Record<string, ImportSelection>>({});
  const [merged, setMerged] = useState<ImportRow[]>([]);
  const [expanded, setExpanded] = useState('');
  const [preview, setPreview] = useState(latest);
  useEffect(() => {
    if (selected.size === 0 && !Object.keys(edits).length && !merged.length) setPreview(latest);
  }, [latest, selected, edits, merged]);
  const rows = useMemo(() => {
    const mergedIDs = new Set(merged.flatMap((r) => r.members.map((m) => m.id)));
    return [...merged, ...preview.rows.filter((r) => !r.members.some(({ id }) => mergedIDs.has(id)))];
  }, [merged, preview.rows]);
  const draftFor = (row: ImportRow) => edits[row.modelID] ?? defaultSelection(row);
  const filtered = rows.filter(
    (r) =>
      (!query || [r.modelID, r.name, ...r.members.map((m) => m.id)].some((id) => id.toLowerCase().includes(query.toLowerCase()))) &&
      (filter === 'all' ||
        (filter === 'new' && !r.version) ||
        (filter === 'changed' && r.changed) ||
        (filter === 'unmatched' && !r.offers.length))
  );
  const getEligible = (row: ImportRow) => !row.archived && !(preview.source === 'fallback' && row.version);
  const toggle = (id: string) =>
    setSelected((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  const chosen = rows.filter((r) => selected.has(r.modelID));
  const choices = chosen.map(draftFor);
  const ready =
    choices.length > 0 &&
    choices.every((r, i) => {
      if (r.version) return r.prices.length > 0;
      return (
        r.name.trim() && r.developer.trim() && r.type &&
        (r.provider || !requiresSource(chosen[i])) &&
        r.routes.every((route) => route.request !== undefined)
      );
    });
  const mergeTarget = chosen[0]?.mergeTarget;
  const canMerge =
    mergeTarget &&
    chosen.length > 1 &&
    chosen.every((r) => r.mergeTarget === mergeTarget && !r.mergeTargetOccupied) &&
    !rows.some((r) => r.modelID === mergeTarget && !selected.has(r.modelID));
  const merge = () => {
    if (!canMerge) return;
    const offers = [...new Map(chosen.flatMap((r) => r.offers).map((o) => [offerKey(o), o])).values()];
    const group: ImportRow = {
      ...chosen[0],
      manualMerge: true,
      members: chosen.flatMap((r) => r.members),
      routes: chosen.flatMap((r) => r.routes),
      modelID: mergeTarget,
      name: mergeTarget,
      offers,
      preferred: null,
    };
    setMerged((prev) => [...prev.filter((r) => !selected.has(r.modelID)), group]);
    setEdits((prev) => ({ ...prev, [mergeTarget]: defaultSelection(group) }));
    setSelected(new Set([mergeTarget]));
    setExpanded(mergeTarget);
    setPage(0);
  };
  const currentPage = Math.min(page, Math.max(0, Math.ceil(filtered.length / pageSize) - 1));
  const visible = filtered.slice(currentPage * pageSize, (currentPage + 1) * pageSize);
  function applySelected() {
    onApply(preview.revision, choices);
    setSelected(new Set());
    setEdits({});
    setMerged([]);
    setExpanded('');
  }
  return (
    <fieldset disabled={applying} className='flex min-h-0 min-w-0 flex-1 flex-col gap-2'>
      <div className='flex flex-wrap gap-2'>
        <Input
          className='min-w-32 flex-1'
          placeholder={t('models.import.search')}
          aria-label={t('models.import.search')}
          value={search}
          onChange={(e) => {
            setSearch(e.target.value);
            setPage(0);
          }}
        />
        <select
          className={selectStyle}
          value={filter}
          aria-label={t('models.import.filterLabel')}
          onChange={(e) => {
            setFilter(e.target.value);
            setPage(0);
          }}
        >
          {['all', 'new', 'changed', 'unmatched'].map((f) => (
            <option key={f} value={f}>
              {t('models.import.filter.' + f)}
            </option>
          ))}
        </select>
      </div>
      <div className='flex flex-wrap items-center gap-2 text-sm'>
        <Button
          variant='outline'
          size='sm'
          data-testid='catalog-import-select-all'
          onClick={() =>
            setSelected(
              (prev) =>
                new Set([
                  ...prev,
                  ...filtered.filter((r) => getEligible(r) && (!r.version || draftFor(r).prices.length > 0)).map((r) => r.modelID),
                ])
            )
          }
        >
          {t('models.import.selectAll')}
        </Button>
        {selected.size > 0 && (
          <Button variant='ghost' size='sm' onClick={() => setSelected(new Set())}>
            {t('models.import.clearSelection')}
          </Button>
        )}
        {canMerge && (
          <Button variant='outline' size='sm' onClick={merge}>
            {t('models.import.merge', { id: mergeTarget })}
          </Button>
        )}
        <span className='text-muted-foreground'>{t('models.import.rowCount', { count: filtered.length })}</span>
      </div>
      <div className='min-h-24 flex-1 overflow-y-auto rounded-md border' data-testid='catalog-import-rows'>
        {!visible.length && <p className='text-muted-foreground p-4 text-sm'>{t('models.import.empty')}</p>}
        {visible.map((row) => {
          const draft = draftFor(row);
          const offer = row.offers.find((o) => o.provider === draft.provider && o.modelID === draft.sourceModelID);
          let status = !row.version ? 'new' : row.changed ? 'changed' : 'existing';
          if (row.archived) status = 'archived';
          else if (!row.offers.length) status = 'unmatched';
          else if (!row.version && !offer && requiresSource(row)) status = 'chooseSource';
          let priceSummary = draft.prices
            .map((a) => {
              const next = a.expected;
              const current = row.prices.find((r) => r.field === a.field)?.value;
              return a.action === 'FIX'
                ? `${t('models.modelCard.' + a.field)} · ${t('models.import.fixCurrent')}`
                : `${t('models.modelCard.' + a.field)} ${current == null ? '' : formatPrice(current) + ' → '}${next == null ? '—' : formatPrice(next)}`;
            })
            .join(' · ');
          if (!priceSummary && !row.version && offer) {
            const hasQuote = offer.rates.some((r) => r.base != null || r.tiers.length > 0);
            priceSummary = t(hasQuote ? 'models.import.noPrices' : 'models.import.noQuotes');
          }
          return (
            <div key={row.modelID} className='space-y-1 border-b p-2 last:border-b-0'>
              <div className='flex items-center gap-3'>
                <Checkbox
                  aria-label={row.modelID}
                  checked={selected.has(row.modelID)}
                  disabled={!getEligible(row)}
                  onCheckedChange={() => toggle(row.modelID)}
                />
                <button
                  type='button'
                  data-testid={'catalog-import-expand-' + row.modelID}
                  aria-expanded={expanded === row.modelID}
                  className='min-w-0 flex-1 text-left'
                  onClick={() => setExpanded(expanded === row.modelID ? '' : row.modelID)}
                >
                  <span className='block text-sm font-medium break-all'>{row.modelID}</span>
                  <span className='text-muted-foreground block text-xs'>
                    {t('models.import.filter.' + status)}
                    {row.mergeTargetOccupied && ' · ' + t('models.import.targetOccupied', { id: row.mergeTarget })}
                    {!row.version && draft.provider && ' · ' + draft.provider}
                    {priceSummary && ' · ' + priceSummary}
                  </span>
                </button>
              </div>
              {expanded === row.modelID && (
                <PriceEditor
                  row={row}
                  draft={draft}
                  offer={offer}
                  onChange={(value) => setEdits((prev) => ({ ...prev, [row.modelID]: value }))}
                />
              )}
            </div>
          );
        })}
      </div>
      <div className='flex flex-wrap items-center justify-between gap-2'>
        <div className='flex items-center gap-2 text-xs'>
          <Button
            size='sm'
            variant='outline'
            aria-label={t('pagination.previousPage')}
            disabled={!currentPage}
            onClick={() => setPage(currentPage - 1)}
          >
            ‹
          </Button>
          {currentPage + 1} / {Math.max(1, Math.ceil(filtered.length / pageSize))}
          <Button
            size='sm'
            variant='outline'
            aria-label={t('pagination.nextPage')}
            disabled={(currentPage + 1) * pageSize >= filtered.length}
            onClick={() => setPage(currentPage + 1)}
          >
            ›
          </Button>
        </div>
        <Button data-testid='catalog-import-apply' disabled={!ready} onClick={applySelected}>
          {t('models.import.apply', { count: selected.size })}
        </Button>
      </div>
      {selected.size > 0 && !ready && (
        <p role='status' className='text-muted-foreground text-xs'>
          {t('models.import.completeRows')}
        </p>
      )}
    </fieldset>
  );
}

export function ModelsCatalogImport() {
  const { t } = useTranslation();
  const { setOpen } = useModels();
  const [mode, setMode] = useState('channels');
  const [provider, setProvider] = useState('');
  const [ignorePrefix, setIgnorePrefix] = useState(false);
  const apply = useCatalogApply();
  const outcomes = apply.data ?? [];
  const scope = useMemo<ImportScope>(
    () => ({
      channels: mode === 'channels',
      providers: mode === 'provider' && provider ? [provider] : [],
      ignorePrefix: mode === 'channels' && ignorePrefix,
    }),
    [mode, provider, ignorePrefix]
  );
  const preview = useCatalogPreview(scope);
  const data = preview.data;
  const refreshingAfterError = apply.isError && preview.fetchStatus !== 'idle';
  return (
    <Dialog
      open
      onOpenChange={(v) => {
        if (!v) setOpen(null);
      }}
    >
      <DialogContent className='flex h-[min(85dvh,52rem)] max-h-[95dvh] flex-col overflow-hidden sm:max-w-4xl'>
        <DialogHeader>
          <DialogTitle>{t('models.import.title')}</DialogTitle>
          <DialogDescription>{t('models.import.description')}</DialogDescription>
        </DialogHeader>
        <div className='flex flex-wrap items-center gap-2'>
          <select
            aria-label={t('models.import.scopeLabel')}
            className={selectStyle}
            value={mode}
            onChange={(e) => {
              setMode(e.target.value);
              apply.reset();
            }}
          >
            {['channels', 'provider'].map((m) => (
              <option key={m} value={m}>
                {t('models.import.scope.' + m)}
              </option>
            ))}
          </select>
          {mode === 'provider' && (
            <select
              aria-label={t('models.import.provider')}
              className={selectStyle}
              value={provider}
              onChange={(e) => {
                setProvider(e.target.value);
                apply.reset();
              }}
            >
              <option value=''>{t('models.import.pickProvider')}</option>
              {(data?.providers ?? (provider ? [provider] : [])).map((p) => (
                <option key={p}>{p}</option>
              ))}
            </select>
          )}
          {mode === 'channels' && (
            <label className='flex items-center gap-2 text-sm'>
              <Checkbox checked={ignorePrefix} onCheckedChange={(v) => setIgnorePrefix(v === true)} />
              {t('models.import.ignorePrefix')}
            </label>
          )}
        </div>
        {outcomes.length > 0 && (
          <div role='status' className='max-h-24 overflow-y-auto text-sm'>
            {t('models.import.applied', { count: outcomes.filter((r) => r.status === 'applied').length })}
            {outcomes
              .filter((r) => r.status !== 'applied')
              .map((r, i) => (
                <p key={i}>
                  {r.modelID} · {t('models.import.result.' + r.status)}
                </p>
              ))}
          </div>
        )}
        {apply.error && (
          <p role='alert' className='text-destructive text-sm'>
            {t('models.import.unknownResult')} {apply.error.message}
          </p>
        )}
        {(preview.isPending || apply.isPending || refreshingAfterError) && <p>{t('common.loading')}</p>}
        {preview.error && (
          <div role='alert'>
            {preview.error.message}
            <Button data-testid='catalog-import-reload' variant='outline' size='sm' onClick={() => preview.refetch()}>
              {t('models.import.preview')}
            </Button>
          </div>
        )}
        {data && !preview.error && !refreshingAfterError && (
          <>
            <p className='text-muted-foreground text-xs break-all'>
              {t('models.import.' + (data.source === 'fallback' ? 'fallback' : 'source'))} · {data.upstreamURL}
            </p>
            <PreviewRows
              key={JSON.stringify([scope, data.revision])}
              preview={data}
              applying={apply.isPending}
              onApply={(revision, selections) => apply.mutate({ scope, revision, selections })}
            />
          </>
        )}
      </DialogContent>
    </Dialog>
  );
}
