# Terios API Contract — v1

> Living reference. Every backend agent implements to this; every frontend agent codes against it.
> If an implementation must deviate, the deviating agent's parent updates THIS file first.
> Base URL: `NEXT_PUBLIC_API_URL` / Render service URL. All routes prefixed `/v1`.

## Conventions

- **Auth:** `Authorization: Bearer <accessToken>` on protected routes.
- **Error shape (all non-2xx):** `{ "error": { "code": "snake_case", "message": "Human readable" } }`
- **Identity:** `user: { "id": string, "email": string, "role": "client"|"practitioner", "name": string }`
- **IDs:** Mongo ObjectID hex strings. **Money:** integer minor units (`priceKobo`). **Timestamps:** RFC 3339 UTC.
- **Pagination (list endpoints):** `?limit=<n>&cursor=<opaque>` → `{ "items": [...], "nextCursor": string|null }`

## Auth (BE-01)

| Method | Route | Body | Success | Errors |
|---|---|---|---|---|
| POST | `/v1/auth/register` | `{email, password, name}` — client self-registration only | 201 `{accessToken, accessTokenExpiresAt, refreshToken, user}` | 409 `email_taken`, 400 `validation_error` |
| POST | `/v1/auth/login` | `{email, password}` | 200 `{accessToken, accessTokenExpiresAt, refreshToken, user}` | 401 `invalid_credentials` (uniform, no enumeration) |
| POST | `/v1/auth/refresh` | `{refreshToken}` | 200 `{accessToken, accessTokenExpiresAt, refreshToken, user}` (rotated) | 401 `token_invalid`, 401 `token_expired` |
| POST | `/v1/auth/logout` | `{refreshToken}` | 204 (idempotent — unknown tokens also 204) | — |
| GET | `/v1/auth/me` | — (Bearer) | 200 `{user}` | 401 `unauthorized`, 401 `token_invalid`, 401 `token_expired` |

- `user` is the full identity shape from Conventions (`{id, email, role, name}`) on every route that returns it, including refresh.
- `accessTokenExpiresAt` is an RFC 3339 UTC timestamp.
- Access token: JWT, 15m, claims `sub`, `role`. Refresh token: opaque, 30d, rotated on every use.
- Password rules: min 12 chars. Role `practitioner` is provisioned via seed only; registration always yields `client`.
- When the database is not configured (dev without Mongo), all `/v1/auth/*` routes answer 503 `service_unavailable`.

### Auth hardening (BE-02)

Two independent throttles guard the credential routes, plus reuse detection on sessions. Both throttles answer **429** and set a `Retry-After` header in whole seconds.

| Control | Scope | Trips at | Response |
|---|---|---|---|
| Rate limit | per client IP, all of `/v1/auth/*` except `/me` | 20 requests / minute (`AUTH_RATE_LIMIT`, `AUTH_RATE_LIMIT_WINDOW`) | 429 `rate_limited` |
| Brute-force lockout | per submitted email, `/v1/auth/login` | 6 failures in a rolling 15 min (`LOGIN_MAX_ATTEMPTS`, `LOGIN_ATTEMPT_WINDOW`), locked for 15 min (`LOGIN_LOCKOUT_COOLDOWN`) | 429 `too_many_attempts` |

