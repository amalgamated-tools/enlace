import { api } from "./client";

export interface OIDCConfig {
  enabled: boolean;
}

export async function getOIDCConfig(): Promise<OIDCConfig> {
  return api.get<OIDCConfig>("/auth/oidc/config");
}

export function getOIDCLoginURL(): string {
  return "/api/v1/auth/oidc/login";
}

export function getOIDCLinkURL(): string {
  return "/api/v1/me/oidc/link";
}
