// No-op stubs for native objects that don't exist in a browser PWA.
// All desktop-only features (SQLite, Discord, LogWatcher, AppApi, etc.)
// are replaced with harmless stubs so shared src/ code doesn't crash on import.

function noop() {}

// SQLite — Execute must return an array (iterable for .forEach); ExecuteJson returns '[]'
const SQLiteStub = {
    Execute: async () => [],
    ExecuteJson: async () => '[]',
    ExecuteNonQuery: async () => undefined
};

// VRCXStorage — backed by localStorage
const VRCXStorageShim = {
    GetValue: (key: string, defaultValue: string) =>
        localStorage.getItem(key) ?? defaultValue,
    SetValue: (key: string, value: string) => {
        localStorage.setItem(key, value);
    },
    RemoveValue: (key: string) => {
        localStorage.removeItem(key);
    }
};

// Silent proxy for all other native objects
const SilentProxy = new Proxy({} as Record<string, unknown>, {
    get(_target, prop) {
        if (prop === 'then') return undefined; // prevent accidental Promise wrapping
        return noop;
    }
});

export function installStubs(): void {
    (window as any).SQLite = SQLiteStub;
    (window as any).VRCXStorage = VRCXStorageShim;
    (window as any).AppApi = SilentProxy;
    (window as any).LogWatcher = SilentProxy;
    (window as any).Discord = SilentProxy;
    (window as any).AssetBundleManager = SilentProxy;
    (window as any).AppApiVrElectron = SilentProxy;
    (window as any).electron = SilentProxy;
}
