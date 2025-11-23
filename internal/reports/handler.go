package reports

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sharath018/temple-management-backend/internal/auditlog"
	"github.com/sharath018/temple-management-backend/middleware"
)

// Handler holds service & repo (repo used for entity lookups here)
type Handler struct {
	service  ReportService
	repo     ReportRepository
	auditSvc auditlog.Service
}

// NewHandler creates a new reports handler
func NewHandler(svc ReportService, repo ReportRepository, auditSvc auditlog.Service) *Handler {
	return &Handler{
		service:  svc,
		repo:     repo,
		auditSvc: auditSvc,
	}
}

// GetActivities handles requests for the activities report
func (h *Handler) GetActivities(c *gin.Context) {
	// Get access context from middleware
	accessContext, exists := c.Get("access_context")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "access context missing"})
		return
	}
	ctx := accessContext.(middleware.AccessContext)

	// Get IP address from context (set by AuditMiddleware)
	ip := middleware.GetIPFromContext(c)

	entityParam := c.Param("id") // either "all" or numeric id
	reportType := c.Query("type")
	if reportType == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "type query param required: events|sevas|bookings|donations"})
		return
	}
	dateRange := c.Query("date_range")
	if dateRange == "" {
		dateRange = DateRangeWeekly
	}
	startDateStr := c.Query("start_date")
	endDateStr := c.Query("end_date")
	format := c.Query("format") // excel, csv, pdf -> if empty return JSON

	// compute start & end
	start, end, err := GetDateRange(dateRange, startDateStr, endDateStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// resolve entity IDs based on access context
	var entityIDs []string
	var actualEntityParam string // Track the actual entity ID for the request
	var tenantID uint

	fmt.Println("individual:", entityParam)
	fmt.Println("individual:", ctx.DirectEntityID)
	if strings.ToLower(entityParam) == "all" {
		fmt.Println("all")
		actualEntityParam = "all" // Keep "all" for request tracking
		
		// Handle based on role
		switch ctx.RoleName {
case "superadmin":
			// When superadmin logs in as tenant, they should have AssignedEntityID set
			if ctx.AssignedEntityID != nil {
				tenantID = *ctx.AssignedEntityID
				ids, err := h.repo.GetEntitiesByTenant(tenantID)
				if err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch tenant entities"})
					return
				}
				if len(ids) == 0 {
					c.JSON(http.StatusOK, gin.H{"data": ReportData{}, "message": "No entities found for tenant"})
					return
				}
				for _, id := range ids {
					entityIDs = append(entityIDs, fmt.Sprint(id))
				}
			} else {
				// Pure superadmin without tenant context - should not happen for this endpoint
				c.JSON(http.StatusBadRequest, gin.H{"error": "superadmin must specify tenant context or use superadmin endpoints"})
				return
			}
		case "templeadmin":
			// Templeadmin can access their own entities - use their user ID as tenant ID
			tenantID = ctx.UserID
			ids, err := h.repo.GetEntitiesByTenant(tenantID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch user entities"})
				return
			}
			if len(ids) == 0 {
				c.JSON(http.StatusOK, gin.H{"data": ReportData{}, "message": "No entities found for tenant"})
				return
			}
			for _, id := range ids {
				entityIDs = append(entityIDs, fmt.Sprint(id))
			}
		case "standarduser", "monitoringuser":
			// standarduser/monitoringuser get all entities for their assigned tenant
			if ctx.AssignedEntityID == nil {
				c.JSON(http.StatusForbidden, gin.H{"error": "no accessible entity"})
				return
			}
			tenantID = *ctx.AssignedEntityID
			ids, err := h.repo.GetEntitiesByTenant(tenantID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch tenant entities"})
				return
			}
			if len(ids) == 0 {
				c.JSON(http.StatusOK, gin.H{"data": ReportData{}, "message": "No entities found for tenant"})
				return
			}
			for _, id := range ids {
				entityIDs = append(entityIDs, fmt.Sprint(id))
			}
		default:
			// Unknown role
			c.JSON(http.StatusForbidden, gin.H{"error": "role not authorized for this endpoint"})
			return
		}
	} else {
		// parse numeric entity id
		eid, err := strconv.ParseUint(entityParam, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid entity_id path param"})
			return
		}

		actualEntityParam = fmt.Sprint(eid)

		// verify user can access this specific entity
		if !h.canAccessEntity(ctx, uint(eid)) {
			c.JSON(http.StatusForbidden, gin.H{"error": "not authorized for this entity"})
			return
		}
		entityIDs = append(entityIDs, fmt.Sprint(eid))
	}

	req := ActivitiesReportRequest{
		EntityID:  actualEntityParam, // Use the properly resolved entity parameter
		Type:      reportType,
		DateRange: dateRange,
		StartDate: start,
		EndDate:   end,
		Format:    format,
		EntityIDs: entityIDs, // Pass the resolved entity IDs
	}

	// If no format -> return JSON preview
	if req.Format == "" {
		data, err := h.service.GetActivities(req)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		// Log report view (optional - for JSON preview)
		details := map[string]interface{}{
			"report_type": req.Type,
			"format":      "json_preview",
			"entity_id":   req.EntityID,
			"entity_ids":  req.EntityIDs,
			"date_range":  req.DateRange,
			"user_role":   ctx.RoleName,
		}
		h.auditSvc.LogAction(c.Request.Context(), &ctx.UserID, nil, "TEMPLE_ACTIVITIES_REPORT_VIEWED", details, ip, "success")

		c.JSON(http.StatusOK, data)
		return
	}
	fmt.Println("Calling ExportActivities")

	// Else export file (format present)
	bytes, fname, mime, err := h.service.ExportActivities(c.Request.Context(), req, &ctx.UserID, ip)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", fname))
	c.Data(http.StatusOK, mime, bytes)
}
// GetSuperAdminActivities handles activities reports with multiple tenant IDs
func (h *Handler) GetSuperAdminActivities(c *gin.Context) {
	// Get access context from middleware
	accessContext, exists := c.Get("access_context")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "access context missing"})
		return
	}
	ctx := accessContext.(middleware.AccessContext)

	// Ensure superadmin role
	if ctx.RoleName != "superadmin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "only superadmin can access this endpoint"})
		return
	}

	// Get IP address from context
	ip := middleware.GetIPFromContext(c)

	// Get request parameters
	reportType := c.Query("type")
	if reportType == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "type query param required: events|sevas|bookings|donations"})
		return
	}

	tenantsParam := c.Query("tenants")
	if tenantsParam == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tenants query param required (comma-separated tenant IDs)"})
		return
	}

	// Clean and validate tenant IDs
	tenantIDStrs := strings.Split(strings.TrimSpace(tenantsParam), ",")
	var validTenantIDs []uint
	for _, tenantIDStr := range tenantIDStrs {
		tenantIDStr = strings.TrimSpace(tenantIDStr)
		if tenantIDStr == "" {
			continue
		}
		tenantID, err := strconv.ParseUint(tenantIDStr, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("invalid tenant ID: %s", tenantIDStr)})
			return
		}
		validTenantIDs = append(validTenantIDs, uint(tenantID))
	}

	if len(validTenantIDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no valid tenant IDs provided"})
		return
	}

	dateRange := c.Query("date_range")
	if dateRange == "" {
		dateRange = DateRangeWeekly
	}
	startDateStr := c.Query("start_date")
	endDateStr := c.Query("end_date")
	format := c.Query("format") // excel, csv, pdf -> if empty return JSON

	// Compute date range
	start, end, err := GetDateRange(dateRange, startDateStr, endDateStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Collect entity IDs for all specified tenants
	var allEntityIDs []string
	tenantsWithEntities := make(map[string][]string) // Track which tenants have entities

	for _, tenantID := range validTenantIDs {
		// Get entities for this tenant
		entityIDs, err := h.repo.GetEntitiesByTenant(tenantID)
		if err != nil {
			// Log the error but continue with other tenants
			fmt.Printf("Warning: failed to fetch entities for tenant %d: %v\n", tenantID, err)
			continue
		}

		if len(entityIDs) > 0 {
			tenantEntityStrs := make([]string, 0, len(entityIDs))
			// Add to the collection
			for _, entityID := range entityIDs {
				entityIDStr := fmt.Sprint(entityID)
				allEntityIDs = append(allEntityIDs, entityIDStr)
				tenantEntityStrs = append(tenantEntityStrs, entityIDStr)
			}
			tenantsWithEntities[fmt.Sprint(tenantID)] = tenantEntityStrs
		}
	}

	if len(allEntityIDs) == 0 {
		c.JSON(http.StatusOK, gin.H{
			"data":         ReportData{},
			"message":      "No entities found for the specified tenants",
			"tenant_count": len(validTenantIDs),
		})
		return
	}

	// Create request object
	req := ActivitiesReportRequest{
		EntityID:  fmt.Sprintf("multiple_tenants_%s", strings.Join(tenantIDStrs, "_")), // Clear identifier for multiple tenants
		Type:      reportType,
		DateRange: dateRange,
		StartDate: start,
		EndDate:   end,
		Format:    format,
		EntityIDs: allEntityIDs,
	}

	// If no format -> return JSON preview
	if req.Format == "" {
		data, err := h.service.GetActivities(req)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		// Log report view
		details := map[string]interface{}{
			"report_type":           req.Type,
			"format":                "json_preview",
			"requested_tenant_ids":  tenantIDStrs,
			"tenants_with_entities": tenantsWithEntities,
			"total_entity_count":    len(req.EntityIDs),
			"date_range":            req.DateRange,
		}
		h.auditSvc.LogAction(c.Request.Context(), &ctx.UserID, nil, "SUPERADMIN_ACTIVITIES_REPORT_VIEWED", details, ip, "success")

		c.JSON(http.StatusOK, data)
		return
	}

	// Else export file (format present)
	bytes, fname, mime, err := h.service.ExportActivities(c.Request.Context(), req, &ctx.UserID, ip)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", fname))
	c.Data(http.StatusOK, mime, bytes)
}

