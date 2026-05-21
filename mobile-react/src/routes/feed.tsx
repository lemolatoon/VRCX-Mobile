import { createFileRoute } from '@tanstack/react-router';

export const Route = createFileRoute('/feed')({
    component: () => (
        <div className="flex items-center justify-center h-full">
            <p className="text-sm text-muted-foreground">Coming soon</p>
        </div>
    )
});
