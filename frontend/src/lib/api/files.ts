import { ApiError } from "./client";

export interface FileInfo {
  id: string;
  name: string;
  size: number;
  mime_type: string;
  created_at: string;
}

export const filesApi = {
  upload: async (shareId: string, files: File[]): Promise<FileInfo[]> => {
    const token = localStorage.getItem("access_token");
    const formData = new FormData();
    files.forEach((file) => formData.append("files", file));

    const response = await fetch(`/api/v1/shares/${shareId}/files`, {
      method: "POST",
      headers: token ? { Authorization: `Bearer ${token}` } : {},
      body: formData,
    });

    const data = await response.json();
    if (!response.ok || !data.success) {
      throw new ApiError(data.error || "Upload failed", response.status);
    }
    return data.data;
  },

  delete: async (fileId: string): Promise<void> => {
    const token = localStorage.getItem("access_token");
    const response = await fetch(`/api/v1/files/${fileId}`, {
      method: "DELETE",
      headers: token ? { Authorization: `Bearer ${token}` } : {},
    });

    const data = await response.json();
    if (!response.ok || !data.success) {
      throw new ApiError(data.error || "Delete failed", response.status);
    }
  },
};
