// Package email is the outbound adapter that turns queued notification
// jobs into rendered brand messages. It owns the templates and the subject
// lines; the mail provider (Resend) is a separate adapter that only ever
// sees final HTML.
//
// The templates are the brand-approved files from design/email, embedded
// here byte-for-byte. templates_sync_test.go fails if the two copies ever
// diverge, so the design source stays the source of truth.
package email

import (
	"embed"
	"fmt"
	"html"
	"strings"
	"time"

	"github.com/xcreativs/terios/api/internal/domain/notification"
	"github.com/xcreativs/terios/api/internal/ports"
)

//go:embed templates/*.html
var templateFS embed.FS

// templateFiles maps each message kind to its brand template.
var templateFiles = map[notification.Kind]string{
	notification.KindBookingPaymentRequired: "templates/booking-payment-required.html",
	notification.KindBookingConfirmation:    "templates/booking-confirmation.html",
	notification.KindSessionReminder:        "templates/session-reminder.html",
	notification.KindBookingRescheduled:     "templates/booking-rescheduled.html",
	notification.KindBookingCancelled:       "templates/booking-cancelled.html",
	notification.KindBookingChangeRequested: "templates/booking-change-requested.html",
	notification.KindFeedbackShared:         "templates/feedback-shared.html",
	notification.KindEnquiryReceived:        "templates/enquiry-notification.html",
	notification.KindAgreementSigned:        "templates/agreement-signed.html",
	notification.KindFormAssigned:           "templates/form-assigned.html",
	notification.KindFormSubmitted:          "templates/form-submitted.html",
}

// subjects are the subject lines, in the brand voice. The reminder's is
// completed with the job's own timeUntil value.
var subjects = map[notification.Kind]string{
	notification.KindBookingPaymentRequired: "Payment required to confirm your session",
	notification.KindBookingConfirmation:    "Your session is confirmed",
	notification.KindSessionReminder:        "Reminder: your session is coming up",
	notification.KindBookingRescheduled:     "Your session has been rescheduled",
	notification.KindBookingCancelled:       "Your session has been cancelled",
	notification.KindBookingChangeRequested: "Client requested a session change",
	notification.KindFeedbackShared:         "Notes and resources from your session",
	notification.KindEnquiryReceived:        "New enquiry from your website",
	notification.KindAgreementSigned:        "A client signed a service agreement",
	notification.KindFormAssigned:           "A form is ready for you",
	notification.KindFormSubmitted:          "A client submitted a form",
}

// ctaLinks names the data key each template's call-to-action points at, so
// the plain-text alternative carries the same destination as the button.
// A mail client that refuses to render our HTML still has a usable link.
var ctaLinks = map[notification.Kind]string{
	notification.KindBookingPaymentRequired: "paymentUrl",
	notification.KindBookingConfirmation:    "manageUrl",
	notification.KindSessionReminder:        "joinUrl",
	notification.KindBookingRescheduled:     "manageUrl",
	notification.KindBookingCancelled:       "bookUrl",
	notification.KindBookingChangeRequested: "dashboardUrl",
	notification.KindFeedbackShared:         "portalUrl",
	notification.KindEnquiryReceived:        "dashboardUrl",
	notification.KindAgreementSigned:        "dashboardUrl",
	notification.KindFormAssigned:           "portalUrl",
	notification.KindFormSubmitted:          "dashboardUrl",
}

// Renderer produces brand messages from jobs.
type Renderer struct {
	// defaults are merged under every job's data, so brand-wide values
	// (year, URLs) need not be stamped onto each queued job.
	defaults map[string]string
}

var _ ports.EmailRenderer = (*Renderer)(nil)

// Options are the deployment-specific values the templates link to.
type Options struct {
	// PortalURL is the client portal root, e.g. https://terioscoach.com/portal.
	PortalURL string
	// DashboardURL is the practice dashboard root.
	DashboardURL string
	// Now supplies the copyright year; nil means the real clock.
	Now func() time.Time
}

// NewRenderer builds a renderer for one deployment.
func NewRenderer(opts Options) *Renderer {
	now := opts.Now
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	portal := strings.TrimRight(opts.PortalURL, "/")
	dashboard := strings.TrimRight(opts.DashboardURL, "/")
	return &Renderer{defaults: map[string]string{
		"year":              fmt.Sprintf("%d", now().Year()),
		"portalUrl":         portal,
		"manageUrl":         portal + "/sessions",
		"joinUrl":           portal + "/sessions",
		"bookUrl":           portal + "/book",
		"dashboardUrl":      dashboard,
		"rescheduleCutoff":  "48 hours",
		"joinWindowMinutes": "10",
	}}
}

// Render turns a job into its final message. Unknown kinds are a
// programming error rather than a runtime condition, so they surface as
// notification.ErrTemplateNotFound rather than an empty email.
func (r *Renderer) Render(job notification.Job) (ports.EmailMessage, error) {
	if job.Kind == notification.KindActionRequired {
		base := r.defaults["portalUrl"]
		path := job.Data["link"]
		if job.Data["audience"] == "practitioner" {
			base = r.defaults["dashboardUrl"]
		} else {
			path = strings.TrimPrefix(path, "/portal")
		}
		if !strings.HasPrefix(path, "/") || strings.HasPrefix(path, "//") {
			return ports.EmailMessage{}, fmt.Errorf("invalid action link")
		}
		link := base + path
		title := job.Data["title"]
		body := job.Data["body"]
		return ports.EmailMessage{To: job.Recipient, Subject: title, Text: title + "\n" + body + "\n" + link, HTML: `<!doctype html><html><body style="font-family:Arial,sans-serif;background:#FAF7F2;color:#1F2922;padding:32px"><main style="max-width:560px;margin:auto"><p>Terios Wellness Spa</p><h1>` + html.EscapeString(title) + `</h1><p>` + html.EscapeString(body) + `</p><p><a href="` + html.EscapeString(link) + `">Review in your account</a></p></main></body></html>`}, nil
	}
	file, ok := templateFiles[job.Kind]
	if !ok {
		return ports.EmailMessage{}, fmt.Errorf("%w: %s", notification.ErrTemplateNotFound, job.Kind)
	}
	if job.Kind == notification.KindAgreementSigned && job.Data["action"] == "submitted" {
		file = "templates/statement-of-work-submitted.html"
	}
	raw, err := templateFS.ReadFile(file)
	if err != nil {
		return ports.EmailMessage{}, fmt.Errorf("read template %s: %w", file, err)
	}

	data := make(map[string]string, len(r.defaults)+len(job.Data))
	for k, v := range r.defaults {
		data[k] = v
	}
	// Job data wins: a value resolved when the message was queued is more
	// specific than a deployment-wide default.
	for k, v := range job.Data {
		data[k] = v
	}

	if job.Kind == notification.KindSessionReminder && job.BookingID != "" {
		data["joinUrl"] = r.defaults["portalUrl"] + "/sessions/" + job.BookingID + "/room"
		if job.Data["audience"] == "practitioner" {
			data["joinUrl"] = r.defaults["dashboardUrl"] + "/sessions/" + job.BookingID + "/room"
		}
	}
	return ports.EmailMessage{
		To:      job.Recipient,
		Subject: subject(job, data),
		HTML:    render(string(raw), data),
		Text:    plainText(job, data),
	}, nil
}

// subject picks the line for the kind, personalising the reminder with the
// same phrase the body uses.
func subject(job notification.Job, data map[string]string) string {
	if job.Kind == notification.KindSessionReminder {
		if until := data["timeUntil"]; until != "" {
			return "Reminder: your session is " + until
		}
	}
	if job.Kind == notification.KindEnquiryReceived {
		if name := data["senderName"]; name != "" {
			return "New enquiry from " + name
		}
	}
	if job.Kind == notification.KindAgreementSigned {
		if name := data["clientName"]; name != "" {
			action := "signed"
			if data["action"] == "submitted" {
				action = "submitted"
			}
			return name + " " + action + " the " + data["agreementTitle"]
		}
	}
	if job.Kind == notification.KindFormAssigned {
		if title := data["formTitle"]; title != "" {
			return "Please complete " + title
		}
	}
	if job.Kind == notification.KindFormSubmitted {
		if name := data["clientName"]; name != "" {
			return name + " submitted " + data["formTitle"]
		}
	}
	if job.Kind == notification.KindBookingChangeRequested {
		if name := data["clientName"]; name != "" {
			return name + " requested a session " + data["requestType"]
		}
	}
	if line, ok := subjects[job.Kind]; ok {
		return line
	}
	return "Terios Wellness Spa"
}

// plainText is the text alternative. It is short on purpose: its job is to
// stay readable in a client that refuses HTML, not to mirror the design.
func plainText(job notification.Job, data map[string]string) string {
	var b strings.Builder
	b.WriteString(subject(job, data))
	b.WriteString("\n\n")
	if name := data["clientName"]; name != "" {
		fmt.Fprintf(&b, "Hello %s,\n\n", name)
	}
	if service := data["serviceName"]; service != "" {
		fmt.Fprintf(&b, "Service: %s\n", service)
	}
	for label, key := range map[string]string{
		"Date and time": "startTime",
		"Previous time": "oldStartTime",
		"New time":      "newStartTime",
	} {
		if value := data[key]; value != "" {
			fmt.Fprintf(&b, "%s: %s (%s)\n", label, value, data["timezone"])
		}
	}
	if link := data["paymentUrl"]; link != "" {
		fmt.Fprintf(&b, "\nYour appointment is not booked yet. It will only be confirmed and placed on the practice calendar after payment is successful.\n\nComplete payment:\n%s\n", link)
	}
	if message := data["message"]; message != "" {
		fmt.Fprintf(&b, "\n%s\n", message)
	}
	if link := data[ctaLinks[job.Kind]]; link != "" && link != data["paymentUrl"] {
		fmt.Fprintf(&b, "\n%s\n", link)
	}
	b.WriteString("\nTerios Wellness Spa")
	return b.String()
}

// render fills a brand template. The templates use a deliberately small
// Mustache-style syntax so the design files stay plain HTML with no Go
// template syntax in them:
//
//	{{key}}            — the value, HTML-escaped
//	{{#key}}...{{/key}} — the block, only when key has a non-empty value
//
// Every substituted value is escaped: client names and enquiry messages are
// attacker-controlled text arriving in an HTML document.
func render(template string, data map[string]string) string {
	out := renderSections(template, data)

	var b strings.Builder
	rest := out
	for {
		start := strings.Index(rest, "{{")
		if start < 0 {
			b.WriteString(rest)
			break
		}
		end := strings.Index(rest[start:], "}}")
		if end < 0 {
			b.WriteString(rest)
			break
		}
		end += start
		b.WriteString(rest[:start])
		key := strings.TrimSpace(rest[start+2 : end])
		b.WriteString(html.EscapeString(data[key]))
		rest = rest[end+2:]
	}
	return b.String()
}

// renderSections resolves {{#key}}…{{/key}} blocks before placeholders, so
// a dropped block's placeholders are never substituted.
func renderSections(template string, data map[string]string) string {
	for {
		open := strings.Index(template, "{{#")
		if open < 0 {
			return template
		}
		nameEnd := strings.Index(template[open:], "}}")
		if nameEnd < 0 {
			return template
		}
		nameEnd += open
		key := strings.TrimSpace(template[open+3 : nameEnd])

		closeTag := "{{/" + key + "}}"
		closeAt := strings.Index(template[nameEnd:], closeTag)
		if closeAt < 0 {
			// Unbalanced section: drop the opening tag rather than
			// emitting it into the message.
			template = template[:open] + template[nameEnd+2:]
			continue
		}
		closeAt += nameEnd

		body := template[nameEnd+2 : closeAt]
		if data[key] == "" {
			body = ""
		}
		template = template[:open] + body + template[closeAt+len(closeTag):]
	}
}
