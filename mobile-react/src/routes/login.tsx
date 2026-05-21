import { createFileRoute, useNavigate } from '@tanstack/react-router';
import { useState } from 'react';
import { OTPInput, REGEXP_ONLY_DIGITS } from 'input-otp';
import { Loader2 } from 'lucide-react';
import { useAuthStore } from '@/stores/auth';
import type { TwoFactorMethod } from '@/types/vrc';

export const Route = createFileRoute('/login')({
    component: LoginPage
});

const METHOD_LABELS: Record<TwoFactorMethod, string> = {
    totp: 'Authenticator App',
    otp: 'Recovery Code',
    emailotp: 'Email Code'
};

function LoginPage() {
    const navigate = useNavigate();
    const { login, verify2FA, requiresTwoFactor, twoFactorMethods } = useAuthStore();

    const [username, setUsername] = useState('');
    const [password, setPassword] = useState('');
    const [selectedMethod, setSelectedMethod] = useState<TwoFactorMethod>('totp');
    const [otpValue, setOtpValue] = useState('');
    const [loading, setLoading] = useState(false);
    const [error, setError] = useState('');

    async function handleLogin(e: React.FormEvent) {
        e.preventDefault();
        if (!username || !password) return;
        setLoading(true);
        setError('');
        try {
            const result = await login(username, password);
            if (!result.requiresTwoFactor) {
                await navigate({ to: '/friends' });
            }
        } catch (err) {
            setError(err instanceof Error ? err.message : 'Login failed');
        } finally {
            setLoading(false);
        }
    }

    async function handleVerify2FA(e: React.FormEvent) {
        e.preventDefault();
        if (otpValue.length < 6) return;
        setLoading(true);
        setError('');
        try {
            await verify2FA(selectedMethod, otpValue);
            await navigate({ to: '/friends' });
        } catch (err) {
            setError(err instanceof Error ? err.message : '2FA verification failed');
            setOtpValue('');
        } finally {
            setLoading(false);
        }
    }

    if (requiresTwoFactor) {
        const methods = twoFactorMethods.length > 0 ? twoFactorMethods : (['totp'] as TwoFactorMethod[]);
        return (
            <div className="flex flex-col items-center justify-center min-h-dvh p-6">
                <div className="w-full max-w-sm space-y-6">
                    <div className="text-center space-y-1">
                        <h1 className="text-2xl font-bold">Two-Factor Auth</h1>
                        <p className="text-sm text-muted-foreground">Enter your verification code</p>
                    </div>

                    {methods.length > 1 && (
                        <div className="flex rounded-lg border overflow-hidden">
                            {methods.map((m) => (
                                <button
                                    key={m}
                                    type="button"
                                    onClick={() => setSelectedMethod(m)}
                                    className={`flex-1 px-3 py-2 text-xs font-medium transition-colors ${
                                        selectedMethod === m
                                            ? 'bg-primary text-primary-foreground'
                                            : 'text-muted-foreground hover:text-foreground'
                                    }`}
                                >
                                    {METHOD_LABELS[m]}
                                </button>
                            ))}
                        </div>
                    )}

                    <form onSubmit={handleVerify2FA} className="space-y-4">
                        <div className="flex justify-center">
                            <OTPInput
                                maxLength={6}
                                pattern={REGEXP_ONLY_DIGITS}
                                value={otpValue}
                                onChange={setOtpValue}
                                autoFocus
                                render={({ slots }) => (
                                    <div className="flex gap-2">
                                        {slots.map((slot, i) => (
                                            <div
                                                key={i}
                                                className={`w-10 h-12 flex items-center justify-center text-lg font-mono border rounded-md transition-colors ${
                                                    slot.isActive ? 'border-primary ring-2 ring-ring' : 'border-input'
                                                }`}
                                            >
                                                {slot.char ?? <span className="w-1 h-5 bg-muted-foreground/40 rounded animate-pulse" />}
                                            </div>
                                        ))}
                                    </div>
                                )}
                            />
                        </div>

                        {error && <p className="text-sm text-destructive text-center">{error}</p>}

                        <button
                            type="submit"
                            disabled={loading || otpValue.length < 6}
                            className="w-full py-2.5 bg-primary text-primary-foreground rounded-md font-medium disabled:opacity-50 flex items-center justify-center gap-2"
                        >
                            {loading && <Loader2 className="w-4 h-4 animate-spin" />}
                            Verify
                        </button>
                    </form>
                </div>
            </div>
        );
    }

    return (
        <div className="flex flex-col items-center justify-center min-h-dvh p-6">
            <div className="w-full max-w-sm space-y-6">
                <div className="text-center space-y-1">
                    <h1 className="text-2xl font-bold">VRCX Mobile</h1>
                    <p className="text-sm text-muted-foreground">Sign in with your VRChat account</p>
                </div>

                <form onSubmit={handleLogin} className="space-y-4">
                    <div className="space-y-2">
                        <label className="text-sm font-medium">Username or Email</label>
                        <input
                            type="text"
                            value={username}
                            onChange={(e) => setUsername(e.target.value)}
                            autoComplete="username"
                            autoCapitalize="none"
                            className="w-full px-3 py-2.5 bg-input border border-border rounded-md text-sm focus:outline-none focus:ring-2 focus:ring-ring"
                        />
                    </div>

                    <div className="space-y-2">
                        <label className="text-sm font-medium">Password</label>
                        <input
                            type="password"
                            value={password}
                            onChange={(e) => setPassword(e.target.value)}
                            autoComplete="current-password"
                            className="w-full px-3 py-2.5 bg-input border border-border rounded-md text-sm focus:outline-none focus:ring-2 focus:ring-ring"
                        />
                    </div>

                    {error && <p className="text-sm text-destructive">{error}</p>}

                    <button
                        type="submit"
                        disabled={loading || !username || !password}
                        className="w-full py-2.5 bg-primary text-primary-foreground rounded-md font-medium disabled:opacity-50 flex items-center justify-center gap-2"
                    >
                        {loading && <Loader2 className="w-4 h-4 animate-spin" />}
                        Sign In
                    </button>
                </form>

                <p className="text-xs text-muted-foreground text-center">
                    You must be on the VRCX Mobile allowlist to access this app.
                </p>
            </div>
        </div>
    );
}
