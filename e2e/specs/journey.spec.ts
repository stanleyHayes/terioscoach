import { env, expect, failOnConsoleErrors, test } from "./fixtures";

/**
 * The whole journey, across both apps (LCH-02).
 *
 * A client books and pays on the customer site; the practitioner runs the
 * session, writes it up and shares it from the dashboard; the client reads
 * the feedback and leaves a review; the practitioner approves it and it
 * appears on the public site. One story, two deployments, one database.
 *
 * The API-level twin of this lives in `journey_test.go` and runs in a
 * second with no infrastructure. What only this can prove is the part that
 * involves two real origins: that the apps' CORS, cookies, token refresh
 * and routing all hold while the story runs.
 */

test.describe.configure({ mode: "serial" });

test("a client books, pays, is seen, and reviews", async ({ clientPage, adminPage }) => {
  const { page, client } = clientPage;
  failOnConsoleErrors(page);

  await test.step("the client books the first slot the site offers", async () => {
    await page.goto(`${env.portal}/portal/book`);
    await page.getByRole("button", { name: /massage|session/i }).first().click();

    const slots = page.getByRole("button", { name: /^\d{1,2}:\d{2}/ });
    await expect(slots.first()).toBeVisible();
    await slots.first().click();
    await page.getByRole("button", { name: /confirm|book/i }).click();

    await expect(page.getByText(/booked|confirmed/i)).toBeVisible();
  });

  await test.step("the booking is in their portal, not just on a success page", async () => {
    await page.goto(`${env.portal}/portal/sessions`);
    await expect(page.getByRole("listitem").first()).toBeVisible();
  });

  await test.step("the client pays through Stripe test checkout", async () => {
    await page.getByRole("button", { name: /pay/i }).first().click();

    // Stripe's own hosted page. Selectors here belong to them, so this
    // step is deliberately shallow: it proves we hand off and come back,
    // not that their form works.
    await page.waitForURL(/checkout\.stripe\.com/, { timeout: 30_000 });
    await page.getByPlaceholder(/card number/i).fill("4242424242424242");
    await page.getByPlaceholder(/mm\s*\/\s*yy|expiry/i).fill("12/30");
    await page.getByPlaceholder(/cvc|cvv/i).fill("123");
    await page.getByRole("button", { name: /pay/i }).click();

    // Back on our site, and the state changed because the webhook and the
    // verify call agreed — not because the redirect said so.
    await page.waitForURL(new RegExp(env.portal.replace(/^https?:\/\//, "")), { timeout: 60_000 });
    await expect(page.getByText(/paid/i).first()).toBeVisible({ timeout: 30_000 });
  });

  await test.step("the practitioner sees the booking on the calendar", async () => {
    await adminPage.goto(`${env.admin}/calendar`);
    await expect(adminPage.getByText(client.name).first()).toBeVisible();
  });

  await test.step("the practitioner writes up the session and shares the feedback", async () => {
    await adminPage.getByText(client.name).first().click();
    await adminPage.getByRole("button", { name: /complete/i }).click();

    await adminPage.getByRole("textbox", { name: /private notes/i })
      .fill("Held a lot of tension in the shoulders. Watch it next time.");
    await adminPage.getByRole("textbox", { name: /shared feedback/i })
      .fill("A good first session — you should feel freer for a few days.");
    await adminPage.getByRole("button", { name: /save/i }).click();

    await adminPage.getByRole("button", { name: /share/i }).click();
    await expect(adminPage.getByText(/shared/i).first()).toBeVisible();
  });

  await test.step("the client reads the shared feedback and not the private notes", async () => {
    await page.goto(`${env.portal}/portal/sessions`);
    await page.getByRole("link", { name: /view|details/i }).first().click();

    await expect(page.getByText(/you should feel freer/i)).toBeVisible();
    // The single most important assertion in this file.
    await expect(page.getByText(/held a lot of tension/i)).toHaveCount(0);
  });

  await test.step("the client reviews the session", async () => {
    await page.getByRole("button", { name: /leave.*(review|feedback)/i }).click();
    await page.getByRole("radio", { name: /5 star|5 out of 5/i }).click();
    await page.getByRole("textbox", { name: /comment|review/i })
      .fill("I left feeling like a different person. Thank you.");
    await page.getByRole("button", { name: /submit|send/i }).click();
    await expect(page.getByText(/thank you|received/i)).toBeVisible();
  });

  await test.step("nothing is public until the practitioner approves it", async () => {
    const visitor = await page.context().browser()!.newContext();
    const anonymous = await visitor.newPage();
    await anonymous.goto(`${env.web}/testimonials`);
    await expect(anonymous.getByText(/like a different person/i)).toHaveCount(0);

    await adminPage.goto(`${env.admin}/reviews`);
    await adminPage.getByText(/like a different person/i).waitFor();
    await adminPage.getByRole("button", { name: /^publish$/i }).first().click();
    await expect(adminPage.getByText(/on the site/i).first()).toBeVisible();

    await anonymous.reload();
    await expect(anonymous.getByText(/like a different person/i)).toBeVisible();
    await visitor.close();
  });

  await test.step("the session and its income reach the reports", async () => {
    await adminPage.goto(`${env.admin}/reports`);
    await expect(adminPage.getByText(/sessions completed/i)).toBeVisible();
    // The number itself is asserted in the API test, where the window is
    // controlled. Here it only has to be present and non-zero.
    await expect(adminPage.getByText(/^0$/)).toHaveCount(0);
  });
});

test("a client cannot reach another client's session", async ({ clientPage, adminPage }) => {
  const { page } = clientPage;

  // Find a booking id belonging to someone else, the way an attacker
  // would: from a URL they were never given.
  await adminPage.goto(`${env.admin}/calendar`);
  await adminPage.getByRole("button", { name: /session|booking/i }).first().click();
  const url = adminPage.url();
  const strangersBooking = url.match(/([0-9a-f]{24})/i)?.[1];
  test.skip(!strangersBooking, "no seeded booking to probe with");

  await page.goto(`${env.portal}/portal/sessions/${strangersBooking}`);
  // 404, not 403 — a stranger must not learn the session exists.
  await expect(page.getByText(/not found|couldn't find/i)).toBeVisible();
  await expect(page.getByText(/not allowed|forbidden|permission/i)).toHaveCount(0);
});