// GetSuperAdminTenantActivities handles single tenant reports for superadmin
func (h *Handler) GetSuperAdminTenantActivities(c *gin.Context) {
	// Get access context from middleware
	accessContext, exists := c.Get("access_context")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "access context missing"})
		return
	}
	ctx := accessContext.(middleware.AccessContext)

	// Ensure superadmin role
	if ctx.RoleName != "superadmin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "only superadmin can access this endpoint"})
		return
	}

	// Get IP address from context
	ip := middleware.GetIPFromContext(c)

	// Get request parameters
	tenantIDParam := c.Param("id") // This should be the tenant ID from the URL path
	if tenantIDParam == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tenant ID is required in URL path"})
		return
	}

	reportType := c.Query("type")
	if reportType == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "type query param required: events|sevas|bookings|donations"})
		return
	}

	dateRange := c.Query("date_range")
	if dateRange == "" {
		dateRange = DateRangeWeekly
	}
	startDateStr := c.Query("start_date")
	endDateStr := c.Query("end_date")
	format := c.Query("format") // excel, csv, pdf -> if empty return JSON

	// Compute date range
	start, end, err := GetDateRange(dateRange, startDateStr, endDateStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Convert tenant ID to uint - this is the actual tenant ID
	tenantIDUint, err := strconv.ParseUint(tenantIDParam, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid tenant ID format"})
		return
	}

	// Get entities for this specific tenant
	entityIDs, err := h.repo.GetEntitiesByTenant(uint(tenantIDUint))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":     "failed to fetch tenant entities",
			"tenant_id": tenantIDParam,
		})
		return
	}

	if len(entityIDs) == 0 {
		c.JSON(http.StatusOK, gin.H{
			"data":      ReportData{},
			"message":   "No entities found for the specified tenant",
			"tenant_id": tenantIDParam,
		})
		return
	}

	// Convert entity IDs to strings
	entityIDStrs := make([]string, 0, len(entityIDs))
	for _, id := range entityIDs {
		entityIDStrs = append(entityIDStrs, fmt.Sprint(id))
	}

	// Create request object with proper tenant context
	req := ActivitiesReportRequest{
		EntityID:  fmt.Sprintf("tenant_%s", tenantIDParam), // Clear identifier that this is for a specific tenant
		Type:      reportType,
		DateRange: dateRange,
		StartDate: start,
		EndDate:   end,
		Format:    format,
		EntityIDs: entityIDStrs, // All entities belonging to this tenant
		// If your struct supports it, you might want to add:
		// TenantID: uint(tenantIDUint),
	}

	// If no format -> return JSON preview
	if req.Format == "" {
		data, err := h.service.GetActivities(req)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		// Log report view with proper tenant context
		details := map[string]interface{}{
			"report_type":   req.Type,
			"format":        "json_preview",
			"tenant_id":     tenantIDParam,
			"tenant_id_int": uint(tenantIDUint),
			"entity_ids":    req.EntityIDs,
			"entity_count":  len(req.EntityIDs),
			"date_range":    req.DateRange,
		}
		h.auditSvc.LogAction(c.Request.Context(), &ctx.UserID, nil, "SUPERADMIN_TENANT_ACTIVITIES_REPORT_VIEWED", details, ip, "success")

		c.JSON(http.StatusOK, data)
		return
	}

	// Else export file (format present)
	bytes, fname, mime, err := h.service.ExportActivities(c.Request.Context(), req, &ctx.UserID, ip)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", fname))
	c.Data(http.StatusOK, mime, bytes)
}

func (h *Handler) GetTempleRegisteredReport(c *gin.Context) {
	fmt.Println("entering GetTempleRegisteredReport1:")
	// Get access context from middleware
	accessContext, exists := c.Get("access_context")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "access context missing"})
		return
	}
	fmt.Println("entering GetTempleRegisteredReport2:")
	ctx := accessContext.(middleware.AccessContext)

	// Get IP address from context (set by AuditMiddleware)
	ip := middleware.GetIPFromContext(c)

	entityParam := c.Param("id") // "all" or specific entity id

	dateRange := c.Query("date_range")
	if dateRange == "" {
		dateRange = DateRangeWeekly
	}
	startDateStr := c.Query("start_date")
	endDateStr := c.Query("end_date")
	status := c.Query("status") // approve|rejected|pending
	format := c.Query("format")

	start, end, err := GetDateRange(dateRange, startDateStr, endDateStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	fmt.Println("entering GetTempleRegisteredReport3:")
	// Resolve entity IDs based on access context
	var entityIDs []string
	var actualEntityID string // Track the actual entity ID for the request
	var tenantID uint

	fmt.Println("individual:", entityParam)
	fmt.Println("individual:", ctx.DirectEntityID)
	if strings.ToLower(entityParam) == "all" {
		fmt.Println("all")
		actualEntityID = "all" // Keep "all" for request tracking
		
		// Handle based on role
		switch ctx.RoleName {
case "superadmin":
			// When superadmin logs in as tenant, they should have AssignedEntityID set
			if ctx.AssignedEntityID != nil {
				tenantID = *ctx.AssignedEntityID
				ids, err := h.repo.GetEntitiesByTenant(tenantID)
				if err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch tenant entities"})
					return
				}
				if len(ids) == 0 {
					c.JSON(http.StatusOK, gin.H{"data": []TempleRegisteredReportRow{}})
					return
				}
				for _, id := range ids {
					entityIDs = append(entityIDs, fmt.Sprint(id))
				}
			} else {
				// Pure superadmin without tenant context - should not happen for this endpoint
				c.JSON(http.StatusBadRequest, gin.H{"error": "superadmin must specify tenant context or use superadmin endpoints"})
				return
			}
		case "templeadmin":
			// For templeadmin, use their user ID as tenant ID
			tenantID = ctx.UserID
			ids, err := h.repo.GetEntitiesByTenant(tenantID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch user entities"})
				return
			}
			if len(ids) == 0 {
				c.JSON(http.StatusOK, gin.H{"data": []TempleRegisteredReportRow{}})
				return
			}
			for _, id := range ids {
				entityIDs = append(entityIDs, fmt.Sprint(id))
			}
		case "standarduser", "monitoringuser":
			// standarduser/monitoringuser use their assigned entity as tenant
			tenantID = *ctx.AssignedEntityID
			ids, err := h.repo.GetEntitiesByTenant(tenantID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch user entities"})
				return
			}
			if len(ids) == 0 {
				c.JSON(http.StatusOK, gin.H{"data": []TempleRegisteredReportRow{}})
				return
			}
			for _, id := range ids {
				entityIDs = append(entityIDs, fmt.Sprint(id))
			}
		default:
			c.JSON(http.StatusForbidden, gin.H{"error": "role not authorized for this endpoint"})
			return
		}
	} else {
		fmt.Println("entering GetTempleRegisteredReport4:")
		eid, err := strconv.ParseUint(entityParam, 10, 64)
		fmt.Println("parse unit:", eid, entityParam)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid entity_id path param"})
			return
		}

		if !h.canAccessEntity(ctx, uint(eid)) {
			c.JSON(http.StatusForbidden, gin.H{"error": "not authorized for this entity"})
			return
		}
		entityIDs = append(entityIDs, fmt.Sprint(eid))
		actualEntityID = entityParam
	}

	req := TempleRegisteredReportRequest{
		DateRange: dateRange,
		StartDate: start,
		EndDate:   end,
		Status:    status,
		Format:    format,
		EntityID:  actualEntityID, // Use the actual entity ID parameter
	}

	// The 'format' query parameter determines the report type for the exporter.
	var reportType string
	switch format {
	case "excel":
		reportType = ReportTypeTempleRegisteredExcel
	case "pdf":
		reportType = ReportTypeTempleRegisteredPDF
	case "csv": // Explicitly handle csv for clarity
		reportType = ReportTypeTempleRegistered
	default:
		fmt.Println("------>DEFAULT")
		// If no format is specified, return JSON preview
		data, err := h.service.GetTempleRegisteredReport(req, entityIDs)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		// Log report view (optional - for JSON preview)
		details := map[string]interface{}{
			"report_type":  "temple_registered",
			"format":       "json_preview",
			"entity_ids":   entityIDs,
			"entity_param": entityParam,
			"status":       status,
			"date_range":   dateRange,
		}
		h.auditSvc.LogAction(c.Request.Context(), &ctx.UserID, nil, "TEMPLE_REGISTER_REPORT_VIEWED", details, ip, "success")

		c.JSON(http.StatusOK, data)
		return
	}

	// Export file (format is present)
	bytes, fname, mime, err := h.service.ExportTempleRegisteredReport(c.Request.Context(), req, entityIDs, reportType, &ctx.UserID, ip)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", fname))
	c.Data(http.StatusOK, mime, bytes)
}
// GetSuperAdminTempleRegisteredReport handles temple registered report for superadmin with multiple tenants
func (h *Handler) GetSuperAdminTempleRegisteredReport(c *gin.Context) {
	// Get access context from middleware
	accessContext, exists := c.Get("access_context")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "access context missing"})
		return
	}
	ctx := accessContext.(middleware.AccessContext)

	// Ensure superadmin role
	if ctx.RoleName != "superadmin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "only superadmin can access this endpoint"})
		return
	}

	// Get IP address from context
	ip := middleware.GetIPFromContext(c)

	// Get request parameters
	tenantsParam := c.Query("tenants")
	if tenantsParam == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tenants query param required"})
		return
	}

	tenantIDs := strings.Split(tenantsParam, ",")

	dateRange := c.Query("date_range")
	if dateRange == "" {
		dateRange = DateRangeWeekly
	}
	startDateStr := c.Query("start_date")
	endDateStr := c.Query("end_date")
	status := c.Query("status") // approve|rejected|pending
	format := c.Query("format")

	// Compute date range
	start, end, err := GetDateRange(dateRange, startDateStr, endDateStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Collect entity IDs for all specified tenants
	var allEntityIDs []string
	var validTenantIDs []string // Track which tenants were successfully processed

	for _, tenantIDStr := range tenantIDs {
		tenantIDStr = strings.TrimSpace(tenantIDStr) // Clean whitespace
		if tenantIDStr == "" {
			continue
		}

		tenantID, err := strconv.ParseUint(tenantIDStr, 10, 64)
		if err != nil {
			continue // Skip invalid tenant IDs
		}

		// Get entities for this tenant
		entityIDs, err := h.repo.GetEntitiesByTenant(uint(tenantID))
		if err != nil {
			continue // Skip if there's an error fetching entities
		}

		// Only add to valid tenants if entities were found
		if len(entityIDs) > 0 {
			validTenantIDs = append(validTenantIDs, tenantIDStr)
			// Add to the collection
			for _, entityID := range entityIDs {
				allEntityIDs = append(allEntityIDs, fmt.Sprint(entityID))
			}
		}
	}

	if len(allEntityIDs) == 0 {
		c.JSON(http.StatusOK, gin.H{"data": []TempleRegisteredReportRow{}, "message": "No entities found for the specified tenants"})
		return
	}

	// Create request object with proper tenant information
	req := TempleRegisteredReportRequest{
		DateRange: dateRange,
		StartDate: start,
		EndDate:   end,
		Status:    status,
		Format:    format,
		EntityID:  strings.Join(validTenantIDs, ","), // Pass valid tenant IDs as entity identifier
	}

	// The 'format' query parameter determines the report type for the exporter.
	var reportType string
	switch format {
	case "excel":
		reportType = ReportTypeTempleRegisteredExcel
	case "pdf":
		reportType = ReportTypeTempleRegisteredPDF
	case "csv": // Explicitly handle csv for clarity
		reportType = ReportTypeTempleRegistered
	default:
		// If no format is specified, return JSON preview
		data, err := h.service.GetTempleRegisteredReport(req, allEntityIDs)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		// Log report view
		details := map[string]interface{}{
			"report_type": "temple_registered",
			"format":      "json_preview",
			"tenant_ids":  validTenantIDs, // Use valid tenant IDs
			"entity_ids":  allEntityIDs,
			"status":      status,
			"date_range":  dateRange,
		}
		h.auditSvc.LogAction(c.Request.Context(), &ctx.UserID, nil, "SUPERADMIN_TEMPLE_REGISTER_REPORT_VIEWED", details, ip, "success")

		c.JSON(http.StatusOK, data)
		return
	}

	// Export file (format is present)
	bytes, fname, mime, err := h.service.ExportTempleRegisteredReport(c.Request.Context(), req, allEntityIDs, reportType, &ctx.UserID, ip)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", fname))
	c.Data(http.StatusOK, mime, bytes)
}

