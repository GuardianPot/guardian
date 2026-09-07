import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { RouterProvider } from 'react-router';
import { router } from '@app/router';
import { AuthProvider } from '@features/auth';
import { RootErrorBoundary } from '@shared/ui';
import '@shared/styles/global.css';

/*
 * No cadence lives here. Every read declares a freshness class in its feature
 * API module, and `@shared/api/freshness` owns what a class means (WCX-07
 * section 9.1). A default `staleTime` here would silently override a class for
 * any query that forgot to declare one, which is exactly the drift this
 * package exists to remove.
 */
const queryClient = new QueryClient({
  defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
});

// The root boundary wraps the router, so an exception anywhere below it leaves
// a page that still names the product and offers a reload, rather than the
// blank document `P1-W11` GAP-2 recorded. It renders inside `#root`, so the
// document language and the theme stylesheet are never unmounted.
createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <RootErrorBoundary>
      <QueryClientProvider client={queryClient}>
        <AuthProvider><RouterProvider router={router} /></AuthProvider>
      </QueryClientProvider>
    </RootErrorBoundary>
  </StrictMode>,
);
