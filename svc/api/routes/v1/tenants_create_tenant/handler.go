package tenants_create_tenant

import (
	"fmt"
	"math/rand"
	"net/http"
	"regexp"
	"strings"
	"time"
	"wappiz/pkg/db"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Request struct {
	Name string `json:"name" binding:"required"`
}

type Handler struct {
	DB db.Database
}

func (h *Handler) Method() string {
	return http.MethodPost
}

func (h *Handler) Path() string {
	return "/v1/tenants"
}

func (h *Handler) Handle(c *gin.Context) {
	var req Request
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID := c.MustGet("user_id").(string)
	base := slugify(req.Name)
	tenantID := uuid.New()

	txx, err := h.DB.Primary().Begin(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	defer txx.Rollback()

	for attempt := range slugMaxRetries {
		slug := base
		if attempt > 0 {
			slug = fmt.Sprintf("%s-%s", base, randomSuffix(5))
		}

		err = db.Query.InsertTenant(c.Request.Context(), txx, db.InsertTenantParams{
			ID:           tenantID,
			Name:         req.Name,
			Slug:         slug,
			Timezone:     "America/Bogota",
			Currency:     "COP",
			Plan:         "free",
			MonthResetAt: time.Time{},
			Settings:     nil,
		})

		if err == nil {
			break
		}
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	err = db.Query.LinkTenantUser(c.Request.Context(), txx, db.LinkTenantUserParams{
		TenantID: tenantID,
		UserID:   userID,
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	txx.Commit()

	c.JSON(http.StatusCreated, gin.H{"tenant": userID})
}

const slugAlphabet = "abcdefghijklmnopqrstuvwxyz0123456789"
const slugConstraint = "tenants_slug_key"
const slugMaxRetries = 5

var nonAlphanumeric = regexp.MustCompile(`[^a-z0-9]+`)

func slugify(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
	s = nonAlphanumeric.ReplaceAllString(s, "-")
	return strings.Trim(s, "-")
}

func randomSuffix(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = slugAlphabet[rand.Intn(len(slugAlphabet))]
	}
	return string(b)
}
