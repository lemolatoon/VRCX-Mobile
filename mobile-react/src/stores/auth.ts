import { create } from 'zustand';
import { me, login as apiLogin, verify2FA as apiVerify2FA, logout as apiLogout } from '@/api/auth';
import type { TwoFactorMethod, VrcCurrentUser } from '@/types/vrc';

interface AuthState {
    currentUser: VrcCurrentUser | null;
    requiresTwoFactor: boolean;
    twoFactorMethods: TwoFactorMethod[];
    twoFactorPending: string;
    isRestoring: boolean;

    login: (username: string, password: string) => Promise<{ requiresTwoFactor: boolean }>;
    verify2FA: (method: TwoFactorMethod, code: string) => Promise<void>;
    logout: () => Promise<void>;
    restoreSession: () => Promise<void>;
    setCurrentUser: (user: VrcCurrentUser | null) => void;
}

export const useAuthStore = create<AuthState>((set) => ({
    currentUser: null,
    requiresTwoFactor: false,
    twoFactorMethods: [],
    twoFactorPending: '',
    isRestoring: true,

    login: async (username, password) => {
        const res = await apiLogin(username, password);
        if (res.requiresTwoFactorAuth && res.requiresTwoFactorAuth.length > 0) {
            set({
                requiresTwoFactor: true,
                twoFactorMethods: res.requiresTwoFactorAuth,
                twoFactorPending: res.pending ?? username
            });
            return { requiresTwoFactor: true };
        }
        const user = await me();
        if (user && user.id) {
            set({ currentUser: user as unknown as VrcCurrentUser, requiresTwoFactor: false, twoFactorPending: '' });
        }
        return { requiresTwoFactor: false };
    },

    verify2FA: async (method, code) => {
        const pending = useAuthStore.getState().twoFactorPending;
        if (!pending) throw new Error('Missing pending 2FA session');
        await apiVerify2FA(method, code, pending);
        const user = await me();
        if (user && user.id) {
            set({
                currentUser: user as unknown as VrcCurrentUser,
                requiresTwoFactor: false,
                twoFactorMethods: [],
                twoFactorPending: ''
            });
        }
    },

    logout: async () => {
        try {
            await apiLogout();
        } finally {
            set({ currentUser: null, requiresTwoFactor: false, twoFactorMethods: [], twoFactorPending: '' });
        }
    },

    restoreSession: async () => {
        set({ isRestoring: true });
        try {
            const user = await me();
            if (user && user.id && !user.requiresTwoFactorAuth) {
                set({ currentUser: user as unknown as VrcCurrentUser });
            }
        } finally {
            set({ isRestoring: false });
        }
    },

    setCurrentUser: (user) => set({ currentUser: user })
}));
