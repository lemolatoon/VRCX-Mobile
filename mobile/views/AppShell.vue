<script setup lang="ts">
import { RouterView, useRoute, useRouter } from 'vue-router';
import { computed } from 'vue';

const route = useRoute();
const router = useRouter();

const tabs = [
    { name: 'friends', icon: 'ri-group-line', label: 'Friends' },
    { name: 'feed', icon: 'ri-newspaper-line', label: 'Feed' },
    { name: 'notifications', icon: 'ri-notification-3-line', label: 'Notifications' },
    { name: 'search', icon: 'ri-search-line', label: 'Search' },
    { name: 'settings', icon: 'ri-settings-3-line', label: 'Settings' }
] as const;

const activeTab = computed(() =>
    tabs.find((t) => route.matched.some((m) => m.name === t.name))?.name ?? 'friends'
);

function navigate(name: string) {
    router.push({ name });
}
</script>

<template>
    <div class="flex min-h-dvh flex-col bg-background text-foreground">
        <!-- Main content area (scrollable) -->
        <main class="flex-1 overflow-y-auto pb-[calc(4rem+env(safe-area-inset-bottom))]">
            <RouterView />
        </main>

        <!-- Bottom tab bar -->
        <nav
            class="fixed bottom-0 left-0 right-0 z-50 flex items-center justify-around border-t border-border bg-background/90 backdrop-blur-md"
            :style="{ paddingBottom: 'env(safe-area-inset-bottom)' }"
        >
            <button
                v-for="tab in tabs"
                :key="tab.name"
                class="flex flex-col items-center gap-0.5 px-3 py-2 transition-colors"
                :class="activeTab === tab.name ? 'text-primary' : 'text-muted-foreground'"
                @click="navigate(tab.name)"
            >
                <i :class="[tab.icon, 'text-xl']" />
                <span class="text-[10px] font-medium">{{ tab.label }}</span>
            </button>
        </nav>
    </div>
</template>
