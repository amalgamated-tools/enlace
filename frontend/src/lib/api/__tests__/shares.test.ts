import { describe, it, expect, vi, beforeEach } from "vitest";
import { sharesApi } from "../shares";

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

describe("sharesApi", () => {
  it("list calls GET /shares", async () => {
    const shares = [{ id: "1", name: "Share 1" }];
    mockedApi.get.mockResolvedValueOnce(shares);

    const result = await sharesApi.list();
    expect(result).toEqual(shares);
    expect(mockedApi.get).toHaveBeenCalledWith("/shares");
  });

  it("get calls GET /shares/:id", async () => {
    const share = { id: "1", name: "Share 1" };
    mockedApi.get.mockResolvedValueOnce(share);

    const result = await sharesApi.get("1");
    expect(result).toEqual(share);
    expect(mockedApi.get).toHaveBeenCalledWith("/shares/1");
  });

  it("create calls POST /shares", async () => {
    const input = { name: "New Share" };
    const created = { id: "1", ...input };
    mockedApi.post.mockResolvedValueOnce(created);

    const result = await sharesApi.create(input);
    expect(result).toEqual(created);
    expect(mockedApi.post).toHaveBeenCalledWith("/shares", input);
  });

  it("update calls PATCH /shares/:id", async () => {
    const input = { name: "Updated" };
    const updated = { id: "1", ...input };
    mockedApi.patch.mockResolvedValueOnce(updated);

    const result = await sharesApi.update("1", input);
    expect(result).toEqual(updated);
    expect(mockedApi.patch).toHaveBeenCalledWith("/shares/1", input);
  });

  it("delete calls DELETE /shares/:id", async () => {
    mockedApi.delete.mockResolvedValueOnce(undefined);

    await sharesApi.delete("1");
    expect(mockedApi.delete).toHaveBeenCalledWith("/shares/1");
  });
});
