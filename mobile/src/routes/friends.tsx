import { createFileRoute } from '@tanstack/react-router';
import { useQuery } from '@tanstack/react-query';
import { RefreshCw, Users, X } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { useMemo, useState } from 'react';
import { fetchFriends, fetchWorld, fetchGroup } from '@/api/auth';
import type { VrcCurrentUser } from '@/types/vrc';
import { parseLocation, isRealInstance, getLocationText } from '@/lib/vrcLocation';

export const Route = createFileRoute('/friends')({
    component: FriendsPage
});

const FRIENDS_REFRESH_INTERVAL_MS = 60 * 60 * 1000;
const ROBOT_AVATAR_URL = 'https://api.vrchat.cloud/api/1/file/file_0e8c4e32-7444-44ea-ade4-313c010d4bae/1/file';
const WORLD_CACHE_MS = 30 * 60 * 1000;

// VRCX default: alphabetical by displayName (localeCompare, case-insensitive)
const sortByName = (a: VrcCurrentUser, b: VrcCurrentUser) =>
    a.displayName.localeCompare(b.displayName, undefined, { sensitivity: 'base' });

function friendImage(friend: VrcCurrentUser): string {
    const image =
        friend.profilePicOverrideThumbnail?.replace('/256', '/128') ||
        friend.profilePicOverride ||
        friend.currentAvatarThumbnailImageUrl?.replace('/256', '/128') ||
        friend.currentAvatarImageUrl ||
        '';
    return image === ROBOT_AVATAR_URL ? '' : image;
}

function statusColor(status: VrcCurrentUser['status'], state: VrcCurrentUser['state']): string {
    if (state === 'offline') return 'var(--status-offline)';
    switch (status) {
        case 'join me': return 'var(--status-joinme)';
        case 'active': return 'var(--status-online)';
        case 'ask me': return 'var(--status-askme)';
        case 'busy': return 'var(--status-busy)';
        default: return 'var(--status-online)';
    }
}

function useResolvedLocation(worldId: string | undefined, groupId: string | null | undefined) {
    const { data: world } = useQuery({
        queryKey: ['vrc-world', worldId],
        queryFn: () => fetchWorld(worldId!),
        enabled: !!worldId,
        staleTime: Infinity,
        gcTime: WORLD_CACHE_MS,
        refetchOnReconnect: false,
        refetchOnWindowFocus: false,
        retry: false
    });
    const { data: group } = useQuery({
        queryKey: ['vrc-group', groupId],
        queryFn: () => fetchGroup(groupId!),
        enabled: !!groupId,
        staleTime: Infinity,
        gcTime: WORLD_CACHE_MS,
        refetchOnReconnect: false,
        refetchOnWindowFocus: false,
        retry: false
    });
    return { worldName: world?.name, groupName: group?.name };
}

// Renders a resolved location string.
// wrap=false (default): block element with ellipsis for use in cards.
// wrap=true: inline element that wraps, for use in the detail modal.
function FriendLocationText({ location, wrap = false }: { location?: string; wrap?: boolean }) {
    const parsed = useMemo(() => parseLocation(location ?? ''), [location]);
    const { worldName, groupName } = useResolvedLocation(
        parsed.isRealInstance ? parsed.worldId : undefined,
        parsed.groupId
    );
    const text = getLocationText(parsed, { worldName, groupName });
    if (!text) return null;
    if (wrap) {
        return <span className="text-xs text-muted-foreground break-words">{text}</span>;
    }
    return <p className="text-xs text-muted-foreground truncate">{text}</p>;
}

// Header for a Same-Instance group block.
function InstanceHeader({ location, count }: { location: string; count: number }) {
    const parsed = useMemo(() => parseLocation(location), [location]);
    const { worldName, groupName } = useResolvedLocation(parsed.worldId || undefined, parsed.groupId);
    const text = getLocationText(parsed, { worldName, groupName });
    return (
        <div className="flex items-center justify-between px-1 py-1">
            <span className="text-xs font-semibold text-foreground/80 truncate flex-1 min-w-0">{text || location}</span>
            <span className="text-xs text-muted-foreground ml-2 flex-shrink-0">{count}</span>
        </div>
    );
}

