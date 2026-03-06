import { test, expect } from "@playwright/test";

const TEST_USER = {
  displayName: "E2E Test User",
  email: `e2e-${Date.now()}@example.com`,
  password: "testpassword123",
};

test.describe("Authentication flow", () => {
  test("register, verify dashboard, sign out, login again", async ({
    page,
  }) => {
    // --- Register ---
    await page.goto("/#/register");

    await page.getByLabel("Display Name").fill(TEST_USER.displayName);
    await page.getByLabel("Email").fill(TEST_USER.email);
    await page.getByLabel(/^Password/).fill(TEST_USER.password);
    await page.getByLabel("Confirm Password").fill(TEST_USER.password);

    await page.getByRole("button", { name: "Create account" }).click();

    // Should land on dashboard
    await expect(page).toHaveURL(/\/#\/$/);
    await expect(page.getByText("Total Shares")).toBeVisible();
    await expect(page.getByText(TEST_USER.displayName)).toBeVisible();

    // --- Sign out ---
    await page.getByRole("button", { name: "Sign out" }).click();

    // Should be on login page
    await expect(page).toHaveURL(/\/#\/login/);

    // --- Login ---
    await page.getByLabel("Email").fill(TEST_USER.email);
    await page.getByLabel("Password").fill(TEST_USER.password);

    await page.getByRole("button", { name: "Sign in" }).click();

    // Should land on dashboard again
    await expect(page).toHaveURL(/\/#\/$/);
    await expect(page.getByText("Total Shares")).toBeVisible();
    await expect(page.getByText(TEST_USER.displayName)).toBeVisible();
  });
});
