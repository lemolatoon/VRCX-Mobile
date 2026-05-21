const BASE = '/api/v1/feed';

async function apiFetch<T>(url: string): Promise<T> {
    const res = await fetch(url, { credentials: 'include' });
    if (!res.ok) {
        const body = await res.text().catch(() => '');
        throw new Error(`${res.status}: ${body}`);
    }
    return res.json() as Promise<T>;
}

export type FeedType = 'GPS' | 'Status' | 'Bio' | 'Avatar' | 'Online' | 'Offline';

export interface FeedEntry {
    id: number;
    type: FeedType;
    created_at: string;
    payload: Record<string, unknown>;
}

export interface FeedPage {
    entries: FeedEntry[];
    next_cursor: string | null;
}

export async function fetchFeedPage(opts: {
    types?: FeedType[];
    before?: string | null;
    limit?: number;
}): Promise<FeedPage> {
    const sp = new URLSearchParams();
    if (opts.types && opts.types.length > 0) sp.set('type', opts.types.join(','));
    if (opts.before) sp.set('before', opts.before);
    if (opts.limit) sp.set('limit', String(opts.limit));
    const qs = sp.toString() ? `?${sp}` : '';
    return apiFetch<FeedPage>(`${BASE}${qs}`);
}
