import { expect, test as base, type Page } from "@playwright/test";

/**
 * Shared fixtures for the LCH-02 browser suite.
 *
 * Everything here exists so a spec reads as the story it is testing rather
 * than as a sequence of selectors. Where a helper hides something, it hides
 * setup — never an assertion.
 */

export const env = {
  web: required("E2E_WEB_URL"),
  admin: required("E2E_ADMIN_URL"),
  api: required("E2E_API_URL"),
  practitionerEmail: required("E2E_PRACTITIONER_EMAIL"),
  practitionerPassword: required("E2E_PRACTITIONER_PASSWORD"),
  seedToken: process.env.E2E_SEED_TOKEN ?? "",
  expectTurn: process.env.E2E_EXPECT_TURN === "1",
};

function required(name: string): string {
  const value = process.env[name];
  if (!value) {
    throw new Error(
      `${name} is not set. Copy e2e/.env.example to e2e/.env and fill it in — ` +
        `these specs run against a deployment, not a dev server.`,
    );
  }
  return value;
}

/**
 * Refuses to run against production.
 *
 * This suite creates clients, bookings and payments. Pointing it at the
 * live site would put test data in front of real clients and, with a live
 * Paystack key, move real money. A guard that can be turned off is not a
 * guard, so there is no override.
 */
export function guardAgainstProduction(): void {
  for (const url of [env.web, env.admin, env.api]) {
    const { hostname } = new URL(url);
    if (hostname === "terioswellness.com" || hostname.endsWith(".terioswellness.com")) {
      throw new Error(
        `Refusing to run against ${hostname}. This suite writes data and takes payments; ` +
          `point it at a preview deployment.`,
      );
    }
  }
}

guardAgainstProduction();

/** A client account created for one spec and thrown away after it. */
export interface TestClient {
  email: string;
  password: string;
  name: string;
}

/** A fresh identity per spec, so specs never collide over one account. */
export function newClient(label: string): TestClient {
  const stamp = `${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 7)}`;
  return {
    email: `e2e-${label}-${stamp}@terios-test.invalid`,
    password: "correct horse battery staple",
    name: `E2E ${label}`,
  };
}

export const test = base.extend<{
  /** A signed-in client on the customer site. */
  clientPage: { page: Page; client: TestClient };
  /** A signed-in practitioner on the dashboard. */
  adminPage: Page;
}>({
  clientPage: async ({ browser }, use, testInfo) => {
    const context = await browser.newContext();
    const page = await context.newPage();
    const client = newClient(testInfo.title.slice(0, 12).replace(/\W+/g, ""));

    await page.goto(`${env.web}/register`);
    await page.getByRole("textbox", { name: /^name/i }).fill(client.name);
    await page.getByRole("textbox", { name: /^email/i }).fill(client.email);
    await page.getByRole("textbox", { name: /^password/i }).fill(client.password);
    await page.getByRole("button", { name: /create account|register|sign up/i }).click();
    await expect(page).toHaveURL(/\/portal/);

    await use({ page, client });
    await context.close();
  },

  adminPage: async ({ browser }, use) => {
    const context = await browser.newContext();
    const page = await context.newPage();

    await page.goto(`${env.admin}/login`);
    await page.getByRole("textbox", { name: /^email/i }).fill(env.practitionerEmail);
    await page.getByRole("textbox", { name: /^password/i }).fill(env.practitionerPassword);
    await page.getByRole("button", { name: /sign in|log in/i }).click();
    await expect(page.getByRole("navigation", { name: /practice/i })).toBeVisible();

    await use(page);
    await context.close();
  },
});

export { expect };

/**
 * Books the first slot offered for a service and returns nothing but the
 * confirmation the client sees.
 *
 * It deliberately does not reach into the API to find a slot. Booking the
 * slot the page actually offered is the whole point: a page that offers a
 * slot the API then refuses is a defect this would catch and a seeded
 * booking would not.
 */
export async function bookFirstAvailableSlot(page: Page, serviceName: RegExp): Promise<void> {
  await page.goto(`${env.web}/book`);
  await page.getByRole("button", { name: serviceName }).click();

  const slots = page.getByRole("button", { name: /^\d{1,2}:\d{2}/ });
  await expect(slots.first()).toBeVisible();
  await slots.first().click();

  await page.getByRole("button", { name: /confirm|book/i }).click();
  await expect(page.getByText(/booked|confirmed/i)).toBeVisible();
}

/** Fails the test on any console error, which is otherwise invisible. */
export function failOnConsoleErrors(page: Page): void {
  page.on("console", (message) => {
    if (message.type() !== "error") return;
    // React's hydration warnings and a failed favicon are noise; anything
    // else on a page a client sees is a defect.
    const text = message.text();
    if (/favicon|ResizeObserver loop/i.test(text)) return;
    throw new Error(`Console error on ${page.url()}: ${text}`);
  });
}
