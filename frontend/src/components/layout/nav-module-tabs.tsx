import { useNavigate, useRouter } from '@tanstack/react-router';
import { firstNavLink } from '@/sidebar';
import { cn } from '@/lib/utils';
import { useSidebarPrefsStore } from '@/stores/sidebarPrefsStore';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import { type NavLink, type NavModule } from './types';

interface NavModuleTabsProps {
  modules: NavModule[];
  activeModuleId: NavModule['id'] | null;
  // `header`: pill tabs that collapse to icons on narrow screens.
  // `sidebar`: full-width segmented control for the mobile sidebar.
  variant?: 'header' | 'sidebar';
  className?: string;
}

function matchesNavTarget(pathname: string, item: NavLink): boolean {
  const target = item.matchPath || String(item.url ?? '');
  if (!target) {
    return false;
  }
  if (target === '/') {
    return pathname === '/';
  }
  return pathname === target || pathname.startsWith(`${target}/`);
}

// A remembered page is only reused while it still belongs to a visible item of
// the module; otherwise the tab lands on the module's first item.
function isPathInModule(module: NavModule, path: string): boolean {
  const pathname = path.split(/[?#]/)[0];
  return module.groups.some((group) =>
    group.items.some((item) => !item.isDisabled && 'url' in item && matchesNavTarget(pathname, item as NavLink))
  );
}

export function NavModuleTabs({ modules, activeModuleId, variant = 'header', className }: NavModuleTabsProps) {
  const navigate = useNavigate();
  const router = useRouter();
  const lastModulePaths = useSidebarPrefsStore((state) => state.lastModulePaths);

  // A single module needs no switcher; the sidebar alone is enough.
  if (modules.length <= 1) {
    return null;
  }

  const openModule = (module: NavModule) => {
    const lastPath = lastModulePaths[module.id];
    if (lastPath && isPathInModule(module, lastPath)) {
      router.history.push(lastPath);
      return;
    }

    const link = firstNavLink(module);
    if (link) {
      void navigate({ to: link.url, ...(link.params ? { params: link.params } : {}) } as never);
    }
  };

  if (variant === 'sidebar') {
    return (
      <nav
        className={cn('bg-muted/60 grid gap-1 rounded-xl p-1', className)}
        style={{ gridTemplateColumns: `repeat(${modules.length}, minmax(0, 1fr))` }}
      >
        {modules.map((module) => {
          const isActive = module.id === activeModuleId;
          return (
            <button
              key={module.id}
              type='button'
              aria-current={isActive ? 'page' : undefined}
              onClick={() => openModule(module)}
              className={cn(
                'flex items-center justify-center gap-1.5 rounded-lg px-2 py-1.5 text-sm font-medium transition-colors',
                isActive ? 'bg-background text-foreground shadow-sm' : 'text-muted-foreground hover:text-foreground'
              )}
            >
              <module.icon className='size-4 shrink-0' />
              <span className='truncate'>{module.title}</span>
            </button>
          );
        })}
      </nav>
    );
  }

  return (
    <nav className={cn('flex items-center gap-1', className)}>
      {modules.map((module) => {
        const isActive = module.id === activeModuleId;
        return (
          <Tooltip key={module.id}>
            <TooltipTrigger asChild>
              <button
                type='button'
                aria-current={isActive ? 'page' : undefined}
                aria-label={module.title}
                onClick={() => openModule(module)}
                className={cn(
                  'inline-flex h-8 items-center gap-1.5 rounded-full px-3 text-sm font-medium transition-colors',
                  isActive ? 'bg-muted text-foreground' : 'text-muted-foreground hover:bg-accent/50 hover:text-foreground'
                )}
              >
                <module.icon className='size-4 shrink-0' />
                <span className='hidden lg:inline'>{module.title}</span>
              </button>
            </TooltipTrigger>
            <TooltipContent className='lg:hidden'>{module.title}</TooltipContent>
          </Tooltip>
        );
      })}
    </nav>
  );
}
