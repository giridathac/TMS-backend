package auditlog

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	service Service
}

func NewHandler(service Service) *Handler {
	return &Handler{service: service}
}

// GetAuditLogs handles GET /auditlogs - retrieves audit logs with filtering and pagination
// @Summary Get audit logs
// @Description Retrieve audit logs with optional filters and pagination (SuperAdmin only)
// @Tags AuditLog
// @Accept json
// @Produce json
// @Param user_id query uint false "Filter by user ID"
// @Param entity_id query uint false "Filter by entity ID"
// @Param action query string false "Filter by action (partial match)"
// @Param status query string false "Filter by status"
// @Param from_date query string false "Filter from date (YYYY-MM-DD)"
// @Param to_date query string false "Filter to date (YYYY-MM-DD)"
// @Param page query int false "Page number (default: 1)"
// @Param limit query int false "Number of records per page (default: 20)"
// @Success 200 {object} PaginatedAuditLogs
// @Failure 400 {object} gin.H
// @Failure 500 {object} gin.H
// @Router /api/v1/auditlogs [get]
func (h *Handler) GetAuditLogs(c *gin.Context) {
	filter := AuditLogFilter{}

	// Parse query parameters
	if userIDStr := c.Query("user_id"); userIDStr != "" {
		if userID, err := strconv.ParseUint(userIDStr, 10, 32); err == nil {
			uid := uint(userID)
			filter.UserID = &uid
		}
	}

	if entityIDStr := c.Query("entity_id"); entityIDStr != "" {
		if entityID, err := strconv.ParseUint(entityIDStr, 10, 32); err == nil {
			eid := uint(entityID)
			filter.EntityID = &eid
		}
	}

	filter.Action = c.Query("action")
	filter.Status = c.Query("status")

	// Parse dates
	if fromDateStr := c.Query("from_date"); fromDateStr != "" {
		if fromDate, err := time.Parse("2006-01-02", fromDateStr); err == nil {
			filter.FromDate = &fromDate
		} else {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid from_date format. Use YYYY-MM-DD"})
			return
		}
	}

	if toDateStr := c.Query("to_date"); toDateStr != "" {
		if toDate, err := time.Parse("2006-01-02", toDateStr); err == nil {
			// Set to end of day
			endOfDay := toDate.Add(23*time.Hour + 59*time.Minute + 59*time.Second)
			filter.ToDate = &endOfDay
		} else {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid to_date format. Use YYYY-MM-DD"})
			return
		}
	}

	// Parse pagination
	filter.Page = 1
	if pageStr := c.Query("page"); pageStr != "" {
		if page, err := strconv.Atoi(pageStr); err == nil && page > 0 {
			filter.Page = page
		}
	}

	filter.Limit = 20
	if limitStr := c.Query("limit"); limitStr != "" {
		if limit, err := strconv.Atoi(limitStr); err == nil && limit > 0 && limit <= 100 {
			filter.Limit = limit
		}
	}

	// Get audit logs
	result, err := h.service.GetAuditLogs(c.Request.Context(), filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve audit logs"})
		return
	}

	c.JSON(http.StatusOK, result)
}

// GetAuditLogByID handles GET /auditlogs/:id - retrieves a specific audit log by ID
// @Summary Get audit log by ID
// @Description Retrieve a specific audit log by ID (SuperAdmin only)
// @Tags AuditLog
// @Accept json
// @Produce json
// @Param id path uint true "Audit Log ID"
// @Success 200 {object} AuditLogResponse
// @Failure 400 {object} gin.H
// @Failure 404 {object} gin.H
// @Failure 500 {object} gin.H
// @Router /api/v1/auditlogs/{id} [get]
func (h *Handler) GetAuditLogByID(c *gin.Context) {
	idParam := c.Param("id")
	id, err := strconv.ParseUint(idParam, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid audit log ID"})
		return
	}

	log, err := h.service.GetAuditLogByID(c.Request.Context(), uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Audit log not found"})
		return
	}

	c.JSON(http.StatusOK, log)
}

// GetAuditLogStats handles GET /auditlogs/stats - retrieves audit log statistics
// @Summary Get audit log statistics
// @Description Retrieve audit log statistics (SuperAdmin only)
// @Tags AuditLog
// @Accept json
// @Produce json
// @Success 200 {object} gin.H
// @Failure 500 {object} gin.H
// @Router /api/v1/auditlogs/stats [get]
func (h *Handler) GetAuditLogStats(c *gin.Context) {
	// Get last 7 days stats
	now := time.Now()
	lastWeek := now.AddDate(0, 0, -7)

	filter := AuditLogFilter{
		FromDate: &lastWeek,
		ToDate:   &now,
		Limit:    1000, // Get more records for stats
	}

	result, err := h.service.GetAuditLogs(c.Request.Context(), filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve audit log stats"})
		return
	}

	// Calculate stats
	stats := map[string]interface{}{
		"total_last_7_days": result.Total,
		"success_count":     0,
		"failure_count":     0,
		"action_breakdown":  make(map[string]int),
	}

	successCount := 0
	failureCount := 0
	actionBreakdown := make(map[string]int)

	for _, log := range result.Data {
		if log.Status == "success" {
			successCount++
		} else {
			failureCount++
		}
		actionBreakdown[log.Action]++
	}

	stats["success_count"] = successCount
	stats["failure_count"] = failureCount
	stats["action_breakdown"] = actionBreakdown

	c.JSON(http.StatusOK, gin.H{"data": stats})
}