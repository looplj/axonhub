import { StrictMode } from 'react';
import ReactDOM from 'react-dom/client';
import { QueryCache, QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { RouterProvider, createRouter } from '@tanstack/react-router';
import { toast } from 'sonner';
import { useAuthStore } from '@/stores/authStore';
import { handleServerError } from '@/utils/handle-server-error';
import { FontProvider } from './context/font-context';
import { SearchProvider } from './context/search-context';
import { ThemeProvider } from './context/theme-context';
import './index.css';
// Initialize i18n
import './lib/i18n';
import i18n from './lib/i18n';
// Generated Routes
import { routeTree } from './routeTree.gen';


// A deploy replaces the hashed chunk files. A tab that is still running the
// previous build keeps asking for chunks that no longer exist, and the dynamic
// import rejects, which leaves the route stuck on a loading state. Reload once to
// pick up the new build. The flag is session-scoped and never cleared, so a tab
// can recover at most once instead of reloading in a loop when the build is gone.
const CHUNK_RELOAD_KEY = 'axonhub:chunk-reload-attempted';
const recoverFromStaleChunk = () => {
  if (sessionStorage.getItem(CHUNK_RELOAD_KEY)) return;
  sessionStorage.setItem(CHUNK_RELOAD_KEY, '1');
  window.location.reload();
};
window.addEventListener('vite:preloadError', () => recoverFromStaleChunk());
// Some dynamic imports fail without emitting vite:preloadError (for example a
// modulepreload hit served by a stale CDN entry). Treat those the same way.
const isChunkLoadFailure = (reason: unknown) =>
  /Failed to fetch dynamically imported module|Importing a module script failed|error loading dynamically imported module/i.test(
    String((reason as { message?: string })?.message ?? reason)
  );
window.addEventListener('unhandledrejection', (event) => {
  if (!isChunkLoadFailure(event.reason)) return;
  recoverFromStaleChunk();
});

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      retry: (failureCount, error) => {
        // eslint-disable-next-line no-console
        if (import.meta.env.DEV) console.log({ failureCount, error });

        if (import.meta.env.DEV) return false;
        if (failureCount > 2) return false;

        // For fetch API errors, we check if it's a Response object with status
        const status =
          error instanceof Response ? error.status : error && typeof error === 'object' && 'status' in error ? (error as any).status : 0;

        // Don't retry auth errors or server errors (500 hammers a failing backend)
        return ![401, 403, 422, 500].includes(status);
      },
      refetchOnWindowFocus: import.meta.env.PROD,
      staleTime: 10 * 1000, // 10s
    },
    mutations: {
      onError: (error) => {
        handleServerError(error);

        // For fetch API errors, we check if it's a Response object with status
        const status =
          error instanceof Response ? error.status : error && typeof error === 'object' && 'status' in error ? (error as any).status : 0;

        if (status === 304) {
          toast.error(i18n.t('common.errors.contentNotModified'));
        }
      },
    },
  },
  queryCache: new QueryCache({
    onError: (error) => {
      // For fetch API errors, we check if it's a Response object with status
      const status =
        error instanceof Response ? error.status : error && typeof error === 'object' && 'status' in error ? (error as any).status : 0;

      if (status === 401 && window.location.pathname !== '/sign-in' && window.location.pathname !== '/initialization') {
        toast.error(i18n.t('common.errors.sessionExpired'));
        useAuthStore.getState().auth.reset();
        const redirect = `${router.history.location.href}`;
        router.navigate({ to: '/sign-in', search: { redirect } });
      }
      if (status === 500) {
        toast.error(i18n.t('common.errors.internalServerError'));
        // router.navigate({ to: '/500' })
      }
    },
  }),
});

// Create a new router instance
const router = createRouter({
  routeTree,
  context: { queryClient },
  defaultPreload: 'intent',
  defaultPreloadStaleTime: 0,
});

// Register the router instance for type safety
declare module '@tanstack/react-router' {
  interface Register {
    router: typeof router;
  }
}

// Render the app
const rootElement = document.getElementById('root')!;
if (!rootElement.innerHTML) {
  const root = ReactDOM.createRoot(rootElement);
  root.render(
    <StrictMode>
      <QueryClientProvider client={queryClient}>
        <ThemeProvider defaultTheme='system' defaultColorScheme='claude'>
          <FontProvider>
            <SearchProvider>
              <RouterProvider router={router} />
            </SearchProvider>
          </FontProvider>
        </ThemeProvider>
      </QueryClientProvider>
    </StrictMode>
  );
}
