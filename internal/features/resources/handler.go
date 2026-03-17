package resources

import (
	"net/http"
	"time"
	"wappiz/internal/shared/jwt"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Handler struct {
	useCases *UseCases
}

func NewHandler(uc *UseCases) *Handler {
	return &Handler{useCases: uc}
}

func (h *Handler) RegisterRoutes(r *gin.Engine) {
	g := r.Group("/api/v1/resources")
	g.Use(jwt.AuthMiddleware())
	{
		g.GET("", h.List)
		g.POST("", h.Create)
		g.GET("/:id", h.Get)
		g.PUT("/:id", h.Update)
		g.DELETE("/:id", h.Delete)
		g.PUT("/sort-order", h.UpdateSortOrder)

		// Working Hours
		g.PUT("/:id/working-hours", h.UpsertWorkingHours)
		g.DELETE("/:id/working-hours/:whid", h.DeleteWorkingHours)

		// Schedule Overrides
		g.GET("/:id/overrides", h.ListOverrides)
		g.POST("/:id/overrides", h.CreateOverride)
		g.DELETE("/:id/overrides/:oid", h.DeleteOverride)

		// Service Assignments
		g.PUT("/:id/services", h.AssignServices)
		g.GET("/:id/services", h.GetServices)
	}
}

// ── Request / Response ────────────────────────────────────────────

type createResourceRequest struct {
	Name      string `json:"name"      binding:"required,min=2"`
	Type      string `json:"type"      binding:"required"`
	AvatarURL string `json:"avatarUrl"`
}

type updateResourceRequest struct {
	Name      string `json:"name"      binding:"required,min=2"`
	Type      string `json:"type"      binding:"required"`
	AvatarURL string `json:"avatarUrl"`
	SortOrder int    `json:"sortOrder"`
}

type sortOrderRequest struct {
	Order []struct {
		ID        uuid.UUID `json:"id"        binding:"required"`
		SortOrder int       `json:"sortOrder"`
	} `json:"order" binding:"required,min=1"`
}

type upsertWorkingHoursRequest struct {
	DayOfWeek int    `json:"dayOfWeek" binding:"min=0,max=6"`
	StartTime string `json:"startTime" binding:"required"`
	EndTime   string `json:"endTime"   binding:"required"`
	IsActive  bool   `json:"isActive"`
}

type createOverrideRequest struct {
	Date      string  `json:"date"      binding:"required"` // "2025-03-15"
	IsDayOff  bool    `json:"isDayOff"`
	StartTime *string `json:"startTime"`
	EndTime   *string `json:"endTime"`
	Reason    string  `json:"reason"`
}

type assignServicesRequest struct {
	ServiceIDs []uuid.UUID `json:"serviceIds" binding:"required"`
}

type resourceResponse struct {
	ID           uuid.UUID              `json:"id"`
	Name         string                 `json:"name"`
	Type         string                 `json:"type"`
	AvatarURL    string                 `json:"avatarUrl"`
	SortOrder    int                    `json:"sortOrder"`
	WorkingHours []workingHoursResponse `json:"workingHours"`
}

type workingHoursResponse struct {
	ID        uuid.UUID `json:"id"`
	DayOfWeek int       `json:"dayOfWeek"`
	DayName   string    `json:"dayName"`
	StartTime string    `json:"startTime"`
	EndTime   string    `json:"endTime"`
	IsActive  bool      `json:"isActive"`
}

type serviceResponse struct {
	ID              string  `json:"id"`
	Name            string  `json:"name"`
	Description     string  `json:"description"`
	DurationMinutes int     `json:"durationMinutes"`
	BufferMinutes   int     `json:"bufferMinutes"`
	Price           float64 `json:"price"`
	SortOrder       int     `json:"sortOrder"`
}

type overrideResponse struct {
	ID        uuid.UUID `json:"id"`
	Date      string    `json:"date"`
	IsDayOff  bool      `json:"isDayOff"`
	StartTime *string   `json:"startTime"`
	EndTime   *string   `json:"endTime"`
	Reason    string    `json:"reason"`
}

func toResourceResponse(res Resource) resourceResponse {
	wh := make([]workingHoursResponse, len(res.WorkingHours))
	for i, h := range res.WorkingHours {
		wh[i] = workingHoursResponse{
			ID:        h.ID,
			DayOfWeek: h.DayOfWeek,
			DayName:   h.DayName(),
			StartTime: h.StartTime,
			EndTime:   h.EndTime,
			IsActive:  h.IsActive,
		}
	}
	return resourceResponse{
		ID:           res.ID,
		Name:         res.Name,
		Type:         string(res.Type),
		AvatarURL:    res.AvatarURL,
		SortOrder:    res.SortOrder,
		WorkingHours: wh,
	}
}

func toOverrideResponse(so ScheduleOverride) overrideResponse {
	return overrideResponse{
		ID:        so.ID,
		Date:      so.Date.Format("2006-01-02"),
		IsDayOff:  so.IsDayOff,
		StartTime: so.StartTime,
		EndTime:   so.EndTime,
		Reason:    so.Reason,
	}
}

// ── Resource CRUD Handlers ────────────────────────────────────────

func (h *Handler) List(c *gin.Context) {
	tenantID := jwt.TenantIDFromContext(c)

	resources, err := h.useCases.GetAll(c.Request.Context(), tenantID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch resources"})
		return
	}

	result := make([]resourceResponse, len(resources))
	for i, r := range resources {
		result[i] = toResourceResponse(r)
	}
	c.JSON(http.StatusOK, result)
}

func (h *Handler) Get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid resource id"})
		return
	}

	tenantID := jwt.TenantIDFromContext(c)

	res, err := h.useCases.GetByID(c.Request.Context(), id, tenantID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "resource not found"})
		return
	}

	c.JSON(http.StatusOK, toResourceResponse(*res))
}

