import { createRouter, createWebHistory } from 'vue-router';
import { watchState } from '@core/services/watchState';

const router = createRouter({
    history: createWebHistory('/'),
    routes: [
        {
            path: '/login',
            name: 'login',
            component: () => import('../views/LoginView.vue'),
            meta: { public: true }
        },
        {
            path: '/',
            component: () => import('../views/AppShell.vue'),
            meta: { requiresAuth: true },
            children: [
                { path: '', redirect: { name: 'friends' } },
                {
                    path: 'friends',
                    name: 'friends',
                    component: () => import('../views/FriendsView.vue')
                },
                {
                    path: 'feed',
                    name: 'feed',
                    component: () => import('../views/FeedView.vue')
                },
                {
                    path: 'notifications',
                    name: 'notifications',
                    component: () => import('../views/NotificationsView.vue')
                },
                {
                    path: 'search',
                    name: 'search',
                    component: () => import('../views/SearchView.vue')
                },
                {
                    path: 'settings',
                    name: 'settings',
                    component: () => import('../views/SettingsView.vue')
                },
                // Detail views (pushed on top of any tab)
                {
                    path: 'user/:userId',
                    name: 'user',
                    component: () => import('../views/UserView.vue'),
                    props: true
                },
                {
                    path: 'world/:worldId',
                    name: 'world',
                    component: () => import('../views/WorldView.vue'),
                    props: true
                },
                {
                    path: 'group/:groupId',
                    name: 'group',
                    component: () => import('../views/GroupView.vue'),
                    props: true
                }
            ]
        },
        { path: '/:pathMatch(.*)*', redirect: { name: 'friends' } }
    ]
});

router.beforeEach((to) => {
    if (!to.meta.public && !watchState.isLoggedIn) {
        return { name: 'login' };
    }
    if (to.name === 'login' && watchState.isLoggedIn) {
        return { name: 'friends' };
    }
});

export default router;
