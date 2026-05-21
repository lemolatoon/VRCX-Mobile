import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import { RouterProvider, createRouter } from '@tanstack/react-router';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';

import { routeTree } from './routeTree.gen';
import { initI18n } from './i18n';
import { useAuthStore } from './stores/auth';
import './styles/index.css';

const queryClient = new QueryClient({
    defaultOptions: {
        queries: {
            staleTime: 30_000,
            retry: 1
        }
    }
});

const router = createRouter({
    routeTree,
    context: {},
    defaultPreload: 'intent'
});

declare module '@tanstack/react-router' {
    interface Register {
        router: typeof router;
    }
}

async function bootstrap() {
    await initI18n();
    await useAuthStore.getState().restoreSession();

    const root = document.getElementById('root');
    if (!root) throw new Error('No #root element');

    createRoot(root).render(
        <StrictMode>
            <QueryClientProvider client={queryClient}>
                <RouterProvider router={router} />
            </QueryClientProvider>
        </StrictMode>
    );
}

bootstrap().catch(console.error);
