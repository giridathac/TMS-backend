package routes

import (
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/sharath018/temple-management-backend/config"
	"github.com/sharath018/temple-management-backend/database"
	"github.com/sharath018/temple-management-backend/internal/auditlog"
	"github.com/sharath018/temple-management-backend/internal/auth"
	"github.com/sharath018/temple-management-backend/internal/donation"
	"github.com/sharath018/temple-management-backend/internal/entity"
	"github.com/sharath018/temple-management-backend/internal/event"
	"github.com/sharath018/temple-management-backend/internal/eventrsvp"
	"github.com/sharath018/temple-management-backend/internal/notification"
	"github.com/sharath018/temple-management-backend/internal/reports"
	"github.com/sharath018/temple-management-backend/internal/seva"
	"github.com/sharath018/temple-management-backend/internal/superadmin"
	"github.com/sharath018/temple-management-backend/internal/tenant"
	"github.com/sharath018/temple-management-backend/internal/userprofile"
	"github.com/sharath018/temple-management-backend/middleware"

	_ "github.com/sharath018/temple-management-backend/docs"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

// Debug route to check upload directory structure
func addUploadDebugging(r *gin.Engine) {
	// Debug route to check upload directory structure - Updated to use persistent volume
	r.GET("/debug/uploads", func(c *gin.Context) {
		uploadPath := "/data/uploads" // Changed from ./uploads to /data/uploads

		// Check if uploads directory exists
		if _, err := os.Stat(uploadPath); os.IsNotExist(err) {
			c.JSON(http.StatusOK, gin.H{
				"error": "uploads directory does not exist",
				"path":  uploadPath,
			})
			return
		}

		// Walk through the uploads directory
		var files []map[string]interface{}

		err := filepath.WalkDir(uploadPath, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}

			// Get file info
			info, err := d.Info()
			if err != nil {
				return err
			}

			relativePath := strings.TrimPrefix(path, uploadPath)
			if relativePath == "" {
				relativePath = "/"
			}

			files = append(files, map[string]interface{}{
				"path":     relativePath,
				"name":     d.Name(),
				"is_dir":   d.IsDir(),
				"size":     info.Size(),
				"mod_time": info.ModTime().Format(time.RFC3339),
			})

			return nil
		})

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "failed to walk directory",
				"details": err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"upload_path": uploadPath,
			"files":       files,
			"total_files": len(files),
		})
	})

	// Debug route to check specific file - Updated path
	r.GET("/debug/file/*filepath", func(c *gin.Context) {
		filePath := c.Param("filepath")
		fullPath := filepath.Join("/data/uploads", filePath) // Changed from ./uploads

		// Check if file exists
		info, err := os.Stat(fullPath)
		if os.IsNotExist(err) {
			c.JSON(http.StatusNotFound, gin.H{
				"error":     "file not found",
				"file_path": filePath,
				"full_path": fullPath,
			})
			return
		}

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "failed to stat file",
				"details": err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"file_path": filePath,
			"full_path": fullPath,
			"exists":    true,
			"size":      info.Size(),
			"is_dir":    info.IsDir(),
			"mod_time":  info.ModTime().Format(time.RFC3339),
		})
	})
}

