package api

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"
	"wappiz/internal/jobs/cleanup_sessions_job"
	"wappiz/internal/jobs/no_show_tracker_job"
	"wappiz/internal/jobs/reminder_job"
	"wappiz/internal/services/slot_finder"
	"wappiz/internal/services/state_machine"
	"wappiz/pkg/db"
	"wappiz/pkg/jwt"
	"wappiz/pkg/logger"
	"wappiz/pkg/mailer"
	"wappiz/pkg/runner"
	"wappiz/pkg/whatsapp"
	"wappiz/svc/api/routes"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// nolint:gocognit
func Run(ctx context.Context, cfg Config) error {
	r := runner.New()
	defer r.Recover()

	database, err := db.New(db.Config{
		PrimaryDns: cfg.DatabaseURL,
	})
	if err != nil {
		return fmt.Errorf("unable to create db: %w", err)
	}

	r.Defer(database.Close)

	encKey := []byte(cfg.EncryptionKey)
	if len(encKey) != 32 {
		logger.Error("ENCRYPTION_KEY must be exactly 32 bytes")
		return errors.New("ENCRYPTION_KEY must be exactly 32 bytes")
	}

	// Initialise the JWKS verifier before any request is served.
	// This performs an eager fetch so the service fails fast if the external
	// authentication endpoint is unreachable.
	if err := jwt.Init(cfg.JWKSEndpoint, cfg.JWTIssuer); err != nil {
		logger.Error("failed to initialise JWT verifier: %v", err)
		return err
	}

	jwt.InitTenantFinder(func(ctx context.Context, userID string) (uuid.UUID, error) {
		tenant, err := db.Query.FindTenantByUserId(ctx, database.Primary(), userID)
		if err != nil {
			return uuid.Nil, fmt.Errorf("failed to find tenant for user %s: %w", userID, err)
		}
		return tenant.ID, nil
	})

	mailerSvc := mailer.New(mailer.Config{
		ApiKey:    cfg.ResendAPIKey,
		FromEmail: cfg.ResendFromEmail,
	})
	waSvc := whatsapp.New(whatsapp.Config{
		BaseURL:    cfg.WhatsappBaseURL,
		ApiVersion: cfg.WhatsappAPIVersion,
	})
	slotFinder := slot_finder.New(database)
	stateMachineSvc := state_machine.New(state_machine.Config{
		DB:         database,
		Whatsapp:   waSvc,
		SlotFinder: slotFinder,
	})

	g := gin.New()

	g.Use(gin.Recovery())
	g.Use(requestIDMiddleware())
	g.Use(logMiddleware())

	routes.Register(g, &routes.Services{
		Database:      database,
		Mailer:        mailerSvc,
		Whatsapp:      waSvc,
		StateMachine:  stateMachineSvc,
		Runner:        r,
		AdminEmail:    cfg.AdminEmail,
		AppSecret:     cfg.WhatsappAppSecret,
		EncryptionKey: encKey,
	})

	reminderJob := reminder_job.New(reminder_job.Config{
		DB:       database,
		Whatsapp: waSvc,
	})

	nowShowTrackerJob := no_show_tracker_job.New(no_show_tracker_job.Config{
		DB:            database,
		Whatsapp:      waSvc,
		EncryptionKey: encKey,
	})

	cleanupSessionJob := cleanup_sessions_job.New(database)

	r.Go(func(ctx context.Context) error {
		reminderJob.Run(ctx)
		return nil
	})

	r.Go(func(ctx context.Context) error {
		nowShowTrackerJob.Run(ctx)
		return nil
	})

	r.Go(func(ctx context.Context) error {
		cleanupSessionJob.Run(ctx)
		return nil
	})

	// Server with graceful shutdown
	srv := &http.Server{
		Handler:      g,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	r.DeferCtx(srv.Shutdown)

	r.Go(func(ctx context.Context) error {
		logger.Info("server starting", "port", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("server failed: %w", err)
		}

		return nil
	})

	// Wait for either OS signals or context cancellation, then shutdown
	if err := r.Wait(ctx, runner.WithTimeout(time.Minute)); err != nil {
		logger.Error("Shutdown failed", "error", err)
		return fmt.Errorf("shutdown failed: %w", err)
	}

	logger.Info("API server shut down successfully")

	return nil
}

func requestIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" {
			requestID = uuid.New().String()
		}
		c.Set("request_id", requestID)
		c.Header("X-Request-ID", requestID)
		c.Next()
	}
}

func logMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path

		ctx, event := logger.StartWideEvent(c,
			fmt.Sprintf("%s %s", c.Request.Method, path),
		)

		defer event.End()

		c.Request = c.Request.WithContext(ctx)
		c.Next()

		if len(c.Errors) > 0 {
			for _, err := range c.Errors {
				event.SetError(err)
			}
		}

		requestID, _ := c.Get("request_id")

		event.Set(slog.Group("http",
			slog.String("request_id", requestID.(string)),
			slog.String("method", c.Request.Method),
			slog.String("path", path),
			slog.String("host", c.Request.Host),
			slog.String("user_agent", c.Request.UserAgent()),
			slog.String("ip_address", c.ClientIP()),
			slog.Int("status_code", c.Writer.Status()),
			slog.Int64("latency_ms", time.Since(start).Milliseconds()),
		))

	}
}
