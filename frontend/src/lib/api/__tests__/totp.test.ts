import { describe, it, expect, vi, beforeEach } from "vitest";

import { totpApi } from "../totp";

vi.mock("../client", () => ({
  api: {
    get: vi.fn(),
    post: vi.fn(),
  },
}));

import { api } from "../client";
const mockedApi = vi.mocked(api);

beforeEach(() => {
  vi.clearAllMocks();
});

describe("totpApi", () => {
  it("getStatus calls GET /me/2fa/status", async () => {
    const status = { enabled: true, require_2fa: false };
    mockedApi.get.mockResolvedValueOnce(status);

    const result = await totpApi.getStatus();

    expect(result).toEqual(status);
    expect(mockedApi.get).toHaveBeenCalledWith("/me/2fa/status");
  });

  it("beginSetup posts to /me/2fa/setup", async () => {
    const setup = { secret: "abc", qr_code: "data", provisioning_uri: "uri" };
    mockedApi.post.mockResolvedValueOnce(setup);

    const result = await totpApi.beginSetup();

    expect(result).toEqual(setup);
    expect(mockedApi.post).toHaveBeenCalledWith(
      "/me/2fa/setup",
      {},
      { overrideToken: undefined },
    );
  });

  it("beginSetup supports a pending 2FA token", async () => {
    mockedApi.post.mockResolvedValueOnce({
      secret: "abc",
      qr_code: "data",
      provisioning_uri: "uri",
    });

    await totpApi.beginSetup("pending-setup-token");

    expect(mockedApi.post).toHaveBeenCalledWith(
      "/me/2fa/setup",
      {},
      {
        overrideToken: "pending-setup-token",
      },
    );
  });

  it("confirmSetup posts the verification code", async () => {
    const confirm = { recovery_codes: ["1", "2"] };
    mockedApi.post.mockResolvedValueOnce(confirm);

    const result = await totpApi.confirmSetup("123456");

    expect(result).toEqual(confirm);
    expect(mockedApi.post).toHaveBeenCalledWith(
      "/me/2fa/confirm",
      {
        code: "123456",
      },
      { overrideToken: undefined },
    );
  });

  it("disable posts the password", async () => {
    mockedApi.post.mockResolvedValueOnce(undefined);

    await totpApi.disable("supersecret");

    expect(mockedApi.post).toHaveBeenCalledWith("/me/2fa/disable", {
      password: "supersecret",
    });
  });

  it("regenerateRecoveryCodes posts to recovery endpoint", async () => {
    const recovery = { recovery_codes: ["a", "b", "c"] };
    mockedApi.post.mockResolvedValueOnce(recovery);

    const result = await totpApi.regenerateRecoveryCodes("pw");

    expect(result).toEqual(recovery);
    expect(mockedApi.post).toHaveBeenCalledWith("/me/2fa/recovery-codes", {
      password: "pw",
    });
  });

  it("verify posts pending token and code", async () => {
    const login = { token: "jwt" };
    mockedApi.post.mockResolvedValueOnce(login);

    const result = await totpApi.verify("pending123", "654321");

    expect(result).toEqual(login);
    expect(mockedApi.post).toHaveBeenCalledWith("/auth/2fa/verify", {
      pending_token: "pending123",
      code: "654321",
    });
  });

  it("recovery posts pending token and recovery code", async () => {
    const login = { token: "jwt2" };
    mockedApi.post.mockResolvedValueOnce(login);

    const result = await totpApi.recovery("pending456", "recovery-1");

    expect(result).toEqual(login);
    expect(mockedApi.post).toHaveBeenCalledWith("/auth/2fa/recovery", {
      pending_token: "pending456",
      recovery_code: "recovery-1",
    });
  });
});
