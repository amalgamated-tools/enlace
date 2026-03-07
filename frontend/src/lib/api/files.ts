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
    if (files.length === 0) {
      return filesApi.uploadMultipart(shareId, files);
    }

    try {
      const uploaded: FileInfo[] = [];
      for (const file of files) {
        uploaded.push(await filesApi.uploadDirect(shareId, file));
      }
      return uploaded;
    } catch (err) {
      if (err instanceof ApiError && err.status === 409) {
        return filesApi.uploadMultipart(shareId, files);
      }
      throw err;
    }
  },

  uploadMultipart: async (shareId: string, files: File[]): Promise<FileInfo[]> => {
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

  uploadDirect: async (shareId: string, file: File): Promise<FileInfo> => {
    const token = localStorage.getItem("access_token");
    const jsonHeaders: Record<string, string> = {
      "Content-Type": "application/json",
    };
    if (token) {
      jsonHeaders.Authorization = `Bearer ${token}`;
    }

    const initiateResponse = await fetch(`/api/v1/shares/${shareId}/files/initiate`, {
      method: "POST",
      headers: jsonHeaders,
      body: JSON.stringify({
        filename: file.name,
        size: file.size,
      }),
    });
    const initiateData = await initiateResponse.json();
    if (!initiateResponse.ok || !initiateData.success) {
      throw new ApiError(initiateData.error || "Upload initialization failed", initiateResponse.status);
    }

    const uploadResponse = await fetch(initiateData.data.upload.url, {
      method: initiateData.data.upload.method || "PUT",
      headers: initiateData.data.upload.headers || {},
      body: file,
    });
    if (!uploadResponse.ok) {
      throw new ApiError("Direct upload failed", uploadResponse.status);
    }

    const finalizeResponse = await fetch(`/api/v1/shares/${shareId}/files/finalize`, {
      method: "POST",
      headers: jsonHeaders,
      body: JSON.stringify({
        upload_id: initiateData.data.upload_id,
        finalize_token: initiateData.data.finalize_token,
      }),
    });
    const finalizeData = await finalizeResponse.json();
    if (!finalizeResponse.ok || !finalizeData.success) {
      throw new ApiError(finalizeData.error || "Upload finalization failed", finalizeResponse.status);
    }
    return finalizeData.data;
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
