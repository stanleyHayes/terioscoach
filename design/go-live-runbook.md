# Go-live runbook

Covers FND-09 (Vercel projects), LCH-04 (backups and the restore drill),
LCH-08 (domain cutover) and the monitoring half of LCH-09.

Every step below needs an account or DNS access that the codebase cannot
create for itself. The configuration each step applies is already written
and in the repo — `render.yaml`, `apps/*/vercel.json`, the env tables
below. What remains is somebody with the credentials doing it.

---

## 0. Before anything

Have these to hand:

- The registrar login for `terioscoach.com` (Google Domains / Squarespace).
- Accounts: MongoDB Atlas, Render, Vercel, Resend, Cloudinary, Stripe,
  Cloudflare (for TURN).
- The practitioner's real email address, for `PRACTICE_EMAIL`.

**Do the whole of steps 1–6 against staging first.** The cutover in step 7
is the only irreversible one, and it should be the first time nothing new
happens.

---

## 1. MongoDB Atlas (FND-05, LCH-04)

1. Create an **M10** cluster (not M0 — the free tier has no backups, which
   makes LCH-04 impossible) in the region nearest Accra: `eu-central-1`,
   matching Render's Frankfurt.
2. Create a database user with `readWrite` on `terios` only. Not an admin
   user; the API never needs to create databases.
3. Network access: allow Render's egress IPs. **Not `0.0.0.0/0`.** A
   connection string is a password, and passwords leak.
4. Enable **daily backups** with a 7-day point-in-time window.
5. Run the index creation once: the API applies `mongodb/indexes.go` at
   startup, so this happens on first deploy — confirm in Atlas that the
   partial unique index on `bookings (practitionerId, startAt)` exists.
   **Without it, two people can book the same slot.** That index is the
   only thing preventing it; the application-level check is a courtesy.

### The restore drill (LCH-04)

Do this once, before launch, and write down how long it took.

1. In Atlas, restore the cluster to a point in time ~1 hour ago, **into a
   new cluster** — never over the live one.
2. Point a local API at the restored cluster with `MONGODB_URI`.
3. Check: a known booking exists, its payment is attached, a signed form
   still reports `integrityOk: true`.
4. Delete the restored cluster.

**A backup nobody has restored from is a hope, not a backup.** The number
worth knowing is not "do we have backups" but "how long from decision to
serving traffic again", and only the drill answers it.

---

## 2. Resend (FND-06)

1. `terioscoach.com` is already registered in Resend with status `not_started`; publish its DKIM/SPF records
   the dashboard gives you at the registrar.
2. Wait for verification. **Until it is verified, mail is accepted and
   silently not delivered** — which looks exactly like everything working.
3. Set `RESEND_FROM` to `Terios Wellness Spa <no-reply@terioscoach.com>`.
4. Send one test to a Gmail address and one to an Outlook address, and
   check both landed in the inbox rather than spam.

---

## 3. Cloudinary (FND-07)

1. Create the account and note the cloud name, API key and secret.
2. Nothing else needs configuring — folders are created on first upload
   and the upload policy is enforced by the API's signature, not by a
   preset. There is deliberately no unsigned preset: one would let anyone
   who found the cloud name upload anything.

---

## 4. Render (FND-08)

1. New → Blueprint → point at this repo. It reads `render.yaml`.
2. Fill in every `sync: false` variable. The blueprint declares all 35 the
   API reads, so nothing falls back to a default silently.
3. **`ALLOWED_ORIGINS` is versioned in `render.yaml`.** It lists the apex,
   `www`, `practice`, and `app` custom domains plus the three stable Vercel
   aliases, exactly and without trailing slashes. The recurring production
   smoke workflow preflights every custom origin so a future domain cutover
   cannot leave the API healthy but unusable from browsers.
4. Confirm `/readyz` returns 200. It checks the database; `/healthz` only
   proves the process is alive.

---

## 5. Vercel (FND-09)

Three projects from the same repo. `apps/*/vercel.json` carries the build
commands, region and security headers already.

| | `terios-web` | `terios-admin` | `terios-portal` |
|---|---|---|---|
| Root directory | `apps/web` | `apps/admin` | `apps/portal` |
| Production domain | `terioscoach.com` | `practice.terioscoach.com` | `app.terioscoach.com` |
| `NEXT_PUBLIC_API_URL` | the Render API URL | the same | the same |
| `NEXT_PUBLIC_SITE_URL` | `https://terioscoach.com` | — | — |
| `NEXT_PUBLIC_PORTAL_URL` | `https://app.terioscoach.com` | — | `https://app.terioscoach.com` |
| `NEXT_PUBLIC_WEBSITE_URL` | — | — | `https://terioscoach.com` |

Both need **Root Directory** set, and "Include files outside the root
directory" enabled — this is a workspace, and the install runs from the
repo root.

Notes:

- Preview deployments get a different `NEXT_PUBLIC_SITE_URL`, which is what
  makes `isIndexable()` false and keeps previews out of search results and
  out of the analytics numbers. Do not set it to the production URL on
  preview.
- Every preview origin that needs to talk to the API must be in
  `ALLOWED_ORIGINS` too, or add a staging API. Previews pointing at the
  production API is a decision to make deliberately, not by accident.
- The admin and portal projects send `X-Robots-Tag: noindex` on everything.

---

## 6. TURN (CX-02)

Create a Cloudflare Calls TURN key and set `TURN_URLS`, `TURN_USERNAME`,
`TURN_CREDENTIAL` on Render.

Then test it properly: two people, **on different networks**, at least one
on mobile data. Two laptops on the same wifi will connect over STUN alone
and prove nothing. Set `E2E_EXPECT_TURN=1` and run `e2e/specs/video.spec.ts`
against the deployment.

