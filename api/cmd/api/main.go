// Terios Wellness Spa — Digital Practice Platform API.
// Hexagonal architecture: cmd wires adapters to ports; the domain core
// stays free of framework and driver imports.
package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/xcreativs/terios/api/internal/adapters/cloudflare"
	"github.com/xcreativs/terios/api/internal/adapters/cloudinary"
	"github.com/xcreativs/terios/api/internal/adapters/email"
	"github.com/xcreativs/terios/api/internal/adapters/httpapi"
	"github.com/xcreativs/terios/api/internal/adapters/memory"
	"github.com/xcreativs/terios/api/internal/adapters/mongodb"
	"github.com/xcreativs/terios/api/internal/adapters/paystack"
	"github.com/xcreativs/terios/api/internal/adapters/resend"
	"github.com/xcreativs/terios/api/internal/adapters/security"
	"github.com/xcreativs/terios/api/internal/adapters/stripe"
	"github.com/xcreativs/terios/api/internal/adapters/wsapi"
	"github.com/xcreativs/terios/api/internal/app/auth"
	bookingapp "github.com/xcreativs/terios/api/internal/app/booking"
	"github.com/xcreativs/terios/api/internal/app/catalog"
	clientsapp "github.com/xcreativs/terios/api/internal/app/clients"
	cmsapp "github.com/xcreativs/terios/api/internal/app/cms"
	documentsapp "github.com/xcreativs/terios/api/internal/app/documents"
	enquiriesapp "github.com/xcreativs/terios/api/internal/app/enquiries"
	formsapp "github.com/xcreativs/terios/api/internal/app/forms"
	notesapp "github.com/xcreativs/terios/api/internal/app/notes"
	notificationsapp "github.com/xcreativs/terios/api/internal/app/notifications"
	paymentsapp "github.com/xcreativs/terios/api/internal/app/payments"
	reportsapp "github.com/xcreativs/terios/api/internal/app/reports"
	reviewsapp "github.com/xcreativs/terios/api/internal/app/reviews"
	schedulingapp "github.com/xcreativs/terios/api/internal/app/scheduling"
	signalingapp "github.com/xcreativs/terios/api/internal/app/signaling"
	"github.com/xcreativs/terios/api/internal/config"
	domainbooking "github.com/xcreativs/terios/api/internal/domain/booking"
	"github.com/xcreativs/terios/api/internal/domain/notification"
	"github.com/xcreativs/terios/api/internal/domain/ops"
	"github.com/xcreativs/terios/api/internal/ports"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

