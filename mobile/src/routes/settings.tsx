import { createFileRoute } from '@tanstack/react-router';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Copy, KeyRound, Trash2 } from 'lucide-react';
import { useState } from 'react';
import { createAgentToken, listAgentTokens, revokeAgentToken, type CreatedAgentToken } from '@/api/gamelog';

function SettingsPage() {
    const queryClient = useQueryClient();
    const [name, setName] = useState('Windows PC');
    const [created, setCreated] = useState<CreatedAgentToken | null>(null);
    const origin = window.location.origin;
    const { data, isLoading, isError } = useQuery({
        queryKey: ['agent-tokens'],
        queryFn: listAgentTokens,
    });
    const createMutation = useMutation({
        mutationFn: () => createAgentToken(name),
        onSuccess: (token) => {
            setCreated(token);
            queryClient.invalidateQueries({ queryKey: ['agent-tokens'] });
        },
    });
    const revokeMutation = useMutation({
        mutationFn: revokeAgentToken,
        onSuccess: () => queryClient.invalidateQueries({ queryKey: ['agent-tokens'] }),
    });

    const setupCommand = created
        ? `vrcx-log-agent.exe setup --server "${origin}" --token "${created.token}"\nvrcx-log-agent.exe install-startup`
        : '';

    return (
        <div className="h-full overflow-y-auto px-4 py-4 space-y-4">
            <section className="space-y-3">
                <div className="flex items-center gap-2">
                    <KeyRound className="w-5 h-5" />
                    <h1 className="text-base font-semibold">GameLog Agent</h1>
                </div>
                <div className="space-y-2">
                    <label className="text-xs text-muted-foreground" htmlFor="agent-name">Token name</label>
                    <div className="flex gap-2">
                        <input
                            id="agent-name"
                            value={name}
                            onChange={(e) => setName(e.target.value)}
                            className="min-w-0 flex-1 px-3 py-2 bg-input border border-border rounded-md text-sm focus:outline-none focus:ring-2 focus:ring-ring"
                        />
                        <button
                            type="button"
                            disabled={createMutation.isPending}
                            onClick={() => createMutation.mutate()}
                            className="shrink-0 px-3 py-2 rounded-md bg-primary text-primary-foreground text-sm font-medium disabled:opacity-60"
                        >
                            Create
                        </button>
                    </div>
                    {createMutation.isError && <p className="text-xs text-destructive">Failed to create token.</p>}
                </div>
            </section>

            {created && (
                <section className="space-y-2 border border-border rounded-md p-3">
                    <div className="flex items-center justify-between gap-2">
                        <h2 className="text-sm font-semibold">New token</h2>
                        <button
                            type="button"
                            onClick={() => navigator.clipboard?.writeText(created.token)}
                            className="p-2 text-muted-foreground hover:text-foreground"
                            aria-label="Copy token"
                        >
                            <Copy className="w-4 h-4" />
                        </button>
                    </div>
                    <p className="text-xs text-muted-foreground">This token is shown once.</p>
                    <pre className="whitespace-pre-wrap break-all rounded bg-muted px-3 py-2 text-xs font-mono">{created.token}</pre>
                    <div className="flex items-center justify-between gap-2">
                        <h3 className="text-sm font-semibold">Windows setup</h3>
                        <button
                            type="button"
                            onClick={() => navigator.clipboard?.writeText(setupCommand)}
                            className="p-2 text-muted-foreground hover:text-foreground"
                            aria-label="Copy setup commands"
                        >
                            <Copy className="w-4 h-4" />
                        </button>
                    </div>
                    <pre className="whitespace-pre-wrap break-all rounded bg-muted px-3 py-2 text-xs font-mono">{setupCommand}</pre>
                </section>
            )}

            <section className="space-y-2">
                <h2 className="text-sm font-semibold">Existing tokens</h2>
                {isLoading && <p className="text-sm text-muted-foreground">Loading...</p>}
                {isError && <p className="text-sm text-destructive">Failed to load tokens.</p>}
                <div className="divide-y divide-border border border-border rounded-md overflow-hidden">
                    {(data?.tokens ?? []).map((token) => (
                        <div key={token.id} className="flex items-center gap-3 px-3 py-2">
                            <div className="min-w-0 flex-1">
                                <p className="text-sm font-medium truncate">{token.name}</p>
                                <p className="text-xs text-muted-foreground">
                                    Created {formatDate(token.created_at)}
                                    {token.last_used_at ? ` · Used ${formatDate(token.last_used_at)}` : ''}
                                    {token.revoked_at ? ' · Revoked' : ''}
                                </p>
                            </div>
                            {!token.revoked_at && (
                                <button
                                    type="button"
                                    disabled={revokeMutation.isPending}
                                    onClick={() => revokeMutation.mutate(token.id)}
                                    className="p-2 text-muted-foreground hover:text-destructive disabled:opacity-60"
                                    aria-label="Revoke token"
                                >
                                    <Trash2 className="w-4 h-4" />
                                </button>
                            )}
                        </div>
                    ))}
                    {!isLoading && (data?.tokens ?? []).length === 0 && (
                        <p className="px-3 py-4 text-sm text-muted-foreground">No tokens yet</p>
                    )}
                </div>
            </section>
        </div>
    );
}

function formatDate(value: string) {
    return new Date(value).toLocaleString();
}

export const Route = createFileRoute('/settings')({
    component: SettingsPage
});
