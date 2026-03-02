import { api } from "./client";

export interface User {
  id: string;
  email: string;
  display_name: string;
  is_admin: boolean;
  oidc_linked?: boolean;
  has_password?: boolean;
}

export interface LoginResponse {
  access_token?: string;
  refresh_token?: string;
  user?: User;
  requires_2fa?: boolean;
  requires_2fa_setup?: boolean;
  pending_token?: string;
}

export interface TokenResponse {
  access_token: string;
  refresh_token: string;
}

export const authApi = {
  register: (email: string, password: string, displayName: string) =>
    api.post<{ user: User }>("/auth/register", {
      email,
      password,
      display_name: displayName,
    }),

  login: (email: string, password: string) =>
    api.post<LoginResponse>("/auth/login", { email, password }),

  refresh: (refreshToken: string) =>
    api.post<TokenResponse>("/auth/refresh", { refresh_token: refreshToken }),

  logout: () => api.post<void>("/auth/logout", {}),
};