func Setup(r *gin.Engine, cfg *config.Config) {
	// Ensure uploads directory exists - Updated to use persistent volume
	uploadPath := "/data/uploads" // Changed from ./uploads
	if err := os.MkdirAll(uploadPath, 0755); err != nil {
		fmt.Printf("Warning: Could not create uploads directory: %v\n", err)
	}

	// Add static file serving for the public directory
	r.Static("/public", "./public")

	// The /uploads static route is now handled in main.go only
	// File serving is now handled by the /files/*filepath route in main.go

	// Add debugging routes (remove in production)
	addUploadDebugging(r)

	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "OK"})
	})
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// NEW: Add a direct route for reset password
	r.GET("/auth-pages/reset-password", func(c *gin.Context) {
		token := c.Query("token")
		if token == "" {
			c.HTML(http.StatusBadRequest, "reset_password.html", gin.H{
				"error": "No reset token provided. Please check your email link.",
			})
			return
		}
		c.HTML(http.StatusOK, "reset_password.html", gin.H{
			"token": token,
		})
	})

	api := r.Group("/api/v1")
	api.Use(middleware.RateLimiter())     // Global rate limit: 5 req/sec per IP
	api.Use(middleware.AuditMiddleware()) // Audit middleware to capture IP

	// ========== Initialize Audit Log Module ==========
	auditRepo := auditlog.NewRepository(database.DB)
	auditSvc := auditlog.NewService(auditRepo)
	auditHandler := auditlog.NewHandler(auditSvc)

	// ========== Auth ==========
	authRepo := auth.NewRepository(database.DB)
	authSvc := auth.NewService(authRepo, cfg)
	authHandler := auth.NewHandler(authSvc)

	authGroup := api.Group("/auth")
	{
		authGroup.POST("/register", authHandler.Register)
		authGroup.POST("/login", authHandler.Login)
		authGroup.POST("/refresh", authHandler.Refresh)

		// Forgot/Reset/Logout
		authGroup.POST("/forgot-password", authHandler.ForgotPassword)
		authGroup.POST("/reset-password", authHandler.ResetPassword)

		// Public roles endpoint for registration (no auth required)
		authGroup.GET("/public-roles", authHandler.GetPublicRoles)

		// Logout requires Auth Middleware
		authGroup.POST("/logout", middleware.AuthMiddleware(cfg, authSvc), authHandler.Logout)
	}

	protected := api.Group("/")
	protected.Use(middleware.AuthMiddleware(cfg, authSvc))

	// Dashboards
	protected.GET("/tenant/dashboard", middleware.RBACMiddleware("templeadmin"), func(c *gin.Context) {
		c.JSON(200, gin.H{"message": "Temple Admin dashboard access granted!"})
	})
	protected.GET("/entity/:id/devotee/dashboard", middleware.RBACMiddleware("devotee"), func(c *gin.Context) {
		c.JSON(200, gin.H{"message": "Devotee dashboard access granted!"})
	})
	protected.GET("/entity/:id/volunteer/dashboard", middleware.RBACMiddleware("volunteer"), func(c *gin.Context) {
		c.JSON(200, gin.H{"message": "Volunteer dashboard access granted!"})
	})

	// ========== Audit Logs (SuperAdmin Only) ==========
	auditRoutes := protected.Group("/auditlogs")
	auditRoutes.Use(middleware.RBACMiddleware("superadmin"))
	{
		auditRoutes.GET("/", auditHandler.GetAuditLogs)
		auditRoutes.GET("/:id", auditHandler.GetAuditLogByID)
		auditRoutes.GET("/stats", auditHandler.GetAuditLogStats)
	}

	// ========== Super Admin ==========
	superadminRepo := superadmin.NewRepository(database.DB)
	superadminService := superadmin.NewService(superadminRepo, auditSvc)
	superadminHandler := superadmin.NewHandler(superadminService)

	superadminRoutes := protected.Group("/superadmin")
	superadminRoutes.Use(middleware.RBACMiddleware("superadmin"))

	{
		// ================ TENANT APPROVAL MANAGEMENT ================
		// Paginated list of all tenants with optional ?status=pending&limit=10&page=1
		superadminRoutes.GET("/tenants", superadminHandler.GetTenantsWithFilters)
		superadminRoutes.PATCH("/tenants/:id/approval", superadminHandler.UpdateTenantApprovalStatus)

		// ================ ENTITY APPROVAL MANAGEMENT ================
		// Paginated list of entities with optional ?status=pending&limit=10&page=1
		superadminRoutes.GET("/entities", superadminHandler.GetEntitiesWithFilters)
		superadminRoutes.PATCH("/entities/:id/approval", superadminHandler.UpdateEntityApprovalStatus)

		superadminRoutes.GET("/tenant-details/:id", superadminHandler.GetTenantDetails)
		superadminRoutes.GET("/tenant-details", superadminHandler.GetTenantDetails)

		// ================ DASHBOARD METRICS ================
		superadminRoutes.GET("/tenant-approval-count", superadminHandler.GetTenantApprovalCounts)
		superadminRoutes.GET("/temple-approval-count", superadminHandler.GetTempleApprovalCounts)

		// ================ USER MANAGEMENT ================
		// Create new user (admin-created users)
		superadminRoutes.POST("/users", superadminHandler.CreateUser)

		// Get all users with pagination and filters (excluding devotee and volunteer)
		// Query params: ?limit=10&page=1&search=john&role=templeadmin&status=active
		superadminRoutes.GET("/users", superadminHandler.GetUsers)

		// Get user by ID
		superadminRoutes.GET("/users/:id", superadminHandler.GetUserByID)

		// Update user
		superadminRoutes.PUT("/users/:id", superadminHandler.UpdateUser)

		// Delete user (soft delete)
		superadminRoutes.DELETE("/users/:id", superadminHandler.DeleteUser)

		// Activate/Deactivate user
		superadminRoutes.PATCH("/users/:id/status", superadminHandler.UpdateUserStatus)

		// Get all available user roles
		superadminRoutes.GET("/user-roles", superadminHandler.GetUserRoles)

		// Role management routes
		superadminRoutes.GET("/roles", superadminHandler.GetRoles)
		superadminRoutes.POST("/roles", superadminHandler.CreateRole)
		superadminRoutes.PUT("/roles/:id", superadminHandler.UpdateRole)
		superadminRoutes.PATCH("/roles/:id/status", superadminHandler.ToggleRoleStatus)

		// Reset user password (superadmin resets any user's password)
		superadminRoutes.POST("/users/:id/reset-password", superadminHandler.ResetUserPassword)
		superadminRoutes.GET("/users/search", superadminHandler.SearchUserByEmail)
		superadminRoutes.GET("/tenants/assignable", superadminHandler.GetTenantsForAssignment)
		// Assigns a list of users to a selected temple/tenant
		superadminRoutes.POST("/users/assign", superadminHandler.AssignUsersToTenant)
		// Bulk upload users via CSV
		superadminRoutes.POST("/users/bulk-upload", superadminHandler.BulkUploadUsers)

		// ================ SUPERADMIN REPORTS ================
		// Add dedicated routes for reports with multiple tenants
		reportsRepo := reports.NewRepository(database.DB)
		reportsExporter := reports.NewReportExporter()
		reportsService := reports.NewReportService(reportsRepo, reportsExporter, auditSvc)
		reportsHandler := reports.NewHandler(reportsService, reportsRepo, auditSvc)

		// Reports endpoints for superadmin with multiple tenants support
		superadminRoutes.GET("/reports/activities", reportsHandler.GetSuperAdminActivities)
		superadminRoutes.GET("/reports/temple-registered", reportsHandler.GetSuperAdminTempleRegisteredReport)
		superadminRoutes.GET("/reports/devotee-birthdays", reportsHandler.GetSuperAdminDevoteeBirthdaysReport)
		superadminRoutes.GET("/reports/devotee-list", reportsHandler.GetSuperAdminDevoteeListReport)
		superadminRoutes.GET("/reports/devotee-profile", reportsHandler.GetSuperAdminDevoteeProfileReport)
		superadminRoutes.GET("/reports/audit-logs", reportsHandler.GetSuperAdminAuditLogsReport)
		superadminRoutes.GET("/reports/approval-status", reportsHandler.GetApprovalStatusReport)
		superadminRoutes.GET("/reports/user-details", reportsHandler.GetUserDetailsReport)

		// Support for tenant-specific routes (for backwards compatibility)
		superadminRoutes.GET("/tenants/:id/reports/activities", reportsHandler.GetSuperAdminTenantActivities)
		superadminRoutes.GET("/tenants/:id/reports/temple-registered", reportsHandler.GetSuperAdminTenantTempleRegisteredReport)
		superadminRoutes.GET("/tenants/:id/reports/devotee-birthdays", reportsHandler.GetSuperAdminTenantDevoteeBirthdaysReport)
		superadminRoutes.GET("/tenants/:id/reports/devotee-list", reportsHandler.GetSuperAdminTenantDevoteeListReport)
		superadminRoutes.GET("/tenants/:id/reports/devotee-profile", reportsHandler.GetSuperAdminTenantDevoteeProfileReport)
		superadminRoutes.GET("/tenants/:id/reports/audit-logs", reportsHandler.GetSuperAdminTenantAuditLogsReport)
	}

	protected.GET("/tenants/selection",
		middleware.RBACMiddleware("superadmin", "standarduser", "monitoringuser"),
		superadminHandler.GetTenantsForSelection)

	// ========== Seva Routes ==========
	// ==================== SEVA ROUTES ====================

