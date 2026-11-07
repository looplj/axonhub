import { useEffect, useRef, useState } from 'react';
import { useRouterState } from '@tanstack/react-router';
import { Activity, Puzzle, Search, Settings, Ellipsis, RefreshCw } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { Button } from '@/components/ui/button';
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from '@/components/ui/dropdown-menu';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import { usePluginUIEntries, type PluginUIAction } from '../data/plugin-ui';
import { resolvePluginTitle } from '../plugin-ui';
import { PluginFrame } from './plugin-frame';

const icons = { puzzle: Puzzle, search: Search, activity: Activity, settings: Settings };

// The layout owns the dialog, so closing the dropdown does not dispose the frame.
export function PluginHeaderActions({ isMobile }: { isMobile: boolean }) {
  const { t, i18n } = useTranslation();
  const entries = usePluginUIEntries();
  const href = useRouterState({ select: (state) => state.location.href });
  const [selected, setSelected] = useState<{ pluginID: string; key: string; href: string } | null>(null);
  const opener = useRef<HTMLElement | null>(null);
  const menuTrigger = useRef<HTMLButtonElement>(null);
  const actions = entries.data?.pluginUIEntries.actions ?? [];
  const active =
    selected?.href === href ? actions.find((action) => action.pluginID === selected.pluginID && action.key === selected.key) : undefined;

  useEffect(() => {
    if (selected && !active) setSelected(null);
  }, [selected, active]);

  function open(action: PluginUIAction, trigger: HTMLElement | null) {
    opener.current = trigger;
    setSelected({ pluginID: action.pluginID, key: action.key, href });
  }

  const title = (action: PluginUIAction) => resolvePluginTitle(action.title, i18n.language, action.key);
  const direct = isMobile ? [] : actions.slice(0, 2);
  const overflow = isMobile ? actions : actions.slice(2);

  return (
    <>
      {entries.isError && (
        <Button
          variant='ghost'
          size='icon'
          className='size-8'
          aria-label={t('plugins.actions.loadError')}
          title={t('plugins.actions.loadError')}
          onClick={() => void entries.refetch()}
          disabled={entries.isFetching}
        >
          <RefreshCw className='size-4' />
        </Button>
      )}
      {direct.map((action) => {
        const Icon = icons[action.icon] ?? Puzzle;
        return (
          <Tooltip key={`${action.pluginID}:${action.key}`}>
            <TooltipTrigger asChild>
              <Button
                variant='ghost'
                size='icon'
                className='size-8'
                aria-label={title(action)}
                data-testid='plugin-header-action'
                onClick={(event) => open(action, event.currentTarget)}
              >
                <Icon className='size-4' />
              </Button>
            </TooltipTrigger>
            <TooltipContent>{title(action)}</TooltipContent>
          </Tooltip>
        );
      })}
      {overflow.length > 0 && (
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button
              ref={menuTrigger}
              variant='ghost'
              size='icon'
              className='size-8'
              aria-label={t('plugins.actions.more')}
              data-testid='plugin-header-overflow'
            >
              {isMobile ? <Puzzle className='size-4' /> : <Ellipsis className='size-4' />}
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent
            align='end'
            onCloseAutoFocus={(event) => {
              if (selected) event.preventDefault();
            }}
          >
            {overflow.map((action) => {
              const Icon = icons[action.icon] ?? Puzzle;
              return (
                <DropdownMenuItem key={`${action.pluginID}:${action.key}`} onSelect={() => open(action, menuTrigger.current)}>
                  <Icon className='size-4' />
                  {title(action)}
                </DropdownMenuItem>
              );
            })}
          </DropdownMenuContent>
        </DropdownMenu>
      )}
      <Dialog
        open={Boolean(active)}
        onOpenChange={(value) => {
          if (!value) setSelected(null);
        }}
      >
        {active && (
          <DialogContent
            className='flex h-[min(80dvh,800px)] flex-col overflow-hidden sm:max-w-4xl'
            aria-describedby={undefined}
            onCloseAutoFocus={(event) => {
              event.preventDefault();
              if (opener.current?.isConnected) opener.current.focus();
            }}
          >
            <DialogHeader className='shrink-0 pr-6'>
              <DialogTitle>{title(active)}</DialogTitle>
            </DialogHeader>
            <div className='flex min-h-0 flex-1 flex-col' data-testid='plugin-action-dialog'>
              <PluginFrame
                key={`${active.pluginID}:${active.key}:${active.revision}:${i18n.language}`}
                pluginId={active.pluginID}
                actionKey={active.key}
                entry={active.effect.entry}
                title={title(active)}
                onNavigate={() => setSelected(null)}
              />
            </div>
          </DialogContent>
        )}
      </Dialog>
    </>
  );
}
