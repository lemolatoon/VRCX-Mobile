import { useState, useEffect, useRef } from 'react';
import { createFileRoute } from '@tanstack/react-router';
import { useInfiniteQuery } from '@tanstack/react-query';
import { useTranslation } from 'react-i18next';
import { MapPin, UserCheck, UserX, Activity, PersonStanding, FileText } from 'lucide-react';
import { cn } from '@/lib/utils';
import { parseLocation, getLocationText } from '@/lib/vrcLocation';
import { fetchFeedPage, type FeedType, type FeedEntry } from '@/api/feed';

// ── Type metadata ───────────────────────────────────────────────────────────

const FEED_TYPES: FeedType[] = ['GPS', 'Online', 'Offline', 'Status', 'Avatar', 'Bio'];

const TYPE_ICON: Record<FeedType, React.FC<{ className?: string }>> = {
    GPS: MapPin,
    Online: UserCheck,
    Offline: UserX,
    Status: Activity,
    Avatar: PersonStanding,
    Bio: FileText,
};

const TYPE_COLOR: Record<FeedType, string> = {
    GPS: 'text-blue-500',
    Online: 'text-green-500',
    Offline: 'text-muted-foreground',
    Status: 'text-yellow-500',
    Avatar: 'text-purple-500',
    Bio: 'text-orange-500',
};

// ── Entry row renderers ─────────────────────────────────────────────────────

function GPSRow({ entry }: { entry: FeedEntry }) {
    const p = entry.payload as { displayName?: string; location?: string; previousLocation?: string };
    const locText = p.location ? getLocationText(parseLocation(p.location)) : '';
    const prevText = p.previousLocation ? getLocationText(parseLocation(p.previousLocation)) : '';
    return (
        <span className="text-xs text-muted-foreground truncate">
            {prevText && <>{prevText} → </>}{locText}
        </span>
    );
}

function OnlineOfflineRow({ entry }: { entry: FeedEntry }) {
    const p = entry.payload as { displayName?: string; location?: string; type?: string };
    const locText = p.location ? getLocationText(parseLocation(p.location)) : '';
    return (
        <span className="text-xs text-muted-foreground truncate">
            {locText || p.type}
        </span>
    );
}

function StatusRow({ entry }: { entry: FeedEntry }) {
    const p = entry.payload as { status?: string; previousStatus?: string; statusDescription?: string };
    return (
        <span className="text-xs text-muted-foreground truncate">
            {p.previousStatus && <>{p.previousStatus} → </>}{p.status}
            {p.statusDescription && <> · {p.statusDescription}</>}
        </span>
    );
}

function AvatarRow({ entry }: { entry: FeedEntry }) {
    const p = entry.payload as { avatarName?: string; currentAvatarThumbnailImageUrl?: string };
    return (
        <span className="text-xs text-muted-foreground truncate">
            {p.avatarName || 'Avatar changed'}
        </span>
    );
}

function BioRow({ entry }: { entry: FeedEntry }) {
    const p = entry.payload as { bio?: string };
    return (
        <span className="text-xs text-muted-foreground truncate">
            {p.bio?.slice(0, 80)}
        </span>
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
    const Icon = TYPE_ICON[entry.type];
    const color = TYPE_COLOR[entry.type];
    const p = entry.payload as { displayName?: string };
    const relTime = formatRelTime(new Date(entry.created_at));

    return (
        <div className="flex items-start gap-3 px-4 py-3 border-b border-border">
            <div className={cn('mt-0.5 shrink-0', color)}>
                <Icon className="w-4 h-4" />
            </div>
            <div className="flex-1 min-w-0">
                <div className="flex items-baseline justify-between gap-2">
                    <span className="text-sm font-medium truncate">{p.displayName}</span>
                    <span className="text-xs text-muted-foreground shrink-0">{relTime}</span>
                </div>
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

function formatRelTime(d: Date): string {
    const diffMs = Date.now() - d.getTime();
    const diffMin = Math.floor(diffMs / 60_000);
    if (diffMin < 1) return 'just now';
    if (diffMin < 60) return `${diffMin}m ago`;
    const diffH = Math.floor(diffMin / 60);
    if (diffH < 24) return `${diffH}h ago`;
    const diffD = Math.floor(diffH / 24);
    return `${diffD}d ago`;
}

export const Route = createFileRoute('/feed')({
    component: FeedPage
});
