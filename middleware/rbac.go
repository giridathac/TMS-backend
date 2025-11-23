package middleware

import (
	"net/http"
	"strings"
	
	"github.com/gin-gonic/gin"
	"github.com/sharath018/temple-management-backend/internal/auth"
)

// RBACMiddleware checks if the user has one of the allowed roles
func RBACMiddleware(allowedRoles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Special case for standard users accessing entities endpoint
		if c.Request.URL.Path == "/api/v1/entities" && 
		   (c.Request.Method == "GET" || c.Request.Method == "POST") {
			userVal, exists := c.Get("user")
			if exists {
				if user, ok := userVal.(auth.User); ok && 
				   (user.Role.RoleName == "standarduser" || user.Role.RoleName == "monitoringuser") {
					// Allow standard users to access this endpoint for both GET and POST
					c.Next()
					return
				}
			}
		}

		userVal, exists := c.Get("user")
		if !exists {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthenticated"})
			return
		}

		user, ok := userVal.(auth.User)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid user object"})
			return
		}

		// Always set both user and userID in context for downstream handlers
		c.Set("user", user)
		c.Set("userID", user.ID)

		// Check if the user has one of the allowed roles
		for _, role := range allowedRoles {
			if user.Role.RoleName == role {
				c.Next()
				return
			}
		}

		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "unauthorized"})
	}
}

// RequireTempleAccess with proper tenant isolation
func RequireTempleAccess() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get user from context
		userVal, exists := c.Get("user")
		if !exists {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "user not found in context"})
			return
		}
		
		user, ok := userVal.(auth.User)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid user object"})
			return
		}
		
		// Get access context from auth middleware
		accessContextVal, exists := c.Get("access_context")
		if !exists {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "access context missing"})
			return
		}
		
		accessContext, ok := accessContextVal.(AccessContext)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid access context"})
			return
		}
		
		// Check if this is a tenant user management endpoint
		// Allow these endpoints even if no temple exists
		if strings.Contains(c.Request.URL.Path, "/tenants/") && strings.Contains(c.Request.URL.Path, "/user") {
			c.Next()
			return
		}
		
		// FIXED: Role-based access control with tenant isolation
		switch user.Role.RoleName {
		case RoleSuperAdmin:
			// Superadmin can access any entity, but should be scoped to requested tenant
			c.Next()
			return
			
		case RoleTempleAdmin:
			// Temple admin can only access their own temple and related entities
			if accessContext.DirectEntityID == nil {
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
					"error": "templeadmin must have a direct entity assigned",
				})
				return
			}
			c.Next()
			return
			
		case RoleStandardUser, RoleMonitoringUser:
			// Standard/monitoring users can only access their assigned tenant
			if accessContext.AssignedEntityID == nil && accessContext.DirectEntityID == nil {
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
					"error": "user must have an assigned entity",
				})
				return
			}
			c.Next()
			return
		
		case RoleDevotee, RoleVolunteer:
			// Devotees and volunteers can access their associated temple
			if accessContext.AssignedEntityID == nil && accessContext.DirectEntityID == nil {
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
					"error": "devotee/volunteer must have an associated entity",
				})
				return
			}
			
			// Set permission type to readonly for regular endpoints
			// This ensures devotees can view but not modify temple data
			accessContext.PermissionType = "readonly"
			c.Set("access_context", accessContext)
			
			c.Next()
			return
			
		default:
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "unsupported role"})
			return
		}
	}
}

// RequireWriteAccess ensures user has write access
func RequireWriteAccess() gin.HandlerFunc {
	return func(c *gin.Context) {
		userVal, exists := c.Get("user")
		if exists {
			user, ok := userVal.(auth.User)
			if ok {
				// Superadmin and templeadmin always have write access
				if user.Role.RoleName == RoleSuperAdmin || user.Role.RoleName == RoleTempleAdmin {
					c.Next()
					return
				}
			}
		}
		
		accessContext, exists := c.Get("access_context")
		if !exists {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "access context missing"})
			return
		}

		ctx, ok := accessContext.(AccessContext)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid access context"})
			return
		}
		
		if !ctx.CanWrite() {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "write access denied"})
			return
		}

		c.Next()
	}
}