// Stub for src/plugins/router.js in the mobile build.
// The desktop router imports every view component; mobile has its own router.
// This stub provides compatible exports so stores that reference the router
// (avatarProvider, gallery) compile without pulling in all the desktop views.
import type { Router } from 'vue-router';

export const router = null as unknown as Router;
export function initRouter(_app: unknown): void {}
