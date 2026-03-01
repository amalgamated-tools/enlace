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
