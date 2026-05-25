import type { AuthMeResponse, LoginResponse, TwoFactorMethod, VrcCurrentUser, VrcGroup, VrcInstanceDetail, VrcUser, VrcWorld } from '@/types/vrc';

const BASE = '/api/v1/auth';

async function apiFetch<T>(url: string, init?: RequestInit): Promise<T> {
    const res = await fetch(url, {
        ...init,
        credentials: 'include',
        headers: {
            'Content-Type': 'application/json',
            ...init?.headers
        }
    });
    if (!res.ok) {
        const text = await res.text().catch(() => '');
        let message = text;
        try { const j = JSON.parse(text); if (j?.error) message = j.error; } catch { /* keep raw */ }
        if (res.status === 503) message = 'VRChat is temporarily unavailable. Check status.vrchat.com and try again.';
        throw new Error(message);
    }
    return res.json() as Promise<T>;
}

export async function login(username: string, password: string): Promise<LoginResponse> {
    return apiFetch<LoginResponse>(`${BASE}/login`, {
        method: 'POST',
        body: JSON.stringify({ username, password })
    });
}

export async function verify2FA(method: TwoFactorMethod, code: string, pending: string): Promise<{ verified: boolean }> {
    return apiFetch(`${BASE}/2fa/${method}`, {
        method: 'POST',
        body: JSON.stringify({ code, pending })
    });
}

export async function me(): Promise<AuthMeResponse | null> {
    try {
        return await apiFetch<AuthMeResponse>(`${BASE}/me`);
    } catch {
        return null;
    }
}

export async function logout(): Promise<void> {
    await apiFetch(`${BASE}/logout`, { method: 'POST' });
}

export async function fetchWorld(worldId: string): Promise<VrcWorld> {
    return apiFetch<VrcWorld>(`/api/v1/proxy/worlds/${encodeURIComponent(worldId)}`);
}

export async function fetchGroup(groupId: string): Promise<VrcGroup> {
    return apiFetch<VrcGroup>(`/api/v1/proxy/groups/${encodeURIComponent(groupId)}`);
}

export async function fetchInstance(worldId: string, instanceId: string): Promise<VrcInstanceDetail> {
    return apiFetch<VrcInstanceDetail>(`/api/v1/proxy/instances/${worldId}:${instanceId}`);
}

export async function fetchUser(userId: string): Promise<VrcUser> {
    return apiFetch<VrcUser>(`/api/v1/proxy/users/${encodeURIComponent(userId)}`);
}

export async function fetchFriends(params?: { offset?: number; n?: number; offline?: boolean }): Promise<VrcCurrentUser[]> {
    const sp = new URLSearchParams();
    if (params?.offset !== undefined) sp.set('offset', String(params.offset));
    if (params?.n !== undefined) sp.set('n', String(params.n));
    if (params?.offline !== undefined) sp.set('offline', String(params.offline));
    const qs = sp.toString() ? `?${sp}` : '';
    const friends = await apiFetch<VrcCurrentUser[]>(`/api/v1/proxy/auth/user/friends${qs}`);
    return friends.map((friend) => ({
        ...friend,
        state: friend.state ?? (friend.location === 'offline' ? 'offline' : 'online')
    }));
}
