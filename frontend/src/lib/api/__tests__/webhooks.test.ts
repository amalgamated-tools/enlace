import { describe, it, expect, vi, beforeEach } from "vitest";
import { webhooksApi } from "../webhooks";

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

describe("webhooksApi", () => {
  it("list calls GET /admin/webhooks", async () => {
    const webhooks = [{ id: "1", name: "My Hook" }];
    mockedApi.get.mockResolvedValueOnce(webhooks);

    const result = await webhooksApi.list();
    expect(result).toEqual(webhooks);
    expect(mockedApi.get).toHaveBeenCalledWith("/admin/webhooks");
  });

  it("create calls POST /admin/webhooks", async () => {
    const input = {
      name: "Hook",
      url: "https://example.com/hook",
      events: ["share.created"],
    };
    const created = { id: "1", secret: "s3cret", ...input };
    mockedApi.post.mockResolvedValueOnce(created);

    const result = await webhooksApi.create(input);
    expect(result).toEqual(created);
    expect(mockedApi.post).toHaveBeenCalledWith("/admin/webhooks", input);
  });

  it("update calls PATCH /admin/webhooks/:id", async () => {
    const input = { name: "Updated" };
    const updated = { id: "1", ...input };
    mockedApi.patch.mockResolvedValueOnce(updated);

    const result = await webhooksApi.update("1", input);
    expect(result).toEqual(updated);
    expect(mockedApi.patch).toHaveBeenCalledWith("/admin/webhooks/1", input);
  });

  it("delete calls DELETE /admin/webhooks/:id", async () => {
    mockedApi.delete.mockResolvedValueOnce(undefined);

    await webhooksApi.delete("1");
    expect(mockedApi.delete).toHaveBeenCalledWith("/admin/webhooks/1");
  });

  it("listDeliveries calls GET /admin/webhooks/deliveries without params", async () => {
    const deliveries = [{ id: "d1" }];
    mockedApi.get.mockResolvedValueOnce(deliveries);

    const result = await webhooksApi.listDeliveries();
    expect(result).toEqual(deliveries);
    expect(mockedApi.get).toHaveBeenCalledWith("/admin/webhooks/deliveries");
  });

  it("listDeliveries builds query string from params", async () => {
    const deliveries = [{ id: "d1" }];
    mockedApi.get.mockResolvedValueOnce(deliveries);

    const result = await webhooksApi.listDeliveries({
      subscription_id: "sub1",
      status: "failed",
      event_type: "share.created",
      limit: 10,
    });
    expect(result).toEqual(deliveries);

    const calledUrl = mockedApi.get.mock.calls[0][0];
    expect(calledUrl).toContain("/admin/webhooks/deliveries?");
    expect(calledUrl).toContain("subscription_id=sub1");
    expect(calledUrl).toContain("status=failed");
    expect(calledUrl).toContain("event_type=share.created");
    expect(calledUrl).toContain("limit=10");
  });

  it("listDeliveries omits empty params", async () => {
    mockedApi.get.mockResolvedValueOnce([]);

    await webhooksApi.listDeliveries({ subscription_id: "sub1" });

    const calledUrl = mockedApi.get.mock.calls[0][0];
    expect(calledUrl).toContain("subscription_id=sub1");
    expect(calledUrl).not.toContain("status=");
    expect(calledUrl).not.toContain("event_type=");
    expect(calledUrl).not.toContain("limit=");
  });
});
