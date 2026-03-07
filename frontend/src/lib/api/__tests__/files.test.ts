import { describe, it, expect, vi, beforeEach } from "vitest";
import { filesApi } from "../files";
import { ApiError } from "../client";

const mockFetch = vi.fn();

beforeEach(() => {
  vi.stubGlobal("fetch", mockFetch);
  mockFetch.mockReset();
  localStorage.clear();
});

describe("filesApi", () => {
  describe("upload", () => {
    it("sends files as FormData", async () => {
      const fileData = [
        {
          id: "f1",
          name: "test.txt",
          size: 100,
          mime_type: "text/plain",
          created_at: "2024-01-01",
        },
      ];
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve({ success: true, data: fileData }),
      });

      const file = new File(["hello"], "test.txt", { type: "text/plain" });
      const result = await filesApi.upload("share-1", [file]);

      expect(result).toEqual(fileData);
      expect(mockFetch).toHaveBeenCalledWith(
        "/api/v1/shares/share-1/files",
        expect.objectContaining({ method: "POST" }),
      );
      const callArgs = mockFetch.mock.calls[0][1];
      expect(callArgs.body).toBeInstanceOf(FormData);
    });

    it("includes auth header when token exists", async () => {
      localStorage.setItem("access_token", "my-token");
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve({ success: true, data: [] }),
      });

      await filesApi.upload("share-1", []);
      const headers = mockFetch.mock.calls[0][1].headers;
      expect(headers.Authorization).toBe("Bearer my-token");
    });

    it("throws ApiError on failure", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: false,
        status: 400,
        json: () => Promise.resolve({ success: false, error: "Too large" }),
      });

      await expect(
        filesApi.upload("share-1", [new File(["x"], "big.bin")]),
      ).rejects.toThrow(ApiError);
    });
  });

  describe("directUploadInit", () => {
    it("sends init request with file metadata", async () => {
      const initData = {
        upload_id: "up-1",
        upload_url: "https://s3.example.com/presigned",
        file_id: "f-1",
        method: "PUT",
        expires_at: "2024-01-01T00:10:00Z",
      };
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve({ success: true, data: initData }),
      });

      const result = await filesApi.directUploadInit(
        "share-1",
        "test.txt",
        100,
        "text/plain",
      );
      expect(result).toEqual(initData);
      expect(mockFetch).toHaveBeenCalledWith(
        "/api/v1/shares/share-1/files/direct/init",
        expect.objectContaining({ method: "POST" }),
      );
    });
  });

  describe("directUploadFinalize", () => {
    it("sends finalize request with upload ID", async () => {
      const fileData = {
        id: "f-1",
        name: "test.txt",
        size: 100,
        mime_type: "text/plain",
        created_at: "2024-01-01",
      };
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve({ success: true, data: fileData }),
      });

      const result = await filesApi.directUploadFinalize("share-1", "up-1");
      expect(result).toEqual(fileData);
      expect(mockFetch).toHaveBeenCalledWith(
        "/api/v1/shares/share-1/files/direct/finalize",
        expect.objectContaining({ method: "POST" }),
      );
    });
  });

  describe("uploadWithDirectTransfer", () => {
    it("uses direct upload when supported", async () => {
      const initData = {
        upload_id: "up-1",
        upload_url: "https://s3.example.com/presigned",
        file_id: "f-1",
        method: "PUT",
        expires_at: "2024-01-01T00:10:00Z",
      };
      const fileData = {
        id: "f-1",
        name: "test.txt",
        size: 5,
        mime_type: "text/plain",
        created_at: "2024-01-01",
      };

      // init call
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve({ success: true, data: initData }),
      });
      // PUT to presigned URL
      mockFetch.mockResolvedValueOnce({ ok: true });
      // finalize call
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve({ success: true, data: fileData }),
      });

      const file = new File(["hello"], "test.txt", { type: "text/plain" });
      const result = await filesApi.uploadWithDirectTransfer("share-1", [file]);

      expect(result).toEqual([fileData]);
      expect(mockFetch).toHaveBeenCalledTimes(3);
      // Verify PUT to presigned URL
      expect(mockFetch.mock.calls[1][0]).toBe(
        "https://s3.example.com/presigned",
      );
      expect(mockFetch.mock.calls[1][1].method).toBe("PUT");
    });

    it("falls back to proxy upload on 409", async () => {
      // init returns 409 (direct transfer unsupported)
      mockFetch.mockResolvedValueOnce({
        ok: false,
        status: 409,
        json: () =>
          Promise.resolve({
            success: false,
            error: "Direct transfer not supported",
          }),
      });
      // fallback proxy upload
      const fileData = [
        {
          id: "f-1",
          name: "test.txt",
          size: 5,
          mime_type: "text/plain",
          created_at: "2024-01-01",
        },
      ];
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve({ success: true, data: fileData }),
      });

      const file = new File(["hello"], "test.txt", { type: "text/plain" });
      const result = await filesApi.uploadWithDirectTransfer("share-1", [file]);

      expect(result).toEqual(fileData);
      // Second call should be the proxy upload to /api/v1/shares/share-1/files
      expect(mockFetch.mock.calls[1][0]).toBe("/api/v1/shares/share-1/files");
    });
  });

  describe("delete", () => {
    it("sends DELETE request", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve({ success: true }),
      });

      await filesApi.delete("file-1");
      expect(mockFetch).toHaveBeenCalledWith(
        "/api/v1/files/file-1",
        expect.objectContaining({ method: "DELETE" }),
      );
    });

    it("throws ApiError on failure", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: false,
        status: 404,
        json: () => Promise.resolve({ success: false, error: "Not found" }),
      });

      await expect(filesApi.delete("missing")).rejects.toThrow(ApiError);
    });
  });
});
