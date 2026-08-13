import { env, expect, failOnConsoleErrors, test } from "./fixtures";

/**
 * The public site (WEB-01…WEB-10, LCH-02).
 *
 * These run on three engines and on a phone, because this is the part a
 * stranger sees first and the part most likely to be reached on a mid-range
 * Android over a slow connection.
 */

const PUBLIC_PAGES = ["/", "/about", "/services", "/blog", "/faq", "/testimonials", "/contact"];

for (const path of PUBLIC_PAGES) {
  test(`${path} loads, is titled, and has one h1`, async ({ page }) => {
    failOnConsoleErrors(page);
    const response = await page.goto(`${env.web}${path}`);

    expect(response?.status()).toBe(200);
    await expect(page).toHaveTitle(/.{10,}/);

    // Exactly one h1. Two is a common consequence of a hero component
    // being reused inside a page that already has a heading, and it makes
    // the page incoherent to a screen reader and to a crawler.
    await expect(page.getByRole("heading", { level: 1 })).toHaveCount(1);

    const description = page.locator('meta[name="description"]');
    await expect(description).toHaveAttribute("content", /.{50,}/);
  });
}

test("every public page carries a canonical and Open Graph tags", async ({ page }) => {
  for (const path of PUBLIC_PAGES) {
    await page.goto(`${env.web}${path}`);
    await expect(page.locator('link[rel="canonical"]')).toHaveAttribute("href", /^https?:\/\//);
    await expect(page.locator('meta[property="og:title"]')).toHaveAttribute("content", /.+/);
    await expect(page.locator('meta[property="og:image"]')).toHaveAttribute("content", /^https?:\/\//);
  }
});

test("the portal is kept out of search results", async ({ page }) => {
  const response = await page.goto(`${env.web}/robots.txt`);
  const body = (await response?.text()) ?? "";

  expect(body).toMatch(/Disallow: \/portal\//);
  expect(body).toMatch(/Disallow: \/login/);

  // A preview deployment must refuse everything, or a preview URL ends up
  // in the index competing with the real site.
  const isPreview = !/^https:\/\/(www\.)?terioswellness\.com/.test(env.web);
  if (isPreview) {
    expect(body).toMatch(/Disallow: \/$/m);
  }
});

test("the sitemap lists the public pages and nothing private", async ({ page }) => {
  const response = await page.goto(`${env.web}/sitemap.xml`);
  const body = (await response?.text()) ?? "";

  expect(body).toContain("<urlset");
  expect(body).toContain("/services");
  expect(body).not.toContain("/portal");
  expect(body).not.toContain("/login");
});

test("an enquiry reaches the practice", async ({ page }) => {
  failOnConsoleErrors(page);
  await page.goto(`${env.web}/contact`);

  await page.getByRole("textbox", { name: /^name/i }).fill("E2E Visitor");
  await page.getByRole("textbox", { name: /^email/i }).fill("e2e-enquiry@terios-test.invalid");
  await page.getByRole("textbox", { name: /message/i })
    .fill("Testing the contact form. Please ignore.");
  await page.getByRole("button", { name: /send|submit/i }).click();

  await expect(page.getByText(/thank you|received|be in touch/i)).toBeVisible();
});

test("the contact form refuses an unusable address without a native bubble", async ({ page }) => {
  await page.goto(`${env.web}/contact`);

  await page.getByRole("textbox", { name: /^name/i }).fill("E2E Visitor");
  await page.getByRole("textbox", { name: /^email/i }).fill("not-an-address");
  await page.getByRole("textbox", { name: /message/i }).fill("Hello");
  await page.getByRole("button", { name: /send|submit/i }).click();

  // Our own message, in our own words, in the page — not the browser's
  // grey bubble, which we cannot style and which vanishes on a scroll.
  await expect(page.getByRole("alert")).toBeVisible();
  const usesNativeValidation = await page.locator("form[novalidate]").count();
  expect(usesNativeValidation).toBeGreaterThan(0);
});

test("no page ships a native select or date input", async ({ page }) => {
  for (const path of [...PUBLIC_PAGES, "/book"]) {
    await page.goto(`${env.web}${path}`);
    expect(await page.locator("select").count(), `select on ${path}`).toBe(0);
    expect(
      await page.locator('input[type="date"], input[type="checkbox"], input[type="radio"]').count(),
      `native control on ${path}`,
    ).toBe(0);
  }
});
