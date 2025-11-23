package notification

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/sharath018/temple-management-backend/config"
	"github.com/sharath018/temple-management-backend/internal/auditlog"
	"github.com/sharath018/temple-management-backend/middleware"
	"github.com/sharath018/temple-management-backend/utils"
)

type Handler struct {
	Service  Service
	AuditSvc auditlog.Service
}

func NewHandler(s Service, auditSvc auditlog.Service) *Handler {
	return &Handler{
		Service:  s,
		AuditSvc: auditSvc,
	}
}

// POST /api/v1/notifications/templates
func (h *Handler) CreateTemplate(c *gin.Context) {
	accessContext, exists := c.Get("access_context")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "access context missing"})
		return
	}

	ctx := accessContext.(middleware.AccessContext)

	if !ctx.CanWrite() {
		c.JSON(http.StatusForbidden, gin.H{"error": "write access denied"})
		return
	}

	entityID := ctx.GetAccessibleEntityID()
	if entityID == nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "no accessible temple"})
		return
	}

	ip := middleware.GetIPFromContext(c)

	var input NotificationTemplate
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	input.UserID = ctx.UserID
	input.EntityID = *entityID

	if err := h.Service.CreateTemplate(c.Request.Context(), &input, ip); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create template"})
		return
	}

	c.JSON(http.StatusCreated, input)
}

// GET /api/v1/notifications/templates
func (h *Handler) GetTemplates(c *gin.Context) {
	accessContext, exists := c.Get("access_context")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "access context missing"})
		return
	}

	ctx := accessContext.(middleware.AccessContext)

	entityID := ctx.GetAccessibleEntityID()
	if entityID == nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "no accessible temple"})
		return
	}

	templates, err := h.Service.GetTemplates(c.Request.Context(), *entityID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch templates"})
		return
	}

	c.JSON(http.StatusOK, templates)
}

// GET /api/v1/notifications/templates/:id
func (h *Handler) GetTemplateByID(c *gin.Context) {
	accessContext, exists := c.Get("access_context")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "access context missing"})
		return
	}

	ctx := accessContext.(middleware.AccessContext)

	entityID := ctx.GetAccessibleEntityID()
	if entityID == nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "no accessible temple"})
		return
	}

	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid template id"})
		return
	}

	template, err := h.Service.GetTemplateByID(c.Request.Context(), uint(id), *entityID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "template not found"})
		return
	}

	c.JSON(http.StatusOK, template)
}

// PUT /api/v1/notifications/templates/:id
func (h *Handler) UpdateTemplate(c *gin.Context) {
	accessContext, exists := c.Get("access_context")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "access context missing"})
		return
	}

	ctx := accessContext.(middleware.AccessContext)

	if !ctx.CanWrite() {
		c.JSON(http.StatusForbidden, gin.H{"error": "write access denied"})
		return
	}

	entityID := ctx.GetAccessibleEntityID()
	if entityID == nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "no accessible temple"})
		return
	}

	ip := middleware.GetIPFromContext(c)

	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid template id"})
		return
	}

	var input NotificationTemplate
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	input.ID = uint(id)
	input.UserID = ctx.UserID
	input.EntityID = *entityID

	if err := h.Service.UpdateTemplate(c.Request.Context(), &input, ip); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update template"})
		return
	}

	c.JSON(http.StatusOK, input)
}

// DELETE /api/v1/notifications/templates/:id
func (h *Handler) DeleteTemplate(c *gin.Context) {
	accessContext, exists := c.Get("access_context")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "access context missing"})
		return
	}

	ctx := accessContext.(middleware.AccessContext)

	if !ctx.CanWrite() {
		c.JSON(http.StatusForbidden, gin.H{"error": "write access denied"})
		return
	}

	entityID := ctx.GetAccessibleEntityID()
	if entityID == nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "no accessible temple"})
		return
	}

	ip := middleware.GetIPFromContext(c)

	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid template id"})
		return
	}

	if err := h.Service.DeleteTemplate(c.Request.Context(), uint(id), *entityID, ctx.UserID, ip); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete template"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "template deleted"})
}