// GetSuperAdminTenantTempleRegisteredReport handles temple registered report for a single tenant by superadmin
func (h *Handler) GetSuperAdminTenantTempleRegisteredReport(c *gin.Context) {
	// Get access context from middleware
	accessContext, exists := c.Get("access_context")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "access context missing"})
		return
	}
	ctx := accessContext.(middleware.AccessContext)

	// Ensure superadmin role
	if ctx.RoleName != "superadmin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "only superadmin can access this endpoint"})
		return
	}

	// Get IP address from context
	ip := middleware.GetIPFromContext(c)

	// Get request parameters - tenant ID from path parameter
	tenantID := c.Param("id")
	if tenantID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tenant ID is required"})
		return
	}

	dateRange := c.Query("date_range")
	if dateRange == "" {
		dateRange = DateRangeWeekly
	}
	startDateStr := c.Query("start_date")
	endDateStr := c.Query("end_date")
	status := c.Query("status") // approve|rejected|pending
	format := c.Query("format")

	// Compute date range
	start, end, err := GetDateRange(dateRange, startDateStr, endDateStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Convert tenant ID to uint
	tenantIDUint, err := strconv.ParseUint(tenantID, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid tenant ID"})
		return
	}

	// Get entities for this tenant
	entityIDs, err := h.repo.GetEntitiesByTenant(uint(tenantIDUint))
	fmt.Println("entityIDd:", entityIDs)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch tenant entities"})
		return
	}

	if len(entityIDs) == 0 {
		c.JSON(http.StatusOK, gin.H{"data": []TempleRegisteredReportRow{}, "message": "No entities found for the specified tenant"})
		return
	}

	// Convert entity IDs to strings
	entityIDStrs := make([]string, 0, len(entityIDs))
	for _, id := range entityIDs {
		entityIDStrs = append(entityIDStrs, fmt.Sprint(id))
	}

	// Create request object - EntityID should represent what we're querying for
	req := TempleRegisteredReportRequest{
		DateRange: dateRange,
		StartDate: start,
		EndDate:   end,
		Status:    status,
		Format:    format,
		EntityID:  fmt.Sprintf("tenant_%s", tenantID), // Clearly indicate this is for a specific tenant
	}

	// The 'format' query parameter determines the report type for the exporter.
	var reportType string
	switch format {
	case "excel":
		reportType = ReportTypeTempleRegisteredExcel
	case "pdf":
		reportType = ReportTypeTempleRegisteredPDF
	case "csv": // Explicitly handle csv for clarity
		reportType = ReportTypeTempleRegistered
	default:
		// If no format is specified, return JSON preview
		data, err := h.service.GetTempleRegisteredReport(req, entityIDStrs)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		// Log report view
		details := map[string]interface{}{
			"report_type": "temple_registered",
			"format":      "json_preview",
			"tenant_id":   tenantID,
			"entity_ids":  entityIDStrs,
			"status":      status,
			"date_range":  dateRange,
		}
		h.auditSvc.LogAction(c.Request.Context(), &ctx.UserID, nil, "SUPERADMIN_TENANT_TEMPLE_REGISTER_REPORT_VIEWED", details, ip, "success")

		c.JSON(http.StatusOK, data)
		return
	}

	// Export file (format is present)
	bytes, fname, mime, err := h.service.ExportTempleRegisteredReport(c.Request.Context(), req, entityIDStrs, reportType, &ctx.UserID, ip)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", fname))
	c.Data(http.StatusOK, mime, bytes)
}