func main() {
	if err := run(); err != nil {
		slog.Error("fatal", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	// Connect to MongoDB when configured. Dev boots fine without it — the
	// readiness probe then always reports not-ready so the gap is visible.
	var opts []httpapi.Option
	var mongoClient *mongodb.Client
	if cfg.MongoURI != "" {
		connectCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		mongoClient, err = mongodb.Connect(connectCtx, cfg.MongoURI)
		cancel()
		if err != nil {
			return fmt.Errorf("connect mongodb: %w", err)
		}
		defer func() {
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			if err := mongoClient.Disconnect(shutdownCtx); err != nil {
				slog.Error("mongodb disconnect", "error", err)
			}
		}()

		db := mongoClient.Database(cfg.MongoDBName)
		indexCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		err = mongodb.EnsureIndexes(indexCtx, db)
		cancel()
		if err != nil {
			return fmt.Errorf("ensure indexes: %w", err)
		}

		authService, err := buildAuthService(cfg, db)
		if err != nil {
			return err
		}

		// Video signaling (CX-01) and its TURN credentials (CX-02).
		// Rooms live in this process; see wsapi's package comment.
		signalingSvc := buildSignalingService(cfg, db)

		// Alerting thresholds are the platform defaults. They are low on
		// purpose: on a normal day every one of these counters is zero,
		// so there is no background noise to filter out.
		opsThresholds := ops.DefaultThresholds()

		// Notifications (BE-09). Without a Resend key the outbox still
		// records everything that would have gone out — the gap is then
		// visible in the collection rather than silently lost.
		notifier := buildNotificationService(cfg, db)
		stopDispatcher := startDispatcher(notifier, cfg.NotificationPollInterval)
		defer stopDispatcher()

		opts = append(opts,
			httpapi.WithReadiness(mongoClient.Ping),
			httpapi.WithAuth(authService, httpapi.WithAuthRateLimit(httpapi.RateLimitPolicy{
				Limit:  cfg.AuthRateLimit,
				Window: cfg.AuthRateLimitWindow,
			})),
			httpapi.WithCatalog(buildCatalogService(db), authService),
			httpapi.WithScheduling(buildSchedulingService(db), authService),
			httpapi.WithBooking(buildBookingService(db, notifier), authService),
			httpapi.WithPayments(buildPaymentService(cfg, db), authService),
			httpapi.WithClients(buildClientService(db), authService),
			httpapi.WithNotes(buildNoteService(db, notifier), authService),
			httpapi.WithContent(buildContentService(db), authService),
			httpapi.WithEnquiries(buildEnquiryService(db, notifier), authService),
			httpapi.WithReviews(buildReviewService(db), authService),
			httpapi.WithForms(buildFormService(db), authService),
			httpapi.WithDocuments(buildDocumentService(cfg, db), authService),
			httpapi.WithReports(buildReportService(db), authService),
			// Operational health (LCH-09): what an uptime monitor polls to
			// learn that mail is backing up or accounts are being locked en
			// masse, neither of which shows up in a liveness probe.
			httpapi.WithOps(
				mongodb.NewOpsSource(db, opsThresholds.Window, notification.DefaultRetryPolicy()),
				authService, opsThresholds,
			),
			httpapi.WithSessions(signalingSvc, wsapi.NewHandler(signalingSvc, wsapi.NewHub(), cfg.AllowedOrigins, slog.Default()), authService),
		)
	} else {
		slog.Warn("MONGODB_URI not set; running without database, readiness will fail and auth routes return 503")
		opts = append(opts,
			httpapi.WithReadiness(func(context.Context) error {
				return errors.New("mongodb not configured")
			}),
			httpapi.WithAuth(nil),
			httpapi.WithCatalog(nil, nil),
			httpapi.WithScheduling(nil, nil),
			httpapi.WithBooking(nil, nil),
			httpapi.WithPayments(nil, nil),
			httpapi.WithClients(nil, nil),
			httpapi.WithNotes(nil, nil),
			httpapi.WithContent(nil, nil),
			httpapi.WithEnquiries(nil, nil),
			httpapi.WithReviews(nil, nil),
			httpapi.WithForms(nil, nil),
			httpapi.WithDocuments(nil, nil),
			httpapi.WithReports(nil, nil),
			httpapi.WithOps(nil, nil, ops.DefaultThresholds()),
			httpapi.WithSessions(nil, nil, nil),
		)
	}

	srv := httpapi.NewServerWith(
		[]httpapi.BuildOption{
			httpapi.WithTrustedProxyHops(cfg.TrustedProxyHops),
			httpapi.WithCORS(httpapi.CORSPolicy{AllowedOrigins: cfg.AllowedOrigins}),
			httpapi.WithProduction(cfg.Env == "production"),
		},
		opts...,
	)
	httpServer := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.Port),
		Handler:           srv.Router,
		ReadHeaderTimeout: 5 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		slog.Info("api listening", "port", cfg.Port, "env", cfg.Env)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return httpServer.Shutdown(shutdownCtx)
}

// buildAuthService wires the auth slice to its MongoDB and security
// adapters. JWT secrets are required in production (enforced by config);
// in development a missing secret falls back to an ephemeral random one so
// the API boots — issued tokens simply die on restart.
func buildAuthService(cfg config.Config, db *mongo.Database) (*auth.Service, error) {
	secret := cfg.JWTAccessSecret
	if secret == "" {
		buf := make([]byte, 32)
		if _, err := rand.Read(buf); err != nil {
			return nil, fmt.Errorf("generate dev jwt secret: %w", err)
		}
		secret = base64.RawURLEncoding.EncodeToString(buf)
		slog.Warn("JWT_ACCESS_SECRET not set; using ephemeral dev secret, tokens invalidate on restart")
	}

	issuer, err := security.NewJWTIssuer(secret, cfg.AccessTokenTTL)
	if err != nil {
		return nil, fmt.Errorf("build token issuer: %w", err)
	}

	var resetMailer ports.Mailer = unconfiguredMailer{}
	if cfg.ResendAPIKey != "" {
		resetMailer = resend.NewClient(cfg.ResendAPIKey, cfg.ResendFrom)
	}
	return auth.NewService(
		mongodb.NewUserRepository(db),
		mongodb.NewRefreshTokenRepository(db),
		security.NewArgon2Hasher(),
		issuer,
		cfg.RefreshTokenTTL,
		// Brute-force lockout (BE-02) is stored in MongoDB, so it holds
		// across restarts and across instances — unlike the per-IP rate
		// limiter in the HTTP adapter, which is per-process by design.
		auth.WithLockout(mongodb.NewLoginAttemptRepository(db), cfg.LockoutPolicy()),
		auth.WithPasswordReset(resetMailer, cfg.PortalURL, cfg.DashboardURL, time.Hour),
	), nil
}

// buildCatalogService wires the services slice to its MongoDB adapter.
func buildCatalogService(db *mongo.Database) *catalog.Service {
	return catalog.NewService(mongodb.NewServiceRepository(db))
}

// buildSchedulingService wires the availability slice to its MongoDB
// adapters. The busy reader feeds confirmed bookings (BE-05) into slot
// generation; until bookings exist it yields an empty schedule.
func buildSchedulingService(db *mongo.Database) *schedulingapp.Service {
	serviceRepo := mongodb.NewServiceRepository(db)
	return schedulingapp.NewService(
		serviceRepo,
		mongodb.NewAvailabilityRepository(db),
		mongodb.NewBusyIntervalReader(db),
	)
}

// buildPaymentService wires the payments slice to its MongoDB and gateway
// adapters. PAYMENT_PROVIDER selects the gateway (paystack by default);
// without the selected provider's secret key the gateway is nil — payment
// and webhook routes then answer 503 instead of crashing, like the other
// unconfigured-service stubs.
func buildPaymentService(cfg config.Config, db *mongo.Database) *paymentsapp.Service {
	var gateway ports.PaymentGateway
	switch cfg.PaymentProvider {
	case "stripe":
		// The webhook is the only payment-confirmation path, so a missing
		// signing secret means payments could never succeed — treat it as
		// unconfigured, like a missing API key.
		if cfg.StripeSecretKey == "" || cfg.StripeWebhookSecret == "" {
			slog.Warn("PAYMENT_PROVIDER=stripe but STRIPE_SECRET_KEY/STRIPE_WEBHOOK_SECRET not set; payment and webhook routes return 503")
			return nil
		}
		gateway = stripe.NewClient(cfg.StripeSecretKey, cfg.StripeWebhookSecret, cfg.PortalURL+"/payments", cfg.PortalURL+"/payments")
	default: // "paystack" — validated by config.Load
		if cfg.PaystackSecretKey == "" {
			slog.Warn("PAYSTACK_SECRET_KEY not set; payment and webhook routes return 503")
			return nil
		}
		gateway = paystack.NewClient(cfg.PaystackSecretKey)
	}
	return paymentsapp.NewService(
		mongodb.NewPaymentRepository(db),
		mongodb.NewBookingRepository(db),
		mongodb.NewServiceRepository(db),
		mongodb.NewUserRepository(db),
		gateway,
	)
}

// buildBookingService wires the booking slice to its MongoDB adapters. It
// shares the availability read side (rules, time-off, busy intervals) so a
// booking is only accepted when the slot engine would have offered the
// slot; the platform-default 24h modification cutoff applies. Confirmations
// and reminders are queued through the notifier.
func buildBookingService(db *mongo.Database, notifier *notificationsapp.Service) *bookingapp.Service {
	return bookingapp.NewService(
		mongodb.NewBookingRepository(db),
		mongodb.NewServiceRepository(db),
		mongodb.NewAvailabilityRepository(db),
		mongodb.NewBusyIntervalReader(db),
		domainbooking.DefaultPolicy(),
		bookingapp.WithNotifications(notifier, mongodb.NewUserRepository(db)),
	)
}

// buildNotificationService wires the messaging slice. The mailer is the
// only part that needs a provider key: without one the outbox still
// records every message, and the dispatcher reports each undeliverable
// attempt rather than pretending it went out.
func buildNotificationService(cfg config.Config, db *mongo.Database) *notificationsapp.Service {
	var mailer ports.Mailer = unconfiguredMailer{}
	if cfg.ResendAPIKey == "" {
		slog.Warn("RESEND_API_KEY not set; notifications are queued but not delivered")
	} else {
		mailer = resend.NewClient(cfg.ResendAPIKey, cfg.ResendFrom)
	}

	renderer := email.NewRenderer(email.Options{
		PortalURL:    cfg.PortalURL,
		DashboardURL: cfg.DashboardURL,
	})
	return notificationsapp.NewService(
		mongodb.NewNotificationJobRepository(db),
		renderer,
		mailer,
		notificationsapp.Options{
			ReminderLead:    cfg.ReminderLead,
			DefaultTimezone: cfg.DefaultTimezone,
			PracticeEmail:   cfg.PracticeEmail,
			Report: func(err error) {
				slog.Error("notification", "error", err)
			},
		},
	)
}

// unconfiguredMailer stands in when no provider key is set. It fails every
// send loudly rather than reporting success, so a deployment that forgot
// RESEND_API_KEY shows up as failed jobs instead of as clients who never
// heard from the practice.
type unconfiguredMailer struct{}

var _ ports.Mailer = unconfiguredMailer{}

func (unconfiguredMailer) Send(context.Context, ports.EmailMessage) error {
	return &ports.GatewayError{StatusCode: 0, Message: "no mail provider configured (RESEND_API_KEY unset)"}
}

// startDispatcher runs the notification outbox on a timer and returns a
// function that stops it and waits for the in-flight pass to finish, so
// shutdown never cuts a send in half.
func startDispatcher(dispatcher ports.Dispatcher, interval time.Duration) func() {
	if interval <= 0 {
		interval = time.Minute
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})

	go func() {
		defer close(done)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				result, err := dispatcher.DispatchDue(ctx, 0)
				if err != nil {
					slog.Error("notification dispatch", "error", err)
					continue
				}
				if result.Sent > 0 || result.Failed > 0 {
					slog.Info("notifications dispatched", "sent", result.Sent, "failed", result.Failed)
				}
			}
		}
	}()

	return func() {
		cancel()
		<-done
	}
}

