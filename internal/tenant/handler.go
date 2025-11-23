package tenant

import (
    "net/http"
    "strconv"
    "github.com/gin-gonic/gin"
    "log"
)

// Handler handles HTTP requests
type Handler struct {
    service *Service
}

// NewHandler creates a new handler instance
func NewHandler(service *Service) *Handler {
    return &Handler{service: service}
}

// UpdateUser handles the PUT request to update a user
// UpdateUser handles the PUT request to update a user
func (h *Handler) UpdateUser(c *gin.Context) {
    // Get tenant ID and user ID from route parameters
    tenantIDStr := c.Param("id")
    userIDStr := c.Param("userId")
    
    log.Printf("🔵 Updating user %s for tenant %s", userIDStr, tenantIDStr)
    
    tenantID, err := strconv.ParseUint(tenantIDStr, 10, 64)
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid tenant ID"})
        return
    }
    
    userID, err := strconv.ParseUint(userIDStr, 10, 64)
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
        return
    }
    
    // First log raw data for debugging
    var rawData map[string]interface{}
    if err := c.ShouldBindJSON(&rawData); err != nil {
        log.Printf("🔴 Error binding raw JSON: %v", err)
        c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON format"})
        return
    }
    
    log.Printf("🔵 Received raw update data: %+v", rawData)
    
    // Check if this is a status-only update
    if status, exists := rawData["Status"].(string); exists && len(rawData) <= 2 { // Allow for Status and maybe ID
        log.Printf("🔵 Processing as status update: %s for user %d", status, userID)
        
        // Check if user belongs to this tenant
        exists, err := h.service.repo.CheckUserBelongsToTenant(uint(userID), uint(tenantID))
        if err != nil {
            log.Printf("🔴 Error checking user-tenant relationship: %v", err)
            c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to verify user-tenant relationship"})
            return
        }
        
        if !exists {
            c.JSON(http.StatusBadRequest, gin.H{"error": "User does not belong to this tenant"})
            return
        }
        
        // Update status in both tables
        err = h.service.repo.UpdateUserStatus(uint(userID), uint(tenantID), status)
        if err != nil {
            log.Printf("🔴 Error updating status: %v", err)
            c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update status: " + err.Error()})
            return
        }
        
        c.JSON(http.StatusOK, gin.H{
            "message": "Status updated successfully",
        })
        return
    }
    
    // Handle as a full user update by creating a UserInput from the raw data
    input := UserInput{}
    
    // Only set fields that are present in the raw data
    if name, ok := rawData["Name"].(string); ok {
        input.Name = name
    }
    if email, ok := rawData["Email"].(string); ok {
        input.Email = email
    }
    if phone, ok := rawData["Phone"].(string); ok {
        input.Phone = phone
    }
    if role, ok := rawData["Role"].(string); ok {
        input.Role = role
    }
    if password, ok := rawData["Password"].(string); ok {
        input.Password = password
    }
    if status, ok := rawData["Status"].(string); ok {
        input.Status = status
    }
    
    log.Printf("🔵 Updating user %d for tenant %d: %s (%s), Role: %s, Status: %s", 
        userID, tenantID, input.Name, input.Email, input.Role, input.Status)
    
    // Check if we have the minimum required fields
    if input.Name == "" || input.Email == "" {
        log.Printf("🔴 Missing required fields: Name or Email")
        c.JSON(http.StatusBadRequest, gin.H{"error": "Name and Email are required fields"})
        return
    }
    
    // Update the user with the gathered input
    user, err := h.service.UpdateUser(uint(tenantID), uint(userID), input, input.Status)
    if err != nil {
        log.Printf("🔴 Error updating user: %v", err)
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update user: " + err.Error()})
        return
    }
    
    log.Printf("✅ User updated successfully: %+v", user)
    c.JSON(http.StatusOK, gin.H{
        "message": "User updated successfully",
        "user": user,
    })
}

// UpdateUserStatus updates only a user's status
func (h *Handler) UpdateUserStatus(c *gin.Context) {
    tenantIDStr := c.Param("id")
    userIDStr := c.Param("userId")

    log.Printf("🔵 Updating status for user %s in tenant %s", userIDStr, tenantIDStr)

    tenantID, err := strconv.ParseUint(tenantIDStr, 10, 64)
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid tenant ID"})
        return
    }

    userID, err := strconv.ParseUint(userIDStr, 10, 64)
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
        return
    }

    var statusData struct {
        Status string `json:"Status"`
    }

    if err := c.ShouldBindJSON(&statusData); err == nil && statusData.Status != "" {
        log.Printf("🔵 Received status update: %s for user %d", statusData.Status, userID)

        user, err := h.service.UpdateUser(uint(tenantID), uint(userID), UserInput{}, statusData.Status)
        if err != nil {
            log.Printf("🔴 Error updating status: %v", err)
            c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update status: " + err.Error()})
            return
        }

        c.JSON(http.StatusOK, gin.H{
            "message": "Status updated successfully",
            "user": user,
        })
        return
    }
} // ✅ Correctly closed


