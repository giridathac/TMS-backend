package auth

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

type Handler struct{ service Service }

func NewHandler(s Service) *Handler { return &Handler{s} }

// ===============================
// Registration
// ===============================

type RegisterRequest struct {
	FullName string `json:"fullName" binding:"required" example:"Sharath Kumar"`
	Email    string `json:"email" binding:"required,email" example:"example@gmail.com"`
	Password string `json:"password" binding:"required,min=6" example:"secret123"`
	Role     string `json:"role" binding:"required" example:"templeadmin"`
	Phone    string `json:"phone" binding:"required" example:"+919876543210"`
	// ✅ Temple admin specific fields
	TempleName        string `json:"templeName" example:"Sri Venkateswara Temple"`
	TemplePlace       string `json:"templePlace" example:"Tirupati"`
	TempleAddress     string `json:"templeAddress" example:"Main Road, Tirupati, Andhra Pradesh"`
	TemplePhoneNo     string `json:"templePhoneNo" example:"+918765432100"`
	TempleDescription string `json:"templeDescription" example:"Historic temple dedicated to Lord Venkateswara."`
}

func (h *Handler) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// ❌ Block superadmin registration
	if strings.ToLower(req.Role) == "superadmin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Super Admin registration is not allowed"})
		return
	}

	// ✅ Validate Gmail only
	if !isGmail(req.Email) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Only @gmail.com emails are allowed for registration"})
		return
	}

	// ✅ Validate templeadmin details early
	if strings.ToLower(req.Role) == "templeadmin" {
		if req.TempleName == "" || req.TemplePlace == "" || req.TempleAddress == "" ||
			req.TemplePhoneNo == "" || req.TempleDescription == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "All temple details are required for Temple Admin registration"})
			return
		}
	}

	// ✅ Map to input and pass to service
	input := RegisterInput(req)

	if err := h.service.Register(input); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if strings.ToLower(req.Role) == "templeadmin" {
		c.JSON(http.StatusCreated, gin.H{"message": "Temple Admin registered. Awaiting approval."})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "Registration successful"})
}

// 🔍 Email helper
func isGmail(email string) bool {
	return strings.HasSuffix(strings.ToLower(email), "@gmail.com")
}

// ===============================
// Login
// ===============================

type loginReq struct {
	Email    string `json:"email" binding:"required,email" example:"sharath@example.com"`
	Password string `json:"password" binding:"required" example:"secret123"`
}

func (h *Handler) Login(c *gin.Context) {
	var req loginReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	tokens, user, err := h.service.Login(LoginInput(req))
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	userPayload := gin.H{
		"id":       user.ID,
		"fullName": user.FullName,
		"email":    user.Email,
		"roleId":   user.RoleID,
	}
	if user.EntityID != nil {
		userPayload["entityId"] = user.EntityID
	}

	c.JSON(http.StatusOK, gin.H{
		"accessToken":  tokens.AccessToken,
		"refreshToken": tokens.RefreshToken,
		"user":         userPayload,
	})
}

// ===============================
// Refresh Token
// ===============================

type refreshReq struct {
	RefreshToken string `json:"refreshToken" binding:"required" example:"your_refresh_token_here"`
}

func (h *Handler) Refresh(c *gin.Context) {
	var req refreshReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	token, err := h.service.Refresh(req.RefreshToken)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"accessToken": token})
}

// ===============================
// Forgot Password - FIXED
// ===============================

type forgotPasswordReq struct {
	Email string `json:"email" binding:"required,email" example:"sharath@example.com"`
}

// Custom error types for better error handling
var (
	ErrUserNotFound    = errors.New("user not found")
	ErrEmailNotSent    = errors.New("failed to send email")
	ErrInvalidEmail    = errors.New("invalid email address")
	ErrEmailService    = errors.New("email service unavailable")
	ErrRateLimitExceed = errors.New("too many requests")
)

