import { api, ApiError } from "./client";

export interface FileInfo {
  id: string;
  name: string;
  size: number;
  mime_type: string;
  created_at: string;
}

interface DirectUploadInitResponse {
  upload_id: string;
  upload_url: string;
  file_id: string;
  method: string;
  expires_at: string;
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

  directUploadInit: async (
    shareId: string,
    filename: string,
    size: number,
    contentType: string,
  ): Promise<DirectUploadInitResponse> => {
    return api.post<DirectUploadInitResponse>(
      `/shares/${shareId}/files/direct/init`,
      { filename, size, content_type: contentType },
    );
  },

  directUploadFinalize: async (
    shareId: string,
    uploadId: string,
  ): Promise<FileInfo> => {
    return api.post<FileInfo>(`/shares/${shareId}/files/direct/finalize`, {
      upload_id: uploadId,
    });
  },

  uploadWithDirectTransfer: async (
    shareId: string,
    files: File[],
  ): Promise<FileInfo[]> => {
    const results: FileInfo[] = [];

    for (const file of files) {
      try {
        // Attempt direct upload
        const init = await filesApi.directUploadInit(
          shareId,
          file.name,
          file.size,
          file.type || "application/octet-stream",
        );

        // Upload directly to storage
        const uploadResp = await fetch(init.upload_url, {
          method: init.method,
          headers: { "Content-Type": file.type || "application/octet-stream" },
          body: file,
        });

        if (!uploadResp.ok) {
          throw new Error("Direct upload to storage failed");
        }

        // Finalize
        const fileInfo = await filesApi.directUploadFinalize(
          shareId,
          init.upload_id,
        );
        results.push(fileInfo);
      } catch (err) {
        // If direct transfer is unsupported (409) or fails, fall back to proxy upload
        if (err instanceof ApiError && err.status === 409) {
          return filesApi.upload(shareId, files);
        }
        throw err;
      }
    }

    return results;
  },
};
