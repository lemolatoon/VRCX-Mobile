import { createFileRoute } from '@tanstack/react-router';
import { useTranslation } from 'react-i18next';

function NotificationsPage() {
    const { t } = useTranslation();
    return (
        <div className="flex items-center justify-center h-full">
            <p className="text-sm text-muted-foreground">{t('mobile.common.coming_soon', { defaultValue: 'Coming soon' })}</p>
        </div>
    );
}

export const Route = createFileRoute('/notifications')({
    component: NotificationsPage
});
