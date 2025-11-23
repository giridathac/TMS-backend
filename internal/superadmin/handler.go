package superadmin

import (
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/sharath018/temple-management-backend/internal/auth"
	"github.com/sharath018/temple-management-backend/middleware"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// =========================== TENANT APPROVAL ===========================

// GET /superadmin/tenants?status=pending&limit=10&page=1
func (h *Handler) GetTenantsWithFilters(c *gin.Context) {
	status := strings.ToLower(c.DefaultQuery("status", "pending"))
	limitStr := c.DefaultQuery("limit", "10")
	pageStr := c.DefaultQuery("page", "1")

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 10
	}
	page, err := strconv.Atoi(pageStr)
	if err != nil || page <= 0 {
		page = 1
	}

	log.Printf("Fetching tenants with status: %s, limit: %d, page: %d", status, limit, page)

	tenants, total, err := h.service.GetTenantsWithFilters(c.Request.Context(), status, limit, page)
	if err != nil {
		log.Printf("Error fetching tenants: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch tenants"})
		return
	}

	log.Printf("Successfully fetched %d tenants (total: %d)", len(tenants), total)
	c.JSON(http.StatusOK, gin.H{
		"data":  tenants,
		"total": total,
		"page":  page,
		"limit": limit,
	})
}

// PATCH /superadmin/tenants/:id
func (h *Handler) UpdateTenantApprovalStatus(c *gin.Context) {
	idStr := c.Param("id")
	userID, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid tenant ID"})
		return
	}

	var body struct {
		Status string `json:"status" binding:"required"`
		Reason string `json:"reason"`
	}

	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Status is required"})
		return
	}

	adminID := c.GetUint("userID")
	action := strings.ToLower(body.Status)
	ip := middleware.GetIPFromContext(c)

	switch action {
	case "approved":
		err = h.service.ApproveTenant(c.Request.Context(), uint(userID), adminID, ip)
	case "rejected":
		if body.Reason == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Rejection reason required"})
			return
		}
		err = h.service.RejectTenant(c.Request.Context(), uint(userID), adminID, body.Reason, ip)
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid status. Use APPROVED or REJECTED"})
		return
	}

	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Tenant status updated successfully"})
}

// =========================== ENTITY APPROVAL ===========================

// GET /superadmin/entities?status=pending&limit=10&page=1
func (h *Handler) GetEntitiesWithFilters(c *gin.Context) {
	// IMPORTANT CHANGE: Don't force status to uppercase, accept any case
	status := c.DefaultQuery("status", "pending")
	limitStr := c.DefaultQuery("limit", "10")
	pageStr := c.DefaultQuery("page", "1")

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 10
	}
	page, err := strconv.Atoi(pageStr)
	if err != nil || page <= 0 {
		page = 1
	}

	log.Printf("Fetching entities with status: %s, limit: %d, page: %d", status, limit, page)

	entities, total, err := h.service.GetEntitiesWithFilters(c.Request.Context(), status, limit, page)
	if err != nil {
		log.Printf("Error fetching entities: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch entities"})
		return
	}

	log.Printf("Successfully fetched %d entities (total: %d)", len(entities), total)
	c.JSON(http.StatusOK, gin.H{
		"data":  entities,
		"total": total,
		"page":  page,
		"limit": limit,
	})
}

// PATCH /superadmin/entities/:id
func (h *Handler) UpdateEntityApprovalStatus(c *gin.Context) {
	idStr := c.Param("id")
	entityID, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid entity ID"})
		return
	}

	var body struct {
		Status string `json:"status" binding:"required"`
		Reason string `json:"reason"`
	}

	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Status is required"})
		return
	}

	adminID := c.GetUint("userID")
	action := strings.ToLower(body.Status)
	ip := middleware.GetIPFromContext(c)

	switch action {
	case "approved":
		err = h.service.ApproveEntity(c.Request.Context(), uint(entityID), adminID, ip)
	case "rejected":
		if body.Reason == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Rejection reason required"})
			return
		}
		err = h.service.RejectEntity(c.Request.Context(), uint(entityID), adminID, body.Reason, ip)
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid status. Use APPROVED or REJECTED"})
		return
	}

	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Entity status updated successfully"})
}

// Replace the duplicate GetTenantDetails functions with this single comprehensive one