func (h *Handler) Create(c *gin.Context) {
	var req createResourceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tenantID := jwt.TenantIDFromContext(c)

	res, err := h.useCases.Create(c.Request.Context(), CreateResourceInput{
		TenantID:  tenantID,
		Name:      req.Name,
		Type:      ResourceType(req.Type),
		AvatarURL: req.AvatarURL,
	})
	if err != nil {
		statusCode := http.StatusInternalServerError
		if isValidationError(err) {
			statusCode = http.StatusBadRequest
		}
		c.JSON(statusCode, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, toResourceResponse(*res))
}

func (h *Handler) Update(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid resource id"})
		return
	}

	var req updateResourceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tenantID := jwt.TenantIDFromContext(c)

	res, err := h.useCases.Update(c.Request.Context(), UpdateResourceInput{
		ID:        id,
		TenantID:  tenantID,
		Name:      req.Name,
		Type:      ResourceType(req.Type),
		AvatarURL: req.AvatarURL,
		SortOrder: req.SortOrder,
	})
	if err != nil {
		statusCode := http.StatusInternalServerError
		if isValidationError(err) {
			statusCode = http.StatusBadRequest
		}
		c.JSON(statusCode, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, toResourceResponse(*res))
}

func (h *Handler) Delete(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid resource id"})
		return
	}

	tenantID := jwt.TenantIDFromContext(c)

	if err := h.useCases.Delete(c.Request.Context(), id, tenantID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete resource"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "resource deleted"})
}

func (h *Handler) UpdateSortOrder(c *gin.Context) {
	var req sortOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tenantID := jwt.TenantIDFromContext(c)

	items := make([]SortItem, len(req.Order))
	for i, o := range req.Order {
		items[i] = SortItem{ID: o.ID, SortOrder: o.SortOrder}
	}

	if err := h.useCases.UpdateSortOrder(c.Request.Context(), tenantID, items); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update sort order"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "sort order updated"})
}

// ── Working Hours Handlers ────────────────────────────────────────

func (h *Handler) UpsertWorkingHours(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid resource id"})
		return
	}

	var req upsertWorkingHoursRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tenantID := jwt.TenantIDFromContext(c)

	wh, err := h.useCases.UpsertWorkingHours(c.Request.Context(), UpsertWorkingHoursInput{
		ResourceID: id,
		TenantID:   tenantID,
		DayOfWeek:  req.DayOfWeek,
		StartTime:  req.StartTime,
		EndTime:    req.EndTime,
		IsActive:   req.IsActive,
	})
	if err != nil {
		statusCode := http.StatusInternalServerError
		if isValidationError(err) {
			statusCode = http.StatusBadRequest
		}
		c.JSON(statusCode, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, workingHoursResponse{
		ID:        wh.ID,
		DayOfWeek: wh.DayOfWeek,
		DayName:   wh.DayName(),
		StartTime: wh.StartTime,
		EndTime:   wh.EndTime,
		IsActive:  wh.IsActive,
	})
}

func (h *Handler) DeleteWorkingHours(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid resource id"})
		return
	}
	whID, err := uuid.Parse(c.Param("whid"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid working hours id"})
		return
	}

	tenantID := jwt.TenantIDFromContext(c)

	if err := h.useCases.DeleteWorkingHours(c.Request.Context(), whID, id, tenantID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete working hours"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "working hours deleted"})
}

// ── Schedule Override Handlers ────────────────────────────────────

func (h *Handler) ListOverrides(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid resource id"})
		return
	}

	// Rango por query params, default: próximos 30 días
	from := time.Now()
	to := from.AddDate(0, 0, 30)

	if fromStr := c.Query("from"); fromStr != "" {
		if t, err := time.Parse("2006-01-02", fromStr); err == nil {
			from = t
		}
	}
	if toStr := c.Query("to"); toStr != "" {
		if t, err := time.Parse("2006-01-02", toStr); err == nil {
			to = t
		}
	}

	tenantID := jwt.TenantIDFromContext(c)

	overrides, err := h.useCases.GetOverrides(c.Request.Context(), id, tenantID, from, to)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch overrides"})
		return
	}

	result := make([]overrideResponse, len(overrides))
	for i, o := range overrides {
		result[i] = toOverrideResponse(o)
	}
	c.JSON(http.StatusOK, result)
}

