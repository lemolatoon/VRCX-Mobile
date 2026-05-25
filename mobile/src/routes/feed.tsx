import { useState, useEffect, useRef } from 'react';
import { createFileRoute } from '@tanstack/react-router';
import { useInfiniteQuery, useQuery } from '@tanstack/react-query';
import { useTranslation } from 'react-i18next';
import { cn } from '@/lib/utils';
import { parseLocation, getLocationText, isRealInstance } from '@/lib/vrcLocation';
import { fetchWorld, fetchGroup } from '@/api/auth';
import { fetchFeedPage, type FeedType, type FeedEntry } from '@/api/feed';
import { useWorldModal } from '@/stores/worldModal';

// ── Type metadata ───────────────────────────────────────────────────────────

const FEED_TYPES: FeedType[] = ['GPS', 'Online', 'Offline', 'Status', 'Avatar', 'Bio'];

// Colored badge classes per type — matches VRCX's feed type color scheme
const TYPE_BADGE: Record<FeedType, string> = {
    GPS:     'bg-blue-100 text-blue-700 dark:bg-blue-900/40 dark:text-blue-300',
    Online:  'bg-green-100 text-green-700 dark:bg-green-900/40 dark:text-green-300',
    Offline: 'bg-muted text-muted-foreground',
    Status:  'bg-yellow-100 text-yellow-700 dark:bg-yellow-900/40 dark:text-yellow-300',
    Avatar:  'bg-purple-100 text-purple-700 dark:bg-purple-900/40 dark:text-purple-300',
    Bio:     'bg-orange-100 text-orange-700 dark:bg-orange-900/40 dark:text-orange-300',
};

// ── World/group name resolution ─────────────────────────────────────────────

const QUERY_OPTS = {
    staleTime: Infinity,
    gcTime: 30 * 60 * 1000,
    refetchOnWindowFocus: false,
    refetchOnReconnect: false,
    retry: false,
} as const;

function useWorldName(worldId: string | undefined) {
    return useQuery({
        queryKey: ['vrc-world', worldId],
        queryFn: () => fetchWorld(worldId!),
        enabled: !!worldId,
        ...QUERY_OPTS,
    });
}

function useGroupName(groupId: string | null | undefined) {
    return useQuery({
        queryKey: ['vrc-group', groupId],
        queryFn: () => fetchGroup(groupId!),
        enabled: !!groupId,
        ...QUERY_OPTS,
    });
}

/** Renders a single resolved location string (world name + access type + region). Tappable when real instance. */
function ResolvedLocation({ location }: { location: string }) {
    const { open } = useWorldModal();
    const parsed = parseLocation(location);
    const { data: world } = useWorldName(parsed.isRealInstance ? parsed.worldId : undefined);
    const { data: group } = useGroupName(parsed.groupId);
    const text = getLocationText(parsed, {
        worldName: world?.name,
        groupName: group?.name,
    });
    if (isRealInstance(location)) {
        return (
            <span
                role="button"
                tabIndex={0}
                className="underline decoration-dotted cursor-pointer hover:text-primary transition-colors"
                onClick={(e) => { e.stopPropagation(); open(location); }}
                onKeyDown={(e) => { if (e.key === 'Enter' || e.key === ' ') { e.stopPropagation(); open(location); } }}
            >
                {text}
            </span>
        );
    }
    return <>{text}</>;
}

// ── Entry row renderers ─────────────────────────────────────────────────────

const subtitleCls = 'text-xs text-muted-foreground truncate';

function GPSRow({ entry }: { entry: FeedEntry }) {
    const p = entry.payload as { location?: string; previousLocation?: string };
    return (
        <p className={subtitleCls}>
            {p.previousLocation && <><ResolvedLocation location={p.previousLocation} /> → </>}
            {p.location && <ResolvedLocation location={p.location} />}
        </p>
    );
}

function OnlineOfflineRow({ entry }: { entry: FeedEntry }) {
    const p = entry.payload as { location?: string };
    if (!p.location) return null;
    return (
        <p className={subtitleCls}>
            <ResolvedLocation location={p.location} />
        </p>
    );
}

function StatusRow({ entry }: { entry: FeedEntry }) {
    const p = entry.payload as { status?: string; previousStatus?: string; statusDescription?: string };
    return (
        <p className={subtitleCls}>
            {p.previousStatus && <>{p.previousStatus} → </>}{p.status}
            {p.statusDescription && <> · {p.statusDescription}</>}
        </p>
    );
}

function AvatarRow({ entry }: { entry: FeedEntry }) {
    const p = entry.payload as { avatarName?: string };
    return (
        <p className={subtitleCls}>
            {p.avatarName || 'Avatar changed'}
        </p>
    );
}

function BioRow({ entry }: { entry: FeedEntry }) {
    const p = entry.payload as { bio?: string };
    return (
        <p className={subtitleCls}>
            {p.bio?.slice(0, 80)}
        </p>
    );
}

function EntrySubtitle({ entry }: { entry: FeedEntry }) {
    switch (entry.type) {
        case 'GPS': return <GPSRow entry={entry} />;
        case 'Online':
        case 'Offline': return <OnlineOfflineRow entry={entry} />;
        case 'Status': return <StatusRow entry={entry} />;
        case 'Avatar': return <AvatarRow entry={entry} />;
        case 'Bio': return <BioRow entry={entry} />;
        default: return null;
    }
}