// GetUsers handles the GET request to fetch tenant users
func (h *Handler) GetUsers(c *gin.Context) {
    // CRITICAL DEBUGGING
    log.Printf("🔴 GET USERS - Request path: %s", c.Request.URL.Path)
    log.Printf("🔴 GET USERS - All params: %v", c.Params)
    
    // Get tenant ID from route parameter
    tenantIDStr := c.Param("id")
    log.Printf("🔴 GET USERS - Raw tenant ID from route param: %s", tenantIDStr)
    
    tenantID, err := strconv.ParseUint(tenantIDStr, 10, 64)
    if err != nil {
        log.Printf("🔴 ERROR - Invalid tenant ID: %v", err)
        c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid tenant ID"})
        return
    }
    
    log.Printf("🔴 GET USERS - Using tenant ID: %d", tenantID)
    
    role := c.Query("role")
    
    users, err := h.service.GetTenantUsers(uint(tenantID), role)
    if err != nil {
        log.Printf("Failed to fetch users: %v", err)
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch users: " + err.Error()})
        return
    }
    
    // Always return an array, even if empty
    if users == nil {
        users = []UserResponse{}
    }
    
    log.Printf("Returning %d users for tenant %d", len(users), tenantID)
    c.JSON(http.StatusOK, users)
}

// CreateOrUpdateUser handles the POST request to create or update a tenant user
func (h *Handler) CreateOrUpdateUser(c *gin.Context) {
    // CRITICAL DEBUGGING
    log.Printf("🔴 CREATE USER - Request path: %s", c.Request.URL.Path)
    log.Printf("🔴 CREATE USER - All params: %v", c.Params)
    
    // Get tenant ID preferring the X-Tenant-ID header over route parameter
    var tenantID uint64
    var err error
    
    // First try to get tenant ID from header
    tenantIDHeader := c.GetHeader("X-Tenant-ID")
    log.Printf("🔴 CREATE USER - X-Tenant-ID header: %s", tenantIDHeader)
    
    if tenantIDHeader != "" {
        tenantID, err = strconv.ParseUint(tenantIDHeader, 10, 64)
        if err == nil {
            log.Printf("🔴 CREATE USER - Using tenant ID from header: %d", tenantID)
        } else {
            log.Printf("🔴 ERROR - Invalid tenant ID in header: %v", err)
        }
    }
    
    // If header parsing failed, fall back to route parameter
    if err != nil || tenantIDHeader == "" {
        tenantIDStr := c.Param("id")
        log.Printf("🔴 CREATE USER - Raw tenant ID from route param: %s", tenantIDStr)
        
        tenantID, err = strconv.ParseUint(tenantIDStr, 10, 64)
        if err != nil {
            log.Printf("🔴 ERROR - Invalid tenant ID in route: %v", err)
            c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid tenant ID"})
            return
        }
        log.Printf("🔴 CREATE USER - Using tenant ID from route param: %d", tenantID)
    }
    
    var input UserInput
    if err := c.ShouldBindJSON(&input); err != nil {
        log.Printf("Invalid input: %v", err)
        c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input: " + err.Error()})
        return
    }
    
    // Get the creator ID from the authenticated user
    // This assumes you have middleware that sets user ID in the context
    creatorID, exists := c.Get("user_id")
    if !exists {
        // If no user ID in context, try to get it from JWT claim or header
        creatorIDStr := c.GetHeader("X-User-ID")
        if creatorIDStr != "" {
            creatorIDUint, err := strconv.ParseUint(creatorIDStr, 10, 64)
            if err == nil {
                creatorID = uint(creatorIDUint)
            }
        }
    }
    
    // Default to 1 if we couldn't get a valid creator ID
    creatorIDUint := uint(1)
    if id, ok := creatorID.(uint); ok {
        creatorIDUint = id
    } else if id, ok := creatorID.(float64); ok {
        creatorIDUint = uint(id)
    } else if id, ok := creatorID.(int); ok {
        creatorIDUint = uint(id)
    } else if id, ok := creatorID.(uint64); ok {
        creatorIDUint = uint(id)
    }
    
    log.Printf("Creating/updating user %s (%s) for tenant %d by creator %d", 
               input.Name, input.Email, tenantID, creatorIDUint)
    
    // Pass the creator ID to the service
    user, err := h.service.CreateOrUpdateTenantUser(uint(tenantID), input, creatorIDUint)
    if err != nil {
        log.Printf("Failed to create/update user: %v", err)
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create/update user: " + err.Error()})
        return
    }
    
    log.Printf("User created/updated successfully: %s (ID: %d) for tenant ID: %d", 
               user.Email, user.ID, tenantID)
    c.JSON(http.StatusOK, gin.H{
        "message": "User created and assigned successfully",
        "user": user,
    })
}