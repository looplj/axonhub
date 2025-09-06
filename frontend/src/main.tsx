import { StrictMode } from 'react'
import ReactDOM from 'react-dom/client'
import {
  QueryCache,
  QueryClient,
  QueryClientProvider,
} from '@tanstack/react-query'
import { RouterProvider, createRouter } from '@tanstack/react-router'
import { toast } from 'sonner'
import { useAuthStore } from '@/stores/authStore'
import { handleServerError } from '@/utils/handle-server-error'
import { FontProvider } from './context/font-context'
import { ThemeProvider } from './context/theme-context'
import { SearchProvider } from './context/search-context'
import './index.css'
// Generated Routes
import { routeTree } from './routeTree.gen'
// Initialize i18n
import './lib/i18n'
import i18n from './lib/i18n'

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      retry: (failureCount, error) => {
        // eslint-disable-next-line no-console
        if (import.meta.env.DEV) console.log({ failureCount, error })

        if (failureCount >= 0 && import.meta.env.DEV) return false
        if (failureCount > 3 && import.meta.env.PROD) return false

        // For fetch API errors, we check if it's a Response object with status
        const status = error instanceof Response ? error.status : 
                      error && typeof error === 'object' && 'status' in error ? (error as any).status : 0;
        
        return ![401, 403,422].includes(status)
      },
      refetchOnWindowFocus: import.meta.env.PROD,
      staleTime: 10 * 1000, // 10s
    },
    mutations: {
      onError: (error) => {
        handleServerError(error)

        // For fetch API errors, we check if it's a Response object with status
        const status = error instanceof Response ? error.status : 
                      error && typeof error === 'object' && 'status' in error ? (error as any).status : 0;
        
        if (status === 304) {
          toast.error(i18n.t('common.errors.contentNotModified'))
        }
      },
    },
  },
  queryCache: new QueryCache({
    onError: (error) => {
      // For fetch API errors, we check if it's a Response object with status
      const status = error instanceof Response ? error.status : 
                    error && typeof error === 'object' && 'status' in error ? (error as any).status : 0;
      
      if (status === 401) {
        toast.error(i18n.t('common.errors.sessionExpired'))
        useAuthStore.getState().auth.reset()
        const redirect = `${router.history.location.href}`
        router.navigate({ to: '/sign-in', search: { redirect } })
      }
      if (status === 500) {
        toast.error(i18n.t('common.errors.internalServerError'))
        router.navigate({ to: '/500' })
      }
      if (status === 403) {
        // router.navigate("/forbidden", { replace: true });
      }
    },
  }),
})

// Create a new router instance
const router = createRouter({
  routeTree,
  context: { queryClient },
  defaultPreload: 'intent',
  defaultPreloadStaleTime: 0,
})

// Register the router instance for type safety
declare module '@tanstack/react-router' {
  interface Register {
    router: typeof router
  }
}

// Render the app
const rootElement = document.getElementById('root')!
if (!rootElement.innerHTML) {
  const root = ReactDOM.createRoot(rootElement)
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
  )
}