---

## 7. Domain cutover (LCH-08)

Do this at a quiet hour, with no session booked for the next two.

1. **Lower the TTL first.** At the registrar, set the TTL on the existing
   records to 300 seconds and wait for the old TTL to expire — if it is
   currently 24 hours, that means doing this the day before. Skipping it
   is what turns a five-minute rollback into a next-day one.
2. Keep Wix nameservers in place and update only the web-routing records.
   Vercel's live domain inspection on 30 August 2026 recommends:

   | Host/name in Wix | Type | Value |
   |---|---|---|
   | `@` | `A` | `76.76.21.21` |
   | `www` | `CNAME` | `cname.vercel-dns.com` |
   | `practice` | `CNAME` | `cname.vercel-dns.com` |
   | `app` | `CNAME` | `cname.vercel-dns.com` |

   Remove the three existing Wix apex `A` records and the Wix `www` CNAME
   before adding their replacements; a hostname cannot have both a CNAME and
   another record. Do **not** remove or modify the verified Resend records:
   `resend._domainkey` TXT, `rsend` CNAME, and `send` CNAME. This targeted
   cutover preserves email while Vercel routes each hostname to the project it
   is already attached to.
3. Wait for Vercel to issue certificates for all three. Do not proceed while
   any shows as pending.
4. Check, in this order:
   - `https://terioscoach.com` loads and is not a certificate warning.
   - `https://practice.terioscoach.com` loads and shows the login.
   - `https://app.terioscoach.com/login` loads and shows the client login.
   - Sign in on the dashboard. If this fails with a network error,
     `ALLOWED_ORIGINS` is wrong — that is the usual first fault. (A missing
     value stops the API booting; a value with the wrong origin in it
     starts fine and fails exactly like this.)
   - Open the browser console on both apps and confirm there are no
     Content Security Policy violations. Both apps send a CSP (PROD-06);
     the dashboard's is nonce-based, so a caching layer that serves one
     visitor's HTML to another would show up here as refused scripts.
   - Book a session end to end with a real card in Stripe **test** mode.
   - Confirm the confirmation email arrives.
5. Switch Stripe to live keys **last**, after the test booking has
   worked. Then take one real payment of the smallest amount Stripe
   allows, confirm it appears in the dashboard, and refund it.
6. Raise the TTL back to something ordinary (3600).

### Rolling back

Point the DNS records back. With a 300-second TTL that is minutes. Nothing
in the database needs undoing — the old site is static and the new one has
its own storage.

---

## 8. Monitoring (LCH-09)

The API exposes `GET /v1/admin/ops/health` (practitioner token required).
Point an uptime monitor at:

| Check | URL | Alert when |
|---|---|---|
| API alive | `/healthz` | non-200, twice in a row |
| API ready | `/readyz` | non-200, twice in a row — this one means the database |
| Operational health | `/v1/admin/ops/health` | body `status` is `critical` or `unknown` |
| Public site | `https://terioscoach.com` | non-200 |
| Dashboard | `https://practice.terioscoach.com/login` | non-200 |
| Client portal | `https://app.terioscoach.com/login` | non-200 |

Two things about the ops endpoint that matter:

- It returns **HTTP 200 even when critical**. Alert on the `status` field,
  not the status code. A monitor treating 5xx as "down" would page for a
  mail backlog while the API is serving fine.
- **`unknown` is not `healthy`.** It means the counters could not be read.
  Alert on it — otherwise a broken health check looks like a healthy system.

Thresholds are in `internal/domain/ops`. They are deliberately low: on a
normal day every one of these counters is zero.

---

## 9. First fortnight

- Check `/v1/admin/ops/health` daily rather than waiting to be paged.
- Watch the first real reminder go out 24 hours before the first real
  session. That is the one automated thing nobody has yet seen work
  against production data.
- Keep Stripe's dashboard and the practice's Payments screen side by
  side for the first few payments. They should always agree; if they ever
  do not, stop and find out why rather than reconciling by hand.

---

## 10. Production practitioner accounts and MFA

`cmd/seed-production` is the only production seeder; it never imports demo
clients, bookings, payments, or notes. Its scopes are deliberately separate:

- `SEED_SCOPE=all` provisions accounts, the minimum launch catalogue,
  baseline availability, and approved launch content.
- `SEED_SCOPE=content` repairs only CMS launch content.
- `SEED_SCOPE=catalog` repairs only the minimum bookable catalogue and
  baseline availability, without requiring or rotating account passwords.

The catalog scope creates the supplied obligation-free 30-minute introductory
conversation under the owner account and weekday 06:00-22:00 availability only
when those records do not already exist. It does not invent prices for Nurse
Coaching or Holistic Coaching; those are created and priced through Dashboard
→ Services, while hours are maintained through Dashboard → Availability.

The command requires all of the following before it will connect:

- `APP_ENV=production`
- `CONFIRM_PRODUCTION_SEED=seed-terios-production`
- `MONGODB_URI` and `MONGODB_DB`
- both password environment variables, each satisfying the password policy
  (only for the `all` scope)

Re-running preserves existing passwords and MFA state. That makes it safe for
provisioning checks without unexpectedly rotating a working administrator's
credentials. Passwords belong in the operator's secret handoff, never in the
command, repository, logs, or Render configuration after provisioning.

MFA is opt-in from Dashboard → Security. Starting enrollment does not enable
it. The practitioner scans the QR code and must enter a valid six-digit TOTP
before `mfaEnabled` becomes true. Authenticator seeds are AES-256-GCM encrypted
under `MFA_ENCRYPTION_KEY`; this key is required in production and must remain
stable. Disabling MFA requires a current TOTP and revokes all refresh sessions.
