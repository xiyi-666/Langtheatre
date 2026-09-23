import { defineConfig } from "@playwright/test";
import { createRequire } from "node:module";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const require = createRequire(import.meta.url);
const clientDir = fileURLToPath(new URL("../", import.meta.url));
const viteCLI = join(dirname(require.resolve("vite/package.json")), "bin", "vite.js");
const baseURL = "http://127.0.0.1:5187";

// 本地健康检查必须直连，避免系统 HTTP 代理误报测试端口已被占用。
const noProxy = [process.env.NO_PROXY, process.env.no_proxy, "127.0.0.1", "localhost"]
  .filter(Boolean)
  .join(",");
process.env.NO_PROXY = noProxy;
process.env.no_proxy = noProxy;

export default defineConfig({
  testDir: fileURLToPath(new URL(".", import.meta.url)),
  testMatch: "mock-exam-flow.pw.mjs",
  outputDir: join(clientDir, "test-results", "mock-exam-flow"),
  forbidOnly: Boolean(process.env.CI),
  fullyParallel: false,
  workers: 1,
  retries: 0,
  timeout: 60_000,
  expect: { timeout: 10_000 },
  reporter: [["list", { printSteps: true }]],
  use: {
    browserName: "chromium",
    baseURL,
    viewport: { width: 1440, height: 1000 },
    actionTimeout: 10_000,
    navigationTimeout: 15_000,
    trace: "retain-on-failure",
    screenshot: "only-on-failure",
  },
  webServer: {
    command: `"${process.execPath}" "${viteCLI}" --host 127.0.0.1 --port 5187 --strictPort`,
    cwd: clientDir,
    url: baseURL,
    env: { VITE_API_URL: "", VITE_APP_EDITION: "COMMERCIAL" },
    timeout: 60_000,
    reuseExistingServer: false,
  },
});
