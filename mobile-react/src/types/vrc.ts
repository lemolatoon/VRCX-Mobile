export type TrustRank =
    | 'Visitor'
    | 'New User'
    | 'User'
    | 'Known User'
    | 'Trusted User'
    | 'Veteran User'
    | 'Legendary User';

export type UserStatus = 'active' | 'join me' | 'ask me' | 'busy' | 'offline';
export type FriendState = 'online' | 'active' | 'offline';

export interface VrcUser {
    id: string;
    displayName: string;
    userIcon: string;
    bio: string;
    bioLinks: string[];
    profilePicOverride: string;
    statusDescription: string;
    currentAvatarImageUrl: string;
    currentAvatarThumbnailImageUrl: string;
    state: FriendState;
    status: UserStatus;
    tags: string[];
    developerType: string;
    last_login: string;
    last_platform: string;
    allowAvatarCopying: boolean;
    isFriend: boolean;
    friendKey: string;
    location?: string;
    instanceId?: string;
    worldId?: string;
    travelingToLocation?: string;
    userIcon2d?: string;
}

export interface VrcCurrentUser extends VrcUser {
    email: string;
    obfuscatedEmail: string;
    obfuscatedPendingEmail?: string;
    emailVerified: boolean;
    hasBirthday: boolean;
    unsubscribe: boolean;
    steamId?: string;
    oculusId?: string;
    hasLoggedInFromClient: boolean;
    homeLocation: string;
    twoFactorAuthEnabled: boolean;
    twoFactorAuthEnabledDate?: string;
    updated_at: string;
    has2FA: boolean;
    activeFriends: string[];
    onlineFriends: string[];
    offlineFriends: string[];
    friendGroupNames: string[];
    onlineFriendsCount?: number;
}

export interface VrcWorld {
    id: string;
    name: string;
    description: string;
    authorId: string;
    authorName: string;
    capacity: number;
    recommendedCapacity: number;
    imageUrl: string;
    thumbnailImageUrl: string;
    releaseStatus: string;
    tags: string[];
    created_at: string;
    updated_at: string;
    publicationDate: string;
    labsPublicationDate: string;
    visits: number;
    popularity: number;
    heat: number;
    publicOccupants: number;
    privateOccupants: number;
    occupants: number;
    instances?: VrcInstance[];
}

export interface VrcInstance {
    id: string;
    location: string;
    instanceId: string;
    name: string;
    worldId: string;
    type: string;
    ownerId?: string;
    tags: string[];
    active: boolean;
    full: boolean;
    n_users: number;
    capacity: number;
    recommendedCapacity: number;
    userCount: number;
}

export type TwoFactorMethod = 'totp' | 'otp' | 'emailotp';

export interface LoginResponse {
    requiresTwoFactorAuth?: TwoFactorMethod[];
    id?: string;
    displayName?: string;
}

export interface AuthMeResponse {
    id: string;
    displayName: string;
    requiresTwoFactorAuth?: TwoFactorMethod[];
}
