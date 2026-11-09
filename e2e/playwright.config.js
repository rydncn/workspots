// @ts-check
// Every project runs with JavaScript disabled (AC29): the site ships none, and
// a journey that needs it would fail here. Two viewports: a desktop and a
// 320-pixel one (AC28, 400% zoom). The server is built and started by
// global-setup.js against its own database; BASE_URL points the tests at it.
const { defineConfig, devices } = require("@playwright/test");

module.exports = defineConfig({
  testDir: "./tests",
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: 0,
  reporter: process.env.CI ? [["list"], ["github"]] : [["list"]],
  globalSetup: require.resolve("./global-setup.js"),
  globalTeardown: require.resolve("./global-teardown.js"),
  use: {
    baseURL: process.env.BASE_URL || "http://127.0.0.1:8090",
    javaScriptEnabled: false,
    ignoreHTTPSErrors: !!process.env.E2E_IGNORE_HTTPS_ERRORS, // the release rehearsal's throwaway certificate
  },
  projects: [
    { name: "desktop", use: { ...devices["Desktop Chrome"] } },
    { name: "narrow-320", use: { ...devices["Desktop Chrome"], viewport: { width: 320, height: 800 } } },
  ],
});
