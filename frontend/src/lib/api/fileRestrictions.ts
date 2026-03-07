import { api } from "./client";

export interface FileRestrictions {
  max_file_size: number | null;
  blocked_extensions: string[];
}

export interface UpdateFileRestrictionsInput {
  max_file_size?: number;
  blocked_extensions?: string;
}

export const fileRestrictionsApi = {
  get: () => api.get<FileRestrictions>("/admin/files"),
  update: (input: UpdateFileRestrictionsInput) =>
    api.put<FileRestrictions>("/admin/files", input),
  reset: () => api.delete<void>("/admin/files"),
};
