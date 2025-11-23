package middleware

import (
	"net"
	"strings"
	"github.com/gin-gonic/gin"
)

// AuditMiddleware extracts and stores IP address for audit logging
func AuditMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := getClientIP(c)
		c.Set("client_ip", ip)
		c.Next()
	}
}

// getClientIP extracts the real client IP from various headers
func getClientIP(c *gin.Context) string {
	// Check X-Forwarded-For header (most common for reverse proxies)
	xff := c.GetHeader("X-Forwarded-For")
	if xff != "" {
		// X-Forwarded-For can contain multiple IPs, take the first one
		ips := strings.Split(xff, ",")
		if len(ips) > 0 {
			ip := strings.TrimSpace(ips[0])
			if isValidIP(ip) {
				return ip
			}
		}
	}

	// Check X-Real-Ip header (used by nginx)
	xri := c.GetHeader("X-Real-Ip")
	if xri != "" && isValidIP(xri) {
		return xri
	}

	// Check CF-Connecting-IP header (Cloudflare)
	cfip := c.GetHeader("CF-Connecting-IP")
	if cfip != "" && isValidIP(cfip) {
		return cfip
	}

	// Check X-Forwarded header
	xf := c.GetHeader("X-Forwarded")
	if xf != "" && isValidIP(xf) {
		return xf
	}

	// Fall back to RemoteAddr
	ip, _, err := net.SplitHostPort(c.Request.RemoteAddr)
	if err != nil {
		return c.Request.RemoteAddr
	}

	return ip
}

// isValidIP validates if the string is a valid IP address
func isValidIP(ip string) bool {
	return net.ParseIP(ip) != nil
}

// GetIPFromContext retrieves IP address from gin context
func GetIPFromContext(c *gin.Context) string {
	if ip, exists := c.Get("client_ip"); exists {
		if ipStr, ok := ip.(string); ok {
			return ipStr
		}
	}
	return getClientIP(c)
}