import { api } from "./client";
import type { LoginResponse } from "./auth";

export interface TOTPStatus {
  enabled: boolean;
  require_2fa: boolean;
}

export interface TOTPSetupResponse {
  secret: string;
  qr_code: string;
  provisioning_uri: string;
}

export interface TOTPConfirmResponse {
  recovery_codes: string[];
  access_token?: string;
  refresh_token?: string;
}

export const totpApi = {
  getStatus: () => api.get<TOTPStatus>("/me/2fa/status"),

  beginSetup: (token?: string) =>
    api.post<TOTPSetupResponse>("/me/2fa/setup", {}, { token }),

  confirmSetup: (code: string, token?: string) =>
    api.post<TOTPConfirmResponse>("/me/2fa/confirm", { code }, { token }),

  disable: (password: string) =>
    api.post<void>("/me/2fa/disable", { password }),

  regenerateRecoveryCodes: (password: string) =>
    api.post<TOTPConfirmResponse>("/me/2fa/recovery-codes", { password }),

  verify: (pendingToken: string, code: string) =>
    api.post<LoginResponse>("/auth/2fa/verify", {
      pending_token: pendingToken,
      code,
    }),

  recovery: (pendingToken: string, recoveryCode: string) =>
    api.post<LoginResponse>("/auth/2fa/recovery", {
      pending_token: pendingToken,
      recovery_code: recoveryCode,
    }),
};
