import { defineConfig, devices } from "@playwright/test";

// End-to-end tests run the real UI in a browser against the deterministic
// in-memory backend (VITE_MOCK_NATIVE=1); no Go process is involved.
export default defineConfig({
  testDir: "./e2e",
  timeout: 30_000,
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 1 : 0,
  reporter: process.env.CI ? [["list"], ["html", { open: "never" }]] : "list",
  use: { baseURL: "http://127.0.0.1:5199", trace: "retain-on-failure", viewport: { width: 1400, height: 900 } },
  projects: [{ name: "chromium", use: { ...devices["Desktop Chrome"], viewport: { width: 1400, height: 900 } } }],
  webServer: {
    command: "npx vite --host 127.0.0.1 --port 5199 --strictPort",
    url: "http://127.0.0.1:5199",
    env: { VITE_MOCK_NATIVE: "1", WAILS_VITE_PORT: "5199" },
    reuseExistingServer: !process.env.CI,
    timeout: 60_000,
  },
});