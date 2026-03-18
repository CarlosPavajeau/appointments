package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"

	"wappiz/internal/config"
	"wappiz/internal/features/admin"
	"wappiz/internal/features/appointments"
	"wappiz/internal/features/customers"
	"wappiz/internal/features/onboarding"
	"wappiz/internal/features/resources"
	"wappiz/internal/features/scheduling"
	"wappiz/internal/features/services"
	"wappiz/internal/features/tenants"
	"wappiz/internal/features/users"
	"wappiz/internal/platform/database"
	"wappiz/internal/platform/mailer"
	"wappiz/internal/platform/whatsapp"
	"wappiz/internal/shared/jwt"
	"wappiz/internal/shared/middleware"
)

func main() {
	cfg := config.Load()

	encKey := []byte(cfg.EncryptionKey)
	if len(encKey) != 32 {
		log.Fatal("ENCRYPTION_KEY must be exactly 32 bytes")
	}

	// ── Auth ─────────────────────────────────────────────
	// Initialise the JWKS verifier before any request is served.
	// This performs an eager fetch so the service fails fast if the external
	// authentication endpoint is unreachable.
	if err := jwt.Init(cfg.JWKSEndpoint, cfg.JWTIssuer); err != nil {
		log.Fatalf("failed to initialise JWT verifier: %v", err)
	}

	// ── Infra ────────────────────────────────────────────
	db := database.Connect(cfg.DatabaseURL)
	wa := whatsapp.NewClient(cfg.WhatsappBaseURL, cfg.WhatsappAPIVersion)
	resendMailer := mailer.NewResendMailer(cfg.ResendAPIKey, cfg.ResendFromEmail)

	// ── Repos ────────────────────────────────────────────
	tenantRepo := tenants.NewRepository(db, encKey)
	serviceRepo := services.NewRepository(db)
	resourceRepo := resources.NewRepository(db)
	customerRepo := customers.NewRepository(db)
	sessionRepo := scheduling.NewSessionRepository(db)
	appointmentRepo := appointments.NewRepository(db)
	availabilityRepo := scheduling.NewAvailabilityRepository(db)
	userRepo := users.NewRepository(db)
	onboardingRepo := onboarding.NewRepository(db)

	// ── Use Cases ────────────────────────────────────────
	userUseCases := users.NewUseCases(userRepo)
	tenantUC := tenants.NewUseCases(tenantRepo)
	onboardingUC := onboarding.NewUseCases(
		onboardingRepo,
		tenantUC,
		resourceRepo,
		serviceRepo,
		resendMailer,
		cfg.AdminEmail,
	)
	adminUC := admin.NewUseCases(tenantUC, resendMailer)
	serviceUC := services.NewUseCases(serviceRepo)
	resourceUC := resources.NewUseCases(resourceRepo, serviceRepo)
	customerUC := customers.NewUseCases(customerRepo)
	appointmentUC := appointments.NewUseCases(appointmentRepo)

	schedulingUC := scheduling.NewUseCases(
		sessionRepo,
		appointmentUC,
		serviceRepo,
		resourceRepo,
		customerRepo,
		availabilityRepo,
		tenantRepo,
	)

	// ── State Machine ────────────────────────────────────
	machine := scheduling.NewStateMachine(sessionRepo, schedulingUC, wa, tenantRepo)

	// ── Router ───────────────────────────────────────────
	r := gin.Default()
	r.Use(cors.New(cors.Config{
		AllowAllOrigins:  true,
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		AllowCredentials: false,
	}))

	// Webhook (WhatsApp)
	webhookHandler := scheduling.NewHandler(machine, tenantRepo, cfg.WebhookVerifyToken)
	r.GET("/webhook", webhookHandler.Verify)
	webhookSigned := r.Group("/")
	webhookSigned.Use(middleware.WhatsAppSignature(cfg.WhatsappAppSecret))
	webhookSigned.POST("/webhook", webhookHandler.Handle)

	// REST API
	adminHandler := admin.NewHandler(adminUC)
	tenantHandler := tenants.NewHandler(tenantUC)
	serviceHandler := services.NewHandler(serviceUC)
	resourceHandler := resources.NewHandler(resourceUC)
	customerHandler := customers.NewHandler(customerUC)
	onboardingHandler := onboarding.NewHandler(onboardingUC)
	userHandler := users.NewHandler(userUseCases)
	appointmentHandler := appointments.NewHandler(appointmentUC)

	adminHandler.RegisterRoutes(r)
	tenantHandler.RegisterRoutes(r)
	serviceHandler.RegisterRoutes(r)
	resourceHandler.RegisterRoutes(r)
	customerHandler.RegisterRoutes(r)
	onboardingHandler.RegisterRoutes(r)
	userHandler.RegisterRoutes(r)
	appointmentHandler.RegisterRoutes(r)

	// ── Background Jobs ──────────────────────────────────
	ctx, cancel := context.WithCancel(context.Background())

	// Reminder job — run every minute
	reminderJob := scheduling.NewReminderJob(
		appointmentUC,
		serviceRepo,
		resourceRepo,
		customerRepo,
		tenantRepo,
		wa,
	)
	go reminderJob.Run(ctx)

	// Cleanup job — delete expired sessions every hour
	go func() {
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if n, err := sessionRepo.DeleteExpired(context.Background()); err != nil {
					log.Printf("cleanup sessions error: %v", err)
				} else if n > 0 {
					log.Printf("cleanup: deleted %d expired sessions", n)
				}
			}
		}
	}()

	// ── Server with graceful shutdown ────────────────────
	srv := &http.Server{
		Addr:         cfg.Port,
		Handler:      r,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

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
}
