const BASE = '/api/v1';

async function apiFetch<T>(url: string, init?: RequestInit): Promise<T> {
    const res = await fetch(url, { credentials: 'include', ...init });
    if (!res.ok) {
        const body = await res.text().catch(() => '');
        throw new Error(`${res.status}: ${body}`);
    }
    return res.json() as Promise<T>;
}

export type GameLogType =
    | 'Location'
    | 'LocationDestination'
    | 'OnPlayerJoined'
    | 'OnPlayerLeft'
    | 'PortalSpawn'
    | 'VideoPlay'
    | 'ResourceLoad'
    | 'Event'
    | 'Unknown';

export interface GameLogEntry {
    id: number;
    type: GameLogType;
    created_at: string;
    payload: Record<string, unknown>;
    raw_line?: string;
}

export interface GameLogPage {
    entries: GameLogEntry[];
    next_cursor: string | null;
}

export interface AgentToken {
    id: string;
    name: string;
    created_at: string;
    last_used_at: string | null;
    revoked_at?: string | null;
}

export interface CreatedAgentToken extends AgentToken {
    token: string;
}

export async function fetchGameLogPage(opts: {
    types?: GameLogType[];
    before?: string | null;
    limit?: number;
    search?: string;
}): Promise<GameLogPage> {
    const sp = new URLSearchParams();
    if (opts.types && opts.types.length > 0) sp.set('type', opts.types.join(','));
    if (opts.before) sp.set('before', opts.before);
    if (opts.limit) sp.set('limit', String(opts.limit));
    if (opts.search) sp.set('search', opts.search);
    const qs = sp.toString() ? `?${sp}` : '';
    return apiFetch<GameLogPage>(`${BASE}/gamelog${qs}`);
}

export async function listAgentTokens(): Promise<{ tokens: AgentToken[] }> {
    return apiFetch<{ tokens: AgentToken[] }>(`${BASE}/agent-tokens`);
}

export async function createAgentToken(name: string): Promise<CreatedAgentToken> {
    return apiFetch<CreatedAgentToken>(`${BASE}/agent-tokens`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ name }),
    });
}

export async function revokeAgentToken(id: string): Promise<{ ok: boolean }> {
    return apiFetch<{ ok: boolean }>(`${BASE}/agent-tokens/${encodeURIComponent(id)}`, {
        method: 'DELETE',
    });
}
