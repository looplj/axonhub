// 职责: 在插件详情中查看、调整和运行定时任务。
// 关联文件:
//   frontend/src/features/plugins/components/plugin-detail-sheet.tsx
//   frontend/src/features/plugins/data/plugins.ts
//   frontend/src/locales/en/plugins.json
//   frontend/src/locales/zh-CN/plugins.json

import { useId } from 'react';
import { useTranslation } from 'react-i18next';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { usePluginTasks, useRunPluginTask, useUpdatePluginTask, type Plugin, type PluginTask } from '../data/plugins';

export function PluginTasks({ plugin, canManage }: { plugin: Plugin; canManage: boolean }) {
  const { t } = useTranslation();
  const { data, isError } = usePluginTasks(plugin.id);
  if (isError) {
    return <p className='text-destructive text-sm'>{t('plugins.tasks.loadFailed')}</p>;
  }
  if (!data?.length) {
    return null;
  }

  return (
    <section className='space-y-3'>
      <h3 className='text-sm font-semibold'>{t('plugins.tasks.title')}</h3>
      <div className='divide-y rounded-xl border'>
        {data.map((task) => (
          <PluginTaskRow
            key={`${plugin.id}:${task.key}`}
            plugin={plugin}
            task={task}
            canManage={canManage}
          />
        ))}
      </div>
    </section>
  );
}

function PluginTaskRow({ plugin, task, canManage }: { plugin: Plugin; task: PluginTask; canManage: boolean }) {
  const { t, i18n } = useTranslation();
  const cronID = useId();
  const timezoneID = useId();
  const update = useUpdatePluginTask(plugin.id);
  const run = useRunPluginTask(plugin.id);
  const formatTime = (value: string) => new Date(value).toLocaleString(i18n.resolvedLanguage);
  const locked = update.isPending || run.isPending;
  const latest = task.runs[0];
  const running = latest?.status === 'running';

  return (
    <div className='space-y-3 p-3'>
      <div className='flex flex-wrap items-center gap-2'>
        <span className='font-mono text-sm'>{task.key}</span>
        {task.paused && <Badge variant='secondary'>{t('plugins.tasks.paused')}</Badge>}
      </div>
      <div className='text-muted-foreground space-y-1 text-xs'>
        <p className='font-mono'>{task.cron} · {task.timezone}</p>
        {task.nextRunAt && <p>{t('plugins.tasks.nextRun', { time: formatTime(task.nextRunAt) })}</p>}
        {latest && (
          <p>{t('plugins.tasks.lastRun', { time: formatTime(latest.startedAt), status: t(`plugins.tasks.status.${latest.status}`) })}</p>
        )}
      </div>

      {canManage && (
        <>
          <div className='flex flex-wrap gap-2'>
            <Button variant='outline' size='sm' disabled={locked || !plugin.enabled || running} onClick={() => run.mutate(task.key)}>
              {t(run.isPending ? 'plugins.tasks.running' : 'plugins.tasks.runNow')}
            </Button>
            <Button variant='outline' size='sm' disabled={locked} onClick={() => update.mutate({ key: task.key, paused: !task.paused })}>
              {t(task.paused ? 'plugins.tasks.resume' : 'plugins.tasks.pause')}
            </Button>
          </div>
          <details className='space-y-2 text-sm'>
            <summary className='cursor-pointer'>{t('plugins.tasks.edit')}</summary>
            <form
              key={JSON.stringify([task.cron, task.timezone])}
              className='space-y-2'
              onSubmit={(event) => {
                event.preventDefault();
                const values = new FormData(event.currentTarget);
                update.mutate({
                  key: task.key,
                  paused: task.paused,
                  cron: String(values.get('cron')).trim(),
                  timezone: String(values.get('timezone')).trim(),
                });
              }}
            >
              <Label htmlFor={cronID}>{t('plugins.tasks.cron')}</Label>
              <Input id={cronID} name='cron' defaultValue={task.cron} maxLength={128} required />
              <Label htmlFor={timezoneID}>{t('plugins.tasks.timezone')}</Label>
              <Input id={timezoneID} name='timezone' defaultValue={task.timezone} maxLength={64} required />
              <div className='flex flex-wrap gap-2'>
                <Button size='sm' type='submit' disabled={locked}>{t('plugins.tasks.save')}</Button>
                {task.overridden && (
                  <Button
                    size='sm'
                    type='button'
                    variant='outline'
                    disabled={locked}
                    onClick={() => update.mutate({ key: task.key, paused: task.paused, cron: '', timezone: '' })}
                  >
                    {t('plugins.tasks.reset')}
                  </Button>
                )}
              </div>
            </form>
          </details>
        </>
      )}

      <details className='space-y-2 text-sm'>
        <summary className='cursor-pointer'>{t('plugins.tasks.history')}</summary>
        {task.runs.length === 0 ? (
          <p className='text-muted-foreground text-xs'>{t('plugins.tasks.noRuns')}</p>
        ) : (
          <ul className='space-y-1'>
            {task.runs.map((execution) => (
              <li key={execution.id} className='flex flex-wrap items-center gap-x-3 text-xs'>
                <time dateTime={execution.startedAt}>{formatTime(execution.startedAt)}</time>
                <span>{t(execution.scheduledAt ? 'plugins.tasks.scheduled' : 'plugins.tasks.manual')}</span>
                <Badge variant='outline'>{t(`plugins.tasks.status.${execution.status}`)}</Badge>
              </li>
            ))}
          </ul>
        )}
      </details>
    </div>
  );
}
