import { resolve } from 'node:path';
import fs from 'node:fs';

import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';
import tailwindcss from '@tailwindcss/vite';
import { VitePWA } from 'vite-plugin-pwa';
import { TanStackRouterVite } from '@tanstack/router-plugin/vite';

const version = fs
    .readFileSync(resolve(import.meta.dirname, '../Version'), 'utf-8')
    .trim();

export default defineConfig({
    root: import.meta.dirname,
    base: '/',
    plugins: [
        TanStackRouterVite({ routesDirectory: './src/routes', generatedRouteTree: './src/routeTree.gen.ts' }),
        react(),
        tailwindcss(),
        VitePWA({
            registerType: 'prompt',
            manifest: {
                name: 'VRCX Mobile',
                short_name: 'VRCX',
                description: 'VRCX companion for VRChat — mobile',
                theme_color: '#1a1a2e',
                background_color: '#0f0f1a',
                display: 'standalone',
                orientation: 'portrait',
                start_url: '/',
                icons: [
                    { src: '/icons/icon-192.png', sizes: '192x192', type: 'image/png' },
                    { src: '/icons/icon-512.png', sizes: '512x512', type: 'image/png' }
                ]
            },
            workbox: {
                navigateFallback: '/index.html',
                runtimeCaching: [
                    {
                        urlPattern: /^https:\/\/api\.vrchat\.cloud\/api\/1\/file\//,
                        handler: 'CacheFirst',
                        options: {
                            cacheName: 'vrchat-images',
                            expiration: { maxAgeSeconds: 60 * 60 * 24 * 7 }
                        }
                    },
                    {
                        urlPattern: /\/api\/v1\/proxy\//,
                        handler: 'NetworkFirst',
                        options: {
                            cacheName: 'vrcx-proxy',
                            expiration: { maxAgeSeconds: 60 * 60 * 24 }
                        }
                    },
                    {
                        urlPattern: /\/api\/v1\/auth\//,
                        handler: 'NetworkOnly'
                    }
                ]
            }
        })
    ],
    resolve: {
        alias: {
            '@': resolve(import.meta.dirname, './src')
        }
    },
    define: {
        __APP_VERSION__: JSON.stringify(version)
    },
    server: {
        port: 5174,
        strictPort: true,
        proxy: {
            '/api': {
                target: 'http://localhost:8080',
                changeOrigin: true
            }
        }
    },
    build: {
        outDir: resolve(import.meta.dirname, '../build/mobile'),
        emptyOutDir: true,
        target: 'es2020'
    }
});
