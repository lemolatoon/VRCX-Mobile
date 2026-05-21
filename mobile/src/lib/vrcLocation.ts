export interface ParsedLocation {
    tag: string;
    isOffline: boolean;
    isPrivate: boolean;
    isTraveling: boolean;
    isRealInstance: boolean;
    worldId: string;
    instanceId: string;
    instanceName: string;
    accessType: string;
    accessTypeName: string;
    region: string;
    shortName: string;
    userId: string | null;
    hiddenId: string | null;
    privateId: string | null;
    friendsId: string | null;
    groupId: string | null;
    groupAccessType: string | null;
    canRequestInvite: boolean;
    strict: boolean;
    ageGate: boolean;
}

// Ported from src/shared/utils/locationParser.js
export function parseLocation(tag: string): ParsedLocation {
    let _tag = String(tag || '');
    const ctx: ParsedLocation = {
        tag: _tag,
        isOffline: false,
        isPrivate: false,
        isTraveling: false,
        isRealInstance: false,
        worldId: '',
        instanceId: '',
        instanceName: '',
        accessType: '',
        accessTypeName: '',
        region: '',
        shortName: '',
        userId: null,
        hiddenId: null,
        privateId: null,
        friendsId: null,
        groupId: null,
        groupAccessType: null,
        canRequestInvite: false,
        strict: false,
        ageGate: false
    };
    if (_tag === 'offline' || _tag === 'offline:offline') {
        ctx.isOffline = true;
    } else if (_tag === 'private' || _tag === 'private:private') {
        ctx.isPrivate = true;
    } else if (_tag === 'traveling' || _tag === 'traveling:traveling') {
        ctx.isTraveling = true;
    } else if (tag && !_tag.startsWith('local')) {
        ctx.isRealInstance = true;
        const sep = _tag.indexOf(':');
        const shortNameQualifier = '&shortName=';
        const shortNameIndex = _tag.indexOf(shortNameQualifier);
        if (shortNameIndex >= 0) {
            ctx.shortName = _tag.substring(shortNameIndex + shortNameQualifier.length);
            _tag = _tag.substring(0, shortNameIndex);
        }
        if (sep >= 0) {
            ctx.worldId = _tag.substring(0, sep);
            ctx.instanceId = _tag.substring(sep + 1);
            ctx.instanceId.split('~').forEach((s, i) => {
                if (i) {
                    const A = s.indexOf('(');
                    const Z = A >= 0 ? s.lastIndexOf(')') : -1;
                    const key = Z >= 0 ? s.substring(0, A) : s;
                    const value = A < Z ? s.substring(A + 1, Z - A - 1) : '';
                    if (key === 'hidden') {
                        ctx.hiddenId = value;
                    } else if (key === 'private') {
                        ctx.privateId = value;
                    } else if (key === 'friends') {
                        ctx.friendsId = value;
                    } else if (key === 'canRequestInvite') {
                        ctx.canRequestInvite = true;
                    } else if (key === 'region') {
                        ctx.region = value;
                    } else if (key === 'group') {
                        ctx.groupId = value;
                    } else if (key === 'groupAccessType') {
                        ctx.groupAccessType = value;
                    } else if (key === 'strict') {
                        ctx.strict = true;
                    } else if (key === 'ageGate') {
                        ctx.ageGate = true;
                    }
                } else {
                    ctx.instanceName = s;
                }
            });
            ctx.accessType = 'public';
            if (ctx.privateId !== null) {
                ctx.accessType = ctx.canRequestInvite ? 'invite+' : 'invite';
                ctx.userId = ctx.privateId;
            } else if (ctx.friendsId !== null) {
                ctx.accessType = 'friends';
                ctx.userId = ctx.friendsId;
            } else if (ctx.hiddenId !== null) {
                ctx.accessType = 'friends+';
                ctx.userId = ctx.hiddenId;
            } else if (ctx.groupId !== null) {
                ctx.accessType = 'group';
            }
            ctx.accessTypeName = ctx.accessType;
            if (ctx.groupAccessType !== null) {
                if (ctx.groupAccessType === 'public') {
                    ctx.accessTypeName = 'groupPublic';
                } else if (ctx.groupAccessType === 'plus') {
                    ctx.accessTypeName = 'groupPlus';
                }
            }
        } else {
            ctx.worldId = _tag;
        }
    }
    return ctx;
}

// Ported from src/shared/utils/instance.js
export function isRealInstance(instanceId: string): boolean {
    if (!instanceId) return false;
    switch (instanceId) {
        case ':':
        case 'offline':
        case 'offline:offline':
        case 'private':
        case 'private:private':
        case 'traveling':
        case 'traveling:traveling':
            return false;
    }
    if (instanceId.startsWith('local')) return false;
    return true;
}

// Ported from src/shared/utils/locationParser.js
export function resolveRegion(L: ParsedLocation): string {
    if (L.isOffline || L.isPrivate || L.isTraveling) return '';
    if (L.region) return L.region;
    if (L.instanceId) return 'us';
    return '';
}

// Derived from src/shared/constants/accessType.js (English labels inline, no i18n needed for Phase 1)
export const ACCESS_TYPE_LABELS: Record<string, string> = {
    public: 'Public',
    'friends+': 'Friends+',
    friends: 'Friends',
    'invite+': 'Invite+',
    invite: 'Invite',
    group: 'Group',
    groupPublic: 'Group Public',
    groupPlus: 'Group+',
    groupMembers: 'Members'
};

/**
 * Build a human-readable location string.
 * Pattern mirrors VRCX's getLocationText (location.js:55-73) + Location.vue suffix assembly.
 * Returns e.g. "Pug Land · Friends+ #JP", "Private", "Traveling", "Offline"
 */
export function getLocationText(
    L: ParsedLocation,
    opts: { worldName?: string; groupName?: string } = {}
): string {
    if (L.isOffline) return 'Offline';
    if (L.isPrivate) return 'Private';
    if (L.isTraveling) return 'Traveling';

    if (!L.worldId) return '';

    const name = opts.worldName || L.worldId;
    const accessLabel = ACCESS_TYPE_LABELS[L.accessTypeName] ?? L.accessTypeName;
    const region = resolveRegion(L);
    const regionSuffix = region ? ` #${region.toUpperCase()}` : '';
    const groupSuffix = opts.groupName ? ` (${opts.groupName})` : '';

    if (L.instanceId) {
        return `${name} · ${accessLabel}${regionSuffix}${groupSuffix}`;
    }
    return name;
}
