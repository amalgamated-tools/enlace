import { writable } from 'svelte/store';

export type ToastType = 'success' | 'error' | 'info' | 'warning';

export interface Toast {
  id: string;
  type: ToastType;
  message: string;
}

const createToastStore = () => {
  const { subscribe, update } = writable<Toast[]>([]);

  const show = (type: ToastType, message: string, duration = 5000) => {
    const id = crypto.randomUUID();
    update((toasts) => [...toasts, { id, type, message }]);

    if (duration > 0) {
      setTimeout(() => {
        update((toasts) => toasts.filter((t) => t.id !== id));
      }, duration);
    }

    return id;
  };

  return {
    subscribe,
    success: (message: string) => show('success', message),
    error: (message: string) => show('error', message),
    info: (message: string) => show('info', message),
    warning: (message: string) => show('warning', message),
    dismiss: (id: string) => {
      update((toasts) => toasts.filter((t) => t.id !== id));
    },
    clear: () => {
      update(() => []);
    },
  };
};

export const toast = createToastStore();