// GetDevoteeBirthdaysReport handles devotee birthdays report for regular users
func (h *Handler) GetDevoteeBirthdaysReport(c *gin.Context) {
	// Get access context from middleware
	accessContext, exists := c.Get("access_context")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "access context missing"})
		return
	}
	ctx := accessContext.(middleware.AccessContext)

	// Get IP address from context (set by AuditMiddleware)
	ip := middleware.GetIPFromContext(c)

	entityParam := c.Param("id") // "all" or specific entity id

	dateRange := c.Query("date_range")
	if dateRange == "" {
		dateRange = DateRangeWeekly
	}
	startDateStr := c.Query("start_date")
	endDateStr := c.Query("end_date")
	format := c.Query("format")

	start, end, err := GetDateRange(dateRange, startDateStr, endDateStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// DEBUG: Log date range
	fmt.Printf("[BIRTHDAY REPORT] Date Range: %s, Start: %s, End: %s\n", 
		dateRange, start.Format("2006-01-02"), end.Format("2006-01-02"))

	// Resolve entity IDs based on access context
	var entityIDs []string
	var actualEntityParam string
	var ids []uint

	if strings.ToLower(entityParam) == "all" {
		actualEntityParam = "all"
		switch ctx.RoleName {
		case "superadmin":
			// When superadmin logs in as tenant, use assigned tenant
			if ctx.AssignedEntityID != nil {
				ids, err = h.repo.GetEntitiesByTenant(*ctx.AssignedEntityID)
				if err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch tenant entities"})
					return
				}
			} else {
				// Pure superadmin without tenant context - should not happen for this endpoint
				c.JSON(http.StatusBadRequest, gin.H{"error": "superadmin must specify tenant context or use superadmin endpoints"})
				return
			}
		case "templeadmin":
			ids, err = h.repo.GetEntitiesByTenant(ctx.UserID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch temple admin entities"})
				return
			}
		case "standarduser", "monitoringuser":
			if ctx.AssignedEntityID == nil {
				c.JSON(http.StatusForbidden, gin.H{"error": "no assigned entity"})
				return
			}
			ids, err = h.repo.GetEntitiesByTenant(*ctx.AssignedEntityID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch tenant entities"})
				return
			}
		default:
			c.JSON(http.StatusForbidden, gin.H{"error": "role not authorized for this endpoint"})
			return
		}

		if len(ids) == 0 {
			c.JSON(http.StatusOK, gin.H{
				"data":    []DevoteeBirthdayReportRow{},
				"message": "No entities found for user",
			})
			return
		}

		for _, id := range ids {
			entityIDs = append(entityIDs, fmt.Sprint(id))
		}
	} else {
		eid, err := strconv.ParseUint(entityParam, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid entity_id path param"})
			return
		}

		actualEntityParam = fmt.Sprint(eid)

		if !h.canAccessEntity(ctx, uint(eid)) {
			c.JSON(http.StatusForbidden, gin.H{"error": "not authorized for this entity"})
			return
		}
		entityIDs = append(entityIDs, fmt.Sprint(eid))
	}

	// DEBUG: Log entity IDs
	fmt.Printf("[BIRTHDAY REPORT] Entity IDs: %v, Role: %s, Format: %s\n", 
		entityIDs, ctx.RoleName, format)

	req := DevoteeBirthdaysReportRequest{
		DateRange: dateRange,
		StartDate: start,
		EndDate:   end,
		Format:    format,
		EntityID:  actualEntityParam,
	}

	// Determine report type
	var reportType string
	switch format {
	case "excel":
		reportType = ReportTypeDevoteeBirthdaysExcel
	case "pdf":
		reportType = ReportTypeDevoteeBirthdaysPDF
	case "csv":
		reportType = ReportTypeDevoteeBirthdays
	default:
		// JSON preview
		fmt.Println("[BIRTHDAY REPORT] Fetching JSON preview data...")
		data, err := h.service.GetDevoteeBirthdaysReport(req, entityIDs)
		if err != nil {
			fmt.Printf("[BIRTHDAY REPORT] Error fetching data: %v\n", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		fmt.Printf("[BIRTHDAY REPORT] JSON preview - Record count: %d\n", len(data))

		details := map[string]interface{}{
			"report_type":  "devotee_birthdays",
			"format":       "json_preview",
			"entity_ids":   entityIDs,
			"entity_param": entityParam,
			"date_range":   dateRange,
			"start_date":   start.Format("2006-01-02"),
			"end_date":     end.Format("2006-01-02"),
			"record_count": len(data),
		}
		h.auditSvc.LogAction(c.Request.Context(), &ctx.UserID, nil, "DEVOTEE_BIRTHDAYS_REPORT_VIEWED", details, ip, "success")

		c.JSON(http.StatusOK, gin.H{
			"data": data,
			"meta": gin.H{
				"entity_ids": entityIDs,
				"date_range": dateRange,
				"start_date": start.Format("2006-01-02"),
				"end_date":   end.Format("2006-01-02"),
			},
		})
		return
	}

	// Export report file
	fmt.Printf("[BIRTHDAY REPORT] Exporting file - Format: %s, ReportType: %s\n", format, reportType)
	fmt.Printf("[BIRTHDAY REPORT] Export request - EntityIDs: %v, DateRange: %s to %s\n", 
		entityIDs, start.Format("2006-01-02"), end.Format("2006-01-02"))
	
	bytes, fname, mime, err := h.service.ExportDevoteeBirthdaysReport(c.Request.Context(), req, entityIDs, reportType, &ctx.UserID, ip)
	if err != nil {
		fmt.Printf("[BIRTHDAY REPORT] Export error: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	fmt.Printf("[BIRTHDAY REPORT] Export successful - Filename: %s, Size: %d bytes, MIME: %s\n", 
		fname, len(bytes), mime)

	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", fname))
	c.Data(http.StatusOK, mime, bytes)
}

// GetSuperAdminDevoteeBirthdaysReport handles devotee birthdays report for superadmin with multiple tenants
func (h *Handler) GetSuperAdminDevoteeBirthdaysReport(c *gin.Context) {
	// Get access context from middleware
	accessContext, exists := c.Get("access_context")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "access context missing"})
		return
	}
	ctx := accessContext.(middleware.AccessContext)

	// Ensure superadmin role
	if ctx.RoleName != "superadmin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "only superadmin can access this endpoint"})
		return
	}

	// Get IP address from context
	ip := middleware.GetIPFromContext(c)

	// Get request parameters
	tenantsParam := c.Query("tenants")
	if tenantsParam == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tenants query param required"})
		return
	}

	tenantIDStrs := strings.Split(tenantsParam, ",")

	dateRange := c.Query("date_range")
	if dateRange == "" {
		dateRange = DateRangeWeekly
	}
	startDateStr := c.Query("start_date")
	endDateStr := c.Query("end_date")
	format := c.Query("format")

	// Compute date range
	start, end, err := GetDateRange(dateRange, startDateStr, endDateStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Collect entity IDs for all specified tenants
	var allEntityIDs []string
	var validTenantIDs []string

	for _, tenantIDStr := range tenantIDStrs {
		tenantIDStr = strings.TrimSpace(tenantIDStr)
		if tenantIDStr == "" {
			continue
		}

		tenantID, err := strconv.ParseUint(tenantIDStr, 10, 64)
		if err != nil {
			continue // Skip invalid tenant IDs
		}

		// Get entities for this tenant
		entityIDs, err := h.repo.GetEntitiesByTenant(uint(tenantID))
		if err != nil {
			continue // Skip if there's an error fetching entities
		}

		// Add to the collection
		if len(entityIDs) > 0 {
			validTenantIDs = append(validTenantIDs, tenantIDStr)
			for _, entityID := range entityIDs {
				allEntityIDs = append(allEntityIDs, fmt.Sprint(entityID))
			}
		}
	}

	if len(allEntityIDs) == 0 {
		c.JSON(http.StatusOK, gin.H{
			"data":    []DevoteeBirthdayReportRow{},
			"message": "No entities found for the specified tenants",
		})
		return
	}

	// Create request object
	req := DevoteeBirthdaysReportRequest{
		DateRange: dateRange,
		StartDate: start,
		EndDate:   end,
		Format:    format,
		EntityID:  fmt.Sprintf("multiple_tenants_%s", strings.Join(validTenantIDs, "_")), // Clear identifier for multiple tenants
	}

	// The 'format' query parameter determines the report type for the exporter.
	var reportType string
	switch format {
	case "excel":
		reportType = ReportTypeDevoteeBirthdaysExcel
	case "pdf":
		reportType = ReportTypeDevoteeBirthdaysPDF
	case "csv":
		reportType = ReportTypeDevoteeBirthdays
	default:
		// If no format is specified, return JSON preview
		data, err := h.service.GetDevoteeBirthdaysReport(req, allEntityIDs)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		// Log report view
		details := map[string]interface{}{
			"report_type": "devotee_birthdays",
			"format":      "json_preview",
			"tenant_ids":  validTenantIDs,
			"entity_ids":  allEntityIDs,
			"date_range":  dateRange,
			"start_date":  start.Format("2006-01-02"),
			"end_date":    end.Format("2006-01-02"),
		}
		h.auditSvc.LogAction(c.Request.Context(), &ctx.UserID, nil, "SUPERADMIN_DEVOTEE_BIRTHDAYS_REPORT_VIEWED", details, ip, "success")

		c.JSON(http.StatusOK, gin.H{
			"data": data,
			"meta": gin.H{
				"tenant_ids": validTenantIDs,
				"entity_ids": allEntityIDs,
				"date_range": dateRange,
				"start_date": start.Format("2006-01-02"),
				"end_date":   end.Format("2006-01-02"),
			},
		})
		return
	}

	// Export file (format is present)
	bytes, fname, mime, err := h.service.ExportDevoteeBirthdaysReport(c.Request.Context(), req, allEntityIDs, reportType, &ctx.UserID, ip)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", fname))
	c.Data(http.StatusOK, mime, bytes)
}

// GetSuperAdminTenantDevoteeBirthdaysReport handles devotee birthdays report for a single tenant by superadmin
func (h *Handler) GetSuperAdminTenantDevoteeBirthdaysReport(c *gin.Context) {
	// Get access context from middleware
	accessContext, exists := c.Get("access_context")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "access context missing"})
		return
	}
	ctx := accessContext.(middleware.AccessContext)

	// Ensure superadmin role
	if ctx.RoleName != "superadmin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "only superadmin can access this endpoint"})
		return
	}

	// Get IP address from context
	ip := middleware.GetIPFromContext(c)

	// Get request parameters - tenant ID from path parameter
	tenantID := c.Param("id")
	if tenantID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tenant ID is required"})
		return
	}

	dateRange := c.Query("date_range")
	if dateRange == "" {
		dateRange = DateRangeWeekly
	}
	startDateStr := c.Query("start_date")
	endDateStr := c.Query("end_date")
	format := c.Query("format")

	// Compute date range
	start, end, err := GetDateRange(dateRange, startDateStr, endDateStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Convert tenant ID to uint
	tenantIDUint, err := strconv.ParseUint(tenantID, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid tenant ID"})
		return
	}

	// Get entities for this tenant
	entityIDs, err := h.repo.GetEntitiesByTenant(uint(tenantIDUint))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch tenant entities"})
		return
	}

	if len(entityIDs) == 0 {
		c.JSON(http.StatusOK, gin.H{
			"data":    []DevoteeBirthdayReportRow{},
			"message": "No entities found for the specified tenant",
		})
		return
	}

	// Convert entity IDs to strings
	entityIDStrs := make([]string, 0, len(entityIDs))
	for _, id := range entityIDs {
		entityIDStrs = append(entityIDStrs, fmt.Sprint(id))
	}

	// Create request object
	req := DevoteeBirthdaysReportRequest{
		DateRange: dateRange,
		StartDate: start,
		EndDate:   end,
		Format:    format,
		EntityID:  fmt.Sprintf("tenant_%s", tenantID), // Clearly indicate this is for a specific tenant
	}

	// The 'format' query parameter determines the report type for the exporter.
	var reportType string
	switch format {
	case "excel":
		reportType = ReportTypeDevoteeBirthdaysExcel
	case "pdf":
		reportType = ReportTypeDevoteeBirthdaysPDF
	case "csv":
		reportType = ReportTypeDevoteeBirthdays
	default:
		// If no format is specified, return JSON preview
		data, err := h.service.GetDevoteeBirthdaysReport(req, entityIDStrs)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		// Log report view
		details := map[string]interface{}{
			"report_type": "devotee_birthdays",
			"format":      "json_preview",
			"tenant_id":   tenantID,
			"entity_ids":  entityIDStrs,
			"date_range":  dateRange,
			"start_date":  start.Format("2006-01-02"),
			"end_date":    end.Format("2006-01-02"),
		}
		h.auditSvc.LogAction(c.Request.Context(), &ctx.UserID, nil, "SUPERADMIN_TENANT_DEVOTEE_BIRTHDAYS_REPORT_VIEWED", details, ip, "success")

		c.JSON(http.StatusOK, gin.H{
			"data": data,
			"meta": gin.H{
				"tenant_id":  tenantID,
				"entity_ids": entityIDStrs,
				"date_range": dateRange,
				"start_date": start.Format("2006-01-02"),
				"end_date":   end.Format("2006-01-02"),
			},
		})
		return
	}

	// Export file (format is present)
	bytes, fname, mime, err := h.service.ExportDevoteeBirthdaysReport(c.Request.Context(), req, entityIDStrs, reportType, &ctx.UserID, ip)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", fname))
	c.Data(http.StatusOK, mime, bytes)
}
// GetDevoteeListReport handles requests for devotee list report
func (h *Handler) GetDevoteeListReport(c *gin.Context) {
	// Get access context from middleware
	accessContext, exists := c.Get("access_context")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "access context missing"})
		return
	}
	ctx := accessContext.(middleware.AccessContext)

	ip := middleware.GetIPFromContext(c)

	entityParam := c.Param("id") // "all" or entity id
	fmt.Println("entityParam:", entityParam)

	dateRange := c.Query("date_range")
	if dateRange == "" {
		dateRange = DateRangeWeekly
	}
	startDateStr := c.Query("start_date")
	endDateStr := c.Query("end_date")
	status := c.Query("status") // active|inactive|blocked etc
	format := c.Query("format")

	start, end, err := GetDateRange(dateRange, startDateStr, endDateStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var entityIDs []string
	var actualEntityParam string
	var tenantID uint

	if strings.ToLower(entityParam) == "all" {
		actualEntityParam = "all"
		
		// Handle based on role
		switch ctx.RoleName {
case "superadmin":
			// When superadmin logs in as tenant, they should have AssignedEntityID set
			if ctx.AssignedEntityID != nil {
				tenantID = *ctx.AssignedEntityID
				ids, err := h.repo.GetEntitiesByTenant(tenantID)
				if err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch tenant entities"})
					return
				}
				if len(ids) == 0 {
					c.JSON(http.StatusOK, gin.H{"data": []DevoteeListReportRow{}})
					return
				}
				for _, id := range ids {
					entityIDs = append(entityIDs, fmt.Sprint(id))
				}
			} else {
				// Pure superadmin without tenant context - should not happen for this endpoint
				c.JSON(http.StatusBadRequest, gin.H{"error": "superadmin must specify tenant context or use superadmin endpoints"})
				return
			}
		case "templeadmin":
			// Templeadmin uses their user ID as tenant ID
			tenantID = ctx.UserID
			ids, err := h.repo.GetEntitiesByTenant(tenantID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch user entities"})
				return
			}
			if len(ids) == 0 {
				c.JSON(http.StatusOK, gin.H{"data": []DevoteeListReportRow{}})
				return
			}
			for _, id := range ids {
				entityIDs = append(entityIDs, fmt.Sprint(id))
			}
		case "standarduser", "monitoringuser":
			// standarduser/monitoringuser use their assigned entity as tenant
			if ctx.AssignedEntityID == nil {
				c.JSON(http.StatusForbidden, gin.H{"error": "no accessible entity"})
				return
			}
			tenantID = *ctx.AssignedEntityID
			ids, err := h.repo.GetEntitiesByTenant(tenantID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch tenant entities"})
				return
			}
			if len(ids) == 0 {
				c.JSON(http.StatusOK, gin.H{"data": []DevoteeListReportRow{}})
				return
			}
			for _, id := range ids {
				entityIDs = append(entityIDs, fmt.Sprint(id))
			}
		default:
			// Unknown role
			c.JSON(http.StatusForbidden, gin.H{"error": "role not authorized for this endpoint"})
			return
		}
	} else {
		eid, err := strconv.ParseUint(entityParam, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid entity_id path param"})
			return
		}

		actualEntityParam = fmt.Sprint(eid)

		if !h.canAccessEntity(ctx, uint(eid)) {
			fmt.Println("actualEntityParam:", actualEntityParam)
			c.JSON(http.StatusForbidden, gin.H{"error": "not authorized for this entity"})
			return
		}
		fmt.Println("actualEntityParam1:", entityIDs)
		entityIDs = append(entityIDs, fmt.Sprint(eid))
		fmt.Println("actualEntityParam2:", entityIDs, actualEntityParam)
	}

	req := DevoteeListReportRequest{
		DateRange: dateRange,
		StartDate: start,
		EndDate:   end,
		Status:    status,
		Format:    format,
		EntityID:  actualEntityParam,
	}

	var reportType string
	switch format {
	case "excel":
		reportType = ReportTypeDevoteeListExcel
	case "pdf":
		reportType = ReportTypeDevoteeListPDF
	case "csv":
		reportType = ReportTypeDevoteeListCSV
	default:
		data, err := h.service.GetDevoteeListReport(req, entityIDs)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		details := map[string]interface{}{
			"report_type":  "devotee_list",
			"format":       "json_preview",
			"entity_ids":   entityIDs,
			"entity_param": entityParam,
			"status":       status,
			"date_range":   dateRange,
		}
		h.auditSvc.LogAction(c.Request.Context(), &ctx.UserID, nil, "DEVOTEE_LIST_REPORT_VIEWED", details, ip, "success")
		c.JSON(http.StatusOK, data)
		return
	}

	bytes, fname, mime, err := h.service.ExportDevoteeListReport(c.Request.Context(), req, entityIDs, reportType, &ctx.UserID, ip)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", fname))
	c.Data(http.StatusOK, mime, bytes)
}

