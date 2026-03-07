import { describe, it, expect, vi, beforeEach } from "vitest";
import { fileRestrictionsApi } from "../fileRestrictions";

vi.mock("../client", () => ({
  api: {
    get: vi.fn(),
    put: vi.fn(),
    delete: vi.fn(),
  },
}));

import { api } from "../client";
const mockedApi = vi.mocked(api);

beforeEach(() => {
  vi.clearAllMocks();
});

describe("fileRestrictionsApi", () => {
  it("get calls GET /admin/files", async () => {
    const config = { max_file_size: 104857600, blocked_extensions: [".exe"] };
    mockedApi.get.mockResolvedValueOnce(config);

    const result = await fileRestrictionsApi.get();
    expect(result).toEqual(config);
    expect(mockedApi.get).toHaveBeenCalledWith("/admin/files");
  });

  it("update calls PUT /admin/files with payload", async () => {
    const input = { max_file_size: 52428800, blocked_extensions: ".exe, .bat" };
    const updated = {
      max_file_size: 52428800,
      blocked_extensions: [".exe", ".bat"],
    };
    mockedApi.put.mockResolvedValueOnce(updated);

    const result = await fileRestrictionsApi.update(input);
    expect(result).toEqual(updated);
    expect(mockedApi.put).toHaveBeenCalledWith("/admin/files", input);
  });

  it("reset calls DELETE /admin/files", async () => {
    mockedApi.delete.mockResolvedValueOnce(undefined);

    await fileRestrictionsApi.reset();
    expect(mockedApi.delete).toHaveBeenCalledWith("/admin/files");
  });
});
