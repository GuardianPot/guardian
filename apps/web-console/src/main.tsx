import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { RouterProvider } from 'react-router-dom';
import { router } from '@app/router';
import { AuthProvider } from '@features/auth';
import '@shared/styles/global.css';

const queryClient = new QueryClient({
  defaultOptions: { queries: { retry: false, staleTime: 5_000 }, mutations: { retry: false } },
});

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <QueryClientProvider client={queryClient}>
      <AuthProvider><RouterProvider router={router} /></AuthProvider>
    </QueryClientProvider>
  </StrictMode>,
);