// GetSuperAdminDevoteeListReport handles devotee list report for superadmin with multiple tenants
func (h *Handler) GetSuperAdminDevoteeListReport(c *gin.Context) {
	// Get access context from middleware
	accessContext, exists := c.Get("access_context")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "access context missing"})
		return
	}
	ctx := accessContext.(middleware.AccessContext)

	// Ensure superadmin role
	if ctx.RoleName != "superadmin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "only superadmin can access this endpoint"})
		return
	}

	// Get IP address from context
	ip := middleware.GetIPFromContext(c)

	// Get request parameters
	tenantsParam := c.Query("tenants")
	if tenantsParam == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tenants query param required"})
		return
	}

	tenantIDStrs := strings.Split(tenantsParam, ",")

	dateRange := c.Query("date_range")
	if dateRange == "" {
		dateRange = DateRangeWeekly
	}
	startDateStr := c.Query("start_date")
	endDateStr := c.Query("end_date")
	status := c.Query("status") // active|inactive|blocked etc
	format := c.Query("format")

	// Compute date range
	start, end, err := GetDateRange(dateRange, startDateStr, endDateStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Collect entity IDs for all specified tenants
	var allEntityIDs []string
	var validTenantIDs []string

	for _, tenantIDStr := range tenantIDStrs {
		tenantIDStr = strings.TrimSpace(tenantIDStr)
		if tenantIDStr == "" {
			continue
		}

		tenantID, err := strconv.ParseUint(tenantIDStr, 10, 64)
		if err != nil {
			continue // Skip invalid tenant IDs
		}

		// Get entities for this tenant
		entityIDs, err := h.repo.GetEntitiesByTenant(uint(tenantID))
		if err != nil {
			continue // Skip if there's an error fetching entities
		}

		// Add to the collection
		if len(entityIDs) > 0 {
			validTenantIDs = append(validTenantIDs, tenantIDStr)
			for _, entityID := range entityIDs {
				allEntityIDs = append(allEntityIDs, fmt.Sprint(entityID))
			}
		}
	}

	if len(allEntityIDs) == 0 {
		c.JSON(http.StatusOK, gin.H{"data": []DevoteeListReportRow{}, "message": "No entities found for the specified tenants"})
		return
	}

	// Create request object
	req := DevoteeListReportRequest{
		DateRange: dateRange,
		StartDate: start,
		EndDate:   end,
		Status:    status,
		Format:    format,
		EntityID:  fmt.Sprintf("multiple_tenants_%s", strings.Join(validTenantIDs, "_")),
	}

	var reportType string
	switch format {
	case "excel":
		reportType = ReportTypeDevoteeListExcel
	case "pdf":
		reportType = ReportTypeDevoteeListPDF
	case "csv":
		reportType = ReportTypeDevoteeListCSV
	default:
		data, err := h.service.GetDevoteeListReport(req, allEntityIDs)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		details := map[string]interface{}{
			"report_type": "devotee_list",
			"format":      "json_preview",
			"tenant_ids":  validTenantIDs,
			"entity_ids":  allEntityIDs,
			"status":      status,
			"date_range":  dateRange,
		}
		h.auditSvc.LogAction(c.Request.Context(), &ctx.UserID, nil, "SUPERADMIN_DEVOTEE_LIST_REPORT_VIEWED", details, ip, "success")
		c.JSON(http.StatusOK, data)
		return
	}

	bytes, fname, mime, err := h.service.ExportDevoteeListReport(c.Request.Context(), req, allEntityIDs, reportType, &ctx.UserID, ip)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", fname))
	c.Data(http.StatusOK, mime, bytes)
}

// GetSuperAdminTenantDevoteeListReport handles devotee list report for a single tenant by superadmin
func (h *Handler) GetSuperAdminTenantDevoteeListReport(c *gin.Context) {
	// Get access context from middleware
	accessContext, exists := c.Get("access_context")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "access context missing"})
		return
	}
	ctx := accessContext.(middleware.AccessContext)

	// Ensure superadmin role
	if ctx.RoleName != "superadmin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "only superadmin can access this endpoint"})
		return
	}

	// Get IP address from context
	ip := middleware.GetIPFromContext(c)

	// Get request parameters - tenant ID from path parameter
	tenantID := c.Param("id")
	if tenantID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tenant ID is required"})
		return
	}

	dateRange := c.Query("date_range")
	if dateRange == "" {
		dateRange = DateRangeWeekly
	}
	startDateStr := c.Query("start_date")
	endDateStr := c.Query("end_date")
	status := c.Query("status") // active|inactive|blocked etc
	format := c.Query("format")

	// Compute date range
	start, end, err := GetDateRange(dateRange, startDateStr, endDateStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Convert tenant ID to uint
	tenantIDUint, err := strconv.ParseUint(tenantID, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid tenant ID"})
		return
	}

	// Get entities for this tenant
	entityIDs, err := h.repo.GetEntitiesByTenant(uint(tenantIDUint))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch tenant entities"})
		return
	}

	if len(entityIDs) == 0 {
		c.JSON(http.StatusOK, gin.H{"data": []DevoteeListReportRow{}, "message": "No entities found for the specified tenant"})
		return
	}

	// Convert entity IDs to strings
	entityIDStrs := make([]string, 0, len(entityIDs))
	for _, id := range entityIDs {
		entityIDStrs = append(entityIDStrs, fmt.Sprint(id))
	}

	// Create request object
	req := DevoteeListReportRequest{
		DateRange: dateRange,
		StartDate: start,
		EndDate:   end,
		Status:    status,
		Format:    format,
		EntityID:  fmt.Sprintf("tenant_%s", tenantID), // Clearly indicate this is for a specific tenant
	}

	var reportType string
	switch format {
	case "excel":
		reportType = ReportTypeDevoteeListExcel
	case "pdf":
		reportType = ReportTypeDevoteeListPDF
	case "csv":
		reportType = ReportTypeDevoteeListCSV
	default:
		data, err := h.service.GetDevoteeListReport(req, entityIDStrs)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		details := map[string]interface{}{
			"report_type": "devotee_list",
			"format":      "json_preview",
			"tenant_id":   tenantID,
			"entity_ids":  entityIDStrs,
			"status":      status,
			"date_range":  dateRange,
		}
		h.auditSvc.LogAction(c.Request.Context(), &ctx.UserID, nil, "SUPERADMIN_TENANT_DEVOTEE_LIST_REPORT_VIEWED", details, ip, "success")
		c.JSON(http.StatusOK, data)
		return
	}

	bytes, fname, mime, err := h.service.ExportDevoteeListReport(c.Request.Context(), req, entityIDStrs, reportType, &ctx.UserID, ip)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", fname))
	c.Data(http.StatusOK, mime, bytes)
}

// GetDevoteeProfileReport handles requests for devotee profile report
func (h *Handler) GetDevoteeProfileReport(c *gin.Context) {
	// Get access context from middleware
	accessContext, exists := c.Get("access_context")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "access context missing"})
		return
	}
	ctx := accessContext.(middleware.AccessContext)

	ip := middleware.GetIPFromContext(c)

	entityParam := c.Param("id") // "all" or entity id

	dateRange := c.Query("date_range")
	if dateRange == "" {
		dateRange = DateRangeWeekly
	}
	startDateStr := c.Query("start_date")
	endDateStr := c.Query("end_date")
	status := c.Query("status") // active|inactive|blocked etc
	format := c.Query("format")

	start, end, err := GetDateRange(dateRange, startDateStr, endDateStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var entityIDs []string
	var actualEntityParam string
	var tenantID uint

	if strings.ToLower(entityParam) == "all" {
		actualEntityParam = "all"
		
		// Handle based on role
		switch ctx.RoleName {
case "superadmin":
			// When superadmin logs in as tenant, they should have AssignedEntityID set
			if ctx.AssignedEntityID != nil {
				tenantID = *ctx.AssignedEntityID
				ids, err := h.repo.GetEntitiesByTenant(tenantID)
				if err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch tenant entities"})
					return
				}
				if len(ids) == 0 {
					c.JSON(http.StatusOK, gin.H{"data": []DevoteeProfileReportRow{}})
					return
				}
				for _, id := range ids {
					entityIDs = append(entityIDs, fmt.Sprint(id))
				}
			} else {
				// Pure superadmin without tenant context - should not happen for this endpoint
				c.JSON(http.StatusBadRequest, gin.H{"error": "superadmin must specify tenant context or use superadmin endpoints"})
				return
			}
		case "templeadmin":
			// Templeadmin uses their user ID as tenant ID
			tenantID = ctx.UserID
			ids, err := h.repo.GetEntitiesByTenant(tenantID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch user entities"})
				return
			}
			if len(ids) == 0 {
				c.JSON(http.StatusOK, gin.H{"data": []DevoteeProfileReportRow{}})
				return
			}
			for _, id := range ids {
				entityIDs = append(entityIDs, fmt.Sprint(id))
			}
		case "standarduser", "monitoringuser":
			// standarduser/monitoringuser use their assigned entity as tenant
			if ctx.AssignedEntityID == nil {
				c.JSON(http.StatusForbidden, gin.H{"error": "no accessible entity"})
				return
			}
			tenantID = *ctx.AssignedEntityID
			ids, err := h.repo.GetEntitiesByTenant(tenantID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch tenant entities"})
				return
			}
			if len(ids) == 0 {
				c.JSON(http.StatusOK, gin.H{"data": []DevoteeProfileReportRow{}})
				return
			}
			for _, id := range ids {
				entityIDs = append(entityIDs, fmt.Sprint(id))
			}
		default:
			// Unknown role
			c.JSON(http.StatusForbidden, gin.H{"error": "role not authorized for this endpoint"})
			return
		}
	} else {
		eid, err := strconv.ParseUint(entityParam, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid entity_id path param"})
			return
		}

		actualEntityParam = fmt.Sprint(eid)

		if !h.canAccessEntity(ctx, uint(eid)) {
			c.JSON(http.StatusForbidden, gin.H{"error": "not authorized for this entity"})
			return
		}
		entityIDs = append(entityIDs, fmt.Sprint(eid))
	}

	req := DevoteeProfileReportRequest{
		DateRange: dateRange,
		StartDate: start,
		EndDate:   end,
		Status:    status,
		Format:    format,
		EntityID:  actualEntityParam,
	}

	var reportType string
	switch format {
	case "excel":
		reportType = ReportTypeDevoteeProfileExcel
	case "pdf":
		reportType = ReportTypeDevoteeProfilePDF
	case "csv":
		reportType = ReportTypeDevoteeProfileCSV
	default:
		data, err := h.service.GetDevoteeProfileReport(req, entityIDs)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		details := map[string]interface{}{
			"report_type":  "devotee_profile",
			"format":       "json_preview",
			"entity_ids":   entityIDs,
			"entity_param": entityParam,
			"status":       status,
			"date_range":   dateRange,
		}
		h.auditSvc.LogAction(c.Request.Context(), &ctx.UserID, nil, "DEVOTEE_PROFILE_REPORT_VIEWED", details, ip, "success")
		c.JSON(http.StatusOK, data)
		return
	}

	bytes, fname, mime, err := h.service.ExportDevoteeProfileReport(c.Request.Context(), req, entityIDs, reportType, &ctx.UserID, ip)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", fname))
	c.Data(http.StatusOK, mime, bytes)
}
// GetSuperAdminDevoteeProfileReport handles devotee profile report for superadmin with multiple tenants
func (h *Handler) GetSuperAdminDevoteeProfileReport(c *gin.Context) {
	// Get access context from middleware
	accessContext, exists := c.Get("access_context")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "access context missing"})
		return
	}
	ctx := accessContext.(middleware.AccessContext)

	// Ensure superadmin role
	if ctx.RoleName != "superadmin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "only superadmin can access this endpoint"})
		return
	}

	// Get IP address from context
	ip := middleware.GetIPFromContext(c)

	// Get request parameters
	tenantsParam := c.Query("tenants")
	if tenantsParam == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tenants query param required"})
		return
	}

	tenantIDStrs := strings.Split(tenantsParam, ",")

	dateRange := c.Query("date_range")
	if dateRange == "" {
		dateRange = DateRangeWeekly
	}
	startDateStr := c.Query("start_date")
	endDateStr := c.Query("end_date")
	status := c.Query("status") // active|inactive|blocked etc
	format := c.Query("format")

	// Compute date range
	start, end, err := GetDateRange(dateRange, startDateStr, endDateStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Collect entity IDs for all specified tenants
	var allEntityIDs []string
	var validTenantIDs []string

	for _, tenantIDStr := range tenantIDStrs {
		tenantIDStr = strings.TrimSpace(tenantIDStr)
		if tenantIDStr == "" {
			continue
		}

		tenantID, err := strconv.ParseUint(tenantIDStr, 10, 64)
		if err != nil {
			continue // Skip invalid tenant IDs
		}

		// Get entities for this tenant
		entityIDs, err := h.repo.GetEntitiesByTenant(uint(tenantID))
		if err != nil {
			continue // Skip if there's an error fetching entities
		}

		// Add to the collection
		if len(entityIDs) > 0 {
			validTenantIDs = append(validTenantIDs, tenantIDStr)
			for _, entityID := range entityIDs {
				allEntityIDs = append(allEntityIDs, fmt.Sprint(entityID))
			}
		}
	}

	if len(allEntityIDs) == 0 {
		c.JSON(http.StatusOK, gin.H{"data": []DevoteeProfileReportRow{}, "message": "No entities found for the specified tenants"})
		return
	}

	// Create request object
	req := DevoteeProfileReportRequest{
		DateRange: dateRange,
		StartDate: start,
		EndDate:   end,
		Status:    status,
		Format:    format,
		EntityID:  fmt.Sprintf("multiple_tenants_%s", strings.Join(validTenantIDs, "_")),
	}

	var reportType string
	switch format {
	case "excel":
		reportType = ReportTypeDevoteeProfileExcel
	case "pdf":
		reportType = ReportTypeDevoteeProfilePDF
	case "csv":
		reportType = ReportTypeDevoteeProfileCSV
	default:
		data, err := h.service.GetDevoteeProfileReport(req, allEntityIDs)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		details := map[string]interface{}{
			"report_type": "devotee_profile",
			"format":      "json_preview",
			"tenant_ids":  validTenantIDs,
			"entity_ids":  allEntityIDs,
			"status":      status,
			"date_range":  dateRange,
		}
		h.auditSvc.LogAction(c.Request.Context(), &ctx.UserID, nil, "SUPERADMIN_DEVOTEE_PROFILE_REPORT_VIEWED", details, ip, "success")
		c.JSON(http.StatusOK, data)
		return
	}

	bytes, fname, mime, err := h.service.ExportDevoteeProfileReport(c.Request.Context(), req, allEntityIDs, reportType, &ctx.UserID, ip)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", fname))
	c.Data(http.StatusOK, mime, bytes)
}

// GetSuperAdminTenantDevoteeProfileReport handles devotee profile report for a single tenant by superadmin
func (h *Handler) GetSuperAdminTenantDevoteeProfileReport(c *gin.Context) {
	// Get access context from middleware
	accessContext, exists := c.Get("access_context")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "access context missing"})
		return
	}
	ctx := accessContext.(middleware.AccessContext)

	// Ensure superadmin role
	if ctx.RoleName != "superadmin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "only superadmin can access this endpoint"})
		return
	}

	// Get IP address from context
	ip := middleware.GetIPFromContext(c)

	// Get request parameters - tenant ID from path parameter
	tenantID := c.Param("id")
	if tenantID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tenant ID is required"})
		return
	}

	dateRange := c.Query("date_range")
	if dateRange == "" {
		dateRange = DateRangeWeekly
	}
	startDateStr := c.Query("start_date")
	endDateStr := c.Query("end_date")
	status := c.Query("status") // active|inactive|blocked etc
	format := c.Query("format")

	// Compute date range
	start, end, err := GetDateRange(dateRange, startDateStr, endDateStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Convert tenant ID to uint
	tenantIDUint, err := strconv.ParseUint(tenantID, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid tenant ID"})
		return
	}

	// Get entities for this tenant
	entityIDs, err := h.repo.GetEntitiesByTenant(uint(tenantIDUint))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch tenant entities"})
		return
	}

	if len(entityIDs) == 0 {
		c.JSON(http.StatusOK, gin.H{"data": []DevoteeProfileReportRow{}, "message": "No entities found for the specified tenant"})
		return
	}

	// Convert entity IDs to strings
	entityIDStrs := make([]string, 0, len(entityIDs))
	for _, id := range entityIDs {
		entityIDStrs = append(entityIDStrs, fmt.Sprint(id))
	}

	// Create request object
	req := DevoteeProfileReportRequest{
		DateRange: dateRange,
		StartDate: start,
		EndDate:   end,
		Status:    status,
		Format:    format,
		EntityID:  fmt.Sprintf("tenant_%s", tenantID), // Clearly indicate this is for a specific tenant
	}

	var reportType string
	switch format {
	case "excel":
		reportType = ReportTypeDevoteeProfileExcel
	case "pdf":
		reportType = ReportTypeDevoteeProfilePDF
	case "csv":
		reportType = ReportTypeDevoteeProfileCSV
	default:
		data, err := h.service.GetDevoteeProfileReport(req, entityIDStrs)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		details := map[string]interface{}{
			"report_type": "devotee_profile",
			"format":      "json_preview",
			"tenant_id":   tenantID,
			"entity_ids":  entityIDStrs,
			"status":      status,
			"date_range":  dateRange,
		}
		h.auditSvc.LogAction(c.Request.Context(), &ctx.UserID, nil, "SUPERADMIN_TENANT_DEVOTEE_PROFILE_REPORT_VIEWED", details, ip, "success")
		c.JSON(http.StatusOK, data)
		return
	}

	bytes, fname, mime, err := h.service.ExportDevoteeProfileReport(c.Request.Context(), req, entityIDStrs, reportType, &ctx.UserID, ip)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", fname))
	c.Data(http.StatusOK, mime, bytes)
}

