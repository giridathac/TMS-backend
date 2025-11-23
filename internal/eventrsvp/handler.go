package eventrsvp

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/sharath018/temple-management-backend/internal/auth"
	"github.com/sharath018/temple-management-backend/internal/event"
	"github.com/sharath018/temple-management-backend/middleware"
)

// Handler holds services needed for RSVP operations
type Handler struct {
	Service      *Service
	EventService *event.Service
}

// NewHandler creates a new RSVP handler
func NewHandler(service *Service, eventService *event.Service) *Handler {
	return &Handler{Service: service, EventService: eventService}
}

// ✅ Utility to extract user from context
func getUserFromContext(c *gin.Context) (*auth.User, bool) {
	userRaw, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthenticated"})
		return nil, false
	}
	user, ok := userRaw.(auth.User)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid user token"})
		return nil, false
	}
	return &user, true
}

// ✅ Utility to extract access context
func getAccessContext(c *gin.Context) (middleware.AccessContext, bool) {
	accessRaw, exists := c.Get("access_context") // ✅ must match AuthMiddleware
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "access context missing"})
		return middleware.AccessContext{}, false
	}

	accessCtx, ok := accessRaw.(middleware.AccessContext)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid access context"})
		return middleware.AccessContext{}, false
	}

	return accessCtx, true
}

// ✅ Request struct
type RSVPRequest struct {
	Status string `json:"status" binding:"required"` // attending | maybe | not_attending
	Notes  string `json:"notes"`
}

// ==============================
// 🎯 Create or Update RSVP - POST /event-rsvps/:eventID
func (h *Handler) CreateRSVP(c *gin.Context) {
	user, ok := getUserFromContext(c)
	if !ok {
		return
	}

	eventID, err := strconv.Atoi(c.Param("eventID"))
	if err != nil || eventID < 1 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid event ID"})
		return
	}

	// 🎯 Ensure event exists with access context
	accessCtx, ok := getAccessContext(c)
	if !ok {
		return
	}

	_, err = h.EventService.GetEventByID(uint(eventID), accessCtx)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Event not found"})
		return
	}

	var req RSVPRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input: " + err.Error()})
		return
	}

	// ✅ Validate status
	validStatuses := map[string]bool{
		"attending": true, "maybe": true, "not_attending": true,
	}
	if !validStatuses[strings.ToLower(req.Status)] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid RSVP status"})
		return
	}

	// 🚀 Try updating RSVP first
	err = h.Service.UpdateRSVPStatus(uint(eventID), user.ID, strings.ToLower(req.Status), req.Notes)
	if err == nil {
		c.JSON(http.StatusOK, gin.H{"message": "RSVP updated successfully"})
		return
	}

	// ✨ Create new RSVP if no existing found
	rsvp := &RSVP{
		EventID: uint(eventID),
		UserID:  user.ID,
		Status:  strings.ToLower(req.Status),
		Notes:   req.Notes,
	}

	if err := h.Service.CreateRSVP(rsvp); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to RSVP: " + err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "RSVP submitted successfully"})
}

// ==============================
// 🧾 My RSVPs - GET /event-rsvps/my
func (h *Handler) GetMyRSVPs(c *gin.Context) {
	user, ok := getUserFromContext(c)
	if !ok {
		return
	}

	rsvps, err := h.Service.GetMyRSVPs(user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve RSVPs"})
		return
	}

	c.JSON(http.StatusOK, rsvps)
}

// ==============================
// 📋 Get RSVPs for an Event - GET /event-rsvps/:eventID
func (h *Handler) GetRSVPsByEvent(c *gin.Context) {
	eventID, err := strconv.Atoi(c.Param("eventID"))
	if err != nil || eventID < 1 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid event ID"})
		return
	}

	rsvps, err := h.Service.GetRSVPsByEvent(uint(eventID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch RSVPs"})
		return
	}

	c.JSON(http.StatusOK, rsvps)
}
