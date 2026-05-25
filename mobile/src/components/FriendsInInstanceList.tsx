import { useMemo } from 'react';
import { useTranslation } from 'react-i18next';
import { FriendAvatar } from '@/components/FriendAvatar';
import type { VrcCurrentUser, VrcUser } from '@/types/vrc';

type FriendsInInstanceListProps = {
    friends: VrcCurrentUser[];
    creatorId?: string | null;
    creator?: VrcUser | null;
    creatorSelectable?: boolean;
    onSelect?: (friend: VrcCurrentUser) => void;
};

export function FriendsInInstanceList({ friends, creatorId, creator, creatorSelectable = false, onSelect }: FriendsInInstanceListProps) {
    const { t } = useTranslation();
    const sortedFriends = useMemo(
        () => friends
            .filter((friend) => friend.id !== creatorId)
            .sort((a, b) => a.displayName.localeCompare(b.displayName, undefined, { sensitivity: 'base' })),
        [creatorId, friends]
    );
    const instanceCreator = creator ?? friends.find((friend) => friend.id === creatorId);
    const selectableCreator = creatorSelectable
        ? friends.find((friend) => friend.id === instanceCreator?.id) ?? null
        : null;
    const creatorLabel = t('instance_detail.instance_creator', { defaultValue: 'Instance Creator' });

    const clickableClassName = 'w-full text-left flex items-center gap-2.5 rounded-md px-1 py-1 hover:bg-accent/50 active:bg-accent transition-colors';
    const staticClassName = 'w-full text-left flex items-center gap-2.5 px-1 py-1';

    const renderFriend = (friend: VrcCurrentUser) => {
        const content = (
            <>
                <FriendAvatar friend={friend} size={8} />
                <div className="flex-1 min-w-0">
                    <p className="text-sm font-medium truncate">{friend.displayName}</p>
                    {friend.statusDescription && (
                        <p className="text-xs text-muted-foreground truncate">{friend.statusDescription}</p>
                    )}
                </div>
            </>
        );

        if (onSelect) {
            return (
                <button
                    key={friend.id}
                    type="button"
                    onClick={() => onSelect(friend)}
                    className={clickableClassName}
                >
                    {content}
                </button>
            );
        }

        return (
            <div key={friend.id} className={staticClassName}>
                {content}
            </div>
        );
    };

    return (
        <div className="space-y-1.5">
            {creatorId && instanceCreator && (
                <button
                    type="button"
                    onClick={() => {
                        if (selectableCreator) onSelect?.(selectableCreator);
                    }}
                    disabled={!selectableCreator}
                    className={selectableCreator ? clickableClassName : staticClassName}
                >
                    <FriendAvatar friend={instanceCreator} size={8} />
                    <div className="flex-1 min-w-0">
                        <p className="text-sm font-medium truncate">{instanceCreator.displayName}</p>
                        <p className="text-xs text-muted-foreground truncate">{creatorLabel}</p>
                    </div>
                </button>
            )}
            {sortedFriends.map(renderFriend)}
        </div>
    );
}
