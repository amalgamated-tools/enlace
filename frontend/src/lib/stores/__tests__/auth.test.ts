import { describe, it, expect, vi, beforeEach } from "vitest";
import { get } from "svelte/store";
import { auth, isAuthenticated, isAdmin } from "../auth";

const mockFetch = vi.fn();

beforeEach(() => {
  vi.stubGlobal("fetch", mockFetch);
  mockFetch.mockReset();
  localStorage.clear();
});

vi.mock("../../api", () => ({
  authApi: {
    login: vi.fn(),
    register: vi.fn(),
    refresh: vi.fn(),
    logout: vi.fn(),
  },
}));

import { authApi } from "../../api";
const mockedAuthApi = vi.mocked(authApi);

describe("auth store", () => {
  describe("init", () => {
    it("sets initialized=true with no user when no token", async () => {
      await auth.init();
      const state = get(auth);
      expect(state.initialized).toBe(true);
      expect(state.user).toBeNull();
      expect(state.loading).toBe(false);
    });

    it("fetches user when token exists", async () => {
      localStorage.setItem("access_token", "valid-token");
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: () =>
          Promise.resolve({
            data: {
              id: "1",
              email: "test@test.com",
              display_name: "Test",
              is_admin: false,
            },
          }),
      });

      await auth.init();
      const state = get(auth);
      expect(state.user).toEqual({
        id: "1",
        email: "test@test.com",
        display_name: "Test",
        is_admin: false,
      });
      expect(state.initialized).toBe(true);
    });

    it("clears tokens on invalid token with no refresh token", async () => {
      localStorage.setItem("access_token", "bad-token");
      mockFetch.mockResolvedValueOnce({ ok: false, status: 401 });

      await auth.init();
      expect(localStorage.getItem("access_token")).toBeNull();
      expect(get(auth).user).toBeNull();
    });

    it("tries refresh token when access token is invalid", async () => {
      localStorage.setItem("access_token", "bad-token");
      localStorage.setItem("refresh_token", "good-refresh");

      mockFetch.mockResolvedValueOnce({ ok: false, status: 401 });

      mockedAuthApi.refresh.mockResolvedValueOnce({
        access_token: "new-access",
        refresh_token: "new-refresh",
      });

      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: () =>
          Promise.resolve({
            data: {
              id: "1",
              email: "test@test.com",
              display_name: "Test",
              is_admin: false,
            },
          }),
      });

      await auth.init();
      expect(localStorage.getItem("access_token")).toBe("new-access");
      expect(localStorage.getItem("refresh_token")).toBe("new-refresh");
      expect(get(auth).user).not.toBeNull();
    });

    it("clears tokens when refresh fails", async () => {
      localStorage.setItem("access_token", "bad-token");
      localStorage.setItem("refresh_token", "bad-refresh");

      mockFetch.mockResolvedValueOnce({ ok: false, status: 401 });
      mockedAuthApi.refresh.mockRejectedValueOnce(new Error("refresh failed"));

      await auth.init();
      expect(localStorage.getItem("access_token")).toBeNull();
      expect(localStorage.getItem("refresh_token")).toBeNull();
      expect(get(auth).user).toBeNull();
    });

    it("handles network error gracefully", async () => {
      localStorage.setItem("access_token", "some-token");
      mockFetch.mockRejectedValueOnce(new Error("network error"));

      await auth.init();
      expect(get(auth).user).toBeNull();
      expect(get(auth).initialized).toBe(true);
    });
  });

  describe("login", () => {
    it("stores tokens and sets user on success", async () => {
      const mockUser = {
        id: "1",
        email: "test@test.com",
        display_name: "Test",
        is_admin: false,
      };
      mockedAuthApi.login.mockResolvedValueOnce({
        access_token: "access-123",
        refresh_token: "refresh-123",
        user: mockUser,
      });

      const result = await auth.login("test@test.com", "password");
      expect(result.success).toBe(true);
      expect(result.user).toEqual(mockUser);
      expect(localStorage.getItem("access_token")).toBe("access-123");
      expect(localStorage.getItem("refresh_token")).toBe("refresh-123");
      expect(get(auth).user).toEqual(mockUser);
    });

    it("returns pendingToken and does not store tokens when 2FA is required", async () => {
      mockedAuthApi.logout.mockResolvedValueOnce(undefined);
      await auth.logout();

      mockedAuthApi.login.mockResolvedValueOnce({
        requires_2fa: true,
        pending_token: "pending-abc",
      });

      const result = await auth.login("test@test.com", "password");
      expect(result.success).toBe(false);
      expect(result.requires2FA).toBe(true);
      expect(result.pendingToken).toBe("pending-abc");
      expect(localStorage.getItem("access_token")).toBeNull();
      expect(localStorage.getItem("refresh_token")).toBeNull();
      expect(get(auth).user).toBeNull();
      expect(get(auth).loading).toBe(false);
    });

    it("returns pending token and does not store tokens when user needs 2FA setup", async () => {
      const mockUser = {
        id: "1",
        email: "test@test.com",
        display_name: "Test",
        is_admin: false,
      };
      mockedAuthApi.login.mockResolvedValueOnce({
        user: mockUser,
        requires_2fa_setup: true,
        pending_token: "pending-setup-123",
      });

      const result = await auth.login("test@test.com", "password");
      expect(result.success).toBe(false);
      expect(result.requires2FASetup).toBe(true);
      expect(result.pendingToken).toBe("pending-setup-123");
      expect(result.user).toEqual(mockUser);
      expect(localStorage.getItem("access_token")).toBeNull();
      expect(localStorage.getItem("refresh_token")).toBeNull();
      expect(get(auth).user).toBeNull();
    });

    it("throws on failed login", async () => {
      mockedAuthApi.login.mockRejectedValueOnce(
        new Error("Invalid credentials"),
      );

      await expect(auth.login("bad@test.com", "wrong")).rejects.toThrow(
        "Invalid credentials",
      );
      expect(get(auth).loading).toBe(false);
    });

    it("throws when access_token is present but refresh_token is missing", async () => {
      mockedAuthApi.login.mockResolvedValueOnce({
        access_token: "access-123",
        user: {
          id: "1",
          email: "test@test.com",
          display_name: "Test",
          is_admin: false,
        },
      });

      await expect(auth.login("test@test.com", "password")).rejects.toThrow(
        "no refresh token received",
      );
      expect(localStorage.getItem("access_token")).toBeNull();
      expect(localStorage.getItem("refresh_token")).toBeNull();
      expect(get(auth).loading).toBe(false);
    });
  });

  describe("logout", () => {
    it("clears user and tokens", async () => {
      localStorage.setItem("access_token", "token");
      localStorage.setItem("refresh_token", "refresh");
      mockedAuthApi.logout.mockResolvedValueOnce(undefined);

      await auth.logout();
      expect(get(auth).user).toBeNull();
      expect(localStorage.getItem("access_token")).toBeNull();
      expect(localStorage.getItem("refresh_token")).toBeNull();
    });

    it("clears state even if API logout fails", async () => {
      localStorage.setItem("access_token", "token");
      mockedAuthApi.logout.mockRejectedValueOnce(new Error("fail"));

      await auth.logout();
      expect(get(auth).user).toBeNull();
      expect(localStorage.getItem("access_token")).toBeNull();
    });
  });

  describe("setUser", () => {
    it("updates the user in state", () => {
      const user = {
        id: "1",
        email: "a@b.com",
        display_name: "A",
        is_admin: true,
      };
      auth.setUser(user);
      expect(get(auth).user).toEqual(user);
    });
  });

  describe("derived stores", () => {
    it("isAuthenticated is true when user exists", async () => {
      mockedAuthApi.login.mockResolvedValueOnce({
        access_token: "t",
        refresh_token: "r",
        user: {
          id: "1",
          email: "a@b.com",
          display_name: "A",
          is_admin: false,
        },
      });
      await auth.login("a@b.com", "p");
      expect(get(isAuthenticated)).toBe(true);
    });

    it("isAuthenticated is false when no user", async () => {
      await auth.logout();
      expect(get(isAuthenticated)).toBe(false);
    });

    it("isAdmin is true when user is admin", () => {
      auth.setUser({
        id: "1",
        email: "a@b.com",
        display_name: "Admin",
        is_admin: true,
      });
      expect(get(isAdmin)).toBe(true);
    });

    it("isAdmin is false when user is not admin", () => {
      auth.setUser({
        id: "1",
        email: "a@b.com",
        display_name: "User",
        is_admin: false,
      });
      expect(get(isAdmin)).toBe(false);
    });
  });
});