// buildClientService wires the client-records slice to its MongoDB
// adapters. The user/booking/payment repositories are shared with their
// own slices — the full record is assembled from those read models, never
// re-queried raw; the document and form counters only count the collections
// owned by the documents and forms slices.
func buildClientService(db *mongo.Database) *clientsapp.Service {
	return clientsapp.NewService(
		mongodb.NewClientProfileRepository(db),
		mongodb.NewUserRepository(db),
		mongodb.NewBookingRepository(db),
		mongodb.NewPaymentRepository(db),
		mongodb.NewDocumentCounter(db),
		mongodb.NewFormSubmissionCounter(db),
	)
}

// buildContentService wires the CMS slice to its MongoDB adapters. The
// public and practitioner read paths share one service; what keeps drafts
// off the site is the split between its Public* use cases and the rest.
func buildContentService(db *mongo.Database) *cmsapp.Service {
	return cmsapp.NewService(
		mongodb.NewPageRepository(db),
		mongodb.NewPostRepository(db),
		mongodb.NewFAQRepository(db),
		mongodb.NewTestimonialRepository(db),
	)
}

// buildEnquiryService wires the contact-form slice to its MongoDB adapter.
// A new enquiry alerts the practice through the notifications outbox.
func buildEnquiryService(db *mongo.Database, notifier *notificationsapp.Service) *enquiriesapp.Service {
	return enquiriesapp.NewService(
		mongodb.NewEnquiryRepository(db),
		enquiriesapp.WithNotifications(notifier),
	)
}