func (h *Handler) canAccessEntity(ctx middleware.AccessContext, entityID uint) bool {
	if ctx.RoleName == "superadmin" {
		// Superadmin can access only the selected entity, not all tenant entities
		return entityID != 0
	}

	if ctx.RoleName == "templeadmin" {
		// Templeadmin can access only entities they created
		ids, err := h.repo.GetEntitiesByTenant(ctx.UserID)
		fmt.Println("entity:", ids, ctx.UserID)
		if err != nil {
			fmt.Println("err1=", err)
			return false
		}
		for _, id := range ids {
			fmt.Println("id=", id, entityID)
			if id == entityID {
				return true
			}
		}
		fmt.Println("returning false=")
		return false
	}

	// Standard or Monitoring users can access only their assigned entity
	accessibleEntityID := ctx.GetAccessibleEntityID()
	if ctx.RoleName == "standarduser" || ctx.RoleName == "monitoringuser" {
		return accessibleEntityID != nil && *accessibleEntityID == entityID
	}

	fmt.Println("accessibleEntityID:", accessibleEntityID, entityID)
	return accessibleEntityID != nil && *accessibleEntityID == entityID
}

// GetAuditLogsReport handles requests for audit logs report
func (h *Handler) GetAuditLogsReport(c *gin.Context) {
	accessContext, exists := c.Get("access_context")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "access context missing"})
		return
	}
	ctx := accessContext.(middleware.AccessContext)
	ip := middleware.GetIPFromContext(c)

	entityParam := c.Param("id") // "all" or specific entity id

	// ✅ FIX: Support multiple possible query param names for action type
	action := c.Query("action_type") // main expected param (matches frontend)
	if action == "" {
		action = c.Query("actionType") // fallback to camelCase
	}
	if action == "" {
		action = c.Query("action") // fallback to legacy name
	}

	status := c.Query("status")
	dateRange := c.Query("date_range")
	if dateRange == "" {
		dateRange = DateRangeWeekly
	}
	startDateStr := c.Query("start_date")
	endDateStr := c.Query("end_date")
	format := c.Query("format")

	// 🔍 DEBUG: Log the received parameters
	fmt.Printf("\n🔍 DEBUG: Audit Logs Request Parameters\n")
	fmt.Printf("   Raw Query: %s\n", c.Request.URL.RawQuery)
	fmt.Printf("   Entity Param: %s\n", entityParam)
	fmt.Printf("   Action Type: '%s'\n", action)
	fmt.Printf("   Status: '%s'\n", status)
	fmt.Printf("   Date Range: %s\n", dateRange)
	fmt.Printf("   Format: %s\n", format)

	start, end, err := GetDateRange(dateRange, startDateStr, endDateStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var entityIDs []string

	// ✅ CASE 1: "all" temples for the tenant
	if strings.ToLower(entityParam) == "all" {
		switch ctx.RoleName {
		case "superadmin":
			ids, err := h.repo.GetAllEntityIDs()
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch all entities"})
				return
			}
			for _, id := range ids {
				entityIDs = append(entityIDs, fmt.Sprint(id))
			}

		case "templeadmin":
			ids, err := h.repo.GetEntitiesByTenant(ctx.UserID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch admin entities"})
				return
			}
			for _, id := range ids {
				entityIDs = append(entityIDs, fmt.Sprint(id))
			}

		case "standarduser", "monitoringuser":
			if ctx.TenantID == 0 {
				c.JSON(http.StatusForbidden, gin.H{"error": "tenant context missing"})
				return
			}
			ids, err := h.repo.GetEntitiesByTenantID(ctx.TenantID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch tenant entities"})
				return
			}
			for _, id := range ids {
				entityIDs = append(entityIDs, fmt.Sprint(id))
			}

		default:
			c.JSON(http.StatusForbidden, gin.H{"error": "role not authorized"})
			return
		}

		if len(entityIDs) == 0 {
			c.JSON(http.StatusOK, gin.H{"data": []AuditLogReportRow{}})
			return
		}

	} else {
		// ✅ CASE 2: Single temple details (entity id)
		eid, err := strconv.ParseUint(entityParam, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid entity_id"})
			return
		}

		if !h.canAccessEntity(ctx, uint(eid)) {
			c.JSON(http.StatusForbidden, gin.H{"error": "not authorized for this entity"})
			return
		}
		entityIDs = append(entityIDs, fmt.Sprint(eid))
	}

	fmt.Printf("   📋 Entity IDs to query: %v\n", entityIDs)

	req := AuditLogReportRequest{
		EntityID:  entityParam,
		Action:    action,
		Status:    status,
		DateRange: dateRange,
		StartDate: start,
		EndDate:   end,
		Format:    format,
	}

	fmt.Printf("   📤 Request object - Action: '%s', Status: '%s'\n", req.Action, req.Status)

	// JSON preview (no export format)
	if format == "" {
		data, err := h.service.GetAuditLogsReport(req, entityIDs)
		if err != nil {
			fmt.Printf("   ❌ Error fetching audit logs: %v\n", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		fmt.Printf("   ✅ Fetched %d audit log records\n", len(data))

		h.auditSvc.LogAction(
			c.Request.Context(),
			&ctx.UserID,
			nil,
			"AUDIT_LOGS_REPORT_VIEWED",
			map[string]interface{}{
				"report_type":  "audit_logs",
				"format":       "json_preview",
				"entity_ids":   entityIDs,
				"action":       action,
				"status":       status,
				"ip_address":   ip,
				"record_count": len(data),
			},
			ip,
			"success",
		)

		c.JSON(http.StatusOK, data)
		return
	}

	// Export formats (Excel, PDF, CSV)
	var reportType string
	switch format {
	case "excel":
		reportType = ReportTypeAuditLogsExcel
	case "pdf":
		reportType = ReportTypeAuditLogsPDF
	case "csv":
		reportType = ReportTypeAuditLogsCSV
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported export format"})
		return
	}

	bytes, fname, mime, err := h.service.ExportAuditLogsReport(
		c.Request.Context(),
		req,
		entityIDs,
		reportType,
		&ctx.UserID,
		ip,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", fname))
	c.Data(http.StatusOK, mime, bytes)
}


// GetSuperAdminAuditLogsReport handles audit logs report for superadmin with multiple tenants
func (h *Handler) GetSuperAdminAuditLogsReport(c *gin.Context) {
	// Get access context from middleware
	accessContext, exists := c.Get("access_context")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "access context missing"})
		return
	}
	ctx := accessContext.(middleware.AccessContext)

	// Ensure superadmin role
	if ctx.RoleName != "superadmin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "only superadmin can access this endpoint"})
		return
	}

	// Get IP address from context
	ip := middleware.GetIPFromContext(c)

	// Get request parameters
	tenantsParam := c.Query("tenants")
	if tenantsParam == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tenants query param required"})
		return
	}

	tenantIDs := strings.Split(tenantsParam, ",")
	action := c.Query("action")
	status := c.Query("status")
	dateRange := c.Query("date_range")
	if dateRange == "" {
		dateRange = DateRangeWeekly
	}
	startDateStr := c.Query("start_date")
	endDateStr := c.Query("end_date")
	format := c.Query("format") // json preview, csv, excel, pdf

	// Compute date range
	start, end, err := GetDateRange(dateRange, startDateStr, endDateStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Collect entity IDs for all specified tenants
	var allEntityIDs []string
	for _, tenantIDStr := range tenantIDs {
		tenantID, err := strconv.ParseUint(tenantIDStr, 10, 64)
		if err != nil {
			continue // Skip invalid tenant IDs
		}

		// Get entities for this tenant
		entityIDs, err := h.repo.GetEntitiesByTenant(uint(tenantID))
		if err != nil {
			continue // Skip if there's an error fetching entities
		}

		// Add to the collection
		for _, entityID := range entityIDs {
			allEntityIDs = append(allEntityIDs, fmt.Sprint(entityID))
		}
	}

	if len(allEntityIDs) == 0 {
		c.JSON(http.StatusOK, gin.H{"data": []AuditLogReportRow{}, "message": "No entities found for the specified tenants"})
		return
	}

	// Create request object
	req := AuditLogReportRequest{
		EntityID:  "multiple", // Indicate multiple entities
		Action:    action,
		Status:    status,
		DateRange: dateRange,
		StartDate: start,
		EndDate:   end,
		Format:    format,
	}

	// Handle JSON preview
	if format == "" {
		data, err := h.service.GetAuditLogsReport(req, allEntityIDs)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		details := map[string]interface{}{
			"report_type": "audit_logs",
			"format":      "json_preview",
			"tenant_ids":  tenantIDs,
			"entity_ids":  allEntityIDs,
			"action":      action,
			"status":      status,
			"date_range":  dateRange,
		}
		h.auditSvc.LogAction(c.Request.Context(), &ctx.UserID, nil, "SUPERADMIN_AUDIT_LOGS_REPORT_VIEWED", details, ip, "success")
		c.JSON(http.StatusOK, data)
		return
	}

	// Export file logic
	var reportType string
	switch format {
	case "excel":
		reportType = ReportTypeAuditLogsExcel
	case "pdf":
		reportType = ReportTypeAuditLogsPDF
	case "csv":
		reportType = ReportTypeAuditLogsCSV
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported export format"})
		return
	}

	bytes, fname, mime, err := h.service.ExportAuditLogsReport(c.Request.Context(), req, allEntityIDs, reportType, &ctx.UserID, ip)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", fname))
	c.Data(http.StatusOK, mime, bytes)
}

