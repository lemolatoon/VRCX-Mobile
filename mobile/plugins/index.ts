// Mobile-specific plugin initialization — no Electron/CefSharp bindings
import configRepository from '@core/services/config.js';
import vrcxJsonStorage from '@core/services/jsonStorage.js';

export { i18n } from '@core/plugins/i18n';

export async function initMobilePlugins(): Promise<void> {
    // configRepository.init() calls SQLite.ExecuteNonQuery (no-op stub in PWA)
    await configRepository.init();
    // vrcxJsonStorage reads from VRCXStorage (localStorage shim in PWA)
    new vrcxJsonStorage((window as any).VRCXStorage);
    // AppApi.SetUserAgent() is a no-op in PWA
}