// ── Feed entry card ─────────────────────────────────────────────────────────

function FeedEntryCard({ entry }: { entry: FeedEntry }) {
    const badge = TYPE_BADGE[entry.type];
    const p = entry.payload as { displayName?: string };
    const time = formatAbsTime(new Date(entry.created_at));

    return (
        <div className="px-4 py-3 border-b border-border">
            <div className="flex items-center gap-2">
                <span className={cn('shrink-0 px-1.5 py-0.5 rounded text-xs font-semibold', badge)}>
                    {entry.type}
                </span>
                <span className="text-sm font-medium truncate flex-1 min-w-0">{p.displayName}</span>
                <span className="text-xs text-muted-foreground shrink-0">{time}</span>
            </div>
            <div className="mt-0.5 min-w-0 overflow-hidden">
                <EntrySubtitle entry={entry} />
            </div>
        </div>
    );
}

// ── Filter pills ────────────────────────────────────────────────────────────

function FilterPills({
    active,
    onToggle,
}: {
    active: Set<FeedType>;
    onToggle: (t: FeedType) => void;
}) {
    const { t } = useTranslation();
    return (
        <div className="flex gap-1.5 overflow-x-auto px-4 py-2 shrink-0">
            {FEED_TYPES.map((type) => (
                <button
                    key={type}
                    onClick={() => onToggle(type)}
                    className={cn(
                        'shrink-0 px-3 py-1 rounded-full text-xs font-medium border transition-colors',
                        active.has(type)
                            ? 'bg-primary text-primary-foreground border-primary'
                            : 'bg-background text-muted-foreground border-border hover:border-foreground'
                    )}
                >
                    {t(`view.feed.filters.${type}`, { defaultValue: type })}
                </button>
            ))}
        </div>
    );
}

// ── Main page ───────────────────────────────────────────────────────────────

function FeedPage() {
    const { t } = useTranslation();
    const [activeTypes, setActiveTypes] = useState<Set<FeedType>>(new Set());
    const bottomRef = useRef<HTMLDivElement>(null);

    const types = activeTypes.size > 0 ? [...activeTypes] : undefined;

    const {
        data,
        fetchNextPage,
        hasNextPage,
        isFetchingNextPage,
        isLoading,
        isError,
    } = useInfiniteQuery({
        queryKey: ['feed', types],
        queryFn: ({ pageParam }) =>
            fetchFeedPage({ types, before: pageParam as string | null | undefined }),
        initialPageParam: null as string | null,
        getNextPageParam: (last) => last.next_cursor ?? undefined,
        refetchInterval: 30_000,
        refetchOnWindowFocus: false,
        refetchOnReconnect: false,
    });

    // Infinite scroll via IntersectionObserver
    useEffect(() => {
        const el = bottomRef.current;
        if (!el) return;
        const obs = new IntersectionObserver(
            ([entry]) => { if (entry.isIntersecting && hasNextPage && !isFetchingNextPage) fetchNextPage(); },
            { threshold: 0.1 }
        );
        obs.observe(el);
        return () => obs.disconnect();
    }, [hasNextPage, isFetchingNextPage, fetchNextPage]);

    const entries = data?.pages.flatMap((p) => p.entries) ?? [];

    function toggleType(type: FeedType) {
        setActiveTypes((prev) => {
            const next = new Set(prev);
            if (next.has(type)) next.delete(type);
            else next.add(type);
            return next;
        });
    }

    return (
        <div className="flex flex-col h-full overflow-hidden">
            <FilterPills active={activeTypes} onToggle={toggleType} />

            <div className="flex-1 overflow-y-auto">
                {isLoading && (
                    <div className="flex items-center justify-center h-32">
                        <span className="text-sm text-muted-foreground">
                            {t('mobile.common.loading', { defaultValue: 'Loading…' })}
                        </span>
                    </div>
                )}

                {isError && (
                    <div className="flex items-center justify-center h-32">
                        <span className="text-sm text-destructive">
                            {t('mobile.common.error', { defaultValue: 'Failed to load feed.' })}
                        </span>
                    </div>
                )}

                {!isLoading && !isError && entries.length === 0 && (
                    <div className="flex items-center justify-center h-32">
                        <span className="text-sm text-muted-foreground">
                            {t('mobile.feed.empty', { defaultValue: 'No events yet' })}
                        </span>
                    </div>
                )}

                {entries.map((entry) => (
                    <FeedEntryCard key={`${entry.type}-${entry.id}`} entry={entry} />
                ))}

                {/* Sentinel for IntersectionObserver */}
                <div ref={bottomRef} className="h-8 flex items-center justify-center">
                    {isFetchingNextPage && (
                        <span className="text-xs text-muted-foreground">
                            {t('common.load_more', { defaultValue: 'Load more' })}…
                        </span>
                    )}
                </div>
            </div>
        </div>
    );
}

// ── Helpers ─────────────────────────────────────────────────────────────────

function formatAbsTime(d: Date): string {
    const p = (n: number) => String(n).padStart(2, '0');
    const md = `${p(d.getMonth() + 1)}-${p(d.getDate())} ${p(d.getHours())}:${p(d.getMinutes())}`;
    return d.getFullYear() === new Date().getFullYear() ? md : `${d.getFullYear()}-${md}`;
}

export const Route = createFileRoute('/feed')({
    component: FeedPage
});
