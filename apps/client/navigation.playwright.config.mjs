import { defineConfig, devices } from "@playwright/test";
import { fileURLToPath } from "node:url";

export default defineConfig({
  testDir: fileURLToPath(new URL(".", import.meta.url)),
  testMatch: "navigation.pw.mjs",
  outputDir: fileURLToPath(new URL("../../output/playwright/impl-001-regression/", import.meta.url)),
  forbidOnly: Boolean(process.env.CI),
  workers: 1,
  retries: 0,
  timeout: 30_000,
  reporter: "list",
  use: {
    baseURL: process.env.NAVIGATION_BASE_URL || "http://localhost:5174",
    trace: "retain-on-failure",
    screenshot: "only-on-failure",
  },
  projects: [
    { name: "desktop", use: { viewport: { width: 1440, height: 900 } } },
    { name: "phone", use: { ...devices["Pixel 7"] } },
    { name: "wide-touch", use: { viewport: { width: 1280, height: 900 }, hasTouch: true } },
  ],
});
