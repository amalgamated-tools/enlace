import { describe, it, expect } from "vitest";
import { getOIDCLoginURL, getOIDCLinkURL } from "../oidc";

describe("oidc", () => {
  it("returns correct OIDC login URL", () => {
    expect(getOIDCLoginURL()).toBe("/api/v1/auth/oidc/login");
  });

  it("returns correct OIDC link URL", () => {
    expect(getOIDCLinkURL()).toBe("/api/v1/me/oidc/link");
  });
});
