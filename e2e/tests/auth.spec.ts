import { test, expect } from "@playwright/test";

test.describe("Authentication flow", () => {
  test("register, verify dashboard, sign out, login again", async ({
    page,
  }) => {
    const testUser = {
      displayName: "E2E Test User",
      email: `e2e-${Date.now()}-${Math.random().toString(36).substring(2, 8)}@example.com`,
      password: "testpassword123",
    };

    // --- Register ---
    await page.goto("/#/register");

    await page.getByLabel("Display Name").fill(testUser.displayName);
    await page.getByLabel("Email").fill(testUser.email);
    await page.getByLabel(/^Password/).fill(testUser.password);
    await page.getByLabel("Confirm Password").fill(testUser.password);

    await page.getByRole("button", { name: "Create account" }).click();

    // Should land on dashboard
    await expect(page).toHaveURL(/\/#\/$/);
    await expect(page.getByText("Total Shares")).toBeVisible();
    await expect(page.getByText(testUser.displayName)).toBeVisible();

    // --- Sign out ---
    await page.getByRole("button", { name: "Sign out" }).click();

    // Should be on login page
    await expect(page).toHaveURL(/\/#\/login/);

    // --- Login ---
    await page.getByLabel("Email").fill(testUser.email);
    await page.getByLabel("Password").fill(testUser.password);

    await page.getByRole("button", { name: "Sign in" }).click();

    // Should land on dashboard again
    await expect(page).toHaveURL(/\/#\/$/);
    await expect(page.getByText("Total Shares")).toBeVisible();
    await expect(page.getByText(testUser.displayName)).toBeVisible();
  });
});
