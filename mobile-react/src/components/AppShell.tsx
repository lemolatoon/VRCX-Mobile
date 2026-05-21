import type { ReactNode } from 'react';
import { Link, useRouterState } from '@tanstack/react-router';
import { Bell, Search, Settings, Users, Rss } from 'lucide-react';
import { cn } from '@/lib/utils';
import { useAuthStore } from '@/stores/auth';

const NAV_ITEMS = [
    { to: '/friends', icon: Users, label: 'Friends' },
    { to: '/feed', icon: Rss, label: 'Feed' },
    { to: '/notifications', icon: Bell, label: 'Alerts' },
    { to: '/search', icon: Search, label: 'Search' },
    { to: '/settings', icon: Settings, label: 'Settings' }
] as const;

export function AppShell({ children }: { children: ReactNode }) {
    const pathname = useRouterState({ select: (s) => s.location.pathname });
    const currentUser = useAuthStore((s) => s.currentUser);
    const isLoginPage = pathname === '/login';

    if (isLoginPage) {
        return <>{children}</>;
    }

    return (
        <div className="flex flex-col h-dvh overflow-hidden">
            {/* Header */}
            <header className="flex items-center justify-between px-4 py-3 border-b border-border bg-background flex-shrink-0"
                style={{ paddingTop: 'calc(0.75rem + env(safe-area-inset-top))' }}>
                <span className="text-sm font-semibold">VRCX Mobile</span>
                {currentUser && (
                    <span className="text-xs text-muted-foreground truncate max-w-[140px]">
                        {currentUser.displayName}
                    </span>
                )}
            </header>

            {/* Content */}
            <main className="flex-1 overflow-hidden">
                {children}
            </main>

            {/* Bottom Tab Bar */}
            <nav
                className="flex border-t border-border bg-background flex-shrink-0"
                style={{ paddingBottom: 'env(safe-area-inset-bottom)' }}
            >
                {NAV_ITEMS.map(({ to, icon: Icon, label }) => {
                    const isActive = pathname === to;
                    return (
                        <Link
                            key={to}
                            to={to}
                            className={cn(
                                'flex-1 flex flex-col items-center gap-1 py-2.5 text-xs font-medium transition-colors',
                                isActive ? 'text-primary' : 'text-muted-foreground hover:text-foreground'
                            )}
                        >
                            <Icon className="w-5 h-5" />
                            <span>{label}</span>
                        </Link>
                    );
                })}
            </nav>
        </div>
    );
}