// buildReviewService wires the reviews slice to its MongoDB adapters. The
// booking repository is what proves a review is earned; the user and
// service repositories only resolve display names for the public list.
func buildReviewService(db *mongo.Database) *reviewsapp.Service {
	return reviewsapp.NewService(
		mongodb.NewReviewRepository(db),
		mongodb.NewBookingRepository(db),
		mongodb.NewUserRepository(db),
		mongodb.NewServiceRepository(db),
	)
}

// buildFormService wires the intake-and-consent slice to its MongoDB
// adapters. The form_submissions collection it owns is the one the client
// record's submission counter reads.
func buildFormService(db *mongo.Database) *formsapp.Service {
	return formsapp.NewService(
		mongodb.NewFormRepository(db),
		mongodb.NewFormSubmissionRepository(db),
	)
}

// buildDocumentService wires the client-files slice to MongoDB and the
// media store. Without Cloudinary credentials the service is nil and the
// document routes answer 503 — the same explicit gap as the other
// unconfigured providers, rather than routes that appear to work and then
// hand out URLs nothing can serve.
func buildDocumentService(cfg config.Config, db *mongo.Database) *documentsapp.Service {
	if cfg.CloudinaryCloudName == "" || cfg.CloudinaryAPIKey == "" || cfg.CloudinaryAPISecret == "" {
		slog.Warn("Cloudinary credentials not set; document routes return 503")
		return nil
	}
	return documentsapp.NewService(
		mongodb.NewDocumentRepository(db),
		cloudinary.NewClient(cfg.CloudinaryCloudName, cfg.CloudinaryAPIKey, cfg.CloudinaryAPISecret),
		documentsapp.Options{DeliveryTTL: cfg.DocumentURLTTL},
	)
}

