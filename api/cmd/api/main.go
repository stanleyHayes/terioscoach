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

	"github.com/xcreativs/terios/api/internal/adapters/httpapi"
	"github.com/xcreativs/terios/api/internal/adapters/mongodb"
	"github.com/xcreativs/terios/api/internal/adapters/security"
	"github.com/xcreativs/terios/api/internal/app/auth"
	bookingapp "github.com/xcreativs/terios/api/internal/app/booking"
	"github.com/xcreativs/terios/api/internal/app/catalog"
	schedulingapp "github.com/xcreativs/terios/api/internal/app/scheduling"
	"github.com/xcreativs/terios/api/internal/config"
	domainbooking "github.com/xcreativs/terios/api/internal/domain/booking"
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

		opts = append(opts,
			httpapi.WithReadiness(mongoClient.Ping),
			httpapi.WithAuth(authService),
			httpapi.WithCatalog(buildCatalogService(db), authService),
			httpapi.WithScheduling(buildSchedulingService(db), authService),
			httpapi.WithBooking(buildBookingService(db), authService),
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
		)
	}

	srv := httpapi.NewServer(opts...)
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

	return auth.NewService(
		mongodb.NewUserRepository(db),
		mongodb.NewRefreshTokenRepository(db),
		security.NewArgon2Hasher(),
		issuer,
		cfg.RefreshTokenTTL,
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

// buildBookingService wires the booking slice to its MongoDB adapters. It
// shares the availability read side (rules, time-off, busy intervals) so a
// booking is only accepted when the slot engine would have offered the
// slot; the platform-default 24h modification cutoff applies.
func buildBookingService(db *mongo.Database) *bookingapp.Service {
	return bookingapp.NewService(
		mongodb.NewBookingRepository(db),
		mongodb.NewServiceRepository(db),
		mongodb.NewAvailabilityRepository(db),
		mongodb.NewBusyIntervalReader(db),
		domainbooking.DefaultPolicy(),
	)
}