function FriendAvatar({ friend, size = 10 }: { friend: VrcCurrentUser; size?: number }) {
    const imgSrc = friendImage(friend);
    const color = statusColor(friend.status, friend.state);
    const sizeClass = `w-${size} h-${size}`;
    return (
        <div className="relative flex-shrink-0">
            {imgSrc ? (
                <img
                    src={imgSrc}
                    alt={friend.displayName}
                    className={`${sizeClass} rounded-full object-cover bg-muted`}
                    loading="lazy"
                />
            ) : (
                <div className={`${sizeClass} rounded-full bg-muted flex items-center justify-center text-sm font-medium text-muted-foreground`}>
                    {friend.displayName[0]?.toUpperCase()}
                </div>
            )}
            <span
                className="absolute -bottom-0.5 -right-0.5 w-3.5 h-3.5 rounded-full border-2 border-card"
                style={{ backgroundColor: color }}
            />
        </div>
    );
}

type FriendCardProps = {
    friend: VrcCurrentUser;
    showLocation?: boolean;
    onClick: (f: VrcCurrentUser) => void;
};

function FriendCard({ friend, showLocation = false, onClick }: FriendCardProps) {
    return (
        <button
            type="button"
            onClick={() => onClick(friend)}
            className="w-full text-left flex items-center gap-3 p-3 rounded-lg bg-card border border-border hover:bg-accent/50 active:bg-accent transition-colors"
        >
            <FriendAvatar friend={friend} />
            <div className="flex-1 min-w-0">
                <p className="text-sm font-medium truncate">{friend.displayName}</p>
                {friend.statusDescription && (
                    <p className="text-xs text-muted-foreground truncate">{friend.statusDescription}</p>
                )}
                {showLocation && <FriendLocationText location={friend.location} />}
            </div>
        </button>
    );
}

function FriendDetailModal({ friend, onClose }: { friend: VrcCurrentUser; onClose: () => void }) {
    const { t } = useTranslation();
    const imgSrc = friendImage(friend);
    const color = statusColor(friend.status, friend.state);

    const lastLogin = friend.last_login
        ? new Date(friend.last_login).toLocaleString(undefined, { dateStyle: 'medium', timeStyle: 'short' })
        : '—';

    return (
        <div className="fixed inset-0 z-50 flex items-center justify-center">
            {/* backdrop */}
            <div className="fixed inset-0 bg-black/50" onClick={onClose} />
            {/* card */}
            <div className="relative bg-card border border-border rounded-xl p-5 w-full max-w-sm mx-4 max-h-[85vh] overflow-y-auto z-10">
                <button
                    type="button"
                    onClick={onClose}
                    className="absolute top-3 right-3 p-1 rounded-md text-muted-foreground hover:text-foreground hover:bg-accent transition-colors"
                >
                    <X className="w-4 h-4" />
                </button>

                {/* header */}
                <div className="flex items-start gap-4 mb-4">
                    <div className="relative flex-shrink-0">
                        {imgSrc ? (
                            <img
                                src={imgSrc}
                                alt={friend.displayName}
                                className="w-16 h-16 rounded-full object-cover bg-muted"
                            />
                        ) : (
                            <div className="w-16 h-16 rounded-full bg-muted flex items-center justify-center text-xl font-medium text-muted-foreground">
                                {friend.displayName[0]?.toUpperCase()}
                            </div>
                        )}
                        <span
                            className="absolute -bottom-0.5 -right-0.5 w-4 h-4 rounded-full border-2 border-card"
                            style={{ backgroundColor: color }}
                        />
                    </div>
                    <div className="flex-1 min-w-0 pt-1">
                        <p className="font-semibold text-base truncate">{friend.displayName}</p>
                        <p className="text-xs text-muted-foreground capitalize">{friend.status}</p>
                    </div>
                </div>

                <div className="space-y-2 text-sm">
                    {friend.statusDescription && (
                        <Row label={t('common.status_desc', { defaultValue: 'Status' })}>
                            {friend.statusDescription}
                        </Row>
                    )}
                    <Row label={t('common.location', { defaultValue: 'Location' })}>
                        <FriendLocationText location={friend.location} wrap />
                    </Row>
                    <Row label={t('common.last_login', { defaultValue: 'Last Login' })}>
                        {lastLogin}
                    </Row>
                    {friend.last_platform && (
                        <Row label={t('common.platform', { defaultValue: 'Platform' })}>
                            {friend.last_platform}
                        </Row>
                    )}
                </div>
            </div>
        </div>
    );
}