// GET /superadmin/tenant-details (get all tenants)
// GET /superadmin/tenant-details/:id (get single tenant by ID)
// GET /superadmin/tenant-details/:ids (get multiple tenants by comma-separated IDs)
func (h *Handler) GetTenantDetails(c *gin.Context) {
	idStr := c.Param("id")
	
	// Case 1: No ID parameter -> return all tenants
	if idStr == "" {
		tenants, err := h.service.GetAllTenantDetails(c.Request.Context())
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch tenant details"})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"data":  tenants,
			"total": len(tenants),
		})
		return
	}
	
	// Case 2: Check if multiple IDs (comma separated)
	if strings.Contains(idStr, ",") {
		ids := strings.Split(idStr, ",")
		tenantIDs := make([]uint, 0, len(ids))
		
		for _, id := range ids {
			tid, err := strconv.ParseUint(strings.TrimSpace(id), 10, 64)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid tenant ID in list"})
				return
			}
			tenantIDs = append(tenantIDs, uint(tid))
		}
		
		details, err := h.service.GetMultipleTenantDetails(c.Request.Context(), tenantIDs)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch tenant details"})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"data":  details,
			"total": len(details),
		})
		return
	}
	
	// Case 3: Single ID
	tenantID, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid tenant ID"})
		return
	}

	details, err := h.service.GetTenantDetails(c.Request.Context(), uint(tenantID))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Tenant details not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": details})
}

