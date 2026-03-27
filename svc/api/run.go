package api

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
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
	"wappiz/pkg/whatsapp"
	"wappiz/svc/api/routes"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func Run(cfg Config) error {
	database, err := db.New(db.Config{
		PrimaryDns: cfg.DatabaseURL,
	})
	if err != nil {
		return fmt.Errorf("unable to create db: %w", err)
	}

	defer database.Close()

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

	mailerSvc := mailer.New(cfg.ResendAPIKey, cfg.ResendFromEmail)
	waSvc := whatsapp.New(cfg.WhatsappBaseURL, cfg.WhatsappAPIVersion)
	slotFinder := slot_finder.New(database)
	stateMachineSvc := state_machine.New(state_machine.Config{
		DB:         database,
		Whatsapp:   waSvc,
		SlotFinder: slotFinder,
	})

	g := gin.Default()
	routes.Register(g, &routes.Services{
		Database:     database,
		Mailer:       mailerSvc,
		Whatsapp:     waSvc,
		StateMachine: stateMachineSvc,
		AdminEmail:   cfg.AdminEmail,
		AppSecret:    cfg.WhatsappAppSecret,
	})

	ctx, cancel := context.WithCancel(context.Background())

	reminderJob := reminder_job.New(reminder_job.Config{
		DB:       database,
		Whatsapp: waSvc,
	})

	nowShowTrackerJob := no_show_tracker_job.New(database)

	cleanupSessionJob := cleanup_sessions_job.New(database)

	go reminderJob.Run(ctx)
	go nowShowTrackerJob.Run(ctx)
	go cleanupSessionJob.Run(ctx)

	// Server with graceful shutdown
	srv := &http.Server{
		Addr:         cfg.Port,
		Handler:      g,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	// TODO: enhance this
	go func() {
		log.Printf("server running on %s", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("shutting down...")
	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("server forced to shutdown: %v", err)
	}

	log.Println("server stopped")

	return nil
}
