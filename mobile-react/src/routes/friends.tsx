import { createFileRoute } from '@tanstack/react-router';
import { useQuery } from '@tanstack/react-query';
import { RefreshCw, Users } from 'lucide-react';
import { fetchFriends } from '@/api/auth';
import type { VrcCurrentUser } from '@/types/vrc';

export const Route = createFileRoute('/friends')({
    component: FriendsPage
});

const FRIENDS_REFRESH_INTERVAL_MS = 60 * 60 * 1000;
const ROBOT_AVATAR_URL = 'https://api.vrchat.cloud/api/1/file/file_0e8c4e32-7444-44ea-ade4-313c010d4bae/1/file';

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

function FriendCard({ friend }: { friend: VrcCurrentUser }) {
    const imgSrc = friendImage(friend);
    const color = statusColor(friend.status, friend.state);
    return (
        <div className="flex items-center gap-3 p-3 rounded-lg bg-card border border-border">
            <div className="relative flex-shrink-0">
                {imgSrc ? (
                    <img
                        src={imgSrc}
                        alt={friend.displayName}
                        className="w-10 h-10 rounded-full object-cover bg-muted"
                        loading="lazy"
                    />
                ) : (
                    <div className="w-10 h-10 rounded-full bg-muted flex items-center justify-center text-sm font-medium text-muted-foreground">
                        {friend.displayName[0]?.toUpperCase()}
                    </div>
                )}
                <span
                    className="absolute -bottom-0.5 -right-0.5 w-3.5 h-3.5 rounded-full border-2 border-card"
                    style={{ backgroundColor: color }}
                />
            </div>
            <div className="flex-1 min-w-0">
                <p className="text-sm font-medium truncate">{friend.displayName}</p>
                {friend.statusDescription && (
                    <p className="text-xs text-muted-foreground truncate">{friend.statusDescription}</p>
                )}
            </div>
        </div>
    );
}

function FriendsPage() {
    const { data: friends, isLoading, isError, refetch, isFetching } = useQuery({
        queryKey: ['friends'],
        queryFn: () => fetchFriends({ n: 100 }),
        refetchInterval: FRIENDS_REFRESH_INTERVAL_MS,
        refetchOnReconnect: false,
        refetchOnWindowFocus: false,
        staleTime: FRIENDS_REFRESH_INTERVAL_MS
    });

    const online = friends?.filter((f) => f.state === 'online') ?? [];
    const active = friends?.filter((f) => f.state === 'active') ?? [];
    const offline = friends?.filter((f) => f.state === 'offline') ?? [];

    return (
        <div className="flex flex-col h-full overflow-hidden">
            <div className="flex items-center justify-between px-4 py-3 border-b border-border flex-shrink-0">
                <div className="flex items-center gap-2">
                    <Users className="w-4 h-4 text-muted-foreground" />
                    <span className="text-sm font-medium">Friends</span>
                    {friends && (
                        <span className="text-xs text-muted-foreground">
                            ({online.length + active.length} online)
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
                        <p className="text-sm">Failed to load friends.</p>
                        <button
                            onClick={() => refetch()}
                            className="mt-2 text-xs text-primary underline"
                        >
                            Try again
                        </button>
                    </div>
                )}

                {friends && online.length === 0 && active.length === 0 && (
                    <p className="text-center py-8 text-sm text-muted-foreground">No friends online</p>
                )}

                {online.length > 0 && (
                    <section className="space-y-2">
                        <h2 className="text-xs font-semibold text-muted-foreground uppercase tracking-wider">
                            Online · {online.length}
                        </h2>
                        {online.map((f) => <FriendCard key={f.id} friend={f} />)}
                    </section>
                )}

                {active.length > 0 && (
                    <section className="space-y-2">
                        <h2 className="text-xs font-semibold text-muted-foreground uppercase tracking-wider">
                            Active · {active.length}
                        </h2>
                        {active.map((f) => <FriendCard key={f.id} friend={f} />)}
                    </section>
                )}

                {offline.length > 0 && (
                    <section className="space-y-2">
                        <h2 className="text-xs font-semibold text-muted-foreground uppercase tracking-wider">
                            Offline · {offline.length}
                        </h2>
                        {offline.map((f) => <FriendCard key={f.id} friend={f} />)}
                    </section>
                )}
            </div>
        </div>
    );
}
