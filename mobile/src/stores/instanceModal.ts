import { create } from 'zustand';

interface InstanceModalState {
    location: string | null;
    open: (location: string) => void;
    close: () => void;
}

export const useInstanceModal = create<InstanceModalState>((set) => ({
    location: null,
    open: (location) => set({ location }),
    close: () => set({ location: null }),
}));