// GetSuperAdminTenantAuditLogsReport handles audit logs report for a single tenant by superadmin
func (h *Handler) GetSuperAdminTenantAuditLogsReport(c *gin.Context) {
	// Get access context from middleware
	accessContext, exists := c.Get("access_context")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "access context missing"})
		return
	}
	ctx := accessContext.(middleware.AccessContext)

	// Ensure superadmin role
	if ctx.RoleName != "superadmin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "only superadmin can access this endpoint"})
		return
	}

	// Get IP address from context
	ip := middleware.GetIPFromContext(c)

	// Get request parameters
	tenantID := c.Param("id")
	if tenantID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tenant ID is required"})
		return
	}

	action := c.Query("action")
	status := c.Query("status")
	dateRange := c.Query("date_range")
	if dateRange == "" {
		dateRange = DateRangeWeekly
	}
	startDateStr := c.Query("start_date")
	endDateStr := c.Query("end_date")
	format := c.Query("format") // json preview, csv, excel, pdf

	// Compute date range
	start, end, err := GetDateRange(dateRange, startDateStr, endDateStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Convert tenant ID to uint
	tenantIDUint, err := strconv.ParseUint(tenantID, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid tenant ID"})
		return
	}

	// Get entities for this tenant
	entityIDs, err := h.repo.GetEntitiesByTenant(uint(tenantIDUint))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch tenant entities"})
		return
	}

	if len(entityIDs) == 0 {
		c.JSON(http.StatusOK, gin.H{"data": []AuditLogReportRow{}, "message": "No entities found for the specified tenant"})
		return
	}

	// Convert entity IDs to strings
	entityIDStrs := make([]string, 0, len(entityIDs))
	for _, id := range entityIDs {
		entityIDStrs = append(entityIDStrs, fmt.Sprint(id))
	}

	// Create request object
	req := AuditLogReportRequest{
		EntityID:  tenantID, // Use tenant ID as entity ID
		Action:    action,
		Status:    status,
		DateRange: dateRange,
		StartDate: start,
		EndDate:   end,
		Format:    format,
	}

	// Handle JSON preview
	if format == "" {
		data, err := h.service.GetAuditLogsReport(req, entityIDStrs)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		details := map[string]interface{}{
			"report_type": "audit_logs",
			"format":      "json_preview",
			"tenant_id":   tenantID,
			"entity_ids":  entityIDStrs,
			"action":      action,
			"status":      status,
			"date_range":  dateRange,
		}
		h.auditSvc.LogAction(c.Request.Context(), &ctx.UserID, nil, "SUPERADMIN_TENANT_AUDIT_LOGS_REPORT_VIEWED", details, ip, "success")
		c.JSON(http.StatusOK, data)
		return
	}

	// Export file logic
	var reportType string
	switch format {
	case "excel":
		reportType = ReportTypeAuditLogsExcel
	case "pdf":
		reportType = ReportTypeAuditLogsPDF
	case "csv":
		reportType = ReportTypeAuditLogsCSV
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported export format"})
		return
	}

	bytes, fname, mime, err := h.service.ExportAuditLogsReport(c.Request.Context(), req, entityIDStrs, reportType, &ctx.UserID, ip)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", fname))
	c.Data(http.StatusOK, mime, bytes)
}
// GetApprovalStatusReport handles requests for approval status reports
func (h *Handler) GetApprovalStatusReport(c *gin.Context) {
	// Access context from middleware
	accessContext, exists := c.Get("access_context")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "access context missing"})
		return
	}
	ctx := accessContext.(middleware.AccessContext)
	ip := middleware.GetIPFromContext(c)

	// Query params
	role := c.Query("role")     // "tenantadmin" or "templeadmin" or empty (both)
	status := c.Query("status") // "approved", "rejected", "pending", etc.
	dateRange := c.Query("date_range")
	if dateRange == "" {
		dateRange = DateRangeWeekly
	}
	startDateStr := c.Query("start_date")
	endDateStr := c.Query("end_date")
	format := c.Query("format") // excel, csv, pdf -> empty = JSON

	fmt.Printf("\n📋 Handler: Processing approval status report\n")
	fmt.Printf("   Role: '%s'\n", role)
	fmt.Printf("   Status: '%s'\n", status)
	fmt.Printf("   DateRange: '%s'\n", dateRange)
	fmt.Printf("   Format: '%s'\n", format)

	// Handle date range - allow fetching all records if not specified
	var start, end time.Time
	var err error

	if startDateStr != "" && endDateStr != "" {
		start, end, err = GetDateRange(dateRange, startDateStr, endDateStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		fmt.Printf("   ✅ Date filter: %s to %s\n", start.Format("2006-01-02"), end.Format("2006-01-02"))
	} else {
		// No date filter - fetch all records
		fmt.Printf("   ⚠️ No date filter - fetching ALL approval records\n")
		start = time.Time{}
		end = time.Time{}
	}

	// Create request object
	req := ApprovalStatusReportRequest{
		Role:      role,
		Status:    status,
		DateRange: dateRange,
		StartDate: start,
		EndDate:   end,
		Format:    format,
		UserID:    ctx.UserID,
	}

	// Determine accessible entities based on role
	var entityIDs []string
	switch ctx.RoleName {
	case "superadmin":
		// Superadmin can access all entities
		fmt.Printf("   👑 SuperAdmin: No entity filter (access all)\n")
		// Keep entityIDs empty for all access
		
	case "templeadmin":
		// Temple admin can only see their own entities
		ids, err := h.repo.GetEntitiesByTenant(ctx.UserID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch temple entities"})
			return
		}
		for _, id := range ids {
			entityIDs = append(entityIDs, fmt.Sprint(id))
		}
		fmt.Printf("   🛕 TempleAdmin: Entity filter = %v\n", entityIDs)
		
	default:
		c.JSON(http.StatusForbidden, gin.H{"error": "role not allowed for approval reports"})
		return
	}

	// Return JSON preview if format not specified
	if req.Format == "" {
		data, err := h.service.GetApprovalStatusReport(req, entityIDs)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		// Log the view action
		h.auditSvc.LogAction(
			c.Request.Context(),
			&ctx.UserID,
			nil,
			"APPROVAL_STATUS_REPORT_VIEWED",
			map[string]interface{}{
				"report_type": "approval_status",
				"entity_ids":  entityIDs,
				"role":        role,
				"status":      status,
				"date_range":  req.DateRange,
				"row_count":   len(data),
			},
			ip,
			"success",
		)

		c.JSON(http.StatusOK, gin.H{
			"report_type": "approval-status",
			"data":        data,
			"meta": gin.H{
				"total_records": len(data),
				"filters": gin.H{
					"role":   role,
					"status": status,
				},
			},
		})
		return
	}

	// Map format to appropriate report type
	var reportType string
	switch format {
	case "excel":
		reportType = ReportTypeApprovalStatusExcel
	case "pdf":
		reportType = ReportTypeApprovalStatusPDF
	case "csv":
		reportType = ReportTypeApprovalStatusCSV
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported format"})
		return
	}

	// Export report if format is specified
	bytes, fname, mime, err := h.service.ExportApprovalStatusReport(
		c.Request.Context(),
		req,
		entityIDs,
		reportType,
		&ctx.UserID,
		ip,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", fname))
	c.Data(http.StatusOK, mime, bytes)
}
// ==============================
// User Details Report Handler
// ==============================
// GetUserDetailsReport handles requests for user details report
// GetUserDetailsReport handles requests for user details report
func (h *Handler) GetUserDetailsReport(c *gin.Context) {
	// Access context
	accessContext, exists := c.Get("access_context")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "access context missing"})
		return
	}
	ctx := accessContext.(middleware.AccessContext)
	ip := middleware.GetIPFromContext(c)

	// Query params
	role := c.Query("role")
	status := c.Query("status")
	dateRange := c.Query("date_range")
	if dateRange == "" {
		dateRange = DateRangeWeekly
	}
	startDateStr := c.Query("start_date")
	endDateStr := c.Query("end_date")
	format := c.Query("format")

	// Compute start & end
	start, end, err := GetDateRange(dateRange, startDateStr, endDateStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	req := UserDetailReportRequest{
		Role:      role,
		Status:    status,
		DateRange: dateRange,
		StartDate: start,
		EndDate:   end,
		Format:    format,
		UserID:    ctx.UserID,
	}

	// Accessible entities
	var entityIDs []string
	switch ctx.RoleName {
	case "superadmin":
		entityIDs = nil // all entities
	case "templeadmin":
		ids, err := h.repo.GetEntitiesByTenant(ctx.UserID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch user entities"})
			return
		}
		for _, id := range ids {
			entityIDs = append(entityIDs, fmt.Sprint(id))
		}
	default:
		accessibleEntityID := ctx.GetAccessibleEntityID()
		if accessibleEntityID == nil {
			c.JSON(http.StatusForbidden, gin.H{"error": "no accessible entity"})
			return
		}
		entityIDs = append(entityIDs, fmt.Sprint(*accessibleEntityID))
	}

	// JSON preview
	if req.Format == "" {
		data, err := h.service.GetUserDetailsReport(req, entityIDs)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		h.auditSvc.LogAction(c.Request.Context(), &ctx.UserID, nil, "USER_DETAILS_REPORT_VIEWED", map[string]interface{}{
			"report_type": "user_details",
			"entity_ids":  entityIDs,
			"role":        role,
			"status":      status,
			"date_range":  req.DateRange,
		}, ip, "success")
		c.JSON(http.StatusOK, gin.H{
			"report_type": "user-details",
			"data":        data,
		})
		return
	}

	// Export report
	// Determine report type based on format
	var reportType string
	switch format {
	case "excel":
		reportType = ReportTypeUserDetailsExcel
	case "pdf":
		reportType = ReportTypeUserDetailsPDF
	case "csv":
		reportType = ReportTypeUserDetailsCSV
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported format"})
		return
	}

	bytes, fname, mime, err := h.service.ExportUserDetailsReport(c.Request.Context(), req, entityIDs, reportType, &ctx.UserID, ip)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", fname))
	c.Data(http.StatusOK, mime, bytes)
}