func (h *Handler) ForgotPassword(c *gin.Context) {
	var req forgotPasswordReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid request",
			"message": "Please provide a valid email address",
		})
		return
	}

	// Validate email format
	if !strings.Contains(req.Email, "@") || !strings.Contains(req.Email, ".") {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid email",
			"message": "Please provide a valid email address",
		})
		return
	}

	// Call service layer
	err := h.service.RequestPasswordReset(req.Email)
	
	if err != nil {
		// 🔍 Determine the type of error and respond accordingly
		switch {
		case errors.Is(err, ErrUserNotFound):
			// ⚠️ Security: Don't reveal if user exists or not
			// Return success message but log the attempt
			c.JSON(http.StatusOK, gin.H{
				"message": "If an account exists with this email, a password reset link has been sent",
			})
			return

		case errors.Is(err, ErrEmailNotSent), errors.Is(err, ErrEmailService):
			// 🚨 Email service failure - return 500
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "failed to send email",
				"message": "Email service is currently unavailable. Please try again later or contact support.",
			})
			return

		case errors.Is(err, ErrRateLimitExceed):
			// 🚫 Rate limit exceeded
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error":   "too many requests",
				"message": "You have requested too many password resets. Please wait 15 minutes and try again.",
			})
			return

		case strings.Contains(err.Error(), "email"):
			// Generic email-related error
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "failed to send email",
				"message": "Unable to send password reset email. Please contact support.",
			})
			return

		default:
			// Unknown error
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "internal server error",
				"message": "An unexpected error occurred. Please try again later.",
			})
			return
		}
	}

	// ✅ Success response
	c.JSON(http.StatusOK, gin.H{
		"message": "If an account exists with this email, a password reset link has been sent",
	})
}

// ===============================
// Reset Password - FIXED
// ===============================

type resetPasswordReq struct {
	Token       string `json:"token" binding:"required" example:"reset_token_abc123"`
	NewPassword string `json:"newPassword" binding:"required,min=6" example:"newsecret123"`
}

var (
	ErrInvalidToken     = errors.New("invalid token")
	ErrExpiredToken     = errors.New("token expired")
	ErrWeakPassword     = errors.New("password too weak")
	ErrTokenNotFound    = errors.New("token not found")
)

func (h *Handler) ResetPassword(c *gin.Context) {
	var req resetPasswordReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid request",
			"message": "Please provide both token and new password",
		})
		return
	}

	// Validate password strength
	if len(req.NewPassword) < 6 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "weak password",
			"message": "Password must be at least 6 characters long",
		})
		return
	}

	// Call service layer
	err := h.service.ResetPassword(req.Token, req.NewPassword)
	
	if err != nil {
		// 🔍 Determine the type of error and respond accordingly
		switch {
		case errors.Is(err, ErrInvalidToken), errors.Is(err, ErrTokenNotFound):
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "invalid token",
				"message": "This password reset link is invalid. Please request a new one.",
			})
			return

		case errors.Is(err, ErrExpiredToken):
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "expired token",
				"message": "This password reset link has expired. Please request a new one.",
			})
			return

		case errors.Is(err, ErrWeakPassword):
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "weak password",
				"message": "Password does not meet security requirements. Please use a stronger password.",
			})
			return

		case strings.Contains(err.Error(), "token"):
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "invalid token",
				"message": "This password reset link is invalid or has expired. Please request a new one.",
			})
			return

		default:
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "internal server error",
				"message": "Unable to reset password. Please try again later.",
			})
			return
		}
	}

	// ✅ Success response
	c.JSON(http.StatusOK, gin.H{
		"message": "Password has been reset successfully. You can now login with your new password.",
	})
}

// ===============================
// Logout
// ===============================

func (h *Handler) Logout(c *gin.Context) {
	_ = h.service.Logout() // stateless
	c.JSON(http.StatusOK, gin.H{"message": "Logged out successfully"})
}

// ===============================
// Public Roles
// ===============================

func (h *Handler) GetPublicRoles(c *gin.Context) {
	roles, err := h.service.GetPublicRoles()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch available roles"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": roles})
}