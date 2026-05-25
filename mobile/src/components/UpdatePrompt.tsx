import { RefreshCw } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { useRegisterSW } from 'virtual:pwa-register/react';

export function UpdatePrompt() {
    const { t } = useTranslation();
    const {
        needRefresh: [needRefresh, setNeedRefresh],
        updateServiceWorker
    } = useRegisterSW();

    if (!needRefresh) return null;

    return (
        <div
            className="fixed inset-x-4 z-50 flex items-center justify-between gap-3 bg-card border border-border rounded-xl px-4 py-3 shadow-lg"
            style={{ bottom: 'calc(3.75rem + env(safe-area-inset-bottom) + 0.5rem)' }}
        >
            <div className="flex items-center gap-2 min-w-0">
                <RefreshCw className="w-4 h-4 flex-shrink-0 text-primary" />
                <span className="text-sm font-medium truncate">
                    {t('update.available', { defaultValue: 'A new version is available.' })}
                </span>
            </div>
            <div className="flex items-center gap-2 flex-shrink-0">
                <button
                    type="button"
                    onClick={() => setNeedRefresh(false)}
                    className="text-xs text-muted-foreground hover:text-foreground transition-colors"
                >
                    {t('update.dismiss', { defaultValue: 'Later' })}
                </button>
                <button
                    type="button"
                    onClick={() => updateServiceWorker(true)}
                    className="text-xs px-3 py-1.5 bg-primary text-primary-foreground rounded-md font-medium"
                >
                    {t('update.reload', { defaultValue: 'Reload' })}
                </button>
            </div>
        </div>
    );
}
