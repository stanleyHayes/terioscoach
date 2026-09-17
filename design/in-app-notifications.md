# In-app notifications

Completes `Terios updates (1).pdf`, In-app Notification 1. The partial notification code was found uncommitted in the main worktree; the six adjacent agent worktrees were clean when inspected. Both apps previously derived temporary action lists from bookings, forms, payments and enquiries. Both now read persisted events.

## Event coverage

| Event | Client portal | Practitioner app |
|---|---|---|
| Booking created, awaiting payment | Checkout/payment-required event | New pending session |
| Payment needed | Payment link/update | Pending booking above |
| Booking confirmed, including verified paid booking | Confirmation | Confirmation |
| Session reminder | At its due time | At its due time |
| Reschedule/cancel request | Request acknowledgement | Request notification |
| Session rescheduled/cancelled | Changed session | Changed session |
| Completed session / no-show | Updated status | Updated status |
| Refund, including paid-slot conflict compensation | Payment update | Payment update |
| Session feedback/resources shared | Shared feedback | Sharing acknowledgement |
| Form assigned | Link to the specific form | Assignment notification |
| Form submitted | Submission acknowledgement | Submission notification |
| Agreement signed / Statement of Work submitted | Completion acknowledgement | Document notification |
| Agreement countersigned | Countersignature notification | Countersignature notification |
| Document shared, changed, hidden or deleted | Update only if shared or previously shared | Client-record link |
| Recording stored | Recording ready | Client-record link |
| Review submitted/edited | Review acknowledgement | Review notification |
| Review moderated | Review status update | Review status update |
| Website enquiry received | Not sent to an unverified visitor | Enquiry notification |

Notifications record relevant persisted workflow changes. Routine navigation, downloads, private clinical-note edits and live room chat are not copied into the inbox. Private documents are not exposed to clients. Existing records are not backfilled as newly occurring events.

## Persistence and access

- `GET /v1/notifications` returns the latest 50 records and a count of all unread entries. Refresh on opening, focus, and every 30 seconds.
- Individual and bulk read endpoints persist read state. Read entries stay in history. Every query and mutation uses the authenticated account's email, never a recipient supplied by the caller.
- Practice email aliases resolve to the actual practitioner account. Without an email routing setting, the single-practitioner account supplies the address. Staff accounts do not inherit the owner's private notification feed.
- Immediate events appear after outbox creation, before email delivery. Scheduled reminders appear only when due; cancelled reminders remain cancelled.
- A unique event/recipient index makes queue/dispatch retries idempotent without resetting read state. In-app-only activities use durable outbox jobs, never send extra email, and retry failed feed persistence.
- New indexes are installed by the existing API startup index initialization.

## Verification

- Full API `go test ./...` and `go vet ./...` passed.
- Admin: 472 tests passed. Portal: 302 tests passed. Both production builds passed.
- Notification components, tests and API clients pass targeted ESLint.
- Chromium against both production builds, using isolated API fixtures: 1440px and 390px, read/reload persistence, mark-all, failed read recovery, Escape, viewport containment and no page errors. Fixtures did not read or mutate production client records.
- Full-app lint is not clean on the starting main revision: admin `ServiceForm.tsx` native checkbox, both `auth.tsx` render-time clocks, portal `AgreementStep.tsx` synchronous effect state. These unrelated files were not changed by this notification work. Existing warnings also remain.
- Actual MongoDB production delivery and deployment are not claimed verified by fixture tests.
