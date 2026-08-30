package email

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/xcreativs/terios/api/internal/domain/notification"
)

func testRenderer() *Renderer {
	return NewRenderer(Options{
		PortalURL:    "https://terioscoach.com/portal/",
		DashboardURL: "https://practice.terioscoach.com",
		Now:          func() time.Time { return time.Date(2026, 8, 11, 9, 0, 0, 0, time.UTC) },
	})
}

func job(kind notification.Kind, data map[string]string) notification.Job {
	return notification.Job{Kind: kind, Recipient: "ama@example.com", Data: data}
}

// TestRendersEveryKind: every message kind the domain defines has a
// template, a subject, and a body that actually got filled in.
func TestRendersEveryKind(t *testing.T) {
	kinds := []notification.Kind{
		notification.KindBookingConfirmation,
		notification.KindSessionReminder,
		notification.KindBookingRescheduled,
		notification.KindBookingCancelled,
		notification.KindFeedbackShared,
		notification.KindEnquiryReceived,
	}
	for _, kind := range kinds {
		t.Run(string(kind), func(t *testing.T) {
			msg, err := testRenderer().Render(job(kind, map[string]string{
				"clientName":  "Ama Serwaa",
				"senderName":  "Ama Serwaa",
				"serviceName": "Deep Tissue Massage",
				"startTime":   "Thursday 20 August, 09:00",
				"timezone":    "Africa/Accra",
			}))
			if err != nil {
				t.Fatalf("Render: %v", err)
			}
			if msg.To != "ama@example.com" || msg.Subject == "" || msg.Text == "" {
				t.Errorf("message = %+v, want recipient, subject and text set", msg)
			}
			if !strings.Contains(msg.HTML, "Terios Wellness Spa") {
				t.Error("body is missing the brand mark")
			}
			if strings.Contains(msg.HTML, "{{") {
				t.Errorf("body still contains unfilled placeholders:\n%s", msg.HTML)
			}
			if strings.Contains(msg.HTML, "2026 Terios") && !strings.Contains(msg.HTML, "© 2026") {
				t.Error("copyright year not substituted")
			}
		})
	}
}

// TestUnknownKindIsAnError: a kind with no template must not silently send
// an empty email.
func TestUnknownKindIsAnError(t *testing.T) {
	_, err := testRenderer().Render(job("not_a_kind", nil))
	if !errors.Is(err, notification.ErrTemplateNotFound) {
		t.Errorf("err = %v, want ErrTemplateNotFound", err)
	}
}

// TestEscapesUntrustedValues: names and enquiry messages are typed by
// strangers and land in an HTML document.
func TestEscapesUntrustedValues(t *testing.T) {
	msg, err := testRenderer().Render(job(notification.KindEnquiryReceived, map[string]string{
		"senderName":  `Ama <script>alert(1)</script>`,
		"senderEmail": "ama@example.com",
		"message":     `"><img src=x onerror=alert(1)>`,
	}))
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	// The injected markup must survive only as inert text. The checks name
	// tags the templates never contain themselves, so a hit can only have
	// come from the untrusted values.
	for _, live := range []string{"<script", "<img"} {
		if strings.Contains(msg.HTML, live) {
			t.Errorf("untrusted input reached the body as live markup (%q):\n%s", live, msg.HTML)
		}
	}
	if !strings.Contains(msg.HTML, "&lt;script&gt;") || !strings.Contains(msg.HTML, "&lt;img src=x") {
		t.Error("expected the injected tags to survive as escaped text")
	}
}

// TestSectionsFollowTheirFlag: the feedback template's optional "and
// resources" clause appears only when there are resources.
func TestSectionsFollowTheirFlag(t *testing.T) {
	data := map[string]string{
		"clientName":       "Ama",
		"practitionerName": "Terios",
		"serviceName":      "Massage",
		"sessionDate":      "20 August",
	}

	// The heading always reads "Notes and resources…", so the assertion has
	// to look at the sentence the section actually wraps.
	const clause = "shared feedback and resources"

	without, err := testRenderer().Render(job(notification.KindFeedbackShared, data))
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if strings.Contains(without.HTML, clause) {
		t.Error("resources clause rendered with no resources")
	}
	if !strings.Contains(without.HTML, "shared feedback from your") {
		t.Errorf("sentence did not close up around the dropped clause:\n%s", without.HTML)
	}

	data["hasResources"] = "true"
	with, err := testRenderer().Render(job(notification.KindFeedbackShared, data))
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !strings.Contains(with.HTML, clause) {
		t.Error("resources clause missing when resources were shared")
	}
	if strings.Contains(with.HTML, "{{#") || strings.Contains(with.HTML, "{{/") {
		t.Errorf("section markers leaked into the body:\n%s", with.HTML)
	}
}

// TestJobDataOverridesDefaults: a value resolved at queue time beats the
// deployment-wide default.
func TestJobDataOverridesDefaults(t *testing.T) {
	msg, err := testRenderer().Render(job(notification.KindBookingConfirmation, map[string]string{
		"clientName": "Ama",
		"manageUrl":  "https://example.test/custom",
	}))
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !strings.Contains(msg.HTML, "https://example.test/custom") {
		t.Error("job-supplied manageUrl did not win over the default")
	}
	if strings.Contains(msg.HTML, "terioscoach.com/portal/sessions") {
		t.Error("default manageUrl rendered alongside the override")
	}
}

