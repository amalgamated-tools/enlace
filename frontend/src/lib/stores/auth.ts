import { writable, derived } from "svelte/store";
import { authApi, type User } from "../api";

interface AuthState {
  user: User | null;
  loading: boolean;
  initialized: boolean;
}

export interface LoginResult {
  success: boolean;
  requires2FA?: boolean;
  requires2FASetup?: boolean;
  pendingToken?: string;
  user?: User;
}

const createAuthStore = () => {
  const { subscribe, set, update } = writable<AuthState>({
    user: null,
    loading: false,
    initialized: false,
  });

  const store = {
    subscribe,

    async init() {
      const token = localStorage.getItem("access_token");
      if (!token) {
        set({ user: null, loading: false, initialized: true });
        return;
      }

      update((s) => ({ ...s, loading: true }));
      try {
        // Try to get current user - if token expired, this will fail
        const response = await fetch("/api/v1/me", {
          headers: { Authorization: `Bearer ${token}` },
        });
        if (response.ok) {
          const data = await response.json();
          set({ user: data.data, loading: false, initialized: true });
        } else {
          // Token invalid, try refresh
          const refreshToken = localStorage.getItem("refresh_token");
          if (refreshToken) {
            try {
              const tokens = await authApi.refresh(refreshToken);
              localStorage.setItem("access_token", tokens.access_token);
              localStorage.setItem("refresh_token", tokens.refresh_token);
              // Retry get user
              const retryResponse = await fetch("/api/v1/me", {
                headers: { Authorization: `Bearer ${tokens.access_token}` },
              });
              if (retryResponse.ok) {
                const data = await retryResponse.json();
                set({ user: data.data, loading: false, initialized: true });
                return;
              }
            } catch {
              // Refresh failed
            }
          }
          // Clear tokens
          localStorage.removeItem("access_token");
          localStorage.removeItem("refresh_token");
          set({ user: null, loading: false, initialized: true });
        }
      } catch {
        set({ user: null, loading: false, initialized: true });
      }
    },

    async login(email: string, password: string): Promise<LoginResult> {
      update((s) => ({ ...s, loading: true }));
      try {
        const response = await authApi.login(email, password);

        if (response.requires_2fa && response.pending_token) {
          update((s) => ({ ...s, loading: false }));
          return {
            success: false,
            requires2FA: true,
            pendingToken: response.pending_token,
          };
        }

        if (response.requires_2fa_setup && response.pending_token) {
          update((s) => ({ ...s, loading: false }));
          return {
            success: false,
            requires2FASetup: true,
            pendingToken: response.pending_token,
            user: response.user,
          };
        }

        if (response.access_token) {
          if (!response.refresh_token) {
            throw new Error("Login failed: no refresh token received");
          }

          localStorage.setItem("access_token", response.access_token);
          localStorage.setItem("refresh_token", response.refresh_token);

          if (!response.user) {
            // Tokens without user data should not happen; clear and fail
            localStorage.removeItem("access_token");
            localStorage.removeItem("refresh_token");
            throw new Error("Login failed: no user data received");
          }

          set({
            user: response.user,
            loading: false,
            initialized: true,
          });

          return {
            success: true,
            requires2FASetup: response.requires_2fa_setup,
            user: response.user,
          };
        }

        throw new Error("Unexpected login response");
      } catch (error) {
        update((s) => ({ ...s, loading: false }));
        throw error;
      }
    },

    async register(email: string, password: string, displayName: string) {
      update((s) => ({ ...s, loading: true }));
      try {
        await authApi.register(email, password, displayName);
        // Auto-login after register
        return store.login(email, password);
      } catch (error) {
        update((s) => ({ ...s, loading: false }));
        throw error;
      }
    },

    async logout() {
      try {
        await authApi.logout();
      } catch {
        // Ignore logout errors
      }
      localStorage.removeItem("access_token");
      localStorage.removeItem("refresh_token");
      set({ user: null, loading: false, initialized: true });
    },

    setUser(user: User) {
      update((s) => ({ ...s, user }));
    },

    setTokens(accessToken: string, refreshToken: string) {
      localStorage.setItem("access_token", accessToken);
      localStorage.setItem("refresh_token", refreshToken);
      store.init();
    },
  };

  return store;
};

export const auth = createAuthStore();
export const isAuthenticated = derived(auth, ($auth) => $auth.user !== null);
export const isAdmin = derived(auth, ($auth) => $auth.user?.is_admin ?? false);
