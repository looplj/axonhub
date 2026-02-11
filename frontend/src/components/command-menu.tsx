import React from 'react';
import { useNavigate, useRouter } from '@tanstack/react-router';
import { IconArrowRightDashed, IconChevronRight, IconDeviceLaptop, IconMoon, IconSun } from '@tabler/icons-react';
import { useTranslation } from 'react-i18next';
import { useSearch } from '@/context/search-context';
import { useTheme } from '@/context/theme-context';
import {
  CommandDialog,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
  CommandSeparator,
} from '@/components/ui/command';
import { useSidebarData } from '../sidebar';
import { ScrollArea } from './ui/scroll-area';

// Inner component that uses sidebar data
function CommandMenuContent({
  open,
  setOpen,
}: {
  open: boolean;
  setOpen: React.Dispatch<React.SetStateAction<boolean>>;
}) {
  const navigate = useNavigate();
  const { setTheme } = useTheme();
  const sidebarData = useSidebarData();
  const { t } = useTranslation();

  const runCommand = React.useCallback(
    (command: () => unknown) => {
      setOpen(false);
      command();
    },
    [setOpen]
  );

  return (
    <CommandDialog modal open={open} onOpenChange={setOpen}>
      <CommandInput placeholder={t('command.placeholder')} />
      <CommandList>
        <ScrollArea type='hover' className='h-72 pr-1'>
          <CommandEmpty>{t('command.noResults')}</CommandEmpty>
          {sidebarData.navGroups.map((group) => (
            <CommandGroup key={group.title} heading={group.title}>
              {group.items.map((navItem, i) => {
                if (navItem.url)
                  return (
                    <CommandItem
                      key={`${navItem.url}-${i}`}
                      value={navItem.title}
                      onSelect={() => {
                        runCommand(() => navigate({ to: navItem.url }));
                      }}
                    >
                      <div className='mr-2 flex h-4 w-4 items-center justify-center'>
                        <IconArrowRightDashed className='text-muted-foreground/80 size-2' />
                      </div>
                      {navItem.title}
                    </CommandItem>
                  );

                return navItem.items?.map((subItem, i) => (
                  <CommandItem
                    key={`${navItem.title}-${subItem.url}-${i}`}
                    value={`${navItem.title}-${subItem.url}`}
                    onSelect={() => {
                      runCommand(() => navigate({ to: subItem.url }));
                    }}
                  >
                    <div className='mr-2 flex h-4 w-4 items-center justify-center'>
                      <IconArrowRightDashed className='text-muted-foreground/80 size-2' />
                    </div>
                    {navItem.title} <IconChevronRight /> {subItem.title}
                  </CommandItem>
                ));
              })}
            </CommandGroup>
          ))}
          <CommandSeparator />
          <CommandGroup heading={t('command.theme')}>
            <CommandItem onSelect={() => runCommand(() => setTheme('light'))}>
              <IconSun /> <span>{t('theme.light')}</span>
            </CommandItem>
            <CommandItem onSelect={() => runCommand(() => setTheme('dark'))}>
              <IconMoon className='scale-90' />
              <span>{t('theme.dark')}</span>
            </CommandItem>
            <CommandItem onSelect={() => runCommand(() => setTheme('system'))}>
              <IconDeviceLaptop />
              <span>{t('theme.system')}</span>
            </CommandItem>
          </CommandGroup>
        </ScrollArea>
      </CommandList>
    </CommandDialog>
  );
}

export function CommandMenu() {
  const router = useRouter();
  const { open, setOpen } = useSearch();

  // Don't render if router is not available
  if (!router) {
    return null;
  }

  // Don't render on auth pages (sign-in, sign-up, initialization, etc.)
  const currentPath = window.location.pathname;
  if (
    currentPath.startsWith('/sign-in') ||
    currentPath.startsWith('/sign-up') ||
    currentPath.startsWith('/initialization') ||
    currentPath.startsWith('/forgot-password') ||
    currentPath.startsWith('/otp')
  ) {
    return null;
  }

  return <CommandMenuContent open={open} setOpen={setOpen} />;
}
