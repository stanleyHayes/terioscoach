# Practitioner handover runbook

For the person running the practice, not the person who built the software.
It assumes no technical background and explains *why* wherever the why
changes what you would do.

---

## 1. The two addresses

| What | Address | Who signs in |
|---|---|---|
| Your public website and client portal | `terioscoach.com` | Anyone; clients sign in for the portal |
| Your practice dashboard | `practice.terioscoach.com` | Only you |

They look different on purpose. Clients never see the dashboard, and there
is no link to it from the public site — if you want it on your phone, add a
bookmark to the home screen.

---

## 2. Your day, in the dashboard

**Overview** is the first thing you see: today's sessions, anything waiting
on you, and how the month is going.

**Calendar** is the week. A session is a block; tap it to open it. From
there you start a video session, mark it complete, mark a no-show, or
cancel it.

Three things about the calendar are worth knowing:

- **You cannot complete a session before it has finished.** The dashboard
  will refuse. This is deliberate: "completed" is what makes a session
  reviewable and countable as income, and a session marked complete at
  9am for a 3pm appointment would make both wrong.
- **Cancelling frees the slot immediately**, and the client is emailed.
- **A no-show is not a cancellation.** Both free the slot; only a no-show
  keeps the session in your records as one that was booked and not
  attended, which is what you want when deciding your policy later.

---

## 3. When you are away

**Availability** is where your working week lives.

- **Weekly hours** — the times you are open, per day. A day with no hours
  is a day nobody can book.
- **Buffer** — minutes kept clear after each session, per day. Set it to
  15 and a 60-minute session occupies 75 in the calendar. Clients never
  see the buffer; they just never get offered a slot that runs into it.
- **Time off** — a date range that blocks bookings without changing your
  weekly hours. Use this for holidays. Do not empty your weekly hours to
  go away for a fortnight: you will have to reconstruct them afterwards
  and will get it slightly wrong.

Changing your hours never moves a session that is already booked. If you
need one moved, move it in the calendar and the client is emailed.

---

## 4. Money

**Payments** lists every payment with its status.

- **Pending** — the client started a payment and has not finished it.
  These usually resolve themselves within a few minutes; a pending payment
  from yesterday means they abandoned the checkout.
- **Successful** — the money is with Paystack and will be settled to your
  account on Paystack's own schedule. This platform never holds your money.
- **Refunded** — you refunded it from this screen.

**Refunds are not reversible.** The button asks you to confirm because
that is the last chance to change your mind. A refund does not cancel the
session; if you meant to do both, cancel it in the calendar too.

If a client says they paid and the dashboard disagrees, check Paystack's
own dashboard before refunding anything. The two should always agree — if
they ever do not, that is worth reporting rather than working around.

---

## 5. Clients and their records

**Clients** is everyone who has ever booked.

Open a client and you see their sessions, their payments, their documents,
their signed forms, and your notes. This is their health record. Two rules
govern everything on that screen:

1. **Only you can see it.** No other client can, whatever they try.
2. **What the client sees is only what you have shared.**

That second rule is the one to internalise, because it is what makes the
notes safe to use properly.

### Notes: private vs shared

Every session note has two halves.

- **Private notes** are yours. Clinical observations, things to watch,
  anything you would write for yourself. **The client never sees these**,
  and there is no button anywhere that would show them.
- **Shared feedback** is written *to* the client, along with any home care
  you want to give them.

You write both, then decide separately whether to share. Nothing is shared
until you press Share.

**Sharing is one-way.** There is no unshare, because unsharing something a
client has already read is a fiction — they have read it. Write the shared
half as if it will be read the moment you press the button, because it
will: the client is emailed when you share.

---

## 6. Forms and consent

**Forms** has two tabs.

**Forms** is where you build them. A form is a list of questions; a
question can be short text, long text, a number, a date, a pick-one, a
pick-any, or a signature. Add a signature field to anything that needs
consenting to.

Send a form to a client with **Send to a client**. It appears in their
portal the next time they sign in.

**Responses** is what has come back. Open one to read it.

A few things are deliberately rigid here, because this is consent:

- **A submitted form cannot be edited.** Not by the client, not by you.
  A record that can be changed after signing is not a record of anything.
- **Editing a form does not change forms already signed.** They keep the
  wording they were signed under. You will see a warning when you edit a
  form somebody has already signed.
- Every signed form shows a line confirming the record **matches its
  signature**. If it ever says otherwise, do not rely on that record and
  report it — it means the stored record no longer matches what was signed.

---

## 7. Video sessions

Start from the calendar: open the session, press **Start video session**.

- The room **opens 10 minutes before** the appointment and closes 15
  minutes after it ends. Earlier than that and even you cannot get in —
  which is what stops a client wandering into the room during someone
  else's session.
- **Only the two of you can join.** Nobody else, with any link.
- Your browser will ask for the camera and microphone the first time.
- **Chrome, Edge or Safari.** Firefox works but is less reliable for this.
- Closing the tab ends your side of the call and releases the camera.

In the room you can:

- **Mute** and **turn the camera off** at any time; the other side sees a
  small indicator, never a frozen frame they have to guess about.
- **Share your screen** — the camera comes back by itself when you stop.
- **Send chat messages** — they stay between the two of you and disappear
  when the session ends; nothing is stored.
- **Raise a hand** or send a quick **reaction**.
- **Record the session** — the file downloads to *your* computer when you
  stop, and the client always sees a red **● Rec** light while it runs.
  Tell them before you press it.
- Switch **microphone or camera** from the settings button without leaving
  the call.
- Turn on **captions** (Chrome only) — each side's speech is transcribed
  on its own computer and shown to the other.
- A dropped connection **reconnects itself**; you do not need to rejoin.

If the video fails to connect, the usual cause is the client's network
rather than yours. Ask them to try a phone on mobile data — that almost
always works and tells you where the problem is.

---

## 8. Your website

**Content** has four tabs, and one rule that runs through all of them:
**saving is not publishing**.

- **Pages** — your standing pages. Write, save as often as you like,
  publish when ready. A live page stays exactly as it was until you save,
  and unpublishing takes it down immediately.
- **Blog** — same, plus a cover image, an excerpt and tags.
- **FAQs** — no draft/live pair; a question is either showing or hidden,
  and you set the order.
- **Testimonials** — nothing appears on the site until you approve it,
  **including ones you type in yourself**. Approving publishes it;
  rejecting takes it down. Both are reversible.

**Reviews** is the same idea for reviews clients leave after a session.
Nothing is public until you publish it.

The reason for all of this is that your website is yours. Nothing a client
writes appears on it on their say-so.

---

## 9. Enquiries

**Enquiries** is the contact form. Each one is emailed to you as it
arrives, and kept here so nothing is lost in an inbox. Opening one marks it
read. Reply from your own email — this is an inbox, not a mail client.

---

## 10. Reports

**Reports** covers a date range you choose: sessions completed, income,
cancellations, no-shows, new clients, and your review average.

Income counts **successful payments**, not bookings. A booked and unpaid
session is not income and is not counted as such.

---

## 11. When something is wrong

| What you see | What it usually means | What to do |
|---|---|---|
| "Your session has expired" | You have been signed out for security | Sign in again |
| A slot a client says was free is refused | Somebody booked it seconds earlier | Offer the next one; nothing is broken |
| A client cannot see shared feedback | It was saved but not shared | Open the session, press Share |
| Video will not connect | Usually the client's network | Ask them to try mobile data |
| Payment shows pending for hours | Abandoned checkout | Ask them to pay again from their portal |
| The dashboard will not load at all | The API or the site is down | Check the status page, then report it |

**Report anything you cannot explain.** In particular: a signed form
reporting that it does not match its signature, a client seeing something
you did not share, or the dashboard and Paystack disagreeing about money.
Those three are not "quirks" and should never be worked around.

---

## 12. Keeping the account safe

- Your dashboard password is the key to every client record you hold. Use
  a long one you use nowhere else, and store it in a password manager.
- Five wrong password attempts locks sign-in for 15 minutes. That is the
  system defending your clients' records; wait it out.
- Signing out on a shared or public computer is not optional.
- **Nobody legitimate will ever ask you for your password**, including
  whoever supports this platform. Support never needs it.

---

## 13. What runs on its own

You do not have to do any of this; it is here so nothing is a surprise.

- **Booking confirmations** go out the moment a client books.
- **Session reminders** go out 24 hours before, automatically. A booking
  made inside 24 hours gets a confirmation and no reminder — there is no
  time for one.
- **Reschedule and cancellation notices** go out when you or the client
  changes a session.
- **Shared feedback** emails the client when you press Share.
- **New enquiries** email you.

If a client says they did not get an email, check their spam folder first —
it is nearly always that.

---

## 14. Backups

The database is backed up daily and can be restored to any point in the
retention window. You do not need to do anything. Ask for a restore drill
to be demonstrated once, so that it has been proved rather than assumed —
a backup nobody has restored from is a hope, not a backup.