sevaRepo := seva.NewRepository(database.DB)
sevaService := seva.NewService(sevaRepo, auditSvc)
sevaHandler := seva.NewHandler(sevaService, auditSvc)

// 🔥 FIX: Ensure protected group has CORS enabled (very important)
protected.Use(cors.New(cors.Config{
    AllowOrigins:     []string{"http://localhost:4173", "http://127.0.0.1:4173", "http://localhost:5173"},
    AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
    AllowHeaders:     []string{"Origin", "Content-Type", "Authorization", "X-Tenant-ID"},
    AllowCredentials: true,
}))

// All Seva routes under: /api/v1/sevas
sevaRoutes := protected.Group("sevas")


sevaRoutes.GET("/booking-counts", sevaHandler.GetBookingCounts)


templeSevaRoutes := sevaRoutes.Group("")
templeSevaRoutes.Use(middleware.RequireTempleAccess()) // access check

{
	
	writeRoutes := templeSevaRoutes.Group("")
	writeRoutes.Use(middleware.RequireWriteAccess()) // only templeadmin + standarduser

	{
		// CRUD for Sevas
		writeRoutes.POST("/", sevaHandler.CreateSeva)
		writeRoutes.PUT("/:id", sevaHandler.UpdateSeva)
		writeRoutes.DELETE("/:id", sevaHandler.DeleteSeva)

		// Booking status update
		writeRoutes.PATCH("/bookings/:id/status", sevaHandler.UpdateBookingStatus)
	}

	
	templeSevaRoutes.GET("/entity-sevas", sevaHandler.ListEntitySevas)
	templeSevaRoutes.GET("/:id", sevaHandler.GetSevaByID)
	templeSevaRoutes.GET("/entity-bookings", sevaHandler.GetEntityBookings)
	templeSevaRoutes.GET("/bookings/:id", sevaHandler.GetBookingByID)
}



