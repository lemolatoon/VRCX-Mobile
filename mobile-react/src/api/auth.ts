import type { AuthMeResponse, LoginResponse, TwoFactorMethod, VrcCurrentUser } from '@/types/vrc';

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
        const body = await res.text().catch(() => '');
        throw new Error(`${res.status}: ${body}`);
    }
    return res.json() as Promise<T>;
}

export async function login(username: string, password: string): Promise<LoginResponse> {
    return apiFetch<LoginResponse>(`${BASE}/login`, {
        method: 'POST',
        body: JSON.stringify({ username, password })
    });
}

export async function verify2FA(method: TwoFactorMethod, code: string): Promise<{ verified: boolean }> {
    return apiFetch(`${BASE}/2fa/${method}`, {
        method: 'POST',
        body: JSON.stringify({ code })
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

export async function fetchFriends(params?: { offset?: number; n?: number; offline?: boolean }): Promise<VrcCurrentUser[]> {
    const sp = new URLSearchParams();
    if (params?.offset !== undefined) sp.set('offset', String(params.offset));
    if (params?.n !== undefined) sp.set('n', String(params.n));
    if (params?.offline !== undefined) sp.set('offline', String(params.offline));
    const qs = sp.toString() ? `?${sp}` : '';
    return apiFetch<VrcCurrentUser[]>(`/api/v1/friends${qs}`);
}
