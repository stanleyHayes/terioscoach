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

## Health

| GET | `/healthz` | 200 `{status:"ok"}` (liveness) |
| GET | `/readyz` | 200 `{status:"ready"}` / 503 when a dependency is down |

---

<!-- Append new slices below as tasks define them: services (BE-03), availability (BE-04),
     booking (BE-05), payments (BE-06), clients/notes/forms/docs (BE-07/08/10/11),
     cms (BE-12), enquiries (BE-13), reviews (BE-14), reporting (BE-15), signaling (CX-01). -->

## Services (BE-03)

| Method | Route | Auth | Body | Success | Errors |
|---|---|---|---|---|---|
| GET | `/v1/services` | public | — | 200 `{items: [service]}` — active, non-deleted, ordered by `sortOrder` then `createdAt` | 503 `service_unavailable` |
| GET | `/v1/services/all` | practitioner | — | 200 `{items: [service]}` — all non-deleted incl. inactive, same ordering | 401, 403 `forbidden`, 503 |
| POST | `/v1/services` | practitioner | `{name, description, durationMinutes, priceKobo, currency?, sortOrder?}` | 201 `{service}` | 400 `validation_error`, 401, 403 |
| PATCH | `/v1/services/{id}` | practitioner | any subset of `{name, description, durationMinutes, priceKobo, currency, active, sortOrder}` | 200 `{service}` | 400 `validation_error`, 401, 403, 404 `service_not_found` |
| DELETE | `/v1/services/{id}` | practitioner | — | 204 | 401, 403, 404 `service_not_found` |

- `service` shape: `{id, practitionerId, name, description, durationMinutes, priceKobo, currency, active, sortOrder, createdAt, updatedAt}` — IDs and timestamps per Conventions.
- `priceKobo` is integer minor units, `>= 0`. `currency` is ISO 4217, defaults to `"GHS"` when omitted on create.
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
- **Statuses:** `confirmed` (the only non-terminal state; blocks the slot), `cancelled`, `completed`, `no_show` (terminal). Transitions: `confirmed → cancelled | completed | no_show`; nothing leaves a terminal state — attempting one answers 409 `invalid_status`.
- **Slot rule (create and reschedule):** `startAt` (RFC 3339 UTC) must exactly match a slot the availability engine (BE-04) would return for that service on that day — same step (`durationMinutes + bufferMinutes`), windows, time-off, and busy-interval rules — and must be in the future. Anything else answers 409 `slot_unavailable`. An unknown or inactive `serviceId` answers 404 `service_not_found`, mirroring `/v1/availability/slots`.
- **Double-booking guard:** a partial unique index on `{practitionerId, startAt}` for `status = "confirmed"` makes a confirmed double-book physically impossible; the loser of a concurrent create/reschedule race answers 409 `slot_unavailable`. Only `confirmed` bookings block slots; cancelling frees the slot immediately, rescheduling frees the old slot as it claims the new one.
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
| POST | `/v1/webhooks/paystack` | **public** (signature-verified) | raw Paystack event JSON | 200 `{}` — for `charge.success` and for every ignored/duplicate event | 401 `invalid_signature`, 503 |
| GET | `/v1/payments/mine` | client | — | 200 `{items: [payment]}` — own payments, ordered by `createdAt` descending | 401, 403 `forbidden`, 503 |
| GET | `/v1/payments` | practitioner | `?from=<RFC 3339>&to=<RFC 3339>` on `createdAt` — both optional | 200 `{items: [payment]}` — payments on the practitioner's bookings, ordered by `createdAt` descending | 400 `validation_error`, 401, 403 `forbidden`, 503 |
| POST | `/v1/payments/{id}/refund` | practitioner | — | 200 `{payment}` — status `refunded` | 401, 403 `forbidden`, 404 `payment_not_found`, 409 `invalid_status`, 502 `payment_gateway_error`, 503 |

- `payment` shape: `{id, bookingId, clientId, amountKobo, currency, status, paystackReference, channel?, paidAt?, createdAt}` — IDs, integer minor units, and RFC 3339 UTC timestamps per Conventions. `channel` (e.g. `card`, `mobile_money`) and `paidAt` appear only once the charge succeeds.
- **Statuses:** `pending` (initialized, awaiting checkout), `success` (webhook-confirmed), `failed`, `refunded` (terminal). Transitions: `pending → success | failed`; `failed → pending` (re-initialize); `success → refunded`. Nothing leaves `refunded`.
- **Hosted checkout only:** card and mobile-money details never touch this API. `initialize` returns Paystack's `authorizationUrl`; the client completes payment on Paystack's hosted page and Paystack calls the webhook.
- **Amount/currency/email are server-derived**, never client-supplied: amount from the booked service's current `priceKobo`, currency from the service, email from the client's account. The `reference` is server-generated, stored on the payment, and sent to Paystack so webhook deliveries join back exactly.
- **Initialize guards:** the booking must belong to the caller (cross-owner access answers 404 `booking_not_found` — isolation, as in Bookings) and must still be `confirmed` (cancelled/completed bookings answer 409 `invalid_status`). A booking whose payment already succeeded answers 409 `already_paid`. Re-initializing a `pending`/`failed` payment is allowed (abandoned checkout): the same payment record is kept and its `paystackReference` is replaced — one payment record per booking, enforced by a unique index on `bookingId`.
- **Webhook verification:** requests must carry a valid `x-paystack-signature` header — hex HMAC-SHA512 of the raw request body under `PAYSTACK_SECRET_KEY`, compared in constant time. Anything else answers 401 `invalid_signature`. The raw body is hashed before any JSON parsing.
- **Webhook handling:** only `charge.success` mutates state, and only after a server-side `GET /transaction/verify/{reference}` confirms `status=success` with amount and currency matching the stored payment. All other events, unknown references, already-`success` payments, and failed verify checks answer 200 without changes — **repeat deliveries are safe (idempotent on `paystackReference`)**. A transient gateway failure during verify answers 502 so Paystack retries.
- **Where payment state lives:** the `payments` collection is the source of truth. On `charge.success` the booking is also stamped with a denormalized `paymentStatus` (`"paid"`) + `paidAt`, and on refund with `paymentStatus: "refunded"` — purely so booking lists can display payment state without a join. These are additive fields: the booking lifecycle statuses (`confirmed`/`cancelled`/`completed`/`no_show`) are unchanged, and `paymentStatus`/`paidAt` appear on the booking shape only once set.
- **Refund:** practitioner-only, and only from `success` — refunding a `pending`/`failed`/`refunded` payment answers 409 `invalid_status`. Calls Paystack's refund API with the stored reference; on success the payment becomes `refunded` and the booking's `paymentStatus` follows.
- **Isolation:** a practitioner touching a payment whose booking is not theirs gets 404 `payment_not_found`. Lists are not paginated (small, single-practice volume), mirroring Bookings.
- When the database **or** Paystack is not configured (`PAYSTACK_SECRET_KEY` unset), all `/v1/payments*` **and** `/v1/webhooks/paystack` routes answer 503 `service_unavailable`.