devoteeSevaRoutes := sevaRoutes.Group("")
devoteeSevaRoutes.Use(middleware.RBACMiddleware("devotee"))

{
	devoteeSevaRoutes.POST("/bookings", sevaHandler.BookSeva)
	devoteeSevaRoutes.GET("/my-bookings", sevaHandler.GetMyBookings)
	devoteeSevaRoutes.GET("/", sevaHandler.GetSevas)
}

// ========== Entity ==========
{
	entityRepo := entity.NewRepository(database.DB)
	profileRepo := userprofile.NewRepository(database.DB)
	profileService := userprofile.NewService(profileRepo, authRepo, auditSvc)
	profileHandler := userprofile.NewHandler(profileService)

	entityService := entity.NewService(entityRepo, profileService, auditSvc)
	// UPDATED: Use persistent volume path and proper file serving path
	entityHandler := entity.NewHandler(entityService, "/data/uploads", "/files")

	// Add special endpoint for templeadmins to view their created entities
	protected.GET("/entities/by-creator", middleware.RBACMiddleware("templeadmin"), func(c *gin.Context) {
		// Get user ID from context
		userVal, exists := c.Get("user")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
			return
		}

		user, ok := userVal.(auth.User)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid user object"})
			return
		}

		// Call repository to get entities created by this user
		entities, err := entityRepo.GetEntitiesByCreator(user.ID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch temples", "details": err.Error()})
			return
		}

		c.JSON(http.StatusOK, entities)
	})

	// Entity routes with proper permission system
	entityRoutes := protected.Group("/entities")
	// Allow templeadmin, standarduser, monitoringuser to access entity routes
	entityRoutes.Use(middleware.RequireTempleAccess())
	{
		// Write operations - only templeadmin and standarduser can access
		writeRoutes := entityRoutes.Group("")
		writeRoutes.Use(middleware.RequireWriteAccess())
		{
			writeRoutes.PUT("/:id", entityHandler.UpdateEntity)
			writeRoutes.DELETE("/:id", entityHandler.DeleteEntity)
			writeRoutes.PATCH("/:id/devotees/:userID/status", entityHandler.UpdateDevoteeMembershipStatus)
		}

		// Read operations - all three roles can access
		entityRoutes.GET("/:id", entityHandler.GetEntityByID)
		entityRoutes.GET("/:id/devotees", entityHandler.GetDevoteesByEntity)
		entityRoutes.GET("/:id/devotee-stats", entityHandler.GetDevoteeStats)
		entityRoutes.GET("/:id/devotees/:userId/profile", profileHandler.GetDevoteeProfileByEntity) // ✅ UPDATED: Changed :entityId to :id
		entityRoutes.GET("/dashboard-summary", entityHandler.GetDashboardSummary)
		
		// File routes for entity documents
		entityRoutes.GET("/:id/files", entityHandler.GetEntityFiles)
		entityRoutes.GET("/directories", entityHandler.GetAllEntityDirectories)
	}

	// Special endpoints that bypass temple access check
	// CreateEntity - allowed for templeadmin, superadmin, standarduser
	protected.POST("/entities",
		middleware.RBACMiddleware("templeadmin", "superadmin", "standarduser"),
		entityHandler.CreateEntity,
	)

	// GetAllEntities - allowed for templeadmin, superadmin, standarduser, monitoringuser
	protected.GET("/entities",
		middleware.RBACMiddleware("templeadmin", "superadmin", "standarduser", "monitoringuser"),
		entityHandler.GetAllEntities,
	)
}

	// ========== Event & RSVP ==========
	eventRepo := event.NewRepository(database.DB)
	eventService := event.NewService(eventRepo, auditSvc)
	var notifSvc notification.Service
	eventHandler := event.NewHandler(eventService)

	// Event routes - all require temple access
	eventRoutes := protected.Group("/events")
	eventRoutes.Use(middleware.RequireTempleAccess())
	{
		// Write operations - only templeadmin and standarduser can access
		writeRoutes := eventRoutes.Group("")
		writeRoutes.Use(middleware.RequireWriteAccess())
		{
			writeRoutes.POST("/", eventHandler.CreateEvent)
			writeRoutes.POST("", eventHandler.CreateEvent)
			writeRoutes.PUT("/:id", eventHandler.UpdateEvent)
			writeRoutes.DELETE("/:id", eventHandler.DeleteEvent)
		}

		// Read operations - all three roles can access
		eventRoutes.GET("/", eventHandler.ListEvents)
		eventRoutes.GET("/:id", eventHandler.GetEventByID)
		eventRoutes.GET("/upcoming", eventHandler.GetUpcomingEvents)
		eventRoutes.GET("/stats", eventHandler.GetEventStats)
	}

	// Event RSVP routes (keeping existing logic for devotee/volunteer access)
	{
		rsvpRepo := eventrsvp.NewRepository(database.DB)
		rsvpService := eventrsvp.NewService(rsvpRepo, eventService)
		rsvpHandler := eventrsvp.NewHandler(rsvpService, eventService)

		rsvpRoutes := protected.Group("/event-rsvps")
		rsvpRoutes.POST("/:eventID", middleware.RBACMiddleware("devotee", "volunteer"), rsvpHandler.CreateRSVP)
		rsvpRoutes.GET("/:eventID", middleware.RBACMiddleware("devotee"), rsvpHandler.GetRSVPsByEvent)
		rsvpRoutes.GET("/my", middleware.RBACMiddleware("devotee", "volunteer"), rsvpHandler.GetMyRSVPs)
	}

	// ========== User Profile & Membership ==========
	profileRepo := userprofile.NewRepository(database.DB)
	profileService := userprofile.NewService(profileRepo, authRepo, auditSvc)
	profileHandler := userprofile.NewHandler(profileService)

	profileRoutes := protected.Group("/profiles")
	{
		profileRoutes.GET("/me", middleware.RBACMiddleware("devotee"), profileHandler.GetMyProfile)
		profileRoutes.POST("/", middleware.RBACMiddleware("devotee"), profileHandler.CreateOrUpdateProfile)
		profileRoutes.PUT("/", middleware.RBACMiddleware("devotee"), profileHandler.CreateOrUpdateProfile)
	
	//entityRoutes.GET("/:entityId/devotees/:userId/profile", profileHandler.GetDevoteeProfileByEntity)
	}
	// ========== Membership (Join Temples) ==========
	membershipRoutes := protected.Group("/memberships")
	{
		membershipRoutes.POST("", middleware.RBACMiddleware("devotee", "volunteer"), profileHandler.JoinTemple)
		membershipRoutes.GET("", middleware.RBACMiddleware("devotee", "volunteer"), profileHandler.ListMemberships)
	}

	// ========== Temple Search ==========
	templeSearchRoutes := protected.Group("/temples")
	{
		templeSearchRoutes.GET("/search", middleware.RBACMiddleware("devotee", "volunteer"), profileHandler.SearchTemples)
		templeSearchRoutes.GET("/recent", middleware.RBACMiddleware("devotee", "volunteer"), profileHandler.GetRecentTemples)
	}

	// ========== Donations with New Permission System ==========
	{
		donationRepo := donation.NewRepository(database.DB)
		donationService := donation.NewService(donationRepo, cfg, auditSvc)
		donationHandler := donation.NewHandler(donationService)

		donationRoutes := protected.Group("/donations")
		{
			// ========== DEVOTEE ROUTES (UNCHANGED) ==========
			devoteeRoutes := donationRoutes.Group("")
			devoteeRoutes.Use(middleware.RBACMiddleware("devotee"))
			{
				devoteeRoutes.POST("/", donationHandler.CreateDonation)
				devoteeRoutes.POST("/verify", donationHandler.VerifyDonation)
				devoteeRoutes.GET("/my", donationHandler.GetMyDonations)
			}

			// ========== TEMPLE ADMIN ROUTES (UPDATED PERMISSIONS) ==========
			templeRoutes := donationRoutes.Group("")
			templeRoutes.Use(middleware.RequireTempleAccess()) // Allow templeadmin, standarduser, monitoringuser
			{
				// Read-only operations - all three roles can access
				templeRoutes.GET("/", donationHandler.GetDonationsByEntity)
				templeRoutes.GET("/dashboard", donationHandler.GetDashboard)
				templeRoutes.GET("/top-donors", donationHandler.GetTopDonors)
				templeRoutes.GET("/analytics", donationHandler.GetAnalytics)

				// Write operations - only templeadmin and standarduser can access
				writeRoutes := templeRoutes.Group("")
				writeRoutes.Use(middleware.RequireWriteAccess())
				{
					writeRoutes.GET("/export", donationHandler.ExportDonations)
				}
			}

			// ========== SHARED ROUTES (BOTH DEVOTEE AND TEMPLE ADMIN) ==========
			// Receipt generation - both devotees and temple admins can access
			donationRoutes.GET("/:id/receipt",
				middleware.RBACMiddleware("devotee", "templeadmin", "standarduser", "monitoringuser"),
				donationHandler.GenerateReceipt)

			// Recent donations - both devotees and temple admins can access
			donationRoutes.GET("/recent",
				middleware.RBACMiddleware("devotee", "templeadmin", "standarduser", "monitoringuser"),
				donationHandler.GetRecentDonations)
		}
	}
