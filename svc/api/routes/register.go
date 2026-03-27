package routes

import (
	"net/http"
	"wappiz/pkg/jwt"
	"wappiz/svc/api/middleware"
	"wappiz/svc/api/routes/v1/admin_activate_tenant"
	"wappiz/svc/api/routes/v1/admin_find_pending_activations"
	"wappiz/svc/api/routes/v1/appointments_get_status_history"
	"wappiz/svc/api/routes/v1/appointments_search"
	"wappiz/svc/api/routes/v1/appointments_update_status"
	"wappiz/svc/api/routes/v1/customers_block"
	"wappiz/svc/api/routes/v1/customers_get"
	"wappiz/svc/api/routes/v1/customers_list"
	"wappiz/svc/api/routes/v1/customers_unblock"
	"wappiz/svc/api/routes/v1/onboarding_get_progress"
	"wappiz/svc/api/routes/v1/onboarding_get_templates"
	"wappiz/svc/api/routes/v1/onboarding_step_barber"
	"wappiz/svc/api/routes/v1/onboarding_step_services"
	"wappiz/svc/api/routes/v1/onboarding_step_whatsapp"
	"wappiz/svc/api/routes/v1/resources_assign_services"
	"wappiz/svc/api/routes/v1/resources_create"
	"wappiz/svc/api/routes/v1/resources_create_override"
	"wappiz/svc/api/routes/v1/resources_delete"
	"wappiz/svc/api/routes/v1/resources_delete_override"
	"wappiz/svc/api/routes/v1/resources_delete_working_hours"
	"wappiz/svc/api/routes/v1/resources_get"
	"wappiz/svc/api/routes/v1/resources_get_services"
	"wappiz/svc/api/routes/v1/resources_list"
	"wappiz/svc/api/routes/v1/resources_list_overrides"
	"wappiz/svc/api/routes/v1/resources_update"
	"wappiz/svc/api/routes/v1/resources_update_sort_order"
	"wappiz/svc/api/routes/v1/resources_upsert_working_hours"
	"wappiz/svc/api/routes/v1/services_create_service"
	"wappiz/svc/api/routes/v1/services_list_services"
	"wappiz/svc/api/routes/v1/services_update_service"
	"wappiz/svc/api/routes/v1/tenants_create_tenant"
	"wappiz/svc/api/routes/v1/tenants_find_by_user"
	"wappiz/svc/api/routes/v1/tenants_find_current"
	"wappiz/svc/api/routes/v1/tenants_update_tenant"
	"wappiz/svc/api/routes/v1/webhooks_process_webhook"
	"wappiz/svc/api/routes/v1/webhooks_verify_webhook"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func Register(g *gin.Engine, svc *Services) {
	defaultMiddlewares := []gin.HandlerFunc{
		cors.New(cors.Config{
			AllowAllOrigins:  true,
			AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
			AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
			AllowCredentials: false,
		}),
		jwt.AuthMiddleware(),
	}

	// ---------------------------------------------------------------------------
	// v1/tenants
	RegisterRoute(g, defaultMiddlewares, &tenants_create_tenant.Handler{DB: svc.Database})
	RegisterRoute(g, defaultMiddlewares, &tenants_find_current.Handler{DB: svc.Database})
	RegisterRoute(g, defaultMiddlewares, &tenants_find_by_user.Handler{DB: svc.Database})
	RegisterRoute(g, defaultMiddlewares, &tenants_update_tenant.Handler{DB: svc.Database})

	// v1/services
	RegisterRoute(g, defaultMiddlewares, &services_create_service.Handler{DB: svc.Database})
	RegisterRoute(g, defaultMiddlewares, &services_list_services.Handler{DB: svc.Database})
	RegisterRoute(g, defaultMiddlewares, &services_update_service.Handler{DB: svc.Database})

	// v1/appointments
	RegisterRoute(g, defaultMiddlewares, &appointments_search.Handler{DB: svc.Database})
	RegisterRoute(g, defaultMiddlewares, &appointments_get_status_history.Handler{DB: svc.Database})
	RegisterRoute(g, defaultMiddlewares, &appointments_update_status.Handler{DB: svc.Database, Whatsapp: svc.Whatsapp})

	// v1/onboarding
	RegisterRoute(g, defaultMiddlewares, &onboarding_get_progress.Handler{DB: svc.Database})
	RegisterRoute(g, defaultMiddlewares, &onboarding_get_templates.Handler{DB: svc.Database})
	RegisterRoute(g, defaultMiddlewares, &onboarding_step_barber.Handler{DB: svc.Database})
	RegisterRoute(g, defaultMiddlewares, &onboarding_step_services.Handler{DB: svc.Database})
	RegisterRoute(g, defaultMiddlewares, &onboarding_step_whatsapp.Handler{DB: svc.Database, Mailer: svc.Mailer, AdminEmail: svc.AdminEmail})

	// v1/customers
	RegisterRoute(g, defaultMiddlewares, &customers_list.Handler{DB: svc.Database})
	RegisterRoute(g, defaultMiddlewares, &customers_get.Handler{DB: svc.Database})
	RegisterRoute(g, defaultMiddlewares, &customers_block.Handler{DB: svc.Database})
	RegisterRoute(g, defaultMiddlewares, &customers_unblock.Handler{DB: svc.Database})

	// v1/resources
	RegisterRoute(g, defaultMiddlewares, &resources_list.Handler{DB: svc.Database})
	RegisterRoute(g, defaultMiddlewares, &resources_get.Handler{DB: svc.Database})
	RegisterRoute(g, defaultMiddlewares, &resources_create.Handler{DB: svc.Database})
	RegisterRoute(g, defaultMiddlewares, &resources_update.Handler{DB: svc.Database})
	RegisterRoute(g, defaultMiddlewares, &resources_delete.Handler{DB: svc.Database})
	RegisterRoute(g, defaultMiddlewares, &resources_update_sort_order.Handler{DB: svc.Database})
	RegisterRoute(g, defaultMiddlewares, &resources_upsert_working_hours.Handler{DB: svc.Database})
	RegisterRoute(g, defaultMiddlewares, &resources_delete_working_hours.Handler{DB: svc.Database})
	RegisterRoute(g, defaultMiddlewares, &resources_list_overrides.Handler{DB: svc.Database})
	RegisterRoute(g, defaultMiddlewares, &resources_create_override.Handler{DB: svc.Database})
	RegisterRoute(g, defaultMiddlewares, &resources_delete_override.Handler{DB: svc.Database})
	RegisterRoute(g, defaultMiddlewares, &resources_assign_services.Handler{DB: svc.Database})
	RegisterRoute(g, defaultMiddlewares, &resources_get_services.Handler{DB: svc.Database})

	// v1/admin
	adminMiddlewares := append(defaultMiddlewares, func(c *gin.Context) {
		if c.GetString("role") != "admin" {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		}
		c.Next()
	})

	RegisterRoute(g, adminMiddlewares, &admin_find_pending_activations.Handler{DB: svc.Database})
	RegisterRoute(g, adminMiddlewares, &admin_activate_tenant.Handler{DB: svc.Database, Mailer: svc.Mailer})

	// webhooks
	RegisterRoute(g, defaultMiddlewares, &webhooks_verify_webhook.Handler{})

	processWebhookMiddlewares := append(defaultMiddlewares, middleware.WhatsAppSignature(svc.AppSecret))
	RegisterRoute(g, processWebhookMiddlewares, &webhooks_process_webhook.Handler{
		DB:           svc.Database,
		StateMachine: svc.StateMachine,
	})
}

func RegisterRoute(g *gin.Engine, middlewares []gin.HandlerFunc, route Route) {
	path := route.Path()
	method := route.Method()
	handleFn := route.Handle

	g.Use(middlewares...).Handle(method, path, handleFn)
}
