<script setup lang="ts">
// Login view — will be implemented in Phase 2
// For now, a placeholder that calls the existing auth store
import { ref } from 'vue';
import { useRouter } from 'vue-router';
import { useAuthStore } from '@core/stores/auth.js';

const router = useRouter();
const authStore = useAuthStore();

const username = ref('');
const password = ref('');
const error = ref('');
const loading = ref(false);

async function handleLogin() {
    if (!username.value || !password.value) return;
    loading.value = true;
    error.value = '';
    try {
        await authStore.authLogin({ username: username.value, password: password.value });
        router.push({ name: 'friends' });
    } catch (e: any) {
        error.value = e?.message ?? 'Login failed';
    } finally {
        loading.value = false;
    }
}
</script>

<template>
    <div class="flex min-h-dvh flex-col items-center justify-center gap-6 p-6 bg-background">
        <div class="flex flex-col items-center gap-2">
            <h1 class="text-2xl font-bold text-foreground">VRCX Mobile</h1>
            <p class="text-sm text-muted-foreground">Sign in with your VRChat account</p>
        </div>

        <form class="flex w-full max-w-sm flex-col gap-3" @submit.prevent="handleLogin">
            <input
                v-model="username"
                type="text"
                placeholder="Username or email"
                autocomplete="username"
                class="rounded-lg border border-border bg-input px-4 py-3 text-foreground placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-primary"
            />
            <input
                v-model="password"
                type="password"
                placeholder="Password"
                autocomplete="current-password"
                class="rounded-lg border border-border bg-input px-4 py-3 text-foreground placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-primary"
            />
            <p v-if="error" class="text-sm text-destructive">{{ error }}</p>
            <button
                type="submit"
                :disabled="loading"
                class="rounded-lg bg-primary px-4 py-3 font-semibold text-primary-foreground disabled:opacity-50"
            >
                {{ loading ? 'Signing in…' : 'Sign In' }}
            </button>
        </form>
    </div>
</template>