func (h *Handler) CreateOverride(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid resource id"})
		return
	}

	var req createOverrideRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	date, err := time.Parse("2006-01-02", req.Date)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "date must be in YYYY-MM-DD format"})
		return
	}

	tenantID := jwt.TenantIDFromContext(c)

	so, err := h.useCases.CreateOverride(c.Request.Context(), CreateOverrideInput{
		ResourceID: id,
		TenantID:   tenantID,
		Date:       date,
		IsDayOff:   req.IsDayOff,
		StartTime:  req.StartTime,
		EndTime:    req.EndTime,
		Reason:     req.Reason,
	})
	if err != nil {
		statusCode := http.StatusInternalServerError
		if isValidationError(err) {
			statusCode = http.StatusBadRequest
		}
		c.JSON(statusCode, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, toOverrideResponse(*so))
}

func (h *Handler) DeleteOverride(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid resource id"})
		return
	}
	oid, err := uuid.Parse(c.Param("oid"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid override id"})
		return
	}

	tenantID := jwt.TenantIDFromContext(c)

	if err := h.useCases.DeleteOverride(c.Request.Context(), oid, id, tenantID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete override"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "override deleted"})
}

// ── Service Assignment Handlers ───────────────────────────────────

func (h *Handler) AssignServices(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid resource id"})
		return
	}

	var req assignServicesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tenantID := jwt.TenantIDFromContext(c)

	if err := h.useCases.AssignServices(c.Request.Context(), id, tenantID, req.ServiceIDs); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to assign services"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "services assigned"})
}

func (h *Handler) GetServices(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid resource id"})
		return
	}

	tenantID := jwt.TenantIDFromContext(c)

	svcs, err := h.useCases.GetServices(c.Request.Context(), id, tenantID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch services"})
		return
	}

	result := make([]serviceResponse, len(svcs))
	for i, s := range svcs {
		result[i] = serviceResponse{
			ID:              s.ID.String(),
			Name:            s.Name,
			Description:     s.Description,
			DurationMinutes: s.DurationMinutes,
			BufferMinutes:   s.BufferMinutes,
			Price:           s.Price,
			SortOrder:       s.SortOrder,
		}
	}
	c.JSON(http.StatusOK, result)
}

func isValidationError(err error) bool {
	_, ok := err.(resourceError)
	return ok
}
