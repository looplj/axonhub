import React from 'react';
import { createRoot } from 'react-dom/client';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { createRootRoute, createRouter, RouterProvider } from '@tanstack/react-router';
import { I18nextProvider } from 'react-i18next';
import i18n from '@/lib/i18n';
import { ThemeProvider, useTheme } from '@/context/theme-context';
import { useProjectStore } from '@/stores/projectStore';
import { PluginFrame } from '@/features/plugins/components/plugin-frame';
import { PluginHeaderActions } from '@/features/plugins/components/plugin-header-actions';

const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
useProjectStore.getState().setSelectedProjectId('gid://axonhub/Project/17');
const mobile = new URLSearchParams(location.search).has('mobile');
const withPage = new URLSearchParams(location.search).has('withPage');
function TestRoot() {
  const { setTheme } = useTheme();
  window.__pluginTest.theme = setTheme;
  return <><PluginHeaderActions isMobile={mobile} /><p>Host content</p>{withPage && <PluginFrame pluginId='example.actions' pageKey='main' entry='ui/action.html' />}</>;
}
const rootRoute = createRootRoute({ component: TestRoot });
const router = createRouter({ routeTree: rootRoute, history: undefined });
// Use the harness URL as the root route so the test does not need the application router.
window.history.replaceState({}, '', '/');
window.__pluginTest = {
  remove: () => client.setQueryData(['pluginUIEntries'], { pluginUIEntries: { pages: [], actions: [] } }),
  locale: () => i18n.changeLanguage('zh'),
  project: () => useProjectStore.getState().setSelectedProjectId('gid://axonhub/Project/99'),
};
createRoot(document.getElementById('root')!).render(
  <QueryClientProvider client={client}><I18nextProvider i18n={i18n}><ThemeProvider><RouterProvider router={router} /></ThemeProvider></I18nextProvider></QueryClientProvider>
);
