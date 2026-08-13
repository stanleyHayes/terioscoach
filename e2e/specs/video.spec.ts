import { env, expect, test, type TestClient } from "./fixtures";
import type { Page } from "@playwright/test";

/**
 * The video room, with two real browsers (CX-05/CX-06, LCH-02).
 *
 * This is the only part of the platform that cannot be proved without a
 * browser at all. Signaling can be tested at the API level — and is — but
 * whether two peers actually exchange media depends on ICE, on the network
 * between them, and on a WebRTC stack no fake can stand in for.
 *
 * The assertion that matters is the connection state. A visible `<video>`
 * element proves only that the DOM has one; a black rectangle looks
 * identical to a working call in a screenshot.
 */

test.describe.configure({ mode: "serial" });

test.use({ browserName: "chromium" });

/** Reads the live peer-connection state the app exposes for exactly this. */
async function connectionState(page: import("@playwright/test").Page): Promise<string> {
  return page.evaluate(
    () => (window as unknown as { __peerConnectionState?: string }).__peerConnectionState ?? "none",
  );
}

test("the practitioner and client meet in the room", async ({
  clientPage,
  adminPage,
}: {
  clientPage: { page: Page; client: TestClient };
  adminPage: Page;
}) => {
  const { page: client, client: account } = clientPage;

  // A session that is happening now. Seeding it through the API is the
  // only honest way — the room opens on the clock, and a browser cannot
  // be asked to wait a week.
  const bookingId = await seedSessionStartingNow(account.email);
  test.skip(!bookingId, "seed route unavailable — set E2E_SEED_TOKEN");

  await test.step("both sides join", async () => {
    await Promise.all([
      client.goto(`${env.web}/portal/sessions/${bookingId}/room`),
      adminPage.goto(`${env.admin}/sessions/${bookingId}/room`),
    ]);

    await expect(client.getByRole("button", { name: /leave|end/i })).toBeVisible();
    await expect(adminPage.getByRole("button", { name: /leave|end/i })).toBeVisible();
  });

  await test.step("the peer connection actually connects", async () => {
    await expect
      .poll(() => connectionState(client), { timeout: 30_000 })
      .toMatch(/^(connected|completed)$/);
    await expect
      .poll(() => connectionState(adminPage), { timeout: 30_000 })
      .toMatch(/^(connected|completed)$/);
  });

  await test.step("media is flowing, not merely negotiated", async () => {
    // Bytes received is the difference between a connected transport and
    // an actual call. A room can reach `connected` and still show nothing.
    const received = await client.evaluate(async () => {
      const pc = (window as unknown as { __peerConnection?: RTCPeerConnection }).__peerConnection;
      if (!pc) return 0;
      let bytes = 0;
      (await pc.getStats()).forEach((stat) => {
        const s = stat as { type?: string; bytesReceived?: number };
        if (s.type === "inbound-rtp" && s.bytesReceived) bytes += s.bytesReceived;
      });
      return bytes;
    });
    expect(received).toBeGreaterThan(0);
  });

  await test.step("chat flows between the two sides", async () => {
    // Open the panel on both sides, send from the practitioner, read on
    // the client. The relay is the signaling socket itself, so a arrived
    // message proves the collaboration channel end to end.
    await adminPage.getByRole("button", { name: /show chat/i }).click();
    await client.getByRole("button", { name: /show chat/i }).click();
    await adminPage.getByLabel(/write a message/i).fill("See you in the room");
    await adminPage.getByRole("button", { name: /^send$/i }).click();
    await expect(client.getByText("See you in the room")).toBeVisible({ timeout: 10_000 });
  });

  await test.step("a third party cannot get in", async () => {
    const outsider = await client.context().browser()!.newContext();
    const page = await outsider.newPage();
    await page.goto(`${env.web}/portal/sessions/${bookingId}/room`);
    // Unauthenticated: sent to sign in, never to the room.
    await expect(page).toHaveURL(/login/);
    await outsider.close();
  });

  await test.step("leaving releases the camera", async () => {
    await client.getByRole("button", { name: /leave|end/i }).click();
    const live = await client.evaluate(() => {
      const stream = (window as unknown as { __localStream?: MediaStream }).__localStream;
      return stream ? stream.getTracks().filter((track) => track.readyState === "live").length : 0;
    });
    // A camera light still on after leaving a session is the kind of thing
    // a client notices and does not forget.
    expect(live).toBe(0);
  });
});

test("TURN is configured where the deployment claims it is", async ({
  clientPage,
  adminPage,
}: {
  clientPage: { page: Page; client: TestClient };
  adminPage: Page;
}) => {
  test.skip(!env.expectTurn, "TURN not expected on this deployment (CX-02)");
  const { page: client, client: account } = clientPage;

  // The ICE servers arrive in the join response; the room exposes them on
  // the debug surface for exactly this assertion.
  const bookingId = await seedSessionStartingNow(account.email);
  test.skip(!bookingId, "seed route unavailable — set E2E_SEED_TOKEN");

  await Promise.all([
    client.goto(`${env.web}/portal/sessions/${bookingId}/room`),
    adminPage.goto(`${env.admin}/sessions/${bookingId}/room`),
  ]);

  await expect
    .poll(
      () =>
        client.evaluate(() => {
          const servers =
            (window as unknown as { __iceServers?: { urls: string[] }[] }).__iceServers ?? [];
          // STUN alone connects two peers on the same LAN and fails behind
          // symmetric NAT, which is how a green suite hides a broken product.
          return servers
            .flatMap((server) => server.urls)
            .some((url) => url.startsWith("turn:") || url.startsWith("turns:"));
        }),
      { timeout: 15_000 },
    )
    .toBe(true);
});

/**
 * Creates a session starting now, via the test-only seed route.
 *
 * Returns an empty string when the route is not available, so the specs
 * skip rather than fail on a deployment that has not enabled it.
 */
async function seedSessionStartingNow(clientEmail: string): Promise<string> {
  if (!env.seedToken) return "";
  const response = await fetch(`${env.api}/v1/testing/sessions`, {
    method: "POST",
    headers: {
      "content-type": "application/json",
      authorization: `Bearer ${env.seedToken}`,
    },
    body: JSON.stringify({ clientEmail, startingIn: 0 }),
  });
  if (!response.ok) return "";
  const body = (await response.json()) as { bookingId?: string };
  return body.bookingId ?? "";
}
