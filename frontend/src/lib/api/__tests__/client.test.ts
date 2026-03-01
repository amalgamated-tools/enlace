import { describe, it, expect, vi, beforeEach } from "vitest";
import { api, ApiError } from "../client";

const mockFetch = vi.fn();

beforeEach(() => {
  vi.stubGlobal("fetch", mockFetch);
  mockFetch.mockReset();
  localStorage.clear();
});

function mockResponse(data: unknown, ok = true, status = 200) {
  mockFetch.mockResolvedValueOnce({
    ok,
    status,
    json: () => Promise.resolve(data),
  });
}

describe("api client", () => {
  describe("request headers", () => {
    it("includes Content-Type header", async () => {
      mockResponse({ success: true, data: {} });
      await api.get("/test");

      expect(mockFetch).toHaveBeenCalledWith(
        "/api/v1/test",
        expect.objectContaining({
          headers: expect.objectContaining({
            "Content-Type": "application/json",
          }),
        }),
      );
    });

    it("includes Authorization header when token exists", async () => {
      localStorage.setItem("access_token", "test-token");
      mockResponse({ success: true, data: {} });
      await api.get("/test");

      expect(mockFetch).toHaveBeenCalledWith(
        "/api/v1/test",
        expect.objectContaining({
          headers: expect.objectContaining({
            Authorization: "Bearer test-token",
          }),
        }),
      );
    });

    it("omits Authorization header when no token", async () => {
      mockResponse({ success: true, data: {} });
      await api.get("/test");

      const headers = mockFetch.mock.calls[0][1].headers;
      expect(headers.Authorization).toBeUndefined();
    });
  });

  describe("GET", () => {
    it("returns data on success", async () => {
      mockResponse({ success: true, data: { id: 1, name: "test" } });
      const result = await api.get<{ id: number; name: string }>("/items/1");

      expect(result).toEqual({ id: 1, name: "test" });
      expect(mockFetch).toHaveBeenCalledWith("/api/v1/items/1", {
        headers: { "Content-Type": "application/json" },
      });
    });
  });

  describe("POST", () => {
    it("sends body as JSON", async () => {
      mockResponse({ success: true, data: { id: 1 } });
      await api.post("/items", { name: "new" });

      expect(mockFetch).toHaveBeenCalledWith(
        "/api/v1/items",
        expect.objectContaining({
          method: "POST",
          body: JSON.stringify({ name: "new" }),
        }),
      );
    });
  });

  describe("PATCH", () => {
    it("sends PATCH method", async () => {
      mockResponse({ success: true, data: {} });
      await api.patch("/items/1", { name: "updated" });

      expect(mockFetch).toHaveBeenCalledWith(
        "/api/v1/items/1",
        expect.objectContaining({
          method: "PATCH",
          body: JSON.stringify({ name: "updated" }),
        }),
      );
    });
  });

  describe("PUT", () => {
    it("sends PUT method", async () => {
      mockResponse({ success: true, data: {} });
      await api.put("/items/1", { name: "replaced" });

      expect(mockFetch).toHaveBeenCalledWith(
        "/api/v1/items/1",
        expect.objectContaining({
          method: "PUT",
          body: JSON.stringify({ name: "replaced" }),
        }),
      );
    });
  });

  describe("DELETE", () => {
    it("sends DELETE method", async () => {
      mockResponse({ success: true, data: undefined });
      await api.delete("/items/1");

      expect(mockFetch).toHaveBeenCalledWith(
        "/api/v1/items/1",
        expect.objectContaining({
          method: "DELETE",
        }),
      );
    });
  });

  describe("error handling", () => {
    it("throws ApiError on non-ok response", async () => {
      mockResponse({ success: false, error: "Not found" }, false, 404);

      try {
        await api.get("/missing");
        expect.unreachable();
      } catch (e) {
        expect(e).toBeInstanceOf(ApiError);
        expect((e as ApiError).message).toBe("Not found");
        expect((e as ApiError).status).toBe(404);
      }
    });

    it("throws ApiError when success is false", async () => {
      mockResponse({ success: false, error: "Validation failed" }, true, 200);

      await expect(api.get("/bad")).rejects.toThrow(ApiError);
    });

    it("includes field errors in ApiError", async () => {
      mockResponse(
        {
          success: false,
          error: "Validation failed",
          fields: { email: "invalid" },
        },
        false,
        422,
      );

      try {
        await api.get("/validate");
        expect.unreachable();
      } catch (e) {
        expect(e).toBeInstanceOf(ApiError);
        expect((e as ApiError).fields).toEqual({ email: "invalid" });
      }
    });

    it("uses default message when error is empty", async () => {
      mockResponse({ success: false }, false, 500);

      try {
        await api.get("/fail");
        expect.unreachable();
      } catch (e) {
        expect((e as ApiError).message).toBe("Request failed");
      }
    });
  });
});

describe("ApiError", () => {
  it("has correct name", () => {
    const err = new ApiError("test", 400);
    expect(err.name).toBe("ApiError");
  });

  it("extends Error", () => {
    const err = new ApiError("test", 400);
    expect(err).toBeInstanceOf(Error);
  });

  it("stores status and fields", () => {
    const err = new ApiError("test", 422, { email: "bad" });
    expect(err.status).toBe(422);
    expect(err.fields).toEqual({ email: "bad" });
  });
});
