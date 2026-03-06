import { defineConfig } from "@playwright/test";
import { join } from "path";
import { mkdtempSync } from "fs";
import { tmpdir } from "os";

const testTmpDir = mkdtempSync(join(tmpdir(), "enlace-e2e-"));
const testPort = 3847;

export default defineConfig({
  testDir: "./tests",
  fullyParallel: false,
  workers: 1,
  retries: 1,
  timeout: 30_000,
  expect: { timeout: 10_000 },

  use: {
    baseURL: `http://localhost:${testPort}`,
    screenshot: "only-on-failure",
    trace: "on-first-retry",
  },

  projects: [
    {
      name: "chromium",
      use: { browserName: "chromium" },
    },
  ],

  webServer: {
    command: `../enlace`,
    url: `http://localhost:${testPort}/health`,
    reuseExistingServer: !process.env.CI,
    timeout: 15_000,
    env: {
      PORT: String(testPort),
      DATABASE_PATH: join(testTmpDir, "e2e.db"),
      JWT_SECRET: "e2e-test-jwt-secret",
      STORAGE_TYPE: "local",
      STORAGE_LOCAL_PATH: join(testTmpDir, "uploads"),
    },
  },
});
