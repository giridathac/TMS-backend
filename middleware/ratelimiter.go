package middleware

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ulule/limiter/v3"
	ginlimiter "github.com/ulule/limiter/v3/drivers/middleware/gin"
	memory "github.com/ulule/limiter/v3/drivers/store/memory"
)

// RateLimiterMiddleware returns a Gin middleware that limits requests per IP
// In middleware/ratelimiter.go
func RateLimiter() gin.HandlerFunc {
    // Increase these values
    store := memory.NewStore()
    rate := limiter.Rate{
        Period: 1 * time.Minute, // Keep the period
        Limit:  100,             // Increase from default (likely 60)
    }

	// 📊 Limiter instance
	instance := limiter.New(store, rate)

	// 🚦 Gin-compatible middleware
	return ginlimiter.NewMiddleware(instance)
}
