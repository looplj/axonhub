import { createFileRoute } from '@tanstack/react-router';
import { useTranslation } from 'react-i18next';
import { Main } from '@/components/layout/main';
import { Button } from '@/components/ui/button';
import NotFoundError from '@/features/errors/not-found-error';
import { PluginFrame } from '@/features/plugins/components/plugin-frame';
import { usePluginUIEntries } from '@/features/plugins/data/plugin-ui';
import { useSelectedProjectId } from '@/stores/projectStore';

export const Route = createFileRoute('/_authenticated/plugins/$pluginId/pages/$page')({
  component: PluginPage,
});

function PluginPage() {
  const { pluginId, page } = Route.useParams();
  const { t, i18n } = useTranslation();
  const projectId = useSelectedProjectId();
  const { data, isPending, isError, refetch } = usePluginUIEntries();

  if (isPending && !data) {
    return (
      <Main fixed className='p-0'>
        <div className='text-muted-foreground flex h-full items-center justify-center'>{t('plugins.loading')}</div>
      </Main>
    );
  }

  if (isError && !data) {
    return (
      <Main fixed className='p-0'>
        <div data-testid='plugin-frame-error' role='alert' className='flex h-full flex-col items-center justify-center gap-4'>
          <p className='text-muted-foreground text-lg'>{t('plugins.page.error')}</p>
          <Button data-testid='plugin-frame-retry' onClick={() => void refetch()}>
            {t('plugins.page.retry')}
          </Button>
        </div>
      </Main>
    );
  }

  const match = data?.pluginUIEntries.pages.find((item) => item.pluginID === pluginId && item.key === page);
  if (!match) {
    return (
      <div data-testid='plugin-page-not-found' aria-label={t('plugins.page.notFound')}>
        <NotFoundError />
      </div>
    );
  }

  return (
    <Main fixed className='p-0'>
      <PluginFrame key={`${i18n.language}:${projectId ?? ''}`} pluginId={match.pluginID} pageKey={match.key} entry={match.entry} />
    </Main>
  );
}
