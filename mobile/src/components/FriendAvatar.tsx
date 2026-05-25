import type { VrcCurrentUser } from '@/types/vrc';

const ROBOT_AVATAR_URL = 'https://api.vrchat.cloud/api/1/file/file_0e8c4e32-7444-44ea-ade4-313c010d4bae/1/file';

export function friendImage(friend: VrcCurrentUser): string {
    const image =
        friend.profilePicOverrideThumbnail?.replace('/256', '/128') ||
        friend.profilePicOverride ||
        friend.currentAvatarThumbnailImageUrl?.replace('/256', '/128') ||
        friend.currentAvatarImageUrl ||
        '';
    return image === ROBOT_AVATAR_URL ? '' : image;
}

export function statusColor(status: VrcCurrentUser['status'], state: VrcCurrentUser['state']): string {
    if (state === 'offline') return 'var(--status-offline)';
    switch (status) {
        case 'join me': return 'var(--status-joinme)';
        case 'active': return 'var(--status-online)';
        case 'ask me': return 'var(--status-askme)';
        case 'busy': return 'var(--status-busy)';
        default: return 'var(--status-online)';
    }
}

export function FriendAvatar({ friend, size = 10 }: { friend: VrcCurrentUser; size?: number }) {
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
