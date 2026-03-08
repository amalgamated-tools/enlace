import { ApiError } from "./client";

export interface FileInfo {
  id: string;
  name: string;
  size: number;
  mime_type: string;
  created_at: string;
}

interface DirectUploadResponse {
  upload_id: string;
  finalize_token: string;
  url: string;
  method: string;
  headers?: Record<string, string>;
}

async function uploadMultipart(
  shareId: string,
  files: File[],
): Promise<FileInfo[]> {
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
}

async function initiateDirectUpload(
  shareId: string,
  file: File,
  authHeaders: Record<string, string>,
): Promise<DirectUploadResponse> {
  const response = await fetch(`/api/v1/shares/${shareId}/files/initiate`, {
    method: "POST",
    headers: {
      ...authHeaders,
      "Content-Type": "application/json",
    },
    body: JSON.stringify({
      filename: file.name,
      size: file.size,
    }),
  });

  const data = await response.json();
  if (!response.ok || !data.success) {
    throw new ApiError(data.error || "Upload failed", response.status);
  }
  return data.data as DirectUploadResponse;
}

async function completeDirectUpload(
  transfer: DirectUploadResponse,
  file: File,
  authHeaders: Record<string, string>,
): Promise<FileInfo> {
  const uploadResponse = await fetch(transfer.url, {
    method: transfer.method,
    headers: transfer.headers ?? {},
    body: file,
  });
  if (!uploadResponse.ok) {
    throw new ApiError("Direct upload failed", uploadResponse.status);
  }

  const finalizeResponse = await fetch(
    `/api/v1/files/uploads/${transfer.upload_id}/finalize`,
    {
      method: "POST",
      headers: {
        ...authHeaders,
        "Content-Type": "application/json",
      },
      body: JSON.stringify({ token: transfer.finalize_token }),
    },
  );
  const finalizeData = await finalizeResponse.json();
  if (!finalizeResponse.ok || !finalizeData.success) {
    throw new ApiError(
      finalizeData.error || "Upload failed",
      finalizeResponse.status,
    );
  }
  return finalizeData.data;
}

export const filesApi = {
  upload: async (shareId: string, files: File[]): Promise<FileInfo[]> => {
    if (files.length === 0) {
      return [];
    }

    const token = localStorage.getItem("access_token");
    const authHeaders: Record<string, string> = token
      ? { Authorization: `Bearer ${token}` }
      : {};

    // Probe with the first file to determine if direct transfer is supported.
    // This avoids partial direct uploads followed by a full multipart retry.
    let firstTransfer: DirectUploadResponse;
    try {
      firstTransfer = await initiateDirectUpload(
        shareId,
        files[0],
        authHeaders,
      );
    } catch (error) {
      if (error instanceof ApiError && error.status === 409) {
        return uploadMultipart(shareId, files);
      }
      throw error;
    }

    // Direct transfer is supported — upload all files via direct path
    const uploadedFiles: FileInfo[] = [];
    uploadedFiles.push(
      await completeDirectUpload(firstTransfer, files[0], authHeaders),
    );

    for (const file of files.slice(1)) {
      const transfer = await initiateDirectUpload(shareId, file, authHeaders);
      uploadedFiles.push(
        await completeDirectUpload(transfer, file, authHeaders),
      );
    }

    return uploadedFiles;
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
