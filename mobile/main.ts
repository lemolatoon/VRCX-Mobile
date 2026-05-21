// Shims MUST be installed before any src/ imports, as those modules
// reference window.WebApi / window.SQLite etc. at call time.
import { installAllShims } from './shims';
installAllShims();

import { VueQueryPlugin } from '@tanstack/vue-query';
import { createApp } from 'vue';

import { queryClient } from './queries/client';
import { initMobilePlugins } from './plugins';
import { pinia, initMobileStores } from './stores';
import App from './App.vue';
import router from './router';

await initMobilePlugins();
await initMobileStores();

// Restore session before first route resolution so the auth guard
// has correct isLoggedIn state on initial load.
await restoreSession();

const app = createApp(App);
app.use(pinia).use(router).use(VueQueryPlugin, { queryClient });
app.mount('#root');

async function restoreSession() {
    try {
        const res = await fetch('/api/v1/auth/me', { credentials: 'include' });
        if (!res.ok) return;
        const json = await res.json();
        const { applyCurrentUser } = await import('@core/coordinators/userCoordinator.js');
        const { watchState } = await import('@core/services/watchState.js');
        applyCurrentUser(json);
        watchState.isLoggedIn = true;
    } catch {
        // No session or network error — router guard will redirect to /login
    }
}
