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

	"appointments/internal/config"
	"appointments/internal/features/admin"
	"appointments/internal/features/auth"
	"appointments/internal/features/customers"
	"appointments/internal/features/onboarding"
	"appointments/internal/features/resources"
	"appointments/internal/features/scheduling"
	"appointments/internal/features/services"
	"appointments/internal/features/tenants"
	"appointments/internal/features/users"
	"appointments/internal/platform/database"
	"appointments/internal/platform/mailer"
	"appointments/internal/platform/whatsapp"
	"appointments/internal/shared/middleware"
)

func main() {
	cfg := config.Load()

	encKey := []byte(cfg.EncryptionKey)
	if len(encKey) != 32 {
		log.Fatal("ENCRYPTION_KEY must be exactly 32 bytes")
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
	appointmentRepo := scheduling.NewAppointmentRepository(db)
	availabilityRepo := scheduling.NewAvailabilityRepository(db)
	refreshTokenRepo := auth.NewRefreshTokenRepository(db)
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
	authUC := auth.NewUseCases(tenantUC, userUseCases, onboardingUC, refreshTokenRepo)
	serviceUC := services.NewUseCases(serviceRepo)
	resourceUC := resources.NewUseCases(resourceRepo, serviceRepo)
	customerUC := customers.NewUseCases(customerRepo)

	schedulingUC := scheduling.NewUseCases(
		sessionRepo,
		appointmentRepo,
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
	webhook := r.Group("/")
	webhook.Use(middleware.WhatsAppSignature(cfg.WhatsappAppSecret))
	webhookHandler.RegisterRoutes(webhook)

	// REST API
	adminHandler := admin.NewHandler(adminUC)
	authHandler := auth.NewHandler(authUC)
	tenantHandler := tenants.NewHandler(tenantUC)
	serviceHandler := services.NewHandler(serviceUC)
	resourceHandler := resources.NewHandler(resourceUC)
	customerHandler := customers.NewHandler(customerUC)
	onboardingHandler := onboarding.NewHandler(onboardingUC)
	userHandler := users.NewHandler(userUseCases)

	adminHandler.RegisterRoutes(r)
	authHandler.RegisterRoutes(r)
	tenantHandler.RegisterRoutes(r)
	serviceHandler.RegisterRoutes(r)
	resourceHandler.RegisterRoutes(r)
	customerHandler.RegisterRoutes(r)
	onboardingHandler.RegisterRoutes(r)
	userHandler.RegisterRoutes(r)

	// ── Background Jobs ──────────────────────────────────
	ctx, cancel := context.WithCancel(context.Background())

	// Reminder job — run every minute
	reminderJob := scheduling.NewReminderJob(
		appointmentRepo,
		serviceRepo,
		resourceRepo,
		customerRepo,
		tenantRepo,
		wa,
	)
	go reminderJob.Run(ctx)

	// Cleanup job — delete expired sessions and refresh tokens every hour
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

				if n, err := refreshTokenRepo.DeleteExpired(context.Background()); err != nil {
					log.Printf("cleanup refresh tokens error: %v", err)
				} else if n > 0 {
					log.Printf("cleanup: deleted %d expired refresh tokens", n)
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