function Row({ label, children }: { label: string; children: React.ReactNode }) {
    return (
        <div className="flex gap-2">
            <span className="text-muted-foreground flex-shrink-0 w-24">{label}</span>
            <span className="flex-1 min-w-0 break-words">{children}</span>
        </div>
    );
}

function SectionHeader({ label, count }: { label: string; count: number }) {
    return (
        <h2 className="text-xs font-semibold text-muted-foreground uppercase tracking-wider">
            {label} · {count}
        </h2>
    );
}

function FriendsPage() {
    const { t } = useTranslation();
    const [selectedFriend, setSelectedFriend] = useState<VrcCurrentUser | null>(null);

    const { data: friends, isLoading, isError, refetch, isFetching } = useQuery({
        queryKey: ['friends'],
        queryFn: () => fetchFriends({ n: 100 }),
        refetchInterval: FRIENDS_REFRESH_INTERVAL_MS,
        refetchOnReconnect: false,
        refetchOnWindowFocus: false,
        staleTime: FRIENDS_REFRESH_INTERVAL_MS
    });

    // Sort all sections alphabetically by displayName (VRCX default)
    const onlineFriends = useMemo(
        () => (friends?.filter((f) => f.state === 'online') ?? []).sort(sortByName),
        [friends]
    );
    const activeFriends = useMemo(
        () => (friends?.filter((f) => f.state === 'active') ?? []).sort(sortByName),
        [friends]
    );
    const offlineFriends = useMemo(
        () => (friends?.filter((f) => f.state === 'offline') ?? []).sort(sortByName),
        [friends]
    );

    // Group online friends sharing the same real instance (2+)
    const locationCounts = useMemo(() => {
        const counts = new Map<string, number>();
        for (const f of onlineFriends) {
            if (f.location && isRealInstance(f.location)) {
                counts.set(f.location, (counts.get(f.location) ?? 0) + 1);
            }
        }
        return counts;
    }, [onlineFriends]);

    // Members within each group are already in alphabetical order (onlineFriends is sorted).
    // Groups themselves are sorted by member count descending.
    const { groupedInstances, sortedGroupEntries, soloOnline } = useMemo(() => {
        const grouped = new Map<string, VrcCurrentUser[]>();
        const solo: VrcCurrentUser[] = [];
        for (const f of onlineFriends) {
            const loc = f.location ?? '';
            if (isRealInstance(loc) && (locationCounts.get(loc) ?? 0) >= 2) {
                const bucket = grouped.get(loc) ?? [];
                bucket.push(f);
                grouped.set(loc, bucket);
            } else {
                solo.push(f);
            }
        }
        const sortedEntries = Array.from(grouped.entries()).sort(
            ([, a], [, b]) => b.length - a.length
        );
        return { groupedInstances: grouped, sortedGroupEntries: sortedEntries, soloOnline: solo };
    }, [onlineFriends, locationCounts]);

    const totalOnline = onlineFriends.length + activeFriends.length;

    return (
        <div className="flex flex-col h-full overflow-hidden">
            <div className="flex items-center justify-between px-4 py-3 border-b border-border flex-shrink-0">
                <div className="flex items-center gap-2">
                    <Users className="w-4 h-4 text-muted-foreground" />
                    <span className="text-sm font-medium">{t('side_panel.friends', { defaultValue: 'Friends' })}</span>
                    {friends && (
                        <span className="text-xs text-muted-foreground">
                            ({totalOnline} online)
                        </span>
                    )}
                </div>
                <button
                    onClick={() => refetch()}
                    disabled={isFetching}
                    className="p-1.5 rounded-md text-muted-foreground hover:text-foreground hover:bg-accent transition-colors disabled:opacity-50"
                >
                    <RefreshCw className={`w-4 h-4 ${isFetching ? 'animate-spin' : ''}`} />
                </button>
            </div>

            <div className="flex-1 overflow-y-auto px-4 py-3 space-y-4">
                {isLoading && (
                    <div className="space-y-2">
                        {Array.from({ length: 8 }).map((_, i) => (
                            <div key={i} className="h-16 rounded-lg bg-muted animate-pulse" />
                        ))}
                    </div>
                )}

                {isError && (
                    <div className="text-center py-8 text-muted-foreground">
                        <p className="text-sm">{t('message.friend.load_failed', { defaultValue: 'Failed to load friends.' })}</p>
                        <button
                            onClick={() => refetch()}
                            className="mt-2 text-xs text-primary underline"
                        >
                            {t('prompt.retry', { defaultValue: 'Try again' })}
                        </button>
                    </div>
                )}

                {friends && totalOnline === 0 && (
                    <p className="text-center py-8 text-sm text-muted-foreground">
                        {t('side_panel.no_friends_online', { defaultValue: 'No friends online' })}
                    </p>
                )}

                {/* Same Instance */}
                {groupedInstances.size > 0 && (
                    <section className="space-y-3">
                        <SectionHeader
                            label={t('view.friends_locations.same_instance', { defaultValue: 'Same Instance' })}
                            count={onlineFriends.length - soloOnline.length}
                        />
                        {sortedGroupEntries.map(([loc, members]) => (
                            <div key={loc} className="space-y-1 rounded-lg border border-border bg-card/50 px-3 py-2">
                                <InstanceHeader location={loc} count={members.length} />
                                <div className="space-y-1 pt-1">
                                    {members.map((f) => (
                                        <FriendCard key={f.id} friend={f} onClick={setSelectedFriend} />
                                    ))}
                                </div>
                            </div>
                        ))}
                    </section>
                )}

                {/* Online (solo) */}
                {soloOnline.length > 0 && (
                    <section className="space-y-2">
                        <SectionHeader
                            label={t('view.friends_locations.online', { defaultValue: 'Online' })}
                            count={soloOnline.length}
                        />
                        {soloOnline.map((f) => (
                            <FriendCard key={f.id} friend={f} showLocation onClick={setSelectedFriend} />
                        ))}
                    </section>
                )}

                {/* Active */}
                {activeFriends.length > 0 && (
                    <section className="space-y-2">
                        <SectionHeader
                            label={t('view.friends_locations.active', { defaultValue: 'Active' })}
                            count={activeFriends.length}
                        />
                        {activeFriends.map((f) => (
                            <FriendCard key={f.id} friend={f} onClick={setSelectedFriend} />
                        ))}
                    </section>
                )}

                {/* Offline */}
                {offlineFriends.length > 0 && (
                    <section className="space-y-2">
                        <SectionHeader
                            label={t('view.friends_locations.offline', { defaultValue: 'Offline' })}
                            count={offlineFriends.length}
                        />
                        {offlineFriends.map((f) => (
                            <FriendCard key={f.id} friend={f} onClick={setSelectedFriend} />
                        ))}
                    </section>
                )}
            </div>

            {selectedFriend && (
                <FriendDetailModal friend={selectedFriend} onClose={() => setSelectedFriend(null)} />
            )}
        </div>
    );
}