// POST /superadmin/tenant-details/multiple (alternative approach for multiple IDs via request body)
func (h *Handler) GetMultipleTenantDetailsByBody(c *gin.Context) {
	var req struct {
		TenantIDs []uint `json:"tenant_ids" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request payload"})
		return
	}

	if len(req.TenantIDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "At least one tenant ID is required"})
		return
	}

	details, err := h.service.GetMultipleTenantDetails(c.Request.Context(), req.TenantIDs)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch tenant details"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":  details,
		"total": len(details),
	})
}

// GET /superadmin/tenant-approval-counts
func (h *Handler) GetTenantApprovalCounts(c *gin.Context) {
	ctx := c.Request.Context()

	counts, err := h.service.GetTenantApprovalCounts(ctx)
	if err != nil {
		log.Printf("Error fetching tenant approval counts: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch tenant approval counts"})
		return
	}

	c.JSON(http.StatusOK, counts)
}

// GET /superadmin/temple-approval-counts
func (h *Handler) GetTempleApprovalCounts(c *gin.Context) {
	ctx := c.Request.Context()

	counts, err := h.service.GetTempleApprovalCounts(ctx)
	if err != nil {
		log.Printf("Error fetching temple approval counts: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch temple approval counts"})
		return
	}

	c.JSON(http.StatusOK, counts)
}

// =========================== USER MANAGEMENT ===========================

// POST /superadmin/users - Create new user (admin-created users)
func (h *Handler) CreateUser(c *gin.Context) {
	var req CreateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate templeadmin details if role is templeadmin
	if strings.ToLower(req.Role) == "templeadmin" {
		if req.TempleName == "" || req.TemplePlace == "" || req.TempleAddress == "" ||
			req.TemplePhoneNo == "" || req.TempleDescription == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "All temple details are required for Temple Admin role"})
			return
		}
	}

	adminID := c.GetUint("userID")
	ip := middleware.GetIPFromContext(c)
	
	if err := h.service.CreateUser(c.Request.Context(), req, adminID, ip); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "User created successfully"})
}

func (h *Handler) GetUsers(c *gin.Context) {
	limitStr := c.DefaultQuery("limit", "10")
	pageStr := c.DefaultQuery("page", "1")
	search := c.DefaultQuery("search", "")
	roleFilter := c.DefaultQuery("role", "") // all, internal, volunteers, devotees
	statusFilter := c.DefaultQuery("status", "")

	limit, _ := strconv.Atoi(limitStr)
	if limit <= 0 { limit = 10 }

	page, _ := strconv.Atoi(pageStr)
	if page <= 0 { page = 1 }

	users, total, err := h.service.GetUsers(
		c.Request.Context(),
		limit,
		page,
		search,
		roleFilter,
		statusFilter,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch users"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":  users,
		"total": total,
		"page":  page,
		"limit": limit,
	})
}


// GET /superadmin/users/:id - Get user by ID
func (h *Handler) GetUserByID(c *gin.Context) {
	idStr := c.Param("id")
	userID, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	user, err := h.service.GetUserByID(c.Request.Context(), uint(userID))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": user})
}

// PUT /superadmin/users/:id - Update user
func (h *Handler) UpdateUser(c *gin.Context) {
	idStr := c.Param("id")
	userID, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	var req UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	adminID := c.GetUint("userID")
	ip := middleware.GetIPFromContext(c)
	
	if err := h.service.UpdateUser(c.Request.Context(), uint(userID), req, adminID, ip); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "User updated successfully"})
}

// DELETE /superadmin/users/:id - Delete user
func (h *Handler) DeleteUser(c *gin.Context) {
	idStr := c.Param("id")
	userID, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	adminID := c.GetUint("userID")
	ip := middleware.GetIPFromContext(c)
	
	if err := h.service.DeleteUser(c.Request.Context(), uint(userID), adminID, ip); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "User deleted successfully"})
}

// PATCH /superadmin/users/:id/status - Activate/Deactivate user
func (h *Handler) UpdateUserStatus(c *gin.Context) {
	idStr := c.Param("id")
	userID, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	var body struct {
		Status string `json:"status" binding:"required"`
	}

	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Status is required"})
		return
	}

	validStatuses := []string{"active", "inactive"}
	status := strings.ToLower(body.Status)
	isValid := false
	for _, validStatus := range validStatuses {
		if status == validStatus {
			isValid = true
			break
		}
	}

	if !isValid {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid status. Use 'active' or 'inactive'"})
		return
	}

	adminID := c.GetUint("userID")
	ip := middleware.GetIPFromContext(c)
	
	if err := h.service.UpdateUserStatus(c.Request.Context(), uint(userID), status, adminID, ip); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "User status updated successfully"})
}

// GET /superadmin/user-roles - Get all available user roles
func (h *Handler) GetUserRoles(c *gin.Context) {
	roles, err := h.service.GetUserRoles(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch user roles"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": roles})
}

// =========================== USER ROLES ===========================

// CreateRole handles the creation of a new user role.
func (h *Handler) CreateRole(c *gin.Context) {
    var req auth.CreateRoleRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request payload. Role name and description are required."})
        return
    }

    adminID := c.GetUint("userID")
    ip := middleware.GetIPFromContext(c)
	err := h.service.CreateRole(c.Request.Context(), &req, adminID, ip)
    if err != nil {
        if strings.Contains(err.Error(), "already exists") {
            c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
            return
        }
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create role."})
        return
    }

    c.JSON(http.StatusCreated, gin.H{"message": "Role created successfully"})
}

// GetRoles retrieves a list of all active user roles.
func (h *Handler) GetRoles(c *gin.Context) {
    roles, err := h.service.GetRoles(c.Request.Context())
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve roles."})
        return
    }

    c.JSON(http.StatusOK, roles)
}

// UpdateRole handles updating a user role.
func (h *Handler) UpdateRole(c *gin.Context) {
	idStr := c.Param("id")
	roleID, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid role ID"})
		return
	}

	var req auth.UpdateRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request payload."})
		return
	}

	adminID := c.GetUint("userID")
	ip := middleware.GetIPFromContext(c)

	err = h.service.UpdateRole(c.Request.Context(), uint(roleID), &req, adminID, ip)
	if err != nil {
		if strings.Contains(err.Error(), "already exists") {
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
			return
		}
		if strings.Contains(err.Error(), "not found") {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update role."})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Role updated successfully"})
}

// ToggleRoleStatus handles activating/inactivating a user role (PATCH request).
func (h *Handler) ToggleRoleStatus(c *gin.Context) {
	idStr := c.Param("id")
	roleID, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid role ID"})
		return
	}

	var req auth.UpdateRoleStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Status is required."})
		return
	}

	adminID := c.GetUint("userID")
	ip := middleware.GetIPFromContext(c)

	err = h.service.ToggleRoleStatus(c.Request.Context(), uint(roleID), req.Status, adminID, ip)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update role status."})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Role status updated successfully"})
}

// =========================== PASSWORD RESET ===========================

// GET /superadmin/users/search?email=user@example.com
func (h *Handler) SearchUserByEmail(c *gin.Context) {
	email := c.Query("email")
	if email == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Email is required"})
		return
	}

	user, err := h.service.SearchUserByEmail(c.Request.Context(), email)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": user})
}

// POST /superadmin/users/:id/reset-password
func (h *Handler) ResetUserPassword(c *gin.Context) {
	idStr := c.Param("id")
	userID, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	var req struct {
		Password  string `json:"password" binding:"required,min=8"`
		SendEmail bool   `json:"sendEmail"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Password must be at least 8 characters"})
		return
	}

	adminID := c.GetUint("userID")

	if err := h.service.ResetUserPassword(c.Request.Context(), uint(userID), req.Password, adminID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Password reset successfully"})
}


// GET /superadmin/tenants/assignable
func (h *Handler) GetTenantsForAssignment(c *gin.Context) {
    // 🎯 Add these lines to parse pagination parameters
    limitStr := c.DefaultQuery("limit", "10")
    pageStr := c.DefaultQuery("page", "1")

    limit, err := strconv.Atoi(limitStr)
    if err != nil || limit <= 0 {
        limit = 10
    }
    page, err := strconv.Atoi(pageStr)
    if err != nil || page <= 0 {
        page = 1
    }

    // 🎯 Pass the pagination parameters to the service layer
    tenants, total, err := h.service.GetTenantsForAssignment(c.Request.Context(), limit, page)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch assignable tenants"})
        return
    }

    // 🎯 Return pagination metadata in the response
    c.JSON(http.StatusOK, gin.H{
        "data": tenants,
        "total": total,
        "page": page,
        "limit": limit,
    })
}
// POST /superadmin/users/assign
func (h *Handler) AssignUsersToTenant(c *gin.Context) {
    var req AssignRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        // Corrected error message to reflect the JSON struct fields.
        c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request payload. 'userId' and 'tenantId' are required"})
        return
    }

    // 🎯 Step 1: Get the user object from the context using the correct key "user".
    user, exists := c.Get("user")
    if !exists {
        c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found in context"})
        return
    }

    // 🎯 Step 2: Type-assert the user object to your `auth.User` struct.
    authenticatedUser, ok := user.(auth.User)
    if !ok {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error: user context type mismatch"})
        return
    }

    // 🎯 Step 3: Use the ID from the authenticated user object.
    adminID := authenticatedUser.ID

    err := h.service.AssignUsersToTenant(c.Request.Context(), req.UserID, req.TenantID, adminID)
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    c.JSON(http.StatusOK, gin.H{"message": "User assigned successfully"})
}

