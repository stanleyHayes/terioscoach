# Security Review — Terios Platform (LCH-01)

> Reviewed against the OWASP Top 10 (2021). Each item states what is in
> place, where it is enforced, and which test proves it. Items marked
> **Open** are genuine gaps, not accepted risks.

## A01 — Broken Access Control

| Control | Where | Proof |
|---|---|---|
| Client data isolation: every client-scoped query leads with the caller's own id | app services (`clients`, `notes`, `forms`, `documents`, `reviews`) | `TestClientsIsolationAndRoles`, `TestNotesIsolationAndRoles`, `TestSubmissionIsolation`, `TestDocumentIsolation`, `TestReviewIsolation` |
| Cross-owner access answers **404, never 403** — no existence leak | same | the isolation tests above assert the code, not just the status |
| Role guards on every practitioner route | `RequireRole` middleware | role-guard tests in each slice's `*_test.go` |
| Unshared documents and unshared notes are invisible to the client, existence included | `document.ReadableBy`, `note.ClientView` | `TestUnsharedDocumentsAreInvisible`, `TestNotesSharingVisibility` |
| Video rooms admit only the two parties to the booking | `signaling.Authorize` | `TestOutsidersGetNotFound`, `TestOutsidersCannotJoin`, `TestRoomsAreIsolated` |
| The signaling socket relays only a closed message set (negotiation, chat, presence, reactions, captions); room events are server-only and sender identity is server-stamped | `signaling.MessageType.Relayable`, `wsapi.Hub.Relay` | `TestOnlySessionTrafficIsRelayable`, `TestRoomEventsCannotBeRelayed`, `TestSenderIsStampedByTheServer` |
| Reports are scoped to the caller's own practice | `reports.Service` | `TestReportIsScopedToTheCallersPractice` |

**Note.** The 404-not-403 rule is applied consistently and is the single
most repeated decision in the codebase. Where a caller could otherwise
learn that a record exists but is not theirs, the answer is "not found".

## A02 — Cryptographic Failures

- Passwords: **Argon2id** (`adapters/security/argon2.go`).
- Refresh tokens: stored as SHA-256 hashes only — a database leak yields no usable session.
- Access tokens: JWT, 15-minute TTL; refresh rotates on every use.
- Consent records: SHA-256 digest binds answers + typed name + timestamp; `VerifyIntegrity` re-checks it (`TestIntegrityHashDetectsTampering`).
- Paystack webhooks: HMAC-SHA512, constant-time compare, hashed **before** JSON parsing. Stripe webhooks: HMAC-SHA256 over `t.payload` under the webhook signing secret, constant-time compare, 5-minute timestamp tolerance against replays.
- Secrets live only in env stores. Scan for hardcoded secrets: clean; no `.env` tracked in git.
- HSTS in production only — sending it from a local http:// dev server would poison the browser for localhost (`TestHSTSOnlyInProduction`).

## A03 — Injection

- **NoSQL:** all MongoDB access goes through the driver with typed `bson.M` filters; no string-built queries anywhere.
- **XSS (email):** every template value is HTML-escaped; enquiry text is attacker-controlled (`TestEscapesUntrustedValues`).
- **XSS (web):** no `dangerouslySetInnerHTML` in either app. CMS bodies render as text paragraphs via `Prose`, which deliberately does **not** interpret HTML.
- **Stored XSS via URL:** `coverImage` and signature images accept only `http(s)`/site-relative and `data:image/png` respectively; `javascript:`, `data:text/html` and SVG are rejected in the domain (`TestCoverImageRejectsScriptURLs`, `TestSignatureValidation`).
- **Header injection:** enquiry addresses containing newlines are rejected (`TestValidationRejectsUnusableInput`).
- **Path traversal:** uploaded filenames are stripped of any path component (`TestFilenameSanitization`).

## A04 — Insecure Design

- Booking double-booking is prevented by a **partial unique index**, not by a check-then-write race.
- One review per booking, one note per booking, one payment per booking — all unique-indexed.
- Notifications are a durable outbox with atomic claim: a booking never fails because mail is down, and no message is sent twice.
- Signed forms and shared notes are **one-way**: the API has no unshare and no re-submit.

## A05 — Security Misconfiguration

| Control | Where | Proof |
|---|---|---|
| CORS: exact-match allowlist, credentialed, **never** `*` or origin-reflection | `httpapi.CORS` | `TestUnknownOriginGetsNoCORSHeaders`, `TestNoWildcardEverAppears` |
| Security headers on every response, errors included | `httpapi.SecurityHeaders` | `TestSecurityHeadersOnEveryResponse`, `TestHeadersSurviveAnErrorResponse` |
| `Cache-Control: no-store` — bearer tokens and client records never sit in a shared proxy cache | same | as above |
| WebSocket origin checking (a WS handshake is **not** covered by same-origin policy) | `wsapi.NewHandler` | origin patterns come from `ALLOWED_ORIGINS` |
| Unconfigured providers answer 503 rather than appearing to work | every `With*` option | `*UnavailableWithoutDatabase` tests |

**Deployment requirement:** `ALLOWED_ORIGINS` must list both app origins
exactly. Empty means no browser origin is permitted — the correct failure
mode for a misconfiguration, but it will take the apps down, so it belongs
in the launch checklist.

## A06 — Vulnerable and Outdated Components

- Go: `chi`, `mongo-driver/v2`, `golang-jwt/v5`, `coder/websocket`, `golang.org/x/crypto`. No SDKs for Paystack, Stripe, Resend or Cloudinary — those are plain `net/http`, so the dependency surface stays small.
- Web/admin: Next.js 16, React 19, Tailwind 4, lucide-react. No charting, date or form libraries.
- `govulncheck` runs on every API build; `npm audit --audit-level=high` on both apps. Added in LCH-03 — this was an open item and is now closed.

## A07 — Identification and Authentication Failures

| Control | Where | Proof |
|---|---|---|
| Per-IP rate limit on credential routes | `httpapi.RateLimit` | `TestAuthRateLimitReturns429` |
| Per-identifier brute-force lockout, MongoDB-backed so it survives restarts and spans instances | `identity.LockoutPolicy` | `TestLockoutTripsAndReleases` |
| Lockout is **not** an account-existence oracle — unknown emails lock identically | `auth.Service.Login` | `TestLockoutIsNotAnEnumerationOracle`, `TestLoginLockoutReturns429WithoutLeakingExistence` |
| Uniform `invalid_credentials` on every login failure | same | `TestLoginInvalidCredentials` |
| Refresh-token reuse detection revokes the whole session family | `auth.Service.Refresh` | `TestRefreshReuseRevokesEverySession` |
| Password recovery uses 256-bit one-time tokens, stores only SHA-256 hashes, expires after one hour, and revokes every existing session after reset | `auth.Service.ForgotPassword` / `ResetPassword` | `TestPasswordResetIsUniformOneTimeAndRevokesSessions` |
| Recovery requests do not disclose whether an email exists | same | unknown and known addresses return the same 204 response and UI copy |
| Client IP resolved from the **right** of `X-Forwarded-For` | `httpapi.RealIP` | `TestForgedForwardedForCannotResetTheLimit` |

**Note.** Reading `X-Forwarded-For` from the left — chi's own `RealIP`, and
the common default — lets any caller mint a fresh rate-limit identity per
request by varying one header. That was found and fixed during this build.

## A08 — Software and Data Integrity Failures

- Consent records carry an integrity digest; a record altered after signing reports `integrityOk: false`.
- Payment webhooks are verified server-side against the gateway (Paystack `GET /transaction/verify/{ref}`, Stripe `GET /v1/checkout/sessions/{id}`) before any state changes — a forged webhook body is not enough.
- Email templates are embedded and asserted byte-identical to the design source, so a drifted copy fails CI.

## A09 — Security Logging and Monitoring Failures

- Structured `slog` throughout; request IDs on every request.
- Notification failures are reported, never swallowed (`Report` sink).
- Failed logins are recorded in `login_attempts` with a TTL.
- Enquiry source IPs are stored for abuse triage and **never returned** by any route (`TestEnquiryBodyHidesSourceIP`).
- Alerting on lockout spikes, notification backlogs, retry-exhausted deliveries and payment-verification failures: `internal/domain/ops` + `GET /v1/admin/ops/health` (LCH-09). Thresholds are pure rules with their own tests. This was an open item and is now closed.

## A10 — Server-Side Request Forgery

- The API makes outbound calls to a fixed set of hosts (Paystack, Stripe, Resend, Cloudinary), all from configuration, none from user input. Only the configured payment provider's host is ever called.
- No user-supplied URL is ever fetched server-side. `coverImage` is stored and rendered by the browser, never retrieved by the API.

## Open items

1. ~~Dependency scanning in CI~~ — **closed.** `govulncheck` on the API, `npm audit --audit-level=high` on both apps, blocking.
2. ~~Alerting on lockout spikes and failed-notification backlogs~~ — **closed.** `internal/domain/ops` and `GET /v1/admin/ops/health`; a monitor still has to be pointed at it once a deployment exists.
3. **`ALLOWED_ORIGINS` must be set at deploy** — the apps cannot call the API without it. It is in `render.yaml` as `sync: false`, so the deploy will not silently default it, but it is still a step someone has to take.
4. **Penetration test** — still open, and the one that cannot be closed from inside the codebase. This is a code review, not an adversarial exercise against a running system.
5. **Live-origin verification** — after deployment, verify headers, redirects, TLS/HSTS, `robots.txt`, sitemap URLs, recovery-email delivery and reset links on the real web/admin origins. Local builds prove generation, not edge behavior.
6. **Browser CSP rollout** — framework-independent headers are now enforced in both Next configs and Vercel manifests. A strict browser CSP remains staged work because Next's generated inline boot scripts require nonce/hash integration; shipping a guessed policy would break the applications.