// buildReportService wires the reporting slice. Every repository it reads
// is the one its own slice owns: a report is a read over the same records
// the rest of the API writes, never a parallel set of numbers.
func buildReportService(db *mongo.Database) *reportsapp.Service {
	return reportsapp.NewService(
		mongodb.NewBookingRepository(db),
		mongodb.NewPaymentRepository(db),
		mongodb.NewServiceRepository(db),
		mongodb.NewReviewRepository(db),
	)
}

// buildSignalingService wires the video slice. Tickets are held in memory
// on purpose (see the memory package). ICE servers come from a provider
// rather than a config value because Cloudflare's TURN credentials expire
// and are minted per session.
func buildSignalingService(cfg config.Config, db *mongo.Database) *signalingapp.Service {
	var ice ports.ICEProvider
	if cfg.TURNConfigured() {
		ice = cloudflare.NewClient(cfg.TURNKeyID, cfg.TURNAPIToken)
	} else {
		// STUN only. Calls between two ordinary networks still connect;
		// calls where either side is behind symmetric NAT will not, and
		// that is most home broadband. This is a warning rather than a
		// fatal error because a development machine has no TURN key and
		// video still works locally.
		slog.Warn("no TURN key configured; video will fail for clients behind symmetric NAT",
			"stun", cfg.STUNUrls)
		ice = cloudflare.StaticProvider{Servers: cfg.StaticICEServers()}
	}

	return signalingapp.NewService(
		mongodb.NewBookingRepository(db),
		memory.NewTicketStore(),
		signalingapp.Options{
			Policy: cfg.JoinPolicy(),
			ICE:    ice,
			Report: func(err error) { slog.Error("ice servers", "error", err) },
		},
	)
}

// buildNoteService wires the session-notes slice to its MongoDB adapters.
// Note ownership follows the booking's practitioner; the first share of a
// note emails the client through the notifier.
func buildNoteService(db *mongo.Database, notifier *notificationsapp.Service) *notesapp.Service {
	return notesapp.NewService(
		mongodb.NewSessionNoteRepository(db),
		mongodb.NewBookingRepository(db),
		notesapp.WithNotifications(notifier, mongodb.NewUserRepository(db), mongodb.NewServiceRepository(db)),
	)
}
