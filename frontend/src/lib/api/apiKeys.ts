import { api } from "./client";

export interface ApiKey {
  id: string;
  name: string;
  key_prefix: string;
  scopes: string[];
  revoked_at?: string;
  last_used_at?: string;
  created_at: string;
}

export interface CreateApiKeyResponse extends ApiKey {
  key: string;
}

export interface CreateApiKeyInput {
  name: string;
  scopes: string[];
}

export const apiKeysApi = {
  list: () => api.get<ApiKey[]>("/admin/api-keys"),
  create: (input: CreateApiKeyInput) =>
    api.post<CreateApiKeyResponse>("/admin/api-keys", input),
  revoke: (id: string) => api.delete<void>(`/admin/api-keys/${id}`),
};
