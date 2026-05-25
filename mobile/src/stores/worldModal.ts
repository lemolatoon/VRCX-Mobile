import { create } from 'zustand';

interface WorldModalState {
    location: string | null;
    open: (location: string) => void;
    close: () => void;
}

export const useWorldModal = create<WorldModalState>((set) => ({
    location: null,
    open: (location) => set({ location }),
    close: () => set({ location: null }),
}));
