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
      expect(mockFetch.mock.calls[2][1].body).toBe(
        JSON.stringify({ token: "token-1" }),
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

    it("returns early when there are no files", async () => {
      await expect(filesApi.upload("share-1", [])).resolves.toEqual([]);
      expect(mockFetch).not.toHaveBeenCalled();
    });

    it("sends multipart FormData when falling back to the legacy path", async () => {
      mockFetch
        .mockResolvedValueOnce({
          ok: false,
          status: 409,
          json: () => Promise.resolve({ success: false, error: "unsupported" }),
        })
        .mockResolvedValueOnce({
          ok: true,
          json: () => Promise.resolve({ success: true, data: [] }),
        });

      await filesApi.upload("share-1", [new File(["hello"], "test.txt")]);

      expect(mockFetch).toHaveBeenCalledWith(
        "/api/v1/shares/share-1/files",
        expect.objectContaining({ method: "POST" }),
      );
      const callArgs = mockFetch.mock.calls[1][1];
      expect(callArgs.body).toBeInstanceOf(FormData);
    });

    it("includes auth header when token exists", async () => {
      localStorage.setItem("access_token", "my-token");
      mockFetch
        .mockResolvedValueOnce({
          ok: false,
          status: 409,
          json: () => Promise.resolve({ success: false, error: "unsupported" }),
        })
        .mockResolvedValueOnce({
          ok: true,
          json: () => Promise.resolve({ success: true, data: [] }),
        });

      await filesApi.upload("share-1", [new File(["hello"], "test.txt")]);
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

    it("uploads multiple files via direct transfer without duplicates", async () => {
      // File A: initiate → PUT → finalize
      mockFetch
        .mockResolvedValueOnce({
          ok: true,
          json: () =>
            Promise.resolve({
              success: true,
              data: {
                upload_id: "upload-a",
                finalize_token: "token-a",
                url: "https://storage.example/a",
                method: "PUT",
              },
            }),
        })
        .mockResolvedValueOnce({ ok: true })
        .mockResolvedValueOnce({
          ok: true,
          json: () =>
            Promise.resolve({
              success: true,
              data: {
                id: "fa",
                name: "a.txt",
                size: 1,
                mime_type: "text/plain",
                created_at: "2024-01-01",
              },
            }),
        })
        // File B: initiate → PUT → finalize
        .mockResolvedValueOnce({
          ok: true,
          json: () =>
            Promise.resolve({
              success: true,
              data: {
                upload_id: "upload-b",
                finalize_token: "token-b",
                url: "https://storage.example/b",
                method: "PUT",
              },
            }),
        })
        .mockResolvedValueOnce({ ok: true })
        .mockResolvedValueOnce({
          ok: true,
          json: () =>
            Promise.resolve({
              success: true,
              data: {
                id: "fb",
                name: "b.txt",
                size: 1,
                mime_type: "text/plain",
                created_at: "2024-01-01",
              },
            }),
        });

      const result = await filesApi.upload("share-1", [
        new File(["a"], "a.txt"),
        new File(["b"], "b.txt"),
      ]);

      expect(result).toHaveLength(2);
      expect(result[0].id).toBe("fa");
      expect(result[1].id).toBe("fb");
      // 6 fetches total: 2x (initiate + PUT + finalize), no multipart fallback
      expect(mockFetch).toHaveBeenCalledTimes(6);
      // No call to the multipart endpoint
      const urls = mockFetch.mock.calls.map((c: unknown[]) => c[0]);
      expect(urls).not.toContain("/api/v1/shares/share-1/files");
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
