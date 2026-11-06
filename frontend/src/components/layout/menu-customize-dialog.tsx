import { useMemo } from 'react';
import { IconEye, IconEyeOff, IconRotate } from '@tabler/icons-react';
import { useRawNavModules } from '@/sidebar';
import { useTranslation } from 'react-i18next';
import { useSidebarPrefsStore } from '@/stores/sidebarPrefsStore';
import { useRoutePermissions } from '@/hooks/useRoutePermissions';
import { Button } from '@/components/ui/button';
import { Checkbox } from '@/components/ui/checkbox';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Label } from '@/components/ui/label';
import { ScrollArea } from '@/components/ui/scroll-area';
import { type NavLink, type NavModule } from '@/components/layout/types';

interface MenuCustomizeDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

export function MenuCustomizeDialog({ open, onOpenChange }: MenuCustomizeDialogProps) {
  const { t } = useTranslation();
  const rawNavModules = useRawNavModules();
  const { filterNavGroups } = useRoutePermissions();
  const hiddenItems = useSidebarPrefsStore((state) => state.hiddenItems);
  const toggleItem = useSidebarPrefsStore((state) => state.toggleItem);
  const reset = useSidebarPrefsStore((state) => state.reset);

  // Only offer items the current user can actually reach.
  const modules = useMemo<NavModule[]>(
    () =>
      rawNavModules
        .map((module) => ({
          ...module,
          groups: filterNavGroups(module.groups)
            .map((group) => ({ ...group, items: group.items.filter((item) => !item.isDisabled) }))
            .filter((group) => group.items.length > 0),
        }))
        .filter((module) => module.groups.length > 0),
    [filterNavGroups, rawNavModules]
  );

  const hasItems = modules.length > 0;

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className='sm:max-w-md'>
        <DialogHeader>
          <DialogTitle>{t('sidebar.menuCustomize.title')}</DialogTitle>
          <DialogDescription>{t('sidebar.menuCustomize.description')}</DialogDescription>
        </DialogHeader>

        {hasItems ? (
          <ScrollArea className='max-h-[60vh] pr-2'>
            <div className='space-y-5'>
              {modules.map((module) => (
                <div key={module.id} className='space-y-3'>
                  <p className='flex items-center gap-1.5 text-sm font-semibold'>
                    <module.icon className='size-4' />
                    {module.title}
                  </p>
                  {module.groups.map((group) => (
                    <div key={`${module.id}-${group.id ?? group.title}`} className='space-y-1.5'>
                      {group.title && (
                        <p className='text-muted-foreground text-xs font-medium tracking-wide uppercase'>{group.title}</p>
                      )}
                      {group.items.map((item) => {
                        if (!('url' in item)) {
                          return null;
                        }

                        const link = item as NavLink;
                        const visible = !hiddenItems.includes(link.url as string);
                        const Icon = link.icon;

                        return (
                          <Label
                            key={`${group.title}-${link.url}`}
                            htmlFor={`nav-visible-${link.url}`}
                            className='hover:bg-accent/50 flex cursor-pointer items-center gap-3 rounded-md px-2 py-1.5 font-normal'
                          >
                            <Checkbox
                              id={`nav-visible-${link.url}`}
                              checked={visible}
                              onCheckedChange={() => toggleItem(link.url as string)}
                            />
                            {Icon && <Icon className='text-muted-foreground text-lg' />}
                            <span className='flex-1 text-sm'>{link.title}</span>
                            {visible ? (
                              <IconEye className='text-muted-foreground size-4' />
                            ) : (
                              <IconEyeOff className='text-muted-foreground size-4' />
                            )}
                          </Label>
                        );
                      })}
                    </div>
                  ))}
                </div>
              ))}
            </div>
          </ScrollArea>
        ) : (
          <p className='text-muted-foreground py-6 text-center text-sm'>{t('sidebar.menuCustomize.empty')}</p>
        )}

        <DialogFooter className='flex-row justify-between gap-2 sm:justify-between'>
          <Button type='button' variant='ghost' size='sm' onClick={reset} disabled={hiddenItems.length === 0}>
            <IconRotate className='mr-1 size-4' />
            {t('sidebar.menuCustomize.reset')}
          </Button>
          <Button type='button' size='sm' onClick={() => onOpenChange(false)}>
            {t('sidebar.menuCustomize.done')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
