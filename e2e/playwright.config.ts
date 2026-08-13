import { defineConfig, devices } from "@playwright/test";

/**
 * Playwright configuration for the LCH-02 browser suite.
 *
 * There is no `webServer` block on purpose. These specs run against a
 * deployment — a Vercel preview or staging — not against a dev server
 * started by the runner. A local `next dev` differs from what ships in
 * exactly the ways this suite exists to check: build output, real origins,
 * real CORS, real cold starts.
 */

const webURL = process.env.E2E_WEB_URL ?? "http://localhost:3000";

export default defineConfig({
  testDir: "./specs",
  // The journey specs are stateful — one client, one booking, carried
  // through — so they must not race each other for the same slot.
  fullyParallel: false,
  workers: 1,
  forbidOnly: !!process.env.CI,
  // One retry, not three. A test that only passes on the third attempt is
  // telling you something, and burying it is how a flaky booking race
  // becomes a production incident.
  retries: process.env.CI ? 1 : 0,
  reporter: process.env.CI
    ? [["list"], ["html", { open: "never" }], ["github"]]
    : [["list"], ["html", { open: "never" }]],

  timeout: 60_000,
  expect: {
    // Slot lists and payment verification are round trips to a cold
    // serverless function; the default 5s is optimistic for both.
    timeout: 15_000,
  },

  use: {
    baseURL: webURL,
    trace: "retain-on-failure",
    video: "retain-on-failure",
    screenshot: "only-on-failure",
    // A real client's timezone, not the runner's. Every slot the booking
    // page shows is computed in the practice's zone and rendered in the
    // visitor's, and a suite running in UTC would never exercise that.
    timezoneId: "Africa/Accra",
    locale: "en-GB",
  },

  projects: [
    {
      name: "chromium",
      use: {
        ...devices["Desktop Chrome"],
        launchOptions: {
          args: [
            // The video spec needs two peers with media and no consent
            // dialog. These flags are Chromium-only, which is why the
            // video project below pins to it.
            "--use-fake-ui-for-media-stream",
            "--use-fake-device-for-media-stream",
          ],
        },
      },
    },
    {
      name: "firefox",
      use: { ...devices["Desktop Firefox"] },
      // The public site and the booking flow, on a second engine. The
      // portal and dashboard specs are Chromium-only: a second engine
      // there costs run time without testing anything the first didn't.
      testMatch: /(marketing|booking)\.spec\.ts/,
    },
    {
      name: "mobile",
      use: { ...devices["Pixel 7"] },
      // Most of this practice's clients arrive on a phone. The booking
      // flow is the one that has to work there.
      testMatch: /(marketing|booking)\.spec\.ts/,
    },
  ],
});