- **No enumeration through the lockout.** Failures are counted against the *submitted* email whether or not an account exists, so `too_many_attempts` — status, code, message, and `Retry-After` alike — is identical for a real address and an invented one. A successful login clears the counter; a locked identifier is refused even with the correct password until the cooldown elapses, and each further attempt while locked extends it.
- **Scope of each layer.** The rate limiter is in-process (one Render service = one edge; scaling out multiplies the effective cap). The lockout lives in MongoDB (`login_attempts`, TTL-reaped), so it survives restarts and holds across instances — it is the layer that stops an attacker spreading attempts across many addresses.
- **Client address resolution.** `X-Forwarded-For` is read from the *right*: with `TRUSTED_PROXY_HOPS` proxies in front (default 1, Render's edge), the entry that many from the end is the address the outermost trusted proxy observed. Entries to its left are caller-supplied, so trusting the leftmost would let anyone mint a new rate-limit identity per request.
- **Refresh-token reuse detection.** Refresh tokens are single-use. Presenting an already-rotated token means it leaked — the legitimate holder and the attacker cannot be told apart — so **every session of that account is revoked** and the call answers 401 `token_invalid`. Both parties must log in again. Ordinary logout revokes only the presented session; other devices stay signed in.

## Health

| GET | `/healthz` | 200 `{status:"ok"}` (liveness) |
| GET | `/readyz` | 200 `{status:"ready"}` / 503 when a dependency is down |

---

<!-- Append new slices below as tasks define them: services (BE-03), availability (BE-04),
     booking (BE-05), payments (BE-06), clients/notes/forms/docs (BE-07/08/10/11),
     cms (BE-12), enquiries (BE-13), reviews (BE-14), reporting (BE-15). -->

## Services (BE-03)

| Method | Route | Auth | Body | Success | Errors |
|---|---|---|---|---|---|
| GET | `/v1/services` | public | — | 200 `{items: [service]}` — active, non-deleted, ordered by `sortOrder` then `createdAt` | 503 `service_unavailable` |
| GET | `/v1/services/all` | practitioner | — | 200 `{items: [service]}` — all non-deleted incl. inactive, same ordering | 401, 403 `forbidden`, 503 |
| POST | `/v1/services` | practitioner | `{name, description, durationMinutes, priceKobo, currency?, sortOrder?}` | 201 `{service}` | 400 `validation_error`, 401, 403 |
| PATCH | `/v1/services/{id}` | practitioner | any subset of `{name, description, durationMinutes, priceKobo, currency, active, sortOrder}` | 200 `{service}` | 400 `validation_error`, 401, 403, 404 `service_not_found` |
| DELETE | `/v1/services/{id}` | practitioner | — | 204 | 401, 403, 404 `service_not_found` |

- `service` shape: `{id, practitionerId, name, description, durationMinutes, priceKobo, currency, active, sortOrder, createdAt, updatedAt}` — IDs and timestamps per Conventions.
- `priceKobo` is integer minor units, `>= 0`. `currency` is ISO 4217, defaults to `"USD"` when omitted on create.
- Validation: `name` required (1–200 chars), `durationMinutes` 5–480, `priceKobo >= 0`, `currency` 3 uppercase letters.
- PATCH with `active: false` deactivates (hides from public list); `sortOrder` reordering is done by PATCHing the new order value(s). Unknown fields are ignored.
- **Delete rule:** if any booking references the service, DELETE soft-deletes (sets `deletedAt`, deactivates; record retained for booking history and never returned by any list endpoint). Without bookings it hard-deletes. Both answer 204 — the difference is invisible to the client.
- Not paginated: a practitioner's catalog is small; `items` is the full list, no `nextCursor`.
- When the database is not configured, all `/v1/services*` routes answer 503 `service_unavailable`.

## Availability (BE-04)

| Method | Route | Auth | Body / Query | Success | Errors |
|---|---|---|---|---|---|
| GET | `/v1/availability/rules` | practitioner | — | 200 `{rules: [rule]}` | 401, 403, 503 |
| PUT | `/v1/availability/rules` | practitioner | `{rules: [rule]}` | 200 `{rules: [rule]}` (replaced set) | 400 `validation_error`, 401, 403 |
| POST | `/v1/availability/time-off` | practitioner | `{startAt, endAt, reason?}` | 201 `{timeOff}` | 400 `validation_error`, 401, 403 |
| GET | `/v1/availability/slots` | public | `?serviceId=<id>&from=<YYYY-MM-DD>&to=<YYYY-MM-DD>&tz=<IANA>` | 200 `{serviceId, durationMinutes, timezone, slots: [{startAt, endAt}]}` | 400 `validation_error`, 400 `invalid_timezone`, 404 `service_not_found`, 503 |

- `rule` shape: `{weekday, windows: [{startMin, endMin}], bufferMinutes}`.
  - `weekday` 0=Sunday … 6=Saturday. A weekday with no rule is closed. PUT replaces the entire weekly schedule (send all 7 days' rules; omit closed days).
  - `startMin`/`endMin` are minutes since local midnight; `0 <= startMin < endMin <= 1440`. Overnight windows are rejected. Windows within a day must be sorted and non-overlapping.
  - `bufferMinutes` 0–120: recovery gap kept free after (and before) every busy interval when computing slots.
- `timeOff` shape: `{id, practitionerId, startAt, endAt, reason, createdAt}`; `startAt < endAt` (RFC 3339 UTC). Time-off blocks slot generation across the whole range.
- **Slot computation** (server-side, deterministic):
  - `from`/`to` are inclusive calendar dates interpreted in `tz`; range max 62 days. `tz` is an IANA name (e.g. `Africa/Accra`, `America/New_York`); defaults to `Africa/Accra` when omitted.
  - Windows are evaluated in local wall-clock time of `tz` (DST-safe). Candidate start times begin at each window start and step by `durationMinutes + bufferMinutes` of that service/day.
  - A candidate is bookable when `[start, start+duration)` fits inside its window, starts in the future, overlaps no time-off, and overlaps no busy interval (existing bookings) expanded by `bufferMinutes` on both sides.
  - `slots[].startAt`/`endAt` are always RFC 3339 **UTC** regardless of `tz`. Only active services yield slots; an unknown or inactive `serviceId` answers 404 `service_not_found`.
- When the database is not configured, all `/v1/availability*` routes answer 503 `service_unavailable`.

## Bookings (BE-05)

| Method | Route | Auth | Body / Query | Success | Errors |
|---|---|---|---|---|---|
| POST | `/v1/bookings` | client | `{serviceId, startAt, tz?}` | 201 `{booking}` | 400 `validation_error`, 400 `invalid_timezone`, 401, 403 `forbidden`, 404 `service_not_found`, 409 `slot_unavailable`, 503 |
| GET | `/v1/bookings/mine` | client | — | 200 `{items: [booking]}` — the client's own bookings, upcoming and past, ordered by `startAt` ascending | 401, 403 `forbidden`, 503 |
| GET | `/v1/bookings` | practitioner | `?from=<RFC 3339>&to=<RFC 3339>&status=<status>` — all optional | 200 `{items: [booking]}` — own bookings, ordered by `startAt` ascending | 400 `validation_error`, 401, 403 `forbidden`, 503 |
| GET | `/v1/bookings/{id}` | client (owner) or practitioner | — | 200 `{booking}` | 401, 404 `booking_not_found`, 503 |
| POST | `/v1/bookings/{id}/reschedule` | client (owner) or practitioner | `{startAt, tz?}` | 200 `{booking}` — old slot freed, new one held | 400 `validation_error`, 400 `invalid_timezone`, 401, 404 `booking_not_found`, 409 `slot_unavailable`, 409 `invalid_status`, 422 `cutoff_passed`, 503 |
| POST | `/v1/bookings/{id}/cancel` | client (owner) or practitioner | — | 200 `{booking}` — slot freed immediately | 401, 404 `booking_not_found`, 409 `invalid_status`, 422 `cutoff_passed`, 503 |
| POST | `/v1/bookings/{id}/complete` | practitioner | — | 200 `{booking}` | 401, 403 `forbidden`, 404 `booking_not_found`, 409 `invalid_status`, 503 |
| POST | `/v1/bookings/{id}/no-show` | practitioner | — | 200 `{booking}` | 401, 403 `forbidden`, 404 `booking_not_found`, 409 `invalid_status`, 503 |

- `booking` shape: `{id, clientId, practitionerId, serviceId, startAt, endAt, status, createdAt, updatedAt, cancelledAt?, completedAt?}` — IDs, money-free, and RFC 3339 UTC timestamps per Conventions. `cancelledAt`/`completedAt` appear only once set (`completedAt` is also set by no-show).
- **Statuses:** `pending_payment` (priced appointment requested but not booked; client-visible and non-blocking), `confirmed` (free service or Stripe-verified payment; blocks the slot), `cancelled`, `completed`, `no_show` (terminal). Transitions: `pending_payment → confirmed` only after verified payment, or `pending_payment → cancelled`; `confirmed → cancelled | completed | no_show`. Nothing leaves a terminal state.
- **Slot rule (create and reschedule):** `startAt` (RFC 3339 UTC) must exactly match a slot the availability engine (BE-04) would return for that service on that day — same step (`durationMinutes + bufferMinutes`), windows, time-off, and busy-interval rules — and must be in the future. Anything else answers 409 `slot_unavailable`. An unknown or inactive `serviceId` answers 404 `service_not_found`, mirroring `/v1/availability/slots`.
- **Payment gate and double-booking guard:** creating a priced service produces `pending_payment`, sends no confirmation/reminder, does not appear in the practitioner's default calendar list, and does not block availability. Payment initialization queues an email containing the Stripe checkout link and explicitly says the appointment is not booked until payment succeeds. A verified Stripe webhook promotes the request to `confirmed`; only then are confirmation/reminder jobs queued and the slot occupied. A partial unique index on `{practitionerId, startAt}` for `status = "confirmed"` makes a confirmed double-book physically impossible. Because several unpaid requests may target one open time, if another payment confirms first, a later successful charge is immediately refunded and its request cancelled.
- **Cutoff rule:** a client may reschedule or cancel only until 24 hours before the appointment's current `startAt` (per-practice setting; 24h is the platform default). Inside the cutoff the client gets 422 `cutoff_passed`. The practitioner is never cutoff-restricted.
- `complete` is allowed only once the appointment has ended (`now >= endAt`); `no-show` only once it has started (`now >= startAt`). Earlier attempts answer 409 `invalid_status`. Both are practitioner-only.
- `tz` (IANA name) tells the server which wall clock to evaluate the schedule in, exactly as on `/v1/availability/slots`; it defaults to `Africa/Accra` when omitted. It never changes the UTC timestamps stored or returned.
- **Isolation:** a client touching another client's booking — or a practitioner touching a booking that is not theirs — gets 404 `booking_not_found` (no existence leak). 403 `forbidden` is reserved for role mismatch on role-guarded routes.
- Lists are not paginated: a client's history and a single practitioner's calendar are small; `items` is the full filtered list.
- When the database is not configured, all `/v1/bookings*` routes answer 503 `service_unavailable`.

## Payments (BE-06)

| Method | Route | Auth | Body / Query | Success | Errors |
|---|---|---|---|---|---|
| POST | `/v1/payments/initialize` | client (booking owner) | `{bookingId}` | 200 `{authorizationUrl, reference}` | 400 `validation_error`, 401, 403 `forbidden`, 404 `booking_not_found`, 409 `already_paid`, 409 `invalid_status`, 502 `payment_gateway_error`, 503 |
| POST | `/v1/webhooks/stripe` | **public** (signature-verified) | raw Stripe event JSON | 200 `{}` — for `checkout.session.completed` and for every ignored/duplicate event | 401 `invalid_signature`, 503 |
| GET | `/v1/payments/mine` | client | — | 200 `{items: [payment]}` — own payments, ordered by `createdAt` descending | 401, 403 `forbidden`, 503 |
| GET | `/v1/payments` | practitioner | `?from=<RFC 3339>&to=<RFC 3339>` on `createdAt` — both optional | 200 `{items: [payment]}` — payments on the practitioner's bookings, ordered by `createdAt` descending | 400 `validation_error`, 401, 403 `forbidden`, 503 |
| POST | `/v1/payments/{id}/refund` | practitioner | — | 200 `{payment}` — status `refunded` | 401, 403 `forbidden`, 404 `payment_not_found`, 409 `invalid_status`, 502 `payment_gateway_error`, 503 |

- `payment` shape: `{id, bookingId, clientId, amountKobo, currency, status, providerReference, channel?, paidAt?, createdAt}` — IDs, integer minor units, and RFC 3339 UTC timestamps per Conventions. `providerReference` holds the Stripe Checkout Session ID (`cs_...`).
- **Statuses:** `pending` (initialized, awaiting checkout), `success` (webhook-confirmed), `failed`, `refunded` (terminal). Transitions: `pending → success | failed`; `failed → pending` (re-initialize); `success → refunded`. Nothing leaves `refunded`.
- **Stripe Checkout:** `initialize` creates a Checkout Session (success/cancel URLs derive from `PORTAL_URL`), stores its session ID, and `checkout.session.completed` on `/v1/webhooks/stripe` confirms the charge. Stripe webhook deliveries must carry a valid `Stripe-Signature` header under `STRIPE_WEBHOOK_SECRET`. Confirmation and refund go through the Checkout Session and its PaymentIntent.
- **Hosted checkout only:** card and mobile-money details never touch this API. `initialize` returns the provider's hosted checkout URL (`authorizationUrl`); the client completes payment on the provider's page and the provider calls its webhook.
- **Amount/currency/email are server-derived**, never client-supplied: amount from the booked service's current `priceKobo`, currency from the service, email from the client's account. Stripe's Checkout Session ID is stored so webhook deliveries join back exactly.
- **Initialize guards:** the booking must belong to the caller (cross-owner access answers 404 `booking_not_found` — isolation, as in Bookings) and must be `pending_payment` or a legacy unpaid `confirmed` booking (cancelled/completed bookings answer 409 `invalid_status`). A booking whose payment already succeeded answers 409 `already_paid`. Re-initializing a `pending`/`failed` payment is allowed (abandoned checkout): the same payment record is kept and its `providerReference` is replaced — one payment record per booking, enforced by a unique index on `bookingId`.
- **Webhook verification:** requests to `/v1/webhooks/stripe` must carry a valid `Stripe-Signature` header. The raw body is verified before JSON parsing.
- **Webhook handling:** only `checkout.session.completed` mutates state, and only after a server-side Stripe verification confirms the charge with amount and currency matching the stored payment. It marks the request `confirmed` and queues the actual booking confirmation/reminder only after that update succeeds. Other events and duplicate deliveries are safely acknowledged.
- **Where payment state lives:** the `payments` collection is the source of truth. On `charge.success` the booking is also stamped with a denormalized `paymentStatus` (`"paid"`) + `paidAt`, and on refund with `paymentStatus: "refunded"` — purely so booking lists can display payment state without a join. These are additive fields: the booking lifecycle statuses (`confirmed`/`cancelled`/`completed`/`no_show`) are unchanged, and `paymentStatus`/`paidAt` appear on the booking shape only once set.
- **Refund:** practitioner-only, and only from `success` — refunding a `pending`/`failed`/`refunded` payment answers 409 `invalid_status`. Calls the gateway's refund API with the stored reference; on success the payment becomes `refunded` and the booking's `paymentStatus` follows.
- **Isolation:** a practitioner touching a payment whose booking is not theirs gets 404 `payment_not_found`. Lists are not paginated (small, single-practice volume), mirroring Bookings.
- Production refuses to start unless `STRIPE_SECRET_KEY` and `STRIPE_WEBHOOK_SECRET` are both configured. In development, an unconfigured gateway leaves `/v1/payments*` and `/v1/webhooks/*` mounted with 503 `service_unavailable` responses.

## Sessions and signaling (CX-01)

A video room **is** a booking: there is no separate room entity. Entry is a two-step handshake — an authenticated POST mints a single-use ticket, and the ticket (not a bearer token) opens the WebSocket, because a browser cannot set headers on a WebSocket handshake.

| Method | Route | Auth | Body / Query | Success | Errors |
|---|---|---|---|---|---|
| POST | `/v1/sessions/{bookingId}/join` | client or practitioner (booking party) | — | 200 `{bookingId, role, ticket, ticketExpiresIn, opensAt, closesAt, iceServers}` | 401, 403 `room_not_open` / `room_closed`, 404 `booking_not_found`, 409 `invalid_status`, 503 |
| GET | `/v1/sessions/{bookingId}/signal?ticket=` | **ticket** (no headers) | WebSocket upgrade | `101` — then envelope frames | 400, 401 `ticket_invalid`, 403 `room_not_open` / `room_closed`, 409 `room_full`, 503 |

- **Join guards:** the booking must belong to the caller (cross-party access answers 404 `booking_not_found` — isolation, so sessions cannot be probed), must be `confirmed`, and `now` must be within the window `[startAt − 10m, endAt + 15m)` (`ROOM_OPEN_BEFORE` / `ROOM_CLOSE_AFTER`). `role` is derived from the booking, never the request.
- **Tickets:** 32-byte random, single-use (spend-on-read), 60-second TTL, bound to booking + user. The booking is re-validated at redemption, so a cancellation inside the ticket's minute still closes the room.
- **Room cap:** exactly 2 participants (`MaxParticipants`). A third connection is refused (`room_full`) rather than silently making a group call out of a private consultation. A reconnecting user *replaces* their own earlier connection.
- **ICE servers:** `iceServers` in the join response are short-lived Cloudflare Realtime TURN credentials (cached server-side) plus configured STUN; without `TURN_KEY_ID`/`TURN_API_TOKEN` the response is STUN-only.
- **Socket lifecycle:** text frames only, 64 KB frame cap, 10 s write timeout, 30 s server pings, strict origin allowlist. The socket is bound to `closesAt` — calls cannot outlive their window.

**Message protocol** — one JSON envelope per frame: `{type, from?, role?, payload?, reason?}`. `from`/`role` are stamped by the server; a peer cannot forge them. The set is closed — unknown types are refused with an `error` frame (the socket survives; one bad frame never drops a call):

| Type | Direction | Payload | Purpose |
|---|---|---|---|
| `offer` / `answer` / `candidate` | peer ↔ peer (relayed) | opaque SDP/ICE | WebRTC negotiation — the server never inspects payloads |
| `admission-request` | client → practitioner | — | asks to enter the clinical room |
| `admission-granted` / `admission-denied` | practitioner → client | — | controls entry; negotiation frames are rejected by the server until admission is granted |
| `recording-request` | either peer → other peer | — | asks for explicit consent before a local recording starts |
| `recording-consent` | either peer → other peer | `{approved}` | accepts or refuses that recording request |
| `session-ended` | practitioner → client | — | ends the consultation for both participants |
| `chat` | peer ↔ peer (relayed) | `{text}` (≤500 chars client-side) | one chat line |
| `state` | peer ↔ peer (relayed) | `{micOn, cameraOn, handRaised, recording}` | presence — drives the peer's tile indicators |
| `reaction` | peer ↔ peer (relayed) | `{emoji}` | one transient reaction |
| `caption` | peer ↔ peer (relayed) | `{text, final}` | own-mic transcription relay (Chrome-only Web Speech) |
| `joined` | server → joining peer | `{peers: [...]}` | room state on entry; the second arrival makes the offer (glare rule) |
| `peer-joined` / `peer-left` | server → other peer | — | room events; only the server may announce them |
| `error` | server → peer | `reason` | refused message or protocol problem |
| `ping` / `pong` | peer ↔ server | — | liveness; handled by the server, never relayed |

- **Client-side features over this protocol:** practitioner-controlled waiting room, explicit two-party recording consent, leave-only versus practitioner **End for everyone**, automatic rejoin with fresh ticket on socket drop (bounded backoff, reusing the camera stream), ICE restart on transport failure, screen share (in-band `replaceTrack`, no signaling change), in-call chat, mute/hand/recording presence, reactions, camera/microphone/speaker switching (speaker output where the browser supports `setSinkId`), connection-quality stats, local recording (MediaRecorder → `.webm` download on the recorder's machine; relayed via `state.recording` so the other side always sees the ● Rec indicator), and Chrome-only captions (each side transcribes its own mic and relays the text).
- **Scaling note:** rooms live in one API process (correct for the single-instance deployment). Scaling out needs peers pinned to one instance or a shared relay.
- When the database or signaling is not configured, both routes answer 503 `service_unavailable`.

**Testing (e2e seed route).** `POST /v1/testing/sessions` seeds a confirmed booking starting `startingIn` seconds from now (0 = now) for an existing client email, so the e2e suite can join a room without waiting for a real appointment. It is mounted **only** when `TESTING_SEED_TOKEN` is set and `APP_ENV` is not production — otherwise the path is a plain 404 — and it authenticates with that shared token as the bearer credential (constant-time comparison), not a user session. The booking belongs to the named client and the (single) practitioner account, borrowing the first active catalog service's id and duration (a labelled placeholder + 60 minutes when the catalog is empty), so it satisfies every join guard above.

| Method | Route | Auth | Body | Success | Errors |
|---|---|---|---|---|---|
| POST | `/v1/testing/sessions` | `TESTING_SEED_TOKEN` bearer | `{clientEmail, startingIn}` | 201 `{bookingId}` | 400 `validation_error`, 401 `unauthorized`, 404 `client_not_found`, 422 `no_practitioner` |

## Forms and signatures (BE-10)

| Method | Route | Auth | Body / Query | Success | Errors |
|---|---|---|---|---|---|
| GET / POST | `/v1/admin/forms` | practitioner | `?active=true` / `{title, description?, fields[], template?, sortOrder?}` | 200 `{items: [form]}` / 201 `{form}` | 400, 401, 403, 503 |
| GET / PATCH / DELETE | `/v1/admin/forms/{id}` | practitioner | — / any subset of the create body plus `active` | 200 `{form}` / 204 | 400, 401, 403, 404 `form_not_found`, 503 |
| POST | `/v1/admin/forms/assign` | practitioner | `{formId, clientId, bookingId?}` | 201 `{submission}` | 400, 401, 403, 404, 409 `already_assigned`, 409 `form_inactive`, 503 |
| GET | `/v1/admin/forms/submissions` | practitioner | `?clientId=&formId=&bookingId=&status=assigned\|submitted` | 200 `{items: [submission]}` | 400, 401, 403, 503 |
| GET | `/v1/admin/forms/submissions/{id}` | practitioner | — | 200 `{submission, form, integrityOk, signatureImage?}` | 401, 403, 404 `submission_not_found`, 503 |
| GET | `/v1/forms/mine` | client | — | 200 `{items: [submission]}` — own only | 401, 403, 503 |
| GET | `/v1/forms/mine/{id}` | client (owner) | — | 200 `{submission, form, integrityOk, signatureImage?}` | 401, 403, 404, 503 |
| POST | `/v1/forms/mine/{id}/submit` | client (owner) | `{answers: {key: {value?, values?}}, signature?: {typedName, imageData}}` | 200 `{submission}` | 400, 401, 403, 404, 409 `already_submitted`, 422 `signature_required`, 503 |

- `form` shape: `{id, title, description?, fields[], template, sortOrder, active, createdAt, updatedAt}`; `field` `{key, label, type, required, helpText?, options[]}`. Field types: `text`, `textarea`, `number`, `date`, `select`, `radio`, `checkbox`, `signature`.
- `submission` shape: `{id, formId, formTitle, clientId, bookingId?, status, answers, signature?: {typedName, signedAt}, assignedAt, submittedAt?}`.
- **Answers are validated against the definition**, which the server loads itself: a required field cannot be skipped, a choice value must be on its list, and an answer to a field that does not exist is rejected rather than stored.
- **Submitting is one-way.** A signed consent record that could be edited afterwards would not be evidence of anything; a second submit answers 409.
- **Signatures**: a typed name plus an inline `data:image/png;base64,…` mark (≤ 256 KB). Remote URLs and other schemes are refused. The server stamps the time and the observed IP — a client cannot choose what the record says about where it was signed from. A SHA-256 digest binds the answers, the name, and the timestamp; `integrityOk` is that digest re-verified, so a record altered after signing shows up as `false`.
- **The drawn signature and the digest are served only with a single record**, never in a listing.
- Assigning a form the client already has open answers 409 `already_assigned`. Deleting a form that has been sent retires it (`active: false`) instead — deleting would strand the signed records pointing at it. Both answer 204.
- **Isolation**: another client's submission answers 404 `submission_not_found`.
- When the database is not configured, all form routes answer 503 `service_unavailable`.

## Documents (BE-11)

| Method | Route | Auth | Body / Query | Success | Errors |
|---|---|---|---|---|---|
| POST | `/v1/admin/documents/sign-upload` | practitioner | `{kind, clientId?, filename}` | 200 `{url, fields, expiresAt}` | 400, 401, 403, 415 `unsupported_file_type`, 503 |
| POST | `/v1/admin/documents` | practitioner | `{kind, clientId?, publicId, filename, bytes}` | 201 `{document}` | 400, 401, 403, 413 `file_too_large`, 415, 503 |
| GET | `/v1/admin/documents` | practitioner | `?clientId=` **required** | 200 `{items: [document]}` | 400, 401, 403, 503 |
| PATCH / DELETE | `/v1/admin/documents/{id}` | practitioner | `{title?, visibleToClient?}` / — | 200 `{document}` / 204 | 400, 401, 403, 404 `document_not_found`, 503 |
| GET | `/v1/admin/documents/{id}/url` | practitioner | — | 200 `{url, expiresIn}` | 401, 403, 404, 503 |
| GET | `/v1/documents/mine` | client | — | 200 `{items: [{id, title, filename, format?, bytes, createdAt}]}` — shared only | 401, 403, 503 |
| GET | `/v1/documents/mine/{id}/url` | client (owner) | — | 200 `{url, expiresIn}` | 401, 403, 404, 503 |

- `document` shape (practitioner): `{id, kind, clientId?, title, filename, format?, bytes, visibleToClient, createdAt, updatedAt}`. Kinds: `client_document`, `signed_form`, `cms_image`.
- **Bytes never pass through the API.** The browser uploads straight to Cloudinary with a signature the API mints (10-minute expiry), then registers the result. Downloads are short-lived signed URLs (`DOCUMENT_URL_TTL`, default 1 hour) returned as a value rather than a redirect, so the signed link does not land in browser history or a referrer.
- **The folder is derived server-side** from the kind and the client and is covered by the signature, so a caller cannot redirect an upload into another client's folder or downgrade a private upload to a public one.
- **The storage id (`publicId`) is never returned** by any route: every download goes through the API so ownership is checked first.
- **Two conditions to read a file**: it is the caller's, and it has been shared. An unshared file is invisible to the client — its existence included. Another client's document answers 404 `document_not_found`.
- Accepted types: pdf, jpg, jpeg, png, webp, ≤ 10 MB. Filenames are stripped of any path component before storage.
- Deleting removes the stored object first, then the record — if storage fails the record stays and the practitioner can retry, because a file with no record is an ungoverned asset.
- Without Cloudinary credentials all document routes answer 503 `service_unavailable`.

## Reporting (BE-15)

| Method | Route | Auth | Query | Success | Errors |
|---|---|---|---|---|---|
| GET | `/v1/admin/reports/practice` | practitioner | `?from=&to=` (RFC 3339, default: current month) `&granularity=day\|week\|month` | 200 report | 400 `validation_error`, 401, 403, 503 |
| GET | `/v1/admin/reports/upcoming-load` | practitioner | `?days=` 1–90, default 7 | 200 `{items: [{date, sessions}]}` | 400, 401, 403, 503 |

- Report shape: `{from, to, granularity, summary, byService[], series[], reviews}`.
  - `summary` `{sessionsCompleted, sessionsUpcoming, cancellations, noShows, newClients, incomeKobo, refundedKobo, netKobo, currency}`.
  - `byService` `[{serviceId, name, sessions, incomeKobo}]`, biggest earner first; a service with sessions but no collected money still appears.
  - `series` `[{start, sessions, incomeKobo}]`, one bucket per period **including empty ones**.
  - `reviews` `{count, average, distribution}` over approved reviews.
- **Definitions** (these are the report): sessions are counted by when they happened, money by when it was **collected** — a session paid for in advance is next month's diary and this month's income. Refunds are dated to the refund, not the original sale, and reported *beside* income rather than netted into it. Cancellations and no-shows are counted apart. `newClients` means first-ever booking in the window, not merely active. A confirmed booking whose time has passed counts as a completed session, so the diary and the report agree.
- Weekly buckets start on Monday; a window that opens mid-week produces a partial first bucket labelled with its true week start.
- The window is half-open (`from` inclusive, `to` exclusive), so adjacent periods add up. Maximum range: two years.
- Every route is scoped to the caller's own practice; there is no cross-practice view.
- When the database is not configured, all reporting routes answer 503 `service_unavailable`.

## Enquiries (BE-13)

| Method | Route | Auth | Body / Query | Success | Errors |
|---|---|---|---|---|---|
| POST | `/v1/enquiries` | **public** | `{name, email, phone?, subject?, message}` | 201 `{received: true}` | 400 `validation_error`, 429 `rate_limited`, 503 |
| GET | `/v1/admin/enquiries` | practitioner | `?status=new\|read\|replied\|archived` | 200 `{items: [enquiry]}` — newest first | 400, 401, 403, 503 |
| GET | `/v1/admin/enquiries/unread-count` | practitioner | — | 200 `{count}` — enquiries still `new` | 401, 403, 503 |
| GET | `/v1/admin/enquiries/{id}` | practitioner | — | 200 `{enquiry}` | 401, 403, 404 `enquiry_not_found`, 503 |
| PATCH | `/v1/admin/enquiries/{id}` | practitioner | `{status}` | 200 `{enquiry}` | 400, 401, 403, 404, 503 |
| DELETE | `/v1/admin/enquiries/{id}` | practitioner | — | 204 | 401, 403, 404, 503 |

- `enquiry` shape: `{id, name, email, phone?, subject?, message, status, createdAt, updatedAt}`.
- **The submit response is an acknowledgement, not the record.** An anonymous caller gets `{received: true}` — echoing an id back would hand them a handle they have no route to use.
- **Rate limited**: 5 submissions per hour per client address, resolved through the same trusted-proxy rule as `/v1/auth`. It is the only unauthenticated write in the API, so it carries a cap of its own.
- Validation: name 1–120, message 1–5000, phone ≤ 40, subject ≤ 200, and an address that has a local part, an `@`, and a dotted domain. Addresses containing newlines are rejected — that is header injection, not a typo.
- The sender's IP is stored for abuse triage and is **never returned** by any route.
- **Triage is free-form**: any status can follow any other, so an archived enquiry can come back and a mis-set "replied" can be undone.
- A new enquiry alerts the practice inbox through the notifications outbox (BE-09) — never the sender's own address.
- When the database is not configured, all enquiry routes answer 503 `service_unavailable`.

## Reviews (BE-14)

| Method | Route | Auth | Body / Query | Success | Errors |
|---|---|---|---|---|---|
| POST | `/v1/reviews` | client | `{bookingId, rating, comment?}` | 201 `{review}` — status `pending` | 400, 401, 403, 404 `booking_not_found`, 409 `review_exists`, 422 `session_not_complete`, 503 |
| GET | `/v1/reviews/mine` | client | — | 200 `{items: [review]}` — own reviews, any state, newest first | 401, 403, 503 |
| PATCH | `/v1/reviews/{id}` | client (author) | `{rating?, comment?}` | 200 `{review}` | 400, 401, 403, 404 `review_not_found`, 409 `already_moderated`, 503 |
| GET | `/v1/admin/reviews` | practitioner | `?status=pending\|approved\|rejected` | 200 `{items: [review]}` — own practice, newest first | 400, 401, 403, 503 |
| POST | `/v1/admin/reviews/{id}/approve` · `/reject` | practitioner | — | 200 `{review}` | 401, 403, 404, 503 |
| GET | `/v1/content/reviews` | **public** | `?limit=` (1–20, default 20) | 200 `{items: [{id, authorName, serviceName?, rating, comment?, createdAt}]}` — approved only | 400, 503 |
| GET | `/v1/content/reviews/summary` | **public** | — | 200 `{count, average, distribution: {"1".."5"}}` — approved only | 503 |

- `review` shape (authenticated): `{id, bookingId, clientId, serviceId?, rating, comment?, status, moderatedAt?, createdAt, updatedAt}`.
- **A review must be earned.** It is accepted only for a booking that belongs to the caller *and* is `completed`. A booking that is not theirs answers 404 `booking_not_found` — never 403, which would confirm the booking exists. Reviewing a session that has not happened answers 422.
- **One review per booking**, enforced by a unique index; a second submission answers 409 `review_exists`.
- **Editing is pending-only.** Once moderated, content is frozen (409 `already_moderated`) — otherwise a client could swap approved text for something nobody reviewed. Rejected reviews are frozen too, so a rewrite cannot fish for a second opinion on different text.
- Rating is 1–5; comment ≤ 2000 characters.
- **The public shape publishes the minimum**: first name only, no client id, no booking id, no state. A review is a person talking about their own health care, and full name + service + date is more identifying than the practice needs to publish.
- `summary` aggregates **every** approved review, not just the page shown, so "4.5 from 38 reviews" stays true; the average is rounded to one decimal.
- **Isolation**: a client touching another client's review, or a practitioner moderating a review on a booking that is not theirs, gets 404 `review_not_found`.
- When the database is not configured, all review routes answer 503 `service_unavailable`.

## Site content — CMS (BE-12)

Two surfaces, deliberately separate. `/v1/content/*` is **public** and can only ever return live content. `/v1/admin/content/*` is **practitioner-only** and sees everything, including drafts and the moderation queue.

### Public

| Method | Route | Query | Success | Errors |
|---|---|---|---|---|
| GET | `/v1/content/pages/{slug}` | — | 200 `{page}` | 404 `page_not_found`, 503 |
| GET | `/v1/content/posts` | `?category=&tag=` | 200 `{items: [post]}` — published only, newest first, **bodies omitted** | 503 |
| GET | `/v1/content/posts/{slug}` | — | 200 `{post}` — full body | 404 `post_not_found`, 503 |
| GET | `/v1/content/faqs` | `?category=` | 200 `{items: [faq]}` — active only, `sortOrder` ascending | 503 |
| GET | `/v1/content/testimonials` | — | 200 `{items: [{id, authorName, authorRole?, quote}]}` — approved only | 503 |

### Practitioner

| Method | Route | Success | Errors |
|---|---|---|---|
| GET / POST | `/v1/admin/content/pages` | 200 `{items: [page]}` / 201 `{page}` | 400, 401, 403, 409 `slug_taken` |
| GET / PATCH / DELETE | `/v1/admin/content/pages/{id}` | 200 `{page}` / 204 | 400, 401, 403, 404 `page_not_found`, 409 |
| POST | `/v1/admin/content/pages/{id}/publish` · `/unpublish` | 200 `{page}` | 401, 403, 404 |
| GET / POST | `/v1/admin/content/posts` | 200 `{items: [post]}` / 201 `{post}` | 400, 401, 403, 409 |
| GET / PATCH / DELETE | `/v1/admin/content/posts/{id}` | 200 `{post}` / 204 | 400, 401, 403, 404 `post_not_found`, 409 |
| POST | `/v1/admin/content/posts/{id}/publish` · `/unpublish` | 200 `{post}` | 401, 403, 404 |
| GET / POST | `/v1/admin/content/faqs` | 200 `{items: [faq]}` / 201 `{faq}` | 400, 401, 403 |
| PATCH / DELETE | `/v1/admin/content/faqs/{id}` | 200 `{faq}` / 204 | 400, 401, 403, 404 `faq_not_found` |
| GET / POST | `/v1/admin/content/testimonials` | 200 `{items: [testimonial]}` (`?status=pending\|approved\|rejected`) / 201 `{testimonial}` | 400, 401, 403 |
| PATCH / DELETE | `/v1/admin/content/testimonials/{id}` | 200 `{testimonial}` / 204 | 400, 401, 403, 404 `testimonial_not_found` |
| POST | `/v1/admin/content/testimonials/{id}/approve` · `/reject` | 200 `{testimonial}` | 401, 403, 404 |

- Shapes: `page` `{id, slug, title, body, metaTitle?, metaDescription?, status, publishedAt?, createdAt, updatedAt}`; `post` adds `{excerpt?, coverImage?, category?, tags[]}`; `faq` `{id, question, answer, category?, sortOrder, active, createdAt, updatedAt}`; `testimonial` (admin) `{id, authorName, authorRole?, quote, status, sortOrder, submittedAt, approvedAt?}`.
- **Nothing is public by default.** Pages and posts are created as `draft`; testimonials as `pending`. Publishing and approving are their own routes — a PATCH cannot change `status`, so an edit can never put content live by accident.
- **A draft is a 404, not a 403.** The existence of unreleased content is itself unreleased.
- `publishedAt` is stamped on first publish and never re-stamped, so unpublishing and re-publishing does not make an old article look new. Rejecting a testimonial is reversible.
- **Slugs are normalized** on write (lowercased, accents transliterated, spaces and separators collapsed to hyphens): "Séance à deux" → `seance-a-deux`. Reads normalize the incoming slug the same way, so a mis-cased URL still resolves. Slugs are unique per collection — a clash answers 409 `slug_taken`.
- `coverImage` accepts only `http(s)://` or site-relative (`/…`) URLs. `javascript:`/`data:` are rejected as `validation_error` — a stored one is an XSS payload waiting for a trusting renderer.
- Limits: title 200, excerpt/meta description 400, body 100 000, category 60, 10 tags of 40, question 300, answer 5000, quote 1000, author 120, URL 500.
- Lists are not paginated: a single practice's content is small.
- When the database is not configured, all `/v1/content*` and `/v1/admin/content*` routes answer 503 `service_unavailable`.

## Notifications (BE-09)

No HTTP surface — this slice reacts to events in the others. It is documented here because its behaviour is part of the contract the apps rely on.

| Event | Message | To | When |
|---|---|---|---|
| Booking created | `booking_confirmation` | client | immediately |
| Booking created | `session_reminder` | client | `startAt − 24h` (`REMINDER_LEAD`) |
| Booking rescheduled | `booking_rescheduled` | client | immediately; the old reminder is cancelled and a new one scheduled |
| Booking cancelled | `booking_cancelled` | client | immediately; the pending reminder is cancelled |
| Notes shared (first time only) | `feedback_shared` | client | immediately |
| Enquiry received | `enquiry_received` | practice inbox (`PRACTICE_EMAIL`) | immediately |

- **Outbox, not inline send.** Every message is written to `notification_jobs` first and delivered by a dispatcher that polls every `NOTIFICATION_POLL_INTERVAL` (default 1 min). A booking therefore never fails because the mail provider is unreachable, and a reminder survives a restart of the process that scheduled it.
- **Exactly-once delivery.** The dispatcher claims each due job with an atomic `findAndModify`, so two instances never send the same email. A claim that goes stale (a process died mid-send) is retried after a 5-minute lease.
- **Retries.** A failed send is retried 5 times with growing backoff (1m, 5m, 15m, 1h, 1h); after that the job is `failed` with the provider's reason recorded for a person to look at. A job that cannot be rendered at all fails on the first pass rather than retrying a certainty. One bad address never blocks the rest of the batch.
- **Short-notice bookings get no reminder.** A session booked inside the lead time would have its reminder arrive alongside the confirmation, so it is skipped.
- **Times are rendered per client.** Each message states the time in the booking's IANA timezone with the zone named; an unrecognised zone falls back to UTC rather than dropping the time.
- **Templates** are the brand files in `design/email/`, embedded in the API and kept byte-identical by a test. All substituted values are HTML-escaped — enquiry text is written by strangers.
- Without `RESEND_API_KEY` the outbox still records every message and each delivery attempt fails loudly, so a misconfigured deployment shows up as failed jobs rather than silence.

## Clients (BE-07)

| Method | Route | Auth | Body | Success | Errors |
|---|---|---|---|---|---|
| GET | `/v1/clients` | practitioner | — | 200 `{items: [clientSummary]}` — every client of the practice, ordered by `lastSessionAt` descending (nulls last), then `name` ascending | 401, 403 `forbidden`, 503 |
| GET | `/v1/clients/me` | client | — | 200 `{profile: clientMe}` — the caller's own profile | 401, 403 `forbidden`, 503 |
| GET | `/v1/clients/{id}` | practitioner | — | 200 `{record: clientRecord}` — the full practice-side record | 401, 403 `forbidden`, 404 `client_not_found`, 503 |
| PATCH | `/v1/clients/{id}` | practitioner | any subset of `{phone, practiceNotes, tags}` | 200 `{profile: clientProfile}` | 400 `validation_error`, 401, 403 `forbidden`, 404 `client_not_found`, 503 |

- **Practice membership:** a user becomes a client of the practice with their first booking (any status, including cancelled). List, record, and PATCH for any other user id answer 404 `client_not_found` — the same isolation rule as Bookings, no existence leak. `{id}` on `/v1/clients/{id}` is the client's **user id** (from the identity shape).
- `clientSummary` shape: `{id, name, email, phone?, tags, totalSessions, lastSessionAt?}` — `phone` appears only once set; `tags` is always an array (empty when none).
  - `totalSessions` counts the client's bookings with this practitioner that are **not** `cancelled` (`confirmed`, `completed`, `no_show`).
  - `lastSessionAt` is the latest `startAt` among those non-cancelled bookings; absent when the client has none (e.g. only cancelled bookings so far).
- `clientMe` shape: `{id, name, email, phone?, createdAt}` — `createdAt` is the account's creation time. Practice-side fields (`practiceNotes`, `tags`) are **never** part of the client's own view.
- `clientRecord` shape: `{id, name, email, phone?, tags, practiceNotes, profileCreatedAt?, profileUpdatedAt?, recentBookings: [booking], payments: {totalPaidKobo, totalRefundedKobo, paymentCount, currency?}, documentCount, formSubmissionCount}`.
  - `recentBookings` are the client's bookings with this practitioner, all statuses, ordered by `startAt` descending, capped at 10 — the full `booking` shape from Bookings.
  - `payments.totalPaidKobo` sums `success` payments; `totalRefundedKobo` sums `refunded`; `paymentCount` counts all payment records on those bookings; `currency` (ISO 4217) comes from the most recent payment and is absent when the client has none. Money is integer minor units per Conventions.
  - `documentCount` / `formSubmissionCount` count the client's uploaded documents and form submissions (full lists are served by the documents/forms slices).
  - `profileCreatedAt`/`profileUpdatedAt` appear once a practice profile exists (see PATCH); before the first PATCH the practice fields are their zero values (`tags: []`, `practiceNotes: ""`).
- **PATCH is practice-side only:** `phone` (max 40 chars), `practiceNotes` (max 5000 chars — the practitioner's private summary of the client), `tags` (max 20 entries, each 1–40 chars, trimmed, duplicates removed). Omitted fields keep their current values; send `""`/`[]` to clear. The first PATCH creates the practice profile (upsert keyed on the user id). Unknown fields are ignored. Clients cannot write any of it — the route is role-guarded.
- Not paginated: a single practice's client list is small; `items` is the full list, mirroring Bookings.
- When the database is not configured, all `/v1/clients*` routes answer 503 `service_unavailable`.

## Session Notes (BE-08)

| Method | Route | Auth | Body | Success | Errors |
|---|---|---|---|---|---|
| GET | `/v1/bookings/{id}/notes` | client (booking owner) or practitioner | — | 200 `{note}` — shape depends on role, see below | 401, 404 `booking_not_found`, 404 `note_not_found`, 503 |
| PUT | `/v1/bookings/{id}/notes` | practitioner | `{privateNotes, sharedFeedback, sharedResources}` | 200 `{note}` — full practitioner shape | 400 `validation_error`, 401, 403 `forbidden`, 404 `booking_not_found`, 503 |
| POST | `/v1/bookings/{id}/notes/share` | practitioner | — | 200 `{note}` — full practitioner shape, `sharedAt` now set | 401, 403 `forbidden`, 404 `booking_not_found`, 404 `note_not_found`, 503 |

- Practitioner `note` shape: `{id, bookingId, clientId, practitionerId, privateNotes, sharedFeedback, sharedResources, sharedAt?, createdAt, updatedAt}` — `sharedAt` appears only once set. `sharedResources` is an array of URL strings (aftercare links, exercises, reading).
- Client `note` shape (shared content only): `{bookingId, sharedFeedback, sharedResources, sharedAt}`.
- **Visibility rule:** a client sees a note only after the practitioner has shared it. Until `sharedAt` is set, the client's GET answers 404 `note_not_found` — indistinguishable from "no note exists" (no leak). `privateNotes` is **never** present in a client response, shared or not.
- **PUT upserts:** exactly one note exists per booking (unique index on `bookingId`). The first PUT creates it; later PUTs replace `privateNotes`, `sharedFeedback`, and `sharedResources` wholesale — `sharedAt` is untouched, so editing after sharing does not unshare or re-stamp. Validation: `privateNotes` max 10000 chars, `sharedFeedback` max 5000 chars, `sharedResources` max 20 entries of max 500 chars each.
- **Share is one-way and idempotent:** the first POST stamps `sharedAt` and makes the shared fields visible to the client; repeat POSTs answer 200 with the note unchanged. Sharing without a note answers 404 `note_not_found` — PUT first. There is no unshare. The post-share feedback email is the notifications slice's job (BE-09), triggered off the first share — this slice only performs the state transition.
- **Isolation:** a client touching another client's booking notes — or a practitioner touching notes on a booking that is not theirs — gets 404 `booking_not_found` (no existence leak), exactly as in Bookings.
- When the database is not configured, all `/v1/bookings/{id}/notes*` routes answer 503 `service_unavailable`.

## Operational health (LCH-09)

`GET /v1/admin/ops/health` — practitioner only.

It lives under `/v1/admin` rather than beside `/healthz` because it reports
how many accounts are locked, which is precisely the signal someone probing
the login would like confirmed. It is not a public status page.

```json
{
  "status": "degraded",
  "checked": "2026-08-12T09:00:00Z",
  "counters": {
    "notificationBacklog": 24,
    "notificationFailures": 0,
    "lockedAccounts": 1,
    "paymentVerificationFailures": 0
  },
  "alerts": [
    {
      "kind": "notification_backlog",
      "severity": "warning",
      "observed": 24,
      "threshold": 20,
      "since": "2026-08-12T08:41:00Z",
      "summary": "24 notifications are queued past their send time — clients are not being told about their sessions"
    }
  ]
}
```

`status` is one of `healthy`, `degraded`, `critical`, `unknown`.

**The HTTP status is always 200**, including when `status` is `critical`.
A monitor that treats 5xx as "the API is down" would page for a mail
backlog, and the API is not down. Alert on the `status` field.

**`unknown` is not `healthy`.** It means the counters could not be read.
Returning four zeroes in that case would be indistinguishable from a
perfectly healthy system, which is the one answer this must never give by
accident — so alert on `unknown` too.

Every alert is also emitted to the log as `ops alert` with structured
fields, so alerting can be driven from the log stream by anyone who never
polls this endpoint.

Thresholds (`internal/domain/ops`, `DefaultThresholds`):

| Condition | Warning | Critical | Window |
|---|---|---|---|
| `notification_backlog` (pending, past due) | 20 | 100 | now |
| `notification_failures` (retries exhausted) | 3 | 10 | 15m |
| `lockout_spike` (distinct accounts locked) | 5 | 20 | 15m |
| `payment_verification_failures` | 3 | 10 | 15m |

Setting a threshold to `0` disables that band. Turning off a check nobody
can act on is better than leaving one everybody learns to ignore.
