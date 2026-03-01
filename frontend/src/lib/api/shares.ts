import { api } from "./client";

export interface Share {
  id: string;
  slug: string;
  name: string;
  description: string;
  has_password: boolean;
  expires_at: string | null;
  max_downloads: number | null;
  download_count: number;
  max_views: number | null;
  view_count: number;
  is_reverse_share: boolean;
  created_at: string;
  updated_at: string;
}

export interface CreateShareInput {
  name: string;
  description?: string;
  slug?: string;
  password?: string;
  expires_at?: string;
  max_downloads?: number;
  max_views?: number;
  is_reverse_share?: boolean;
}

export const sharesApi = {
  list: () => api.get<Share[]>("/shares"),
  get: (id: string) => api.get<Share>(`/shares/${id}`),
  create: (input: CreateShareInput) => api.post<Share>("/shares", input),
  update: (id: string, input: Partial<CreateShareInput>) =>
    api.patch<Share>(`/shares/${id}`, input),
  delete: (id: string) => api.delete<void>(`/shares/${id}`),
};
