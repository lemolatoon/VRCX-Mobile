import { useMemo } from 'react';
import { X } from 'lucide-react';
import { useQuery } from '@tanstack/react-query';
import { useTranslation } from 'react-i18next';
import { useInstanceModal } from '@/stores/instanceModal';
import { fetchWorld, fetchGroup, fetchInstance, fetchFriends } from '@/api/auth';
import { parseLocation, getLocationText, ACCESS_TYPE_LABELS, resolveRegion } from '@/lib/vrcLocation';
import { FriendAvatar } from '@/components/FriendAvatar';

const WORLD_CACHE_MS = 30 * 60 * 1000;
const FRIENDS_STALE_MS = 60 * 60 * 1000;

const QUERY_OPTS = {
    staleTime: Infinity,
    gcTime: WORLD_CACHE_MS,
    refetchOnWindowFocus: false,
    refetchOnReconnect: false,
    retry: false,
} as const;

function StatRow({ label, children }: { label: string; children: React.ReactNode }) {
    return (
        <div className="flex justify-between text-sm">
            <span className="text-muted-foreground">{label}</span>
            <span className="font-medium text-right">{children}</span>
        </div>
    );
}

export function InstanceDetailModal() {
    const { t } = useTranslation();
    const { location, close } = useInstanceModal();

    const parsed = useMemo(() => (location ? parseLocation(location) : null), [location]);

    const { data: world, isLoading: worldLoading } = useQuery({
        queryKey: ['vrc-world', parsed?.worldId],
        queryFn: () => fetchWorld(parsed!.worldId),
        enabled: !!parsed?.worldId,
        ...QUERY_OPTS,
    });

    const { data: group } = useQuery({
        queryKey: ['vrc-group', parsed?.groupId],
        queryFn: () => fetchGroup(parsed!.groupId!),
        enabled: !!parsed?.groupId,
        ...QUERY_OPTS,
    });

    const { data: instance } = useQuery({
        queryKey: ['vrc-instance', location],
        queryFn: () => fetchInstance(parsed!.worldId, parsed!.instanceId),
        enabled: !!parsed?.worldId && !!parsed?.instanceId,
        staleTime: 60_000,
        gcTime: 5 * 60 * 1000,
        refetchOnWindowFocus: false,
        refetchOnReconnect: false,
        retry: false,
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

    const friendsHere = useMemo(
        () => (friends ?? []).filter((f) => f.location === location),
        [friends, location]
    );

    if (!location || !parsed) return null;

    const worldName = world?.name ?? parsed.worldId;
    const locationText = getLocationText(parsed, { worldName, groupName: group?.name });
    const accessLabel = ACCESS_TYPE_LABELS[parsed.accessTypeName] ?? parsed.accessTypeName;
    const region = resolveRegion(parsed);

    const nUsers = instance?.n_users ?? world?.publicOccupants ?? 0;
    const capacity = instance?.capacity ?? world?.capacity ?? 0;
    const recCap = instance?.recommendedCapacity ?? world?.recommendedCapacity ?? 0;

    const thumbnail = world?.thumbnailImageUrl || world?.imageUrl;

    return (
        <div className="fixed inset-0 z-50 flex items-center justify-center">
            <div className="fixed inset-0 bg-black/50" onClick={close} />
            <div className="relative bg-card border border-border rounded-xl w-full max-w-sm mx-4 max-h-[85vh] overflow-y-auto z-10">
                {/* World header */}
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
                    {/* Title */}
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

                    {/* Location line */}
                    <div className="flex items-center gap-1.5 flex-wrap">
                        <span className="text-xs px-2 py-0.5 rounded-full bg-primary/10 text-primary font-medium">
                            {accessLabel}
                        </span>
                        {region && (
                            <span className="text-xs px-2 py-0.5 rounded-full bg-muted text-muted-foreground font-medium">
                                #{region.toUpperCase()}
                            </span>
                        )}
                        {group?.name && (
                            <span className="text-xs text-muted-foreground">({group.name})</span>
                        )}
                        {locationText && (
                            <span className="text-xs text-muted-foreground truncate">{locationText}</span>
                        )}
                    </div>

                    {/* Live stats */}
                    <div className="space-y-1.5 rounded-lg bg-muted/50 p-3">
                        <StatRow label={t('instance_detail.players', { defaultValue: 'Players' })}>
                            {nUsers} / {recCap}
                            {recCap !== capacity && capacity > 0 && (
                                <span className="text-muted-foreground text-xs"> ({capacity})</span>
                            )}
                        </StatRow>
                        {instance && (
                            <>
                                {(instance.platforms.standalonewindows > 0 || instance.platforms.android > 0) && (
                                    <StatRow label={t('instance_detail.platform', { defaultValue: 'Platform' })}>
                                        {[
                                            instance.platforms.standalonewindows > 0 && `PC ${instance.platforms.standalonewindows}`,
                                            instance.platforms.android > 0 && `Quest ${instance.platforms.android}`,
                                            instance.platforms.ios > 0 && `iOS ${instance.platforms.ios}`,
                                        ].filter(Boolean).join(' / ')}
                                    </StatRow>
                                )}
                                {instance.queueEnabled && (
                                    <StatRow label={t('instance_detail.queue', { defaultValue: 'Queue' })}>
                                        {instance.queueSize}
                                    </StatRow>
                                )}
                                {instance.full && (
                                    <p className="text-xs text-destructive font-medium">
                                        {t('instance_detail.full', { defaultValue: 'Instance full' })}
                                    </p>
                                )}
                            </>
                        )}
                    </div>

                    {/* World description */}
                    {world?.description && (
                        <p className="text-xs text-muted-foreground leading-relaxed line-clamp-3">
                            {world.description}
                        </p>
                    )}

                    {/* Friends here */}
                    {friendsHere.length > 0 && (
                        <div className="space-y-2">
                            <p className="text-xs font-semibold text-muted-foreground uppercase tracking-wider">
                                {t('instance_detail.friends_here', { defaultValue: 'Friends here' })} · {friendsHere.length}
                            </p>
                            <div className="space-y-1.5">
                                {friendsHere.map((f) => (
                                    <div key={f.id} className="flex items-center gap-2.5">
                                        <FriendAvatar friend={f} size={8} />
                                        <div className="flex-1 min-w-0">
                                            <p className="text-sm font-medium truncate">{f.displayName}</p>
                                            {f.statusDescription && (
                                                <p className="text-xs text-muted-foreground truncate">{f.statusDescription}</p>
                                            )}
                                        </div>
                                    </div>
                                ))}
                            </div>
                        </div>
                    )}
                </div>
            </div>
        </div>
    );
}
