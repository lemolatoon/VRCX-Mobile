// Minimal Pinia store set for the mobile read-only PWA.
// Desktop-only stores (gameLog, photon, vr, discord, etc.) are excluded.
import { createPinia } from 'pinia';

export const pinia = createPinia();

export async function initMobileStores(): Promise<void> {
    // Eagerly initialize stores that run setup logic in their constructors.
    // Stores are created lazily by useXxxStore() in components, but a few
    // need to run init() or start polling here.
    const { useAuthStore } = await import('@core/stores/auth.js');
    const { useUiStore } = await import('@core/stores/ui.js');
    const { useVrcStatusStore } = await import('@core/stores/vrcStatus.js');

    // Trigger creation so watchers/timers start
    useAuthStore();
    useUiStore();
    useVrcStatusStore();
}