// Alternative handler implementation if userRole is not available in context
// Replace the previous handler method with this one:

// NEW: GET /tenants/selection - Get tenants for selection based on user role
func (h *Handler) GetTenantsForSelection(c *gin.Context) {
	// Get the user object from context (set by AuthMiddleware)
	userVal, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found in context"})
		return
	}

	// Type assert to auth.User
	user, ok := userVal.(auth.User)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user context type"})
		return
	}

	userID := user.ID
	userRole := user.Role.RoleName

	tenants, err := h.service.GetTenantsForSelection(c.Request.Context(), userID, userRole)
	if err != nil {
		if strings.Contains(err.Error(), "unauthorized") {
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch tenants for selection"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": tenants,
		"total": len(tenants),
	})
}

// GET /superadmin/tenants
// GET /superadmin/tenants
func (h *Handler) GetTenants(c *gin.Context) {
    role := c.Query("role")
    status := c.Query("status")
    
    // Always use the enhanced method to include temple details
    tenants, err := h.service.GetTenantsWithTempleDetails(c.Request.Context(), role, status)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    
    c.JSON(http.StatusOK, gin.H{"data": tenants})
}


// POST /superadmin/users/bulk-upload
func (h *Handler) BulkUploadUsers(c *gin.Context) {
    file, err := c.FormFile("file")
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "CSV file is required"})
        return
    }

    f, err := file.Open()
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to open file"})
        return
    }
    defer f.Close()

    adminID := c.GetUint("userID")
    ip := middleware.GetIPFromContext(c)

    result, err := h.service.BulkUploadUsers(c.Request.Context(), f, adminID, ip)
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    c.JSON(http.StatusOK, result)
}






