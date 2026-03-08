import { ApiError } from "./client";
import type { EncryptedFile } from "../crypto/e2e";

export interface FileInfo {
  id: string;
  name: string;
  size: number;
  mime_type: string;
  encryption_iv?: string;
  encrypted_metadata?: string;
  created_at: string;
}

export interface EncryptedUploadOptions {
  encryptionIV: string;
  encryptedMetadata: string;
}

export const filesApi = {
  upload: async (
    shareId: string,
    files: File[],
    encryptedFiles?: EncryptedFile[],
  ): Promise<FileInfo[]> => {
    const token = localStorage.getItem("access_token");
    const formData = new FormData();

    if (encryptedFiles && encryptedFiles.length === files.length) {
      // E2E encrypted upload: use encrypted blobs and attach metadata
      encryptedFiles.forEach((ef, i) => {
        const encryptedBlob = new File([ef.blob], files[i].name, {
          type: "application/octet-stream",
        });
        formData.append("files", encryptedBlob);
      });
      // All files in a share use the same metadata format; send per-file IVs
      // as comma-separated values
      formData.append(
        "encryption_iv",
        encryptedFiles.map((ef) => ef.iv).join(","),
      );
      formData.append(
        "encrypted_metadata",
        encryptedFiles.map((ef) => ef.encryptedMeta).join(","),
      );
    } else {
      files.forEach((file) => formData.append("files", file));
    }

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
