# End-to-end suite (LCH-02)

The browser half of LCH-02. The other half — the same journey driven
through the API with in-memory adapters — lives in
`api/internal/adapters/httpapi/journey_test.go` and runs on every commit
with no infrastructure at all.

**These specs need a running stack.** They drive real browsers against real
deployments, so they are gated on FND-05 (Atlas), FND-09 (Vercel) and a
seeded practitioner account. Until those exist they are written and
reviewable but not runnable, and CI does not run them.

## What each half proves

| | API journey test | This suite |
|---|---|---|
| Runs without infrastructure | yes | no |
| Slices compose in sequence | yes | yes |
| Mongo's unique indexes hold under a real race | no | yes |
| Paystack's real checkout and webhook | no | yes |
| WebRTC actually connects two browsers | no | yes |
| The apps' own routing, auth persistence and forms | no | yes |

The API test is the one that catches a broken seam in five seconds. This
one is the one that catches a seam nobody thought to model.

## Running it

```sh
cd e2e
npm install
npx playwright install --with-deps chromium firefox

cp .env.example .env   # then fill it in
npx playwright test
```

### Environment

| Variable | What it is |
|---|---|
| `E2E_WEB_URL` | The customer site, e.g. a Vercel preview URL |
| `E2E_PORTAL_URL` | The separately deployed client portal |
| `E2E_ADMIN_URL` | The practice dashboard |
| `E2E_API_URL` | The API, used for seeding and teardown |
| `E2E_PRACTITIONER_EMAIL` / `_PASSWORD` | A seeded practitioner account |
| `E2E_SEED_TOKEN` | Authorises the test-only seed/reset routes |

**Never point this at production.** The suite creates clients, bookings and
payments, and cleans up after itself; a failed run leaves rows behind. It
refuses to start against a `terioscoach.com` origin for that reason —
see `guardAgainstProduction` in `fixtures.ts`.

### Payments

Paystack test mode only. `PAYSTACK_SECRET_KEY` on the target deployment
must be a `sk_test_` key; the card journey uses Paystack's published test
card. A run against a live key would take real money.

### Video

`video.spec.ts` runs two browser contexts with fake media devices
(`--use-fake-device-for-media-stream`), so it needs no camera and no
consent prompt. It asserts that the peer connection reaches `connected`,
which is the only assertion that actually proves the WebRTC path works —
seeing a `<video>` element on screen does not.

TURN is required for this to pass from a restricted network (CX-02). With
STUN only, two peers on the same LAN will connect and two peers behind
symmetric NAT will not, which makes a green run meaningless. Set
`E2E_EXPECT_TURN=1` on the deployment that has TURN configured.
