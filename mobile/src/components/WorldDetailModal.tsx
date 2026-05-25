import { useMemo } from 'react';
import type { ReactNode } from 'react';
import { X } from 'lucide-react';
import { useQuery } from '@tanstack/react-query';
import { useTranslation } from 'react-i18next';
import { useWorldModal } from '@/stores/worldModal';
import { fetchWorld, fetchGroup, fetchInstance, fetchFriends } from '@/api/auth';
import { parseLocation, ACCESS_TYPE_LABELS, resolveRegion } from '@/lib/vrcLocation';
import { FriendsInInstanceList } from '@/components/FriendsInInstanceList';
import type { ParsedLocation } from '@/lib/vrcLocation';
import type { VrcCurrentUser, VrcWorld } from '@/types/vrc';

const WORLD_CACHE_MS = 30 * 60 * 1000;
const FRIENDS_STALE_MS = 60 * 60 * 1000;

const QUERY_OPTS = {
    staleTime: Infinity,
    gcTime: WORLD_CACHE_MS,
    refetchOnWindowFocus: false,
    refetchOnReconnect: false,
    retry: false,
} as const;

type InstanceRoomData = {
    instanceId: string;
    occupants: number;
    tag: string;
    parsed: ParsedLocation;
    isOrigin: boolean;
    friendCount: number;
};

function StatRow({ label, children }: { label: string; children: ReactNode }) {
    return (
        <div className="flex justify-between gap-3 text-sm">
            <span className="text-muted-foreground">{label}</span>
            <span className="font-medium text-right">{children}</span>
        </div>
    );
}

function Badge({ children, tone = 'muted' }: { children: ReactNode; tone?: 'primary' | 'muted' }) {
    return (
        <span className={`text-xs px-2 py-0.5 rounded-full font-medium ${
            tone === 'primary'
                ? 'bg-primary/10 text-primary'
                : 'bg-muted text-muted-foreground'
        }`}>
            {children}
        </span>
    );
}

function buildRooms(world: VrcWorld | undefined, origin: ParsedLocation, friends: VrcCurrentUser[]): InstanceRoomData[] {
    const rooms = new Map<string, { instanceId: string; occupants: number }>();
    for (const tuple of world?.instances ?? []) {
        const [instanceId, occupants] = tuple;
        if (instanceId) rooms.set(instanceId, { instanceId, occupants });
    }
    if (origin.instanceId && !rooms.has(origin.instanceId)) {
        rooms.set(origin.instanceId, { instanceId: origin.instanceId, occupants: 0 });
    }

    return Array.from(rooms.values())
        .map((room) => {
            const tag = `${origin.worldId}:${room.instanceId}`;
            return {
                ...room,
                tag,
                parsed: parseLocation(tag),
                isOrigin: room.instanceId === origin.instanceId,
                friendCount: friends.filter((friend) => friend.location === tag).length,
            };
        })
        .sort((a, b) => {
            if (a.isOrigin !== b.isOrigin) return a.isOrigin ? -1 : 1;
            if (a.friendCount !== b.friendCount) return b.friendCount - a.friendCount;
            return b.occupants - a.occupants;
        });
}

function WorldStats({ world }: { world: VrcWorld }) {
    const { t } = useTranslation();
    return (
        <div className="grid grid-cols-3 gap-2 rounded-lg bg-muted/50 p-3 text-center">
            <div>
                <p className="text-xs text-muted-foreground">{t('instance_detail.public', { defaultValue: 'Public' })}</p>
                <p className="text-sm font-semibold">{world.publicOccupants ?? 0}</p>
            </div>
            <div>
                <p className="text-xs text-muted-foreground">{t('instance_detail.private', { defaultValue: 'Private' })}</p>
                <p className="text-sm font-semibold">{world.privateOccupants ?? 0}</p>
            </div>
            <div>
                <p className="text-xs text-muted-foreground">{t('instance_detail.capacity', { defaultValue: 'Capacity' })}</p>
                <p className="text-sm font-semibold">
                    {world.recommendedCapacity ?? 0}
                    {world.capacity !== world.recommendedCapacity && world.capacity > 0 && (
                        <span className="text-xs text-muted-foreground"> ({world.capacity})</span>
                    )}
                </p>
            </div>
        </div>
    );
}

