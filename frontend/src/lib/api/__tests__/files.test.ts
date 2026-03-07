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
    it("prefers direct upload when supported", async () => {
      mockFetch
        .mockResolvedValueOnce({
          ok: true,
          json: () =>
            Promise.resolve({
              success: true,
              data: {
                upload_id: "upload-1",
                finalize_token: "token-1",
                url: "https://storage.example/upload",
                method: "PUT",
                headers: { "Content-Type": "text/plain" },
              },
            }),
        })
        .mockResolvedValueOnce({
          ok: true,
        })
        .mockResolvedValueOnce({
          ok: true,
          json: () =>
            Promise.resolve({
              success: true,
              data: {
                id: "f1",
                name: "test.txt",
                size: 5,
                mime_type: "text/plain",
                created_at: "2024-01-01",
              },
            }),
        });

      const result = await filesApi.upload("share-1", [
        new File(["hello"], "test.txt", { type: "text/plain" }),
      ]);

      expect(result).toHaveLength(1);
      expect(mockFetch).toHaveBeenNthCalledWith(
        1,
        "/api/v1/shares/share-1/files/initiate",
        expect.objectContaining({ method: "POST" }),
      );
      expect(mockFetch).toHaveBeenNthCalledWith(
        2,
        "https://storage.example/upload",
        expect.objectContaining({ method: "PUT" }),
      );
      expect(mockFetch).toHaveBeenNthCalledWith(
        3,
        "/api/v1/files/uploads/upload-1/finalize",
        expect.objectContaining({ method: "POST" }),
      );
    });

    it("falls back to multipart upload on 409", async () => {
      const fileData = [
        {
          id: "f1",
          name: "test.txt",
          size: 100,
          mime_type: "text/plain",
          created_at: "2024-01-01",
        },
      ];
      mockFetch
        .mockResolvedValueOnce({
          ok: false,
          status: 409,
          json: () =>
            Promise.resolve({
              success: false,
              error: "direct transfer unavailable",
            }),
        })
        .mockResolvedValueOnce({
          ok: true,
          json: () => Promise.resolve({ success: true, data: fileData }),
        });

      const result = await filesApi.upload("share-1", [
        new File(["hello"], "test.txt", { type: "text/plain" }),
      ]);

      expect(result).toEqual(fileData);
      expect(mockFetch).toHaveBeenNthCalledWith(
        2,
        "/api/v1/shares/share-1/files",
        expect.objectContaining({ method: "POST" }),
      );
    });

    it("sends multipart FormData when using the legacy path", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve({ success: true, data: [] }),
      });

      await filesApi.upload("share-1", []);

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
