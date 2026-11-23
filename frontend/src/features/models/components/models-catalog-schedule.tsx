// 职责: 管理每日模型检查的站点时间和服务商范围，展示最近一次检查结果。
// 关联文件:
//   frontend/src/features/models/data/providers.ts
//   frontend/src/features/models/data/catalog-import.ts
//   frontend/src/features/models/components/models-catalog-import.tsx
//   frontend/src/locales/en.json
//   frontend/src/locales/zh.json
// 注意事项:
//   首次设置使用浏览器时区，保存后所有管理员共享站点时区。
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Button } from '@/components/ui/button';
import { Checkbox } from '@/components/ui/checkbox';
import { Input } from '@/components/ui/input';
import { useProvidersData } from '../data/providers';
import { useSaveCatalogSchedule, useCheckCatalogNow, type CheckStatus } from '../data/catalog-import';

export function CatalogSchedule({
  status,
  canWrite,
  onReview,
}: {
  status: CheckStatus;
  canWrite: boolean;
  onReview?: () => void;
}) {
  const { t, i18n } = useTranslation();
  const [draft, setDraft] = useState(() => ({
    ...status.settings,
    timezone: status.settings.timezone || Intl.DateTimeFormat().resolvedOptions().timeZone,
  }));
  const [search, setSearch] = useState('');
  const save = useSaveCatalogSchedule();
  const check = useCheckCatalogNow();
  const { data } = useProvidersData();
  const error = save.error ?? check.error;
  const formatTime = (value: string | null) =>
    value ? new Date(value).toLocaleString(i18n.language, { timeZone: status.settings.timezone || undefined, timeZoneName: 'short' }) : '—';
  const providerOptions = [...new Set([...Object.keys(data.providers), ...draft.providers])]
    .sort()
    .filter((id) => id.toLowerCase().includes(search.toLowerCase()));
  return (
    <div className='min-h-0 space-y-4 overflow-y-auto text-sm'>
      <section className='space-y-2'>
        <p>
          {status.lastSuccess
            ? t('models.import.lastCounts', { available: status.available, changed: status.changed })
            : t('models.import.neverChecked')}
        </p>
        <dl className='grid grid-cols-[auto_1fr] gap-x-3 gap-y-1'>
          {(['lastAttempt', 'lastSuccess', 'nextCheck'] as const).map((key) => (
            <div className='contents' key={key}>
              <dt className='text-muted-foreground'>{t('models.import.' + key)}</dt>
              <dd>{formatTime(status[key])}</dd>
            </div>
          ))}
        </dl>
        {status.failed && <p role='status'>{t('models.import.checkFailed')}</p>}
        {onReview && (
          <div className='flex flex-wrap gap-2'>
            <Button size='sm' variant='outline' disabled={check.isPending} onClick={() => check.mutate()}>
              {t('models.import.checkNow')}
            </Button>
            <Button size='sm' variant='outline' onClick={onReview}>
              {t('models.import.reviewWatch')}
            </Button>
          </div>
        )}
      </section>
      <fieldset disabled={!canWrite || save.isPending} className='space-y-3 border-t pt-4'>
        <label className='flex items-center gap-2 font-medium'>
          <Checkbox
            disabled={!canWrite}
            checked={draft.enabled}
            onCheckedChange={(value) => setDraft({ ...draft, enabled: value === true })}
          />
          {t('models.import.daily')}
        </label>
        <div className='flex flex-wrap gap-3'>
          <label>
            {t('models.import.time')}
            <Input type='time' value={draft.time} onChange={(e) => setDraft({ ...draft, time: e.target.value })} />
          </label>
          <label>
            {t('models.import.timezone')}
            <Input value={draft.timezone} onChange={(e) => setDraft({ ...draft, timezone: e.target.value })} />
          </label>
        </div>
        <label className='flex items-center gap-2'>
          <Checkbox
            disabled={!canWrite}
            checked={draft.ignorePrefix}
            onCheckedChange={(v) => setDraft({ ...draft, ignorePrefix: v === true })}
          />
          {t('models.import.ignorePrefix')}
        </label>
        <label className='block'>
          {t('models.import.providers')}
          <Input value={search} placeholder={t('models.import.findProvider')} onChange={(e) => setSearch(e.target.value)} />
        </label>
        <div className='grid max-h-36 grid-cols-2 gap-2 overflow-y-auto rounded-md border p-2 sm:grid-cols-3'>
          {providerOptions.map((id) => (
            <label key={id} className='flex items-center gap-2 break-all'>
              <Checkbox
                checked={draft.providers.includes(id)}
                disabled={!canWrite}
                onCheckedChange={(v) =>
                  setDraft({ ...draft, providers: v === true ? [...draft.providers, id] : draft.providers.filter((p) => p !== id) })
                }
              />
              {id}
            </label>
          ))}
        </div>
        {canWrite && (
          <Button size='sm' disabled={!draft.time || !draft.timezone.trim()} onClick={() => save.mutate(draft)}>
            {t('common.buttons.save')}
          </Button>
        )}
      </fieldset>
      {error && (
        <p role='alert' className='text-destructive'>
          {error.message}
        </p>
      )}
    </div>
  );
}
