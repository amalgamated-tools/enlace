import { describe, it, expect, vi, beforeEach } from "vitest";
import { apiKeysApi } from "../apiKeys";

vi.mock("../client", () => ({
  api: {
    get: vi.fn(),
    post: vi.fn(),
    delete: vi.fn(),
  },
}));

import { api } from "../client";
const mockedApi = vi.mocked(api);

beforeEach(() => {
  vi.clearAllMocks();
});

describe("apiKeysApi", () => {
  it("list calls GET /me/api-keys", async () => {
    const keys = [{ id: "1", name: "My Key" }];
    mockedApi.get.mockResolvedValueOnce(keys);

    const result = await apiKeysApi.list();
    expect(result).toEqual(keys);
    expect(mockedApi.get).toHaveBeenCalledWith("/me/api-keys");
  });

  it("create calls POST /me/api-keys", async () => {
    const input = { name: "New Key", scopes: ["shares:read"] };
    const created = { id: "1", key: "enl_abc123", ...input };
    mockedApi.post.mockResolvedValueOnce(created);

    const result = await apiKeysApi.create(input);
    expect(result).toEqual(created);
    expect(mockedApi.post).toHaveBeenCalledWith("/me/api-keys", input);
  });

  it("revoke calls DELETE /me/api-keys/:id", async () => {
    mockedApi.delete.mockResolvedValueOnce(undefined);

    await apiKeysApi.revoke("1");
    expect(mockedApi.delete).toHaveBeenCalledWith("/me/api-keys/1");
  });
});