// ========== Notifications (UPDATED WITH FCM) ==========
{
	notificationRepo := notification.NewRepository(database.DB)
	notifSvc = notification.NewService(notificationRepo, authRepo, cfg, auditSvc)
	notificationHandler := notification.NewHandler(notifSvc, auditSvc)

	// Updated to use new middleware system
	notificationRoutes := protected.Group("/notifications")
	notificationRoutes.Use(middleware.RequireTempleAccess()) // Allow templeadmin, standarduser, monitoringuser
	{
		// Write operations - only templeadmin and standarduser can access
		writeRoutes := notificationRoutes.Group("")
		writeRoutes.Use(middleware.RequireWriteAccess())
		{
			// Templates
			writeRoutes.POST("/templates", notificationHandler.CreateTemplate)
			writeRoutes.PUT("/templates/:id", notificationHandler.UpdateTemplate)
			writeRoutes.DELETE("/templates/:id", notificationHandler.DeleteTemplate)

			// Send Notification (Email, SMS, WhatsApp, Push)
			writeRoutes.POST("/send", notificationHandler.SendNotification)

			// ✅ NEW: FCM Push Notifications (Write Access Required)
			writeRoutes.POST("/fcm/send", notificationHandler.SendFCMNotification)
		}

		// Read operations - all three roles can access
		notificationRoutes.GET("/templates", notificationHandler.GetTemplates)
		notificationRoutes.GET("/templates/:id", notificationHandler.GetTemplateByID)

		// View Logs
		notificationRoutes.GET("/logs", notificationHandler.GetMyNotifications)

		// In-app
		notificationRoutes.GET("/inapp", notificationHandler.GetMyInApp)
		notificationRoutes.PUT("/inapp/:id/read", notificationHandler.MarkInAppRead)
		notificationRoutes.GET("/stream", notificationHandler.StreamInApp)
	}

	// ✅ NEW: FCM Device Token Management (All authenticated users can register their devices)
	fcmRoutes := protected.Group("/notifications/fcm")
	{
		// Any authenticated user can register/unregister their own device
		fcmRoutes.POST("/register", notificationHandler.RegisterFCMToken)
		fcmRoutes.DELETE("/unregister", notificationHandler.UnregisterFCMToken)
	}
}
	// Public token-based SSE stream (no auth middleware required)
	api.GET("/notifications/stream-token", func(c *gin.Context) {
		notificationRepo := notification.NewRepository(database.DB)
		notifSvc := notification.NewService(notificationRepo, authRepo, cfg, auditSvc)
		handler := notification.NewHandler(notifSvc, auditSvc)
		handler.StreamInAppWithToken(c)
	})

	// Now inject notifSvc into eventService
	eventService.NotifSvc = notifSvc
	sevaService.SetNotifService(notifSvc)

	// ========== Tenant User Management ==========
	tenantRepo := tenant.NewRepository(database.DB)
	tenantService := tenant.NewService(tenantRepo)
	tenantHandler := tenant.NewHandler(tenantService)

	// Tenant user routes (templeadmin + standarduser manage, monitoringuser read-only)
	tenantRoutes := protected.Group("/tenants/:id/user")
	tenantRoutes.Use(middleware.RequireTempleAccess()) // restrict to members of this temple
	{
		// Read operations - all 3 roles can access
		tenantRoutes.GET("/management", tenantHandler.GetUsers)
		// Add this inside the tenant routes group
		tenantRoutes.PATCH("/:id/user/:userId/status", tenantHandler.UpdateUserStatus)

		// Write operations - only templeadmin + standarduser
		writeRoutes := tenantRoutes.Group("")
		writeRoutes.Use(middleware.RequireWriteAccess())
		{
			writeRoutes.POST("/", tenantHandler.CreateOrUpdateUser)
			writeRoutes.PUT("/:userId", tenantHandler.UpdateUser)
		}
	}

	// ========== Reports ==========
	{
		reportsRepo := reports.NewRepository(database.DB)
		reportsExporter := reports.NewReportExporter()
		reportsService := reports.NewReportService(reportsRepo, reportsExporter, auditSvc)
		reportsHandler := reports.NewHandler(reportsService, reportsRepo, auditSvc)

		reportsRoutes := protected.Group("/entities/:id/reports")
		reportsRoutes.Use(middleware.RequireTempleAccess()) // Allow templeadmin, standarduser, monitoringuser
		{
			// All report endpoints are read-only by default, but may generate downloadable files
			// Since report generation can be considered a "sensitive" operation, we can optionally require write access
			// However, for now, let's allow all three roles to view and export reports

			reportsRoutes.GET("/activities", reportsHandler.GetActivities)
			reportsRoutes.GET("/temple-registered", reportsHandler.GetTempleRegisteredReport)
			reportsRoutes.GET("/devotee-birthdays", reportsHandler.GetDevoteeBirthdaysReport)
			reportsRoutes.GET("/devotee-list", reportsHandler.GetDevoteeListReport)
			reportsRoutes.GET("/devotee-profile", reportsHandler.GetDevoteeProfileReport)
			reportsRoutes.GET("/audit-logs", reportsHandler.GetAuditLogsReport)

			// If you want to restrict export functionality to only users with write access,
			// you can create a separate group with write access requirement:
			/*
				exportRoutes := reportsRoutes.Group("")
				exportRoutes.Use(middleware.RequireWriteAccess())
				{
					exportRoutes.GET("/activities", reportsHandler.GetActivities) // when ?format= is provided
					exportRoutes.GET("/temple-registered", reportsHandler.GetTempleRegisteredReport) // when ?format= is provided
					// ... other export endpoints
				}
			*/
		}
	}

	// Serve the SPA (Single Page Application) for any other route
	r.NoRoute(func(c *gin.Context) {
		// Check if the request is for an API endpoint
		if strings.HasPrefix(c.Request.URL.Path, "/api") {
			// If it's an API route that wasn't found, return 404 JSON
			c.JSON(http.StatusNotFound, gin.H{"error": "API endpoint not found"})
			return
		}

		// For upload files that don't exist, provide helpful error - Updated path
		if strings.HasPrefix(c.Request.URL.Path, "/uploads") {
			c.JSON(http.StatusNotFound, gin.H{
				"error":   "File not found",
				"message": "The requested file does not exist or has been moved",
				"path":    c.Request.URL.Path,
				"note":    "Files are now served from /data/uploads directory",
			})
			return
		}

		// Serve the index.html file for all other routes
		c.File("./public/index.html")
	})
}