// TestSubjectsAreSpecific: the reminder and enquiry subjects carry their
// most useful detail rather than a generic line.
func TestSubjectsAreSpecific(t *testing.T) {
	reminder, err := testRenderer().Render(job(notification.KindSessionReminder, map[string]string{
		"timeUntil": "tomorrow at 09:00",
	}))
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if reminder.Subject != "Reminder: your session is tomorrow at 09:00" {
		t.Errorf("subject = %q, want the timeUntil phrase", reminder.Subject)
	}

	enquiry, err := testRenderer().Render(job(notification.KindEnquiryReceived, map[string]string{
		"senderName": "Ama Serwaa",
	}))
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if enquiry.Subject != "New enquiry from Ama Serwaa" {
		t.Errorf("subject = %q, want the sender's name", enquiry.Subject)
	}
}

// TestMissingValuesRenderEmpty: a placeholder with no value leaves a blank,
// never the raw token.
func TestMissingValuesRenderEmpty(t *testing.T) {
	msg, err := testRenderer().Render(job(notification.KindBookingConfirmation, nil))
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if strings.Contains(msg.HTML, "{{") || strings.Contains(msg.HTML, "}}") {
		t.Errorf("unfilled placeholder left in the body:\n%s", msg.HTML)
	}
}

// TestTemplatesMatchTheDesignSource: the embedded copies must stay
// byte-identical to design/email, which is where the brand owns them.
// Editing one without the other is the failure this catches.
func TestTemplatesMatchTheDesignSource(t *testing.T) {
	designDir := filepath.Join("..", "..", "..", "..", "design", "email")
	entries, err := os.ReadDir(designDir)
	if err != nil {
		t.Fatalf("read design/email: %v", err)
	}

	seen := 0
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".html" {
			continue
		}
		seen++
		want, err := os.ReadFile(filepath.Join(designDir, entry.Name()))
		if err != nil {
			t.Fatalf("read %s: %v", entry.Name(), err)
		}
		got, err := templateFS.ReadFile("templates/" + entry.Name())
		if err != nil {
			t.Errorf("%s is in design/email but not embedded here: %v", entry.Name(), err)
			continue
		}
		if string(got) != string(want) {
			t.Errorf("%s has drifted from design/email — copy the design file over the embedded one", entry.Name())
		}
	}
	if seen == 0 {
		t.Fatal("no design templates found; the sync check would pass vacuously")
	}

	embedded, err := templateFS.ReadDir("templates")
	if err != nil {
		t.Fatalf("read embedded templates: %v", err)
	}
	if len(embedded) != seen {
		t.Errorf("embedded %d templates, design/email has %d — the sets must match exactly", len(embedded), seen)
	}
}

// TestEveryKindHasASubject: a kind with a template but no subject line
// would send with a generic one.
func TestEveryKindHasASubject(t *testing.T) {
	for kind := range templateFiles {
		if subjects[kind] == "" {
			t.Errorf("kind %q has a template but no subject line", kind)
		}
	}
}

// TestEveryLinkResolvesToARealRoute pins the URLs a client is sent to
// against the routes the customer app actually serves.
//
// This exists because the failure it catches is silent and permanent.
// PORTAL_URL was deployed without its "/portal" suffix, so every
// confirmation, reminder, reschedule and cancellation linked to
// terioscoach.com/sessions — a 404 on the marketing site. Nothing
// noticed: this file's other tests assert against whatever value the test
// injected, so they were equally green with a broken deployment.
//
// The route list is duplicated from apps/web deliberately. A test that
// derived the routes from the same constant the renderer uses would agree
// with itself and prove nothing; these are written out the way the Next.js
// directory tree has them, so a moved route breaks the build here.
func TestEveryLinkResolvesToARealRoute(t *testing.T) {
	// Mirrors apps/web/src/app/(portal)/portal/**. Update only alongside
	// an actual route move.
	realRoutes := map[string]bool{
		"/portal":           true,
		"/portal/sessions":  true,
		"/portal/book":      true,
		"/portal/payments":  true,
		"/portal/documents": true,
		"/portal/forms":     true,
	}

	const portalRoot = "https://terioscoach.com/portal"
	renderer := NewRenderer(Options{
		PortalURL:    portalRoot,
		DashboardURL: "https://practice.terioscoach.com",
	})

	for name, link := range renderer.defaults {
		if !strings.HasPrefix(link, portalRoot) {
			continue
		}
		path := strings.TrimPrefix(link, "https://terioscoach.com")
		if !realRoutes[path] {
			t.Errorf("%s links to %q, which is not a route the customer app serves", name, link)
		}
	}
}

// TestPortalURLMustCarryItsPathSegment states the deployment requirement
// the renderer depends on, in the place someone editing render.yaml would
// look for it.
func TestPortalURLMustCarryItsPathSegment(t *testing.T) {
	// The bare origin, which is what render.yaml shipped with. Every link
	// it produces lands outside the portal.
	bare := NewRenderer(Options{
		PortalURL:    "https://terioscoach.com",
		DashboardURL: "https://practice.terioscoach.com",
	})
	if got := bare.defaults["joinUrl"]; got != "https://terioscoach.com/sessions" {
		t.Fatalf("joinUrl = %q; this test no longer describes how links are built", got)
	}

	// The correct configuration puts every link inside the portal.
	correct := NewRenderer(Options{
		PortalURL:    "https://terioscoach.com/portal",
		DashboardURL: "https://practice.terioscoach.com",
	})
	for _, key := range []string{"joinUrl", "manageUrl", "bookUrl"} {
		got := correct.defaults[key]
		if !strings.HasPrefix(got, "https://terioscoach.com/portal/") {
			t.Errorf("%s = %q, want a /portal-scoped URL", key, got)
		}
	}
}
