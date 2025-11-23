package donation

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sharath018/temple-management-backend/middleware"
)

// Handler represents the donation HTTP handler
type Handler struct {
	svc Service
}

// NewHandler creates a new donation handler
func NewHandler(svc Service) *Handler {
	return &Handler{svc: svc}
}

// ===========================
// 📌 Extract Access Context - NEW: Same as event handler
func getAccessContextFromContext(c *gin.Context) (middleware.AccessContext, bool) {
	accessContextRaw, exists := c.Get("access_context")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "access context missing"})
		return middleware.AccessContext{}, false
	}
	
	accessContext, ok := accessContextRaw.(middleware.AccessContext)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid access context"})
		return middleware.AccessContext{}, false
	}
	
	return accessContext, true
}

// ===========================
// 🌟 Extract Entity ID - FIXED: Removed unused variable
func getEntityIDFromRequest(c *gin.Context, accessContext middleware.AccessContext) (uint, error) {
	// Try URL parameter first
	entityIDParam := c.Param("entity_id")
	if entityIDParam != "" {
		id, err := strconv.ParseUint(entityIDParam, 10, 32)
		if err == nil {
			return uint(id), nil
		}
	}
	
	// Try header next
	entityIDHeader := c.GetHeader("X-Entity-ID")
	if entityIDHeader != "" {
		id, err := strconv.ParseUint(entityIDHeader, 10, 32)
		if err == nil {
			return uint(id), nil
		}
	}
	
	// Try query parameter
	entityIDQuery := c.Query("entity_id")
	if entityIDQuery != "" {
		id, err := strconv.ParseUint(entityIDQuery, 10, 32)
		if err == nil {
			return uint(id), nil
		}
	}
	
	// Fall back to access context
	contextEntityID := accessContext.GetAccessibleEntityID()
	if contextEntityID != nil {
		return *contextEntityID, nil
	}
	
	return 0, nil
}

// ==============================
// 🌟 1. Create Donation - UPDATED: Entity-based approach
// ==============================
func (h *Handler) CreateDonation(c *gin.Context) {
	// NEW: Use access context instead of direct user context
	accessContext, ok := getAccessContextFromContext(c)
	if !ok {
		return
	}
	
	// NEW: Extract entity ID using the same logic as events
	entityID, err := getEntityIDFromRequest(c, accessContext)
	if err != nil || entityID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "user is not linked to a temple and no entity_id provided"})
		return
	}

	var req CreateDonationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// NEW: Use access context data and extracted entity ID
	req.UserID = accessContext.UserID
	req.EntityID = entityID // NEW: Use extracted entity ID
	req.IPAddress = middleware.GetIPFromContext(c)

	order, err := h.svc.StartDonation(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":    order,
		"success": true,
	})
}

// ==============================
// ✅ 2. Verify Razorpay Signature - UNCHANGED
// ==============================
func (h *Handler) VerifyDonation(c *gin.Context) {
	var req VerifyPaymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	req.IPAddress = middleware.GetIPFromContext(c)

	if err := h.svc.VerifyAndUpdateDonation(req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Donation verified successfully",
		"success": true,
	})
}

// ==============================
// 🔍 3. Get My Donations - UPDATED: Entity-based approach
// ==============================
func (h *Handler) GetMyDonations(c *gin.Context) {
	// NEW: Use access context
	accessContext, ok := getAccessContextFromContext(c)
	if !ok {
		return
	}
	
	// NEW: Extract entity ID for filtering
	entityID, err := getEntityIDFromRequest(c, accessContext)
	if err != nil || entityID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "user is not linked to a temple and no entity_id provided"})
		return
	}

	// NEW: Pass entity ID to service for filtering
	donations, err := h.svc.GetDonationsByUserAndEntity(accessContext.UserID, entityID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":    donations,
		"count":   len(donations),
		"success": true,
	})
}

// ==============================
// 🔍 4. Get All Donations for Temple - UPDATED: Enhanced entity handling
// ==============================
func (h *Handler) GetDonationsByEntity(c *gin.Context) {
	accessContext, ok := getAccessContextFromContext(c)
	if !ok {
		return
	}

	// NEW: Extract entity ID using same logic as events
	entityID, err := getEntityIDFromRequest(c, accessContext)
	if err != nil || entityID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "user is not linked to a temple and no entity_id provided"})
		return
	}

	// Parse query parameters
	filters := DonationFilters{
		EntityID: entityID, // NEW: Use extracted entity ID
		Page:     parseIntQuery(c, "page", 1),
		Limit:    parseIntQuery(c, "limit", 20),
		Status:   c.Query("status"),
		Type:     c.Query("type"),
		Method:   c.Query("method"),
		Search:   c.Query("search"),
	}

	// Parse date filters
	if fromStr := c.Query("from"); fromStr != "" {
		if from, err := time.Parse("2006-01-02", fromStr); err == nil {
			filters.From = &from
		}
	}
	if toStr := c.Query("to"); toStr != "" {
		if to, err := time.Parse("2006-01-02", toStr); err == nil {
			filters.To = &to
		}
	}

	// Parse amount filters
	if minStr := c.Query("min"); minStr != "" {
		if min, err := strconv.ParseFloat(minStr, 64); err == nil {
			filters.MinAmount = &min
		}
	}
	if maxStr := c.Query("max"); maxStr != "" {
		if max, err := strconv.ParseFloat(maxStr, 64); err == nil {
			filters.MaxAmount = &max
		}
	}

	// Handle date range presets
	switch c.Query("dateRange") {
	case "today":
		today := time.Now().Truncate(24 * time.Hour)
		filters.From = &today
		tomorrow := today.Add(24 * time.Hour)
		filters.To = &tomorrow
	case "week":
		weekAgo := time.Now().AddDate(0, 0, -7)
		filters.From = &weekAgo
	case "month":
		monthAgo := time.Now().AddDate(0, -1, 0)
		filters.From = &monthAgo
	case "year":
		yearAgo := time.Now().AddDate(-1, 0, 0)
		filters.From = &yearAgo
	}

	donations, total, err := h.svc.GetDonationsWithFilters(filters, accessContext)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":        donations,
		"total":       total,
		"page":        filters.Page,
		"limit":       filters.Limit,
		"total_pages": (total + filters.Limit - 1) / filters.Limit,
		"success":     true,
	})
}

// ==============================
// 📊 5. Get Donation Dashboard Stats - UPDATED: Enhanced entity handling
// ==============================
func (h *Handler) GetDashboard(c *gin.Context) {
	accessContext, ok := getAccessContextFromContext(c)
	if !ok {
		return
	}

	// NEW: Extract entity ID using same logic as events
	entityID, err := getEntityIDFromRequest(c, accessContext)
	if err != nil || entityID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "user is not linked to a temple and no entity_id provided"})
		return
	}

	stats, err := h.svc.GetDashboardStats(entityID, accessContext)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":    stats,
		"success": true,
	})
}

// ==============================
// 🏆 6. Get Top Donors - UPDATED: Enhanced entity handling
// ==============================
func (h *Handler) GetTopDonors(c *gin.Context) {
	accessContext, ok := getAccessContextFromContext(c)
	if !ok {
		return
	}

	// NEW: Extract entity ID using same logic as events
	entityID, err := getEntityIDFromRequest(c, accessContext)
	if err != nil || entityID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "user is not linked to a temple and no entity_id provided"})
		return
	}

	limit := parseIntQuery(c, "limit", 5)
	topDonors, err := h.svc.GetTopDonors(entityID, limit, accessContext)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":    topDonors,
		"success": true,
	})
}

// ==============================
// 📄 7. Generate Receipt - UPDATED: Enhanced entity and user handling
// ==============================
func (h *Handler) GenerateReceipt(c *gin.Context) {
	donationIDStr := c.Param("id")
	donationID, err := strconv.ParseUint(donationIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid donation ID"})
		return
	}

	// NEW: Use access context instead of direct user context
	accessContext, ok := getAccessContextFromContext(c)
	if !ok {
		return
	}

	// NEW: For devotees, ensure they can only access their own donations by entity
	var entityID uint
	if accessContext.RoleName == "devotee" {
		extractedEntityID, err := getEntityIDFromRequest(c, accessContext)
		if err != nil || extractedEntityID == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "user is not linked to a temple and no entity_id provided"})
			return
		}
		entityID = extractedEntityID
	} else {
		// For temple admins, get their accessible entity
		extractedEntityID, err := getEntityIDFromRequest(c, accessContext)
		if err != nil || extractedEntityID == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "user is not linked to a temple and no entity_id provided"})
			return
		}
		entityID = extractedEntityID
	}

	receipt, err := h.svc.GenerateReceipt(uint(donationID), accessContext.UserID, &accessContext, entityID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":    receipt,
		"success": true,
	})
}

// ==============================
// 📈 8. Get Donation Analytics - UPDATED: Enhanced entity handling
// ==============================
func (h *Handler) GetAnalytics(c *gin.Context) {
	accessContext, ok := getAccessContextFromContext(c)
	if !ok {
		return
	}

	// NEW: Extract entity ID using same logic as events
	entityID, err := getEntityIDFromRequest(c, accessContext)
	if err != nil || entityID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "user is not linked to a temple and no entity_id provided"})
		return
	}

	days := parseIntQuery(c, "days", 30)
	analytics, err := h.svc.GetAnalytics(entityID, days, accessContext)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":    analytics,
		"success": true,
	})
}

// ==============================
// 📊 9. Export Donations - UPDATED: Enhanced entity handling
// ==============================
func (h *Handler) ExportDonations(c *gin.Context) {
	accessContext, ok := getAccessContextFromContext(c)
	if !ok {
		return
	}

	// NEW: Extract entity ID using same logic as events
	entityID, err := getEntityIDFromRequest(c, accessContext)
	if err != nil || entityID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "user is not linked to a temple and no entity_id provided"})
		return
	}

	format := c.DefaultQuery("format", "csv")

	// Build filters for export
	filters := DonationFilters{
		EntityID: entityID, // NEW: Use extracted entity ID
		Status:   c.Query("status"),
		Type:     c.Query("type"),
		Method:   c.Query("method"),
		Search:   c.Query("search"),
		Page:     1,
		Limit:    10000, // Large limit for export
	}

	// Parse date filters
	if fromStr := c.Query("from"); fromStr != "" {
		if from, err := time.Parse("2006-01-02", fromStr); err == nil {
			filters.From = &from
		}
	}
	if toStr := c.Query("to"); toStr != "" {
		if to, err := time.Parse("2006-01-02", toStr); err == nil {
			filters.To = &to
		}
	}

	fileContent, filename, err := h.svc.ExportDonations(filters, format, accessContext)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Header("Content-Disposition", "attachment; filename="+filename)
	c.Header("Content-Type", "application/octet-stream")
	c.Data(http.StatusOK, "application/octet-stream", fileContent)
}

// ==============================
// 🕐 10. Get Recent Donations - UPDATED: Enhanced entity handling
// ==============================
func (h *Handler) GetRecentDonations(c *gin.Context) {
	accessContext, ok := getAccessContextFromContext(c)
	if !ok {
		return
	}

	limitStr := c.DefaultQuery("limit", "10")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 10
	}

	// NEW: Extract entity ID using same logic as events
	entityID, err := getEntityIDFromRequest(c, accessContext)
	if err != nil || entityID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "user is not linked to a temple and no entity_id provided"})
		return
	}

	// For devotees, get their own donations within the entity
	if accessContext.RoleName == "devotee" {
		recent, err := h.svc.GetRecentDonationsByUserAndEntity(c.Request.Context(), accessContext.UserID, entityID, limit)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not fetch recent donations"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"recent_donations": recent,
			"success": true,
		})
		return
	}

	// For temple admins, get donations for their accessible entity
	recent, err := h.svc.GetRecentDonationsByEntity(c.Request.Context(), entityID, limit, accessContext)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not fetch recent donations"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"recent_donations": recent,
		"success": true,
	})
}

// Helper function to parse integer query parameters
func parseIntQuery(c *gin.Context, key string, defaultValue int) int {
	if str := c.Query(key); str != "" {
		if val, err := strconv.Atoi(str); err == nil && val > 0 {
			return val
		}
	}
	return defaultValue
}