// POST /api/v1/notifications/send
func (h *Handler) SendNotification(c *gin.Context) {
	accessContext, exists := c.Get("access_context")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "access context missing"})
		return
	}

	ctx := accessContext.(middleware.AccessContext)

	if !ctx.CanWrite() {
		c.JSON(http.StatusForbidden, gin.H{"error": "write access denied"})
		return
	}

	entityID := ctx.GetAccessibleEntityID()
	if entityID == nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "no accessible temple"})
		return
	}

	ip := middleware.GetIPFromContext(c)

	var req struct {
		TemplateID *uint    `json:"template_id"`
		Channel    string   `json:"channel" binding:"required"` // email, sms, whatsapp, push
		Subject    string   `json:"subject"`
		Body       string   `json:"body" binding:"required"`
		Recipients []string `json:"recipients"`
		Audience   string   `json:"audience"` // all, devotees, volunteers
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// If recipients not provided, resolve using audience
	if len(req.Recipients) == 0 {
		switch req.Audience {
		case "all":
			emails, err := h.Service.GetEmailsByAudience(*entityID, "all")
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch users for audience"})
				return
			}
			req.Recipients = emails

		case "devotees", "volunteers":
			emails, err := h.Service.GetEmailsByAudience(*entityID, req.Audience)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch users for audience"})
				return
			}
			req.Recipients = emails

		default:
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid or missing audience if no recipients are provided"})
			return
		}
	}

	// Send notification asynchronously
	go func() {
		bgCtx := context.Background()
		
		if err := h.Service.SendNotification(
			bgCtx,
			ctx.UserID,
			*entityID,
			req.TemplateID,
			req.Channel,
			req.Subject,
			req.Body,
			req.Recipients,
			ip,
		); err != nil {
			fmt.Printf("❌ Background notification send error: %v\n", err)
		}
	}()

	c.JSON(http.StatusAccepted, gin.H{
		"message":          "notification queued for sending",
		"recipients_count": len(req.Recipients),
		"status":           "processing",
	})
}

// GET /api/v1/notifications/logs
func (h *Handler) GetMyNotifications(c *gin.Context) {
	accessContext, exists := c.Get("access_context")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "access context missing"})
		return
	}

	ctx := accessContext.(middleware.AccessContext)

	logs, err := h.Service.GetNotificationsByUser(c.Request.Context(), ctx.UserID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch logs"})
		return
	}

	c.JSON(http.StatusOK, logs)
}

// GET /api/v1/notifications/inapp
func (h *Handler) GetMyInApp(c *gin.Context) {
	accessContext, exists := c.Get("access_context")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "access context missing"})
		return
	}
	ctx := accessContext.(middleware.AccessContext)
	var entityIDPtr *uint
	if id := ctx.GetAccessibleEntityID(); id != nil {
		entityIDPtr = id
	}
	limitStr := c.DefaultQuery("limit", "20")
	limit, _ := strconv.Atoi(limitStr)

	items, err := h.Service.ListInAppByUser(c.Request.Context(), ctx.UserID, entityIDPtr, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch in-app notifications"})
		return
	}
	c.JSON(http.StatusOK, items)
}

// PUT /api/v1/notifications/inapp/:id/read
func (h *Handler) MarkInAppRead(c *gin.Context) {
	accessContext, exists := c.Get("access_context")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "access context missing"})
		return
	}
	ctx := accessContext.(middleware.AccessContext)
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	if err := h.Service.MarkInAppAsRead(c.Request.Context(), uint(id), ctx.UserID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to mark as read"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "marked as read"})
}

// GET /api/v1/notifications/stream (SSE)
func (h *Handler) StreamInApp(c *gin.Context) {
	accessContext, exists := c.Get("access_context")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "access context missing"})
		return
	}
	ctx := accessContext.(middleware.AccessContext)

	// ✅ Set CORS headers for SSE
	origin := c.GetHeader("Origin")
	if origin == "" {
		origin = "http://localhost:4173"
	}
	c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
	c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
	
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no")

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		c.Status(http.StatusInternalServerError)
		return
	}

	channel := "notifications:user:" + strconv.FormatUint(uint64(ctx.UserID), 10)
	sub := utils.RedisClient.Subscribe(c, channel)
	defer sub.Close()

	_, _ = c.Writer.Write([]byte(":ok\n\n"))
	flusher.Flush()

	ch := sub.Channel()
	for {
		select {
		case msg, ok := <-ch:
			if !ok {
				return
			}
			_, _ = c.Writer.Write([]byte("event: inapp\n"))
			_, _ = c.Writer.Write([]byte("data: " + msg.Payload + "\n\n"))
			flusher.Flush()
		case <-c.Request.Context().Done():
			return
		}
	}
}

// GET /api/v1/notifications/stream-token?token=JWT (SSE without auth middleware)
func (h *Handler) StreamInAppWithToken(c *gin.Context) {
	tokenStr := c.Query("token")
	if tokenStr == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing token"})
		return
	}

	cfg := config.Load()
	token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
		return []byte(cfg.JWTAccessSecret), nil
	})
	if err != nil || !token.Valid {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
		return
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid claims"})
		return
	}
	uidFloat, ok := claims["user_id"].(float64)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user_id missing"})
		return
	}
	userID := uint(uidFloat)

	// ✅ CRITICAL FIX: Set specific CORS headers for SSE (cannot use wildcard with credentials)
	origin := c.GetHeader("Origin")
	allowedOrigins := map[string]bool{
		"http://localhost:5173":   true,
		"http://127.0.0.1:5173":   true,
		"http://localhost:4173":   true,
		"http://127.0.0.1:4173":   true,
	}
	
	// Validate origin and set specific header
	if origin == "" || !allowedOrigins[origin] {
		origin = "http://localhost:4173" // Default fallback
	}
	
	c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
	c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
	c.Writer.Header().Set("Access-Control-Expose-Headers", "Content-Type")
	
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no")

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		c.Status(http.StatusInternalServerError)
		return
	}

	channel := "notifications:user:" + strconv.FormatUint(uint64(userID), 10)
	sub := utils.RedisClient.Subscribe(c, channel)
	defer sub.Close()

	_, _ = c.Writer.Write([]byte(":ok\n\n"))
	flusher.Flush()

	ch := sub.Channel()
	for {
		select {
		case msg, ok := <-ch:
			if !ok {
				return
			}
			_, _ = c.Writer.Write([]byte("event: inapp\n"))
			_, _ = c.Writer.Write([]byte("data: " + msg.Payload + "\n\n"))
			flusher.Flush()
		case <-c.Request.Context().Done():
			return
		}
	}
}

// ✅ NEW: POST /api/v1/notifications/fcm/register
// Register FCM device token for current user
func (h *Handler) RegisterFCMToken(c *gin.Context) {
	accessContext, exists := c.Get("access_context")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "access context missing"})
		return
	}

	ctx := accessContext.(middleware.AccessContext)

	entityID := ctx.GetAccessibleEntityID()
	if entityID == nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "no accessible temple"})
		return
	}

	var req struct {
		DeviceToken string `json:"device_token" binding:"required"`
		DeviceType  string `json:"device_type"`  // android, ios, web
		DeviceName  string `json:"device_name"`  // optional
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err := h.Service.RegisterDeviceToken(
		c.Request.Context(),
		ctx.UserID,
		*entityID,
		req.DeviceToken,
		req.DeviceType,
		req.DeviceName,
	)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to register device token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "device token registered successfully"})
}

// ✅ NEW: DELETE /api/v1/notifications/fcm/unregister
// Remove FCM device token
func (h *Handler) UnregisterFCMToken(c *gin.Context) {
	accessContext, exists := c.Get("access_context")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "access context missing"})
		return
	}

	ctx := accessContext.(middleware.AccessContext)

	var req struct {
		DeviceToken string `json:"device_token" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err := h.Service.RemoveDeviceToken(c.Request.Context(), ctx.UserID, req.DeviceToken)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to unregister device token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "device token removed successfully"})
}

// ✅ NEW: POST /api/v1/notifications/fcm/send
// Send push notification to specific users
func (h *Handler) SendFCMNotification(c *gin.Context) {
	accessContext, exists := c.Get("access_context")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "access context missing"})
		return
	}

	ctx := accessContext.(middleware.AccessContext)

	if !ctx.CanWrite() {
		c.JSON(http.StatusForbidden, gin.H{"error": "write access denied"})
		return
	}

	entityID := ctx.GetAccessibleEntityID()
	if entityID == nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "no accessible temple"})
		return
	}

	ip := middleware.GetIPFromContext(c)

	var req struct {
		Title    string `json:"title" binding:"required"`
		Body     string `json:"body" binding:"required"`
		UserIDs  []uint `json:"user_ids"`  // specific user IDs
		Roles    []string `json:"roles"`    // or by roles (devotees, volunteers, etc.)
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Send to specific users or roles
	go func() {
		bgCtx := context.Background()
		var err error

		if len(req.UserIDs) > 0 {
			err = h.Service.SendPushNotification(bgCtx, ctx.UserID, *entityID, req.Title, req.Body, req.UserIDs, ip)
		} else if len(req.Roles) > 0 {
			err = h.Service.SendPushToRoles(bgCtx, ctx.UserID, *entityID, req.Title, req.Body, req.Roles, ip)
		} else {
			fmt.Println("❌ No user IDs or roles specified for push notification")
			return
		}

		if err != nil {
			fmt.Printf("❌ Failed to send push notification: %v\n", err)
		}
	}()

	c.JSON(http.StatusAccepted, gin.H{
		"message": "push notification queued for sending",
		"status":  "processing",
	})
}