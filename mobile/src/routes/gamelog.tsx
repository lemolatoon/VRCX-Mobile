import { useEffect, useRef, useState } from 'react';
import { createFileRoute } from '@tanstack/react-router';
import { useInfiniteQuery } from '@tanstack/react-query';
import { Search, X } from 'lucide-react';
import { cn } from '@/lib/utils';
import { fetchGameLogPage, ALL_GAMELOG_TYPES, type GameLogEntry, type GameLogType } from '@/api/gamelog';
import { parseLocation, getLocationText, isRealInstance } from '@/lib/vrcLocation';
import { useWorldModal } from '@/stores/worldModal';

const TYPES: GameLogType[] = ['Location', 'OnPlayerJoined', 'OnPlayerLeft', 'VideoPlay', 'ResourceLoad', 'Event', 'Unknown'];

const TYPE_BADGE: Record<GameLogType, string> = {
    Location: 'bg-blue-100 text-blue-700 dark:bg-blue-900/40 dark:text-blue-300',
    LocationDestination: 'bg-sky-100 text-sky-700 dark:bg-sky-900/40 dark:text-sky-300',
    OnPlayerJoined: 'bg-green-100 text-green-700 dark:bg-green-900/40 dark:text-green-300',
    OnPlayerLeft: 'bg-muted text-muted-foreground',
    PortalSpawn: 'bg-cyan-100 text-cyan-700 dark:bg-cyan-900/40 dark:text-cyan-300',
    VideoPlay: 'bg-purple-100 text-purple-700 dark:bg-purple-900/40 dark:text-purple-300',
    ResourceLoad: 'bg-orange-100 text-orange-700 dark:bg-orange-900/40 dark:text-orange-300',
    Event: 'bg-yellow-100 text-yellow-700 dark:bg-yellow-900/40 dark:text-yellow-300',
    Unknown: 'bg-zinc-100 text-zinc-700 dark:bg-zinc-800 dark:text-zinc-300',
};

function ResolvedLocation({ location }: { location: string }) {
    const { open } = useWorldModal();
    const parsed = parseLocation(location);
    const text = getLocationText(parsed);
    if (isRealInstance(location)) {
        return (
            <button
                type="button"
                className="underline decoration-dotted text-left hover:text-primary"
                onClick={(e) => { e.stopPropagation(); open(location); }}
            >
                {text}
            </button>
        );
    }
    return <>{text}</>;
}

function EntryBody({ entry }: { entry: GameLogEntry }) {
    const p = entry.payload;
    if (entry.type === 'Location') {
        const location = String(p.location ?? '');
        return (
            <p className="text-xs text-muted-foreground truncate">
                {location ? <ResolvedLocation location={location} /> : 'Location changed'}
                {p.worldName ? <> · {String(p.worldName)}</> : null}
            </p>
        );
    }
    if (entry.type === 'OnPlayerJoined' || entry.type === 'OnPlayerLeft') {
        return <p className="text-xs text-muted-foreground truncate">{String(p.displayName ?? p.userId ?? '')}</p>;
    }
    if (entry.type === 'VideoPlay') {
        return <p className="text-xs text-muted-foreground truncate">{String(p.videoUrl ?? '')}</p>;
    }
    if (entry.type === 'ResourceLoad') {
        return <p className="text-xs text-muted-foreground truncate">{String(p.resourceType ?? 'Resource')} · {String(p.resourceUrl ?? '')}</p>;
    }
    if (entry.type === 'Unknown') {
        return <pre className="mt-1 whitespace-pre-wrap break-words text-xs text-muted-foreground font-mono">{entry.raw_line || String(p.message ?? '')}</pre>;
    }
    return <p className="text-xs text-muted-foreground truncate">{String(p.message ?? entry.raw_line ?? '')}</p>;
}

function GameLogCard({ entry }: { entry: GameLogEntry }) {
    return (
        <div className="px-4 py-3 border-b border-border">
            <div className="flex items-center gap-2">
                <span className={cn('shrink-0 px-1.5 py-0.5 rounded text-xs font-semibold', TYPE_BADGE[entry.type])}>
                    {entry.type}
                </span>
                <span className="text-xs text-muted-foreground shrink-0 ml-auto">{formatAbsTime(new Date(entry.created_at))}</span>
            </div>
            <div className="mt-1 min-w-0 overflow-hidden">
                <EntryBody entry={entry} />
            </div>
        </div>
    );
}

function SearchBox({ value, onChange }: { value: string; onChange: (v: string) => void }) {
    return (
        <div className="relative flex items-center px-4 pt-2 shrink-0">
            <Search className="absolute left-7 w-4 h-4 text-muted-foreground pointer-events-none" />
            <input
                type="text"
                value={value}
                onChange={(e) => onChange(e.target.value)}
                placeholder="Search GameLog"
                className="w-full pl-9 pr-9 py-2 bg-input border border-border rounded-md text-sm focus:outline-none focus:ring-2 focus:ring-ring"
            />
            {value && (
                <button type="button" onClick={() => onChange('')} className="absolute right-7 text-muted-foreground hover:text-foreground">
                    <X className="w-4 h-4" />
                </button>
            )}
        </div>
    );
}

function GameLogPage() {
    const [activeTypes, setActiveTypes] = useState<Set<GameLogType>>(new Set());
    const [searchText, setSearchText] = useState('');
    const [debouncedSearch, setDebouncedSearch] = useState('');
    const bottomRef = useRef<HTMLDivElement>(null);

    useEffect(() => {
        const timer = setTimeout(() => setDebouncedSearch(searchText.trim()), 300);
        return () => clearTimeout(timer);
    }, [searchText]);

    // Default: all types except Unknown (noise/fallthrough lines). Unknown is shown
    // only when the user explicitly selects that chip.
    const DEFAULT_TYPES = ALL_GAMELOG_TYPES.filter((t) => t !== 'Unknown');
    const types = activeTypes.size > 0 ? [...activeTypes] : DEFAULT_TYPES;
    const { data, fetchNextPage, hasNextPage, isFetchingNextPage, isLoading, isError } = useInfiniteQuery({
        queryKey: ['gamelog', types, debouncedSearch],
        queryFn: ({ pageParam }) => fetchGameLogPage({ types, search: debouncedSearch || undefined, before: pageParam as string | null | undefined }),
        initialPageParam: null as string | null,
        getNextPageParam: (last) => last.next_cursor ?? undefined,
        refetchInterval: 10_000,
        refetchOnWindowFocus: false,
    });

    useEffect(() => {
        const el = bottomRef.current;
        if (!el) return;
        const obs = new IntersectionObserver(([entry]) => {
            if (entry.isIntersecting && hasNextPage && !isFetchingNextPage) fetchNextPage();
        }, { threshold: 0.1 });
        obs.observe(el);
        return () => obs.disconnect();
    }, [hasNextPage, isFetchingNextPage, fetchNextPage]);

    const entries = data?.pages.flatMap((p) => p.entries) ?? [];

    return (
        <div className="flex flex-col h-full overflow-hidden">
            <SearchBox value={searchText} onChange={setSearchText} />
            <div className="flex gap-1.5 overflow-x-auto px-4 py-2 shrink-0">
                {TYPES.map((type) => (
                    <button
                        key={type}
                        onClick={() => setActiveTypes((prev) => {
                            const next = new Set(prev);
                            if (next.has(type)) next.delete(type);
                            else next.add(type);
                            return next;
                        })}
                        className={cn(
                            'shrink-0 px-3 py-1 rounded-full text-xs font-medium border transition-colors',
                            activeTypes.has(type) ? 'bg-primary text-primary-foreground border-primary' : 'bg-background text-muted-foreground border-border'
                        )}
                    >
                        {type}
                    </button>
                ))}
            </div>
            <div className="flex-1 overflow-y-auto">
                {isLoading && <div className="flex items-center justify-center h-32 text-sm text-muted-foreground">Loading...</div>}
                {isError && <div className="flex items-center justify-center h-32 text-sm text-destructive">Failed to load GameLog.</div>}
                {!isLoading && !isError && entries.length === 0 && <div className="flex items-center justify-center h-32 text-sm text-muted-foreground">No GameLog entries yet</div>}
                {entries.map((entry) => <GameLogCard key={`${entry.type}-${entry.id}`} entry={entry} />)}
                <div ref={bottomRef} className="h-8 flex items-center justify-center">
                    {isFetchingNextPage && <span className="text-xs text-muted-foreground">Load more...</span>}
                </div>
            </div>
        </div>
    );
}

function formatAbsTime(d: Date): string {
    const p = (n: number) => String(n).padStart(2, '0');
    const md = `${p(d.getMonth() + 1)}-${p(d.getDate())} ${p(d.getHours())}:${p(d.getMinutes())}`;
    return d.getFullYear() === new Date().getFullYear() ? md : `${d.getFullYear()}-${md}`;
}

export const Route = createFileRoute('/gamelog')({
    component: GameLogPage
});
