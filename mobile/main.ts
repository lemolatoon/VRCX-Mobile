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

const app = createApp(App);
app.use(pinia).use(router).use(VueQueryPlugin, { queryClient });
app.mount('#root');
