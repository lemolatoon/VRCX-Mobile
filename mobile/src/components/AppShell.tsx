import type { ReactNode } from 'react';
import { Link, useRouterState } from '@tanstack/react-router';
import { Bell, Search, Settings, Users, Rss, Logs } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { cn } from '@/lib/utils';
import { useAuthStore } from '@/stores/auth';
import { WorldDetailModal } from '@/components/WorldDetailModal';

const NAV_ITEMS = [
    { to: '/friends', icon: Users, labelKey: 'side_panel.friends', fallback: 'Friends' },
    { to: '/feed', icon: Rss, labelKey: 'side_panel.feed', fallback: 'Feed' },
    { to: '/gamelog', icon: Logs, labelKey: 'side_panel.game_log', fallback: 'GameLog' },
    { to: '/notifications', icon: Bell, labelKey: 'side_panel.notifications', fallback: 'Alerts' },
    { to: '/search', icon: Search, labelKey: 'side_panel.search', fallback: 'Search' },
    { to: '/settings', icon: Settings, labelKey: 'view.settings.header', fallback: 'Settings' }
] as const;

export function AppShell({ children }: { children: ReactNode }) {
    const { t } = useTranslation();
    const pathname = useRouterState({ select: (s) => s.location.pathname });
    const currentUser = useAuthStore((s) => s.currentUser);
    const isLoginPage = pathname === '/login';

    if (isLoginPage) {
        return <>{children}</>;
    }

    return (
        <div className="h-dvh overflow-hidden bg-background">
            {/* Header */}
            <header className="fixed inset-x-0 top-0 z-40 flex items-center justify-between px-4 py-3 border-b border-border bg-background"
                style={{ paddingTop: 'calc(0.75rem + env(safe-area-inset-top))' }}>
                <span className="text-sm font-semibold">VRCX Mobile</span>
                {currentUser && (
                    <span className="text-xs text-muted-foreground truncate max-w-[140px]">
                        {currentUser.displayName}
                    </span>
                )}
            </header>

            {/* Content */}
            <main
                className="fixed inset-x-0 overflow-hidden"
                style={{
                    top: 'calc(3rem + env(safe-area-inset-top))',
                    bottom: 'calc(3.75rem + env(safe-area-inset-bottom))'
                }}
            >
                {children}
            </main>

            {/* Bottom Tab Bar */}
            <nav
                className="fixed inset-x-0 bottom-0 z-40 flex border-t border-border bg-background"
                style={{ paddingBottom: 'env(safe-area-inset-bottom)' }}
            >
                {NAV_ITEMS.map(({ to, icon: Icon, labelKey, fallback }) => {
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
                            <span>{t(labelKey, { defaultValue: fallback })}</span>
                        </Link>
                    );
                })}
            </nav>

            <WorldDetailModal />
        </div>
    );
}
