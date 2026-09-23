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

## September 2026 durable workflow update

The verification counts above describe the September 17 revision, **not this update**. Current implementation has compilation/static checks only; test execution, real database checks, browser journeys and inbox delivery are reserved for the owner.

Covered Mongo business writes now commit an allowlisted metadata event to `workflow_events` in the **same transaction**. Failed journal insertion rolls back the business write; provider or PDF failures happen later and cannot roll back a committed booking/signature. No historical events are bulk replayed on deployment. Startup rejects standalone Mongo; Atlas, a replica set or sharded cluster is required.

| Mutation / implementation | Client | Practitioner | Destination / routing |
|---|---|---|---|
| Booking create, pending payment | Feed + email | Feed | Exact pending booking on Payments; assigned practitioner calendar detail |
| Confirmation / reschedule | Feed + email | Feed + email | Session preparation / exact calendar booking; old and new times on reschedule |
| Cancellation / unpaid expiry | Feed + email | Feed + email | Session / calendar; pending requests never described as confirmed |
| Change request saved on booking | Feed receipt | Feed + email | Exact calendar booking displays request type, proposed time and reason; request reason is excluded from email/journal |
| Participant declaration revised | Feed + signature-request email | Feed | Required documents for that booking |
| Agreement wording / required service document assignment changed | Feed + signature-request email | Feed receipt | Relevant future booking's required documents |
| Contextual agreement / SOW / guardian execution inserted | Feed receipt | Feed + email | Exact execution in portal Documents / practitioner client record |
| Practitioner countersignature saved | Feed + email | Feed receipt | Exact executed document |
| Form assigned / submitted | Assignment email / submission receipt, both feed | Assignment receipt / submission email, both feed | Exact form submission; booking practitioner wins, otherwise recorded assigning practitioner |
| Shared document inserted / updated / hidden / deleted | Feed while shared or previously shared | Feed receipt | Exact document, or Documents list after withdrawal; private documents produce no client event |
| Session notes shared / recording stored | Feed | Feed receipt | Session/client record; no clinical text or recording content copied into notices |
| Payment pending / failed / refunded | Feed + email | Feed; material failure/refund email | Exact payment row; booking practitioner |
| Reminder due | Feed + email | Feed + email | Exact role-appropriate room URL; booking reread before delivery |
| Session completed / no-show | Feed | Feed | Session/client record |
| Review created / edited / moderated | Feed | Feed; email on new submission | Reviews; comments excluded from journal |
| Enquiry created | None to unverified sender | Feed + email | Enquiries; no enquiry message copied into journal |

Document rejection/revision-request actions and explicit request-decline actions have no current domain/API operation; this update does not invent them. Existing reschedule/cancel actions resolve the persisted change request. Form templates and non-booking practice work use the existing practice-wide fallback only when no assigned practitioner exists. Guardian emails are contact declarations, never notification recipients; notices go to authenticated account holders.

Recovery and delivery:

- `workflow_events` records pending/due/attempts, queued, or dead-letter status. The worker retries event fan-out and signature archival independently of email; partial success is safe to replay. Ten failed fan-out attempts dead-letter the event with a generic error. Job retry policy remains five delivery attempts (1m, 5m, 15m, 1h backoffs).
- A unique hash of event identity, recipient and kind deduplicates channel jobs. The existing event/recipient inbox index preserves read state. New committed transitions have new event IDs; replaying the same transition does not create new mail jobs or reset unread state.
- Claimed jobs have expiring leases and claim tokens. Before provider sending, the exact rendered payload is frozen without releasing the lease. Reminders resolve the user's saved zone at first delivery, retain `renderedTimezone`, and re-use the frozen payload on retries. Cancelled, moved and expired sessions suppress obsolete reminders; replacement reminders use committed UTC starts.
- Resend receives `Idempotency-Key: notification-<jobId>`. Provider acceptance before local acknowledgement is still a retry window. Resend documents a **24-hour** idempotency window; this is not an exactly-once promise. Inspect provider delivery history before operator replay outside that window. [Resend idempotency documentation](https://resend.com/docs/dashboard/emails/idempotency-keys).
- Worker default is now 15 seconds (`NOTIFICATION_POLL_INTERVAL` remains configurable); both existing inboxes poll at 30 seconds and on open/focus. Healthy-operation visibility includes both worker and UI polling delays. Mailbox receipt, lag targets and provider delivery are unmeasured until the owner runs acceptance checks.
- Event logs carry correlation IDs and failure counts, without signed answers or private notes. Delivery jobs retain attempts/status/error; `workflowctl` reports pending/dead-letter/failed counts. See the implementation runbook for single-ID replay and read-only audit commands.

New coverage files include `notifications/workflow_test.go` (queue outage, replay/read-state preservation, recipient zones, cancellation, archive outage, provider acceptance/local-ack loss) and `mongodb/workflow_repository_test.go` (privacy allowlist and a disposable-replica-set atomicity test). These tests have been written and compiled, **not executed**.
