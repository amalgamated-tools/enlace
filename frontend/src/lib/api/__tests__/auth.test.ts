import { describe, it, expect, vi, beforeEach } from "vitest";
import { authApi } from "../auth";

vi.mock("../client", () => ({
  api: {
    get: vi.fn(),
    post: vi.fn(),
    patch: vi.fn(),
    delete: vi.fn(),
  },
}));

import { api } from "../client";
const mockedApi = vi.mocked(api);

beforeEach(() => {
  vi.clearAllMocks();
});

describe("authApi", () => {
  it("register calls POST /auth/register with correct payload", async () => {
    const response = {
      user: {
        id: "1",
        email: "test@example.com",
        display_name: "Test",
        is_admin: false,
      },
    };
    mockedApi.post.mockResolvedValueOnce(response);

    const result = await authApi.register(
      "test@example.com",
      "password123",
      "Test",
    );
    expect(result).toEqual(response);
    expect(mockedApi.post).toHaveBeenCalledWith("/auth/register", {
      email: "test@example.com",
      password: "password123",
      display_name: "Test",
    });
  });

  it("login calls POST /auth/login with correct payload", async () => {
    const response = {
      access_token: "access-token",
      refresh_token: "refresh-token",
      user: {
        id: "1",
        email: "test@example.com",
        display_name: "Test",
        is_admin: false,
      },
    };
    mockedApi.post.mockResolvedValueOnce(response);

    const result = await authApi.login("test@example.com", "password123");
    expect(result).toEqual(response);
    expect(mockedApi.post).toHaveBeenCalledWith("/auth/login", {
      email: "test@example.com",
      password: "password123",
    });
  });

  it("refresh calls POST /auth/refresh with refresh token", async () => {
    const response = {
      access_token: "new-access-token",
      refresh_token: "new-refresh-token",
    };
    mockedApi.post.mockResolvedValueOnce(response);

    const result = await authApi.refresh("old-refresh-token");
    expect(result).toEqual(response);
    expect(mockedApi.post).toHaveBeenCalledWith("/auth/refresh", {
      refresh_token: "old-refresh-token",
    });
  });

  it("logout calls POST /auth/logout", async () => {
    mockedApi.post.mockResolvedValueOnce(undefined);

    await authApi.logout();
    expect(mockedApi.post).toHaveBeenCalledWith("/auth/logout", {});
  });
});