function InstanceRoom({ worldId, room, friends }: { worldId: string; room: InstanceRoomData; friends: VrcCurrentUser[] }) {
    const { t } = useTranslation();
    const { parsed } = room;
    const { data: group } = useQuery({
        queryKey: ['vrc-group', parsed.groupId],
        queryFn: () => fetchGroup(parsed.groupId!),
        enabled: !!parsed.groupId,
        ...QUERY_OPTS,
    });
    const { data: instance } = useQuery({
        queryKey: ['vrc-instance', room.tag],
        queryFn: () => fetchInstance(worldId, room.instanceId),
        enabled: !!worldId && !!room.instanceId,
        staleTime: 60_000,
        gcTime: 5 * 60 * 1000,
        refetchOnWindowFocus: false,
        refetchOnReconnect: false,
        retry: false,
    });

    const friendsHere = useMemo(
        () => friends.filter((friend) => friend.location === room.tag),
        [friends, room.tag]
    );
    const accessLabel = ACCESS_TYPE_LABELS[parsed.accessTypeName] ?? parsed.accessTypeName;
    const region = resolveRegion(parsed);
    const nUsers = instance?.n_users ?? instance?.userCount ?? room.occupants;
    const capacity = instance?.capacity;
    const platforms = instance?.platforms;
    const platformText = platforms
        ? [
            platforms.standalonewindows > 0 && `PC ${platforms.standalonewindows}`,
            platforms.android > 0 && `Quest ${platforms.android}`,
            platforms.ios > 0 && `iOS ${platforms.ios}`,
        ].filter(Boolean).join(' / ')
        : '';

    return (
        <section className="rounded-lg border border-border bg-card/50 p-3 space-y-3">
            <div className="flex items-start justify-between gap-3">
                <div className="min-w-0 space-y-1">
                    <div className="flex items-center gap-1.5 flex-wrap">
                        <Badge tone="primary">{accessLabel}</Badge>
                        {region && <Badge>#{region.toUpperCase()}</Badge>}
                        {room.isOrigin && <Badge>{t('instance_detail.selected', { defaultValue: 'Selected' })}</Badge>}
                    </div>
                    {group?.name && (
                        <p className="text-xs text-muted-foreground truncate">{group.name}</p>
                    )}
                </div>
                <div className="text-right shrink-0">
                    <p className={`text-sm font-semibold ${instance?.full ? 'text-destructive' : ''}`}>
                        {capacity ? `${nUsers}/${capacity}` : nUsers}
                    </p>
                    <p className="text-xs text-muted-foreground">{room.instanceId.split('~')[0]}</p>
                </div>
            </div>

            {(platformText || instance?.queueEnabled || instance?.full) && (
                <div className="space-y-1.5 rounded-md bg-muted/40 p-2">
                    {platformText && (
                        <StatRow label={t('instance_detail.platform', { defaultValue: 'Platform' })}>
                            {platformText}
                        </StatRow>
                    )}
                    {instance?.queueEnabled && (
                        <StatRow label={t('instance_detail.queue', { defaultValue: 'Queue' })}>
                            {instance.queueSize}
                        </StatRow>
                    )}
                    {instance?.full && (
                        <p className="text-xs text-destructive font-medium">
                            {t('instance_detail.full', { defaultValue: 'Instance full' })}
                        </p>
                    )}
                </div>
            )}

            {friendsHere.length > 0 && (
                <div className="space-y-2">
                    <p className="text-xs font-semibold text-muted-foreground uppercase tracking-wider">
                        {t('instance_detail.friends_here', { defaultValue: 'Friends here' })} · {friendsHere.length}
                    </p>
                    <FriendsInInstanceList friends={friendsHere} creatorId={parsed.userId} />
                </div>
            )}
        </section>
    );
}

export function WorldDetailModal() {
    const { t } = useTranslation();
    const { location, close } = useWorldModal();
    const parsed = useMemo(() => (location ? parseLocation(location) : null), [location]);

    const { data: world, isLoading: worldLoading } = useQuery({
        queryKey: ['vrc-world', parsed?.worldId],
        queryFn: () => fetchWorld(parsed!.worldId),
        enabled: !!parsed?.worldId,
        ...QUERY_OPTS,
    });
    const { data: friends } = useQuery({
        queryKey: ['friends'],
        queryFn: () => fetchFriends({ n: 100 }),
        staleTime: FRIENDS_STALE_MS,
        gcTime: FRIENDS_STALE_MS,
        refetchOnWindowFocus: false,
        refetchOnReconnect: false,
        retry: false,
    });

    const rooms = useMemo(
        () => (parsed ? buildRooms(world, parsed, friends ?? []) : []),
        [friends, parsed, world]
    );

    if (!location || !parsed) return null;

    const thumbnail = world?.thumbnailImageUrl || world?.imageUrl;

    return (
        <div className="fixed inset-0 z-50 flex items-center justify-center">
            <div className="fixed inset-0 bg-black/50" onClick={close} />
            <div className="relative bg-card border border-border rounded-xl w-full max-w-sm mx-4 max-h-[85vh] overflow-y-auto z-10">
                <div className="relative">
                    {thumbnail ? (
                        <img
                            src={thumbnail}
                            alt={world?.name}
                            className="w-full h-36 object-cover rounded-t-xl bg-muted"
                        />
                    ) : (
                        <div className="w-full h-36 rounded-t-xl bg-muted animate-pulse" />
                    )}
                    <button
                        type="button"
                        onClick={close}
                        className="absolute top-2 right-2 p-1.5 rounded-full bg-black/50 text-white hover:bg-black/70 transition-colors"
                    >
                        <X className="w-4 h-4" />
                    </button>
                </div>

                <div className="p-4 space-y-4">
                    <div>
                        {worldLoading && !world ? (
                            <div className="h-5 w-40 bg-muted rounded animate-pulse" />
                        ) : (
                            <p className="font-semibold text-base">{world?.name ?? parsed.worldId}</p>
                        )}
                        {world?.authorName && (
                            <p className="text-xs text-muted-foreground">
                                {t('instance_detail.by', { defaultValue: 'by' })} {world.authorName}
                            </p>
                        )}
                    </div>

                    {world && <WorldStats world={world} />}

                    {world?.description && (
                        <p className="text-xs text-muted-foreground leading-relaxed line-clamp-3">
                            {world.description}
                        </p>
                    )}

                    <div className="space-y-2">
                        <p className="text-xs font-semibold text-muted-foreground uppercase tracking-wider">
                            {t('instance_detail.instances', { defaultValue: 'Instances' })} · {rooms.length}
                        </p>
                        {rooms.length > 0 ? (
                            rooms.map((room) => (
                                <InstanceRoom
                                    key={room.tag}
                                    worldId={parsed.worldId}
                                    room={room}
                                    friends={friends ?? []}
                                />
                            ))
                        ) : (
                            <p className="text-sm text-muted-foreground">
                                {t('instance_detail.no_instances', { defaultValue: 'No active instances' })}
                            </p>
                        )}
                    </div>
                </div>
            </div>
        </div>
    );
}
