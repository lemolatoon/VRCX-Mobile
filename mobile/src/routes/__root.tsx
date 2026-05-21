import { createRootRoute, Outlet, redirect } from '@tanstack/react-router';
import { AppShell } from '@/components/AppShell';
import { useAuthStore } from '@/stores/auth';

export const Route = createRootRoute({
    beforeLoad: ({ location }) => {
        const { currentUser, isRestoring } = useAuthStore.getState();
        if (isRestoring) return;
        if (!currentUser && location.pathname !== '/login') {
            throw redirect({ to: '/login' });
        }
        if (currentUser && location.pathname === '/login') {
            throw redirect({ to: '/friends' });
        }
    },
    component: () => <AppShell><Outlet /></AppShell>
});
