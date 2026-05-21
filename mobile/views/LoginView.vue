<script setup lang="ts">
import { ref } from 'vue';
import { useRouter } from 'vue-router';
import { applyCurrentUser } from '@core/coordinators/userCoordinator.js';
import { watchState } from '@core/services/watchState.js';

const router = useRouter();

// Step: 'credentials' | 'totp' | 'emailotp' | 'otp'
const step = ref<'credentials' | 'totp' | 'emailotp' | 'otp'>('credentials');
const username = ref('');
const password = ref('');
const otpCode = ref('');
const pendingKey = ref('');
const error = ref('');
const loading = ref(false);

async function handleCredentials() {
    if (!username.value || !password.value) return;
    loading.value = true;
    error.value = '';
    try {
        const res = await fetch('/api/v1/auth/login', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            credentials: 'include',
            body: JSON.stringify({ username: username.value, password: password.value })
        });
        const json = await res.json();
        if (!res.ok) {
            error.value = json.error ?? 'Login failed';
            return;
        }
        if (json.requiresTwoFactorAuth) {
            pendingKey.value = json.pending ?? username.value;
            // Pick the first available 2FA method (lowercase)
            const method = (json.requiresTwoFactorAuth[0] as string).toLowerCase();
            step.value = method as typeof step.value;
            return;
        }
        await finalize(json.currentUser);
    } catch {
        error.value = 'Network error — is the server reachable?';
    } finally {
        loading.value = false;
    }
}

async function handleOtp() {
    if (!otpCode.value) return;
    loading.value = true;
    error.value = '';
    try {
        const method = step.value; // totp | emailotp | otp
        const res = await fetch(`/api/v1/auth/2fa/${method}`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            credentials: 'include',
            body: JSON.stringify({ code: otpCode.value, pending: pendingKey.value })
        });
        const json = await res.json();
        if (!res.ok) {
            error.value = json.error ?? '2FA verification failed';
            return;
        }
        await finalize(json.currentUser);
    } catch {
        error.value = 'Network error — is the server reachable?';
    } finally {
        loading.value = false;
    }
}

async function finalize(currentUser: object) {
    applyCurrentUser(currentUser);
    watchState.isLoggedIn = true;
    router.replace({ name: 'friends' });
}

function otpLabel() {
    if (step.value === 'emailotp') return 'Email verification code';
    if (step.value === 'otp') return 'Recovery code';
    return 'Authenticator code';
}
</script>

<template>
    <div class="flex min-h-dvh flex-col items-center justify-center gap-8 p-6 bg-background">
        <div class="flex flex-col items-center gap-2 text-center">
            <img src="@core/../images/VRCX.png" alt="VRCX" class="h-16 w-16 rounded-2xl" />
            <h1 class="text-2xl font-bold text-foreground">VRCX Mobile</h1>
            <p class="text-sm text-muted-foreground">Sign in with your VRChat account</p>
        </div>

        <!-- Credentials step -->
        <form
            v-if="step === 'credentials'"
            class="flex w-full max-w-sm flex-col gap-3"
            @submit.prevent="handleCredentials"
        >
            <input
                v-model="username"
                type="text"
                placeholder="Username or email"
                autocomplete="username"
                :disabled="loading"
                class="rounded-lg border border-border bg-input px-4 py-3 text-foreground placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-primary disabled:opacity-50"
            />
            <input
                v-model="password"
                type="password"
                placeholder="Password"
                autocomplete="current-password"
                :disabled="loading"
                class="rounded-lg border border-border bg-input px-4 py-3 text-foreground placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-primary disabled:opacity-50"
            />
            <p v-if="error" class="text-sm text-destructive">{{ error }}</p>
            <button
                type="submit"
                :disabled="loading || !username || !password"
                class="rounded-lg bg-primary px-4 py-3 font-semibold text-primary-foreground disabled:opacity-50"
            >
                {{ loading ? 'Signing in…' : 'Sign In' }}
            </button>
        </form>

        <!-- 2FA step -->
        <form
            v-else
            class="flex w-full max-w-sm flex-col gap-4"
            @submit.prevent="handleOtp"
        >
            <div class="flex flex-col gap-1">
                <p class="text-sm font-medium text-foreground">{{ otpLabel() }}</p>
                <p class="text-xs text-muted-foreground">
                    <span v-if="step === 'emailotp'">Enter the code sent to your email.</span>
                    <span v-else-if="step === 'otp'">Enter one of your recovery codes.</span>
                    <span v-else>Enter the 6-digit code from your authenticator app.</span>
                </p>
            </div>
            <input
                v-model="otpCode"
                type="text"
                inputmode="numeric"
                :maxlength="step === 'otp' ? 40 : 6"
                placeholder="000000"
                autocomplete="one-time-code"
                :disabled="loading"
                class="rounded-lg border border-border bg-input px-4 py-4 text-center text-2xl tracking-[0.4em] text-foreground placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-primary disabled:opacity-50"
            />
            <p v-if="error" class="text-sm text-destructive">{{ error }}</p>
            <button
                type="submit"
                :disabled="loading || !otpCode"
                class="rounded-lg bg-primary px-4 py-3 font-semibold text-primary-foreground disabled:opacity-50"
            >
                {{ loading ? 'Verifying…' : 'Verify' }}
            </button>
            <button
                type="button"
                class="text-sm text-muted-foreground underline"
                @click="step = 'credentials'; error = ''"
            >
                Back to login
            </button>
        </form>
    </div>
</template>
