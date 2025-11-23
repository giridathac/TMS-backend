package auth

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/sharath018/temple-management-backend/config"
	"github.com/sharath018/temple-management-backend/utils"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type TokenPair struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
}

type Service interface {
	Register(input RegisterInput) error
	Login(input LoginInput) (*TokenPair, *User, error)
	Refresh(refreshToken string) (string, error)
	GetUserByID(userID uint) (User, error)

	// Password reset methods
	RequestPasswordReset(email string) error
	ResetPassword(token string, newPassword string) error
	Logout() error
	
	// NEW: Public roles method
	GetPublicRoles() ([]PublicRoleResponse, error)
}

type service struct {
	repo          Repository
	accessSecret  string
	refreshSecret string
	accessTTL     time.Duration
	refreshTTL    time.Duration
}

func NewService(r Repository, cfg *config.Config) Service {
	return &service{
		repo:          r,
		accessSecret:  cfg.JWTAccessSecret,
		refreshSecret: cfg.JWTRefreshSecret,
		accessTTL:     time.Duration(cfg.JWTAccessTTLHours) * time.Hour,
		refreshTTL:    time.Duration(cfg.JWTRefreshTTLHours) * time.Hour,
	}
}

// =============================
// Register
// =============================

type RegisterInput struct {
	FullName          string
	Email             string
	Password          string
	Role              string
	Phone             string
	TempleName        string
	TemplePlace       string
	TempleAddress     string
	TemplePhoneNo     string
	TempleDescription string
}

func (s *service) Register(in RegisterInput) error {
	roleName := strings.ToLower(in.Role)
	role, err := s.repo.FindRoleByName(roleName)
	if err != nil {
		return errors.New("invalid role")
	}

	// ✅ Enforce Gmail-only email validation
	if !strings.HasSuffix(strings.ToLower(in.Email), "@gmail.com") {
		return errors.New("only @gmail.com emails are allowed")
	}

	// ✅ Hash password
	hash, err := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	// ✅ Status logic
	status := "active"
	if roleName == "templeadmin" {
		status = "pending"
	}

	// ✅ Clean phone number
	phone, err := cleanPhone(in.Phone)
	if err != nil {
		return err
	}

	// ✅ Create the user
	user := &User{
		FullName:     in.FullName,
		Email:        in.Email,
		PasswordHash: string(hash),
		RoleID:       role.ID,
		Status:       status,
		Phone:        phone,
		CreatedBy: "system",
	}

	if err := s.repo.Create(user); err != nil {
		return err
	}

	// ✅ Extra step for templeadmin: Save tenant details + approval request
	if roleName == "templeadmin" {
		tenant := &TenantDetails{
			UserID:            user.ID,
			TempleName:        in.TempleName,
			TemplePlace:       in.TemplePlace,
			TempleAddress:     in.TempleAddress,
			TemplePhoneNo:     in.TemplePhoneNo,
			TempleDescription: in.TempleDescription,
		}

		if err := s.repo.CreateTenantDetails(tenant); err != nil {
			return errors.New("failed to save tenant details")
		}

		if err := s.repo.CreateApprovalRequest(user.ID, "tenant_approval"); err != nil {
			return errors.New("failed to create approval request")
		}
	}

	return nil
}

// =============================
// Login
// =============================

type LoginInput struct {
	Email    string
	Password string
}

func (s *service) Login(in LoginInput) (*TokenPair, *User, error) {
	user, err := s.repo.FindByEmail(in.Email)
	if err != nil {
		// Check if it's a "record not found" error and return user-friendly message
		if err == gorm.ErrRecordNotFound || strings.Contains(err.Error(), "record not found") || strings.Contains(err.Error(), "not found") {
			return nil, nil, errors.New ("Couldn't find your Account")
		}
		return nil, nil, err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(in.Password)); err != nil {
		return nil, nil, errors.New("invalid credentials")
	}

	switch user.Status {
	case "pending":
		return nil, nil, errors.New("your account is pending approval")
	case "rejected":
		return nil, nil, errors.New("your account was rejected by admin")
	case "inactive":
		return nil, nil, errors.New("your account is inactive")
	}

	if user.EntityID == nil && (user.Role.RoleName == "templeadmin" || user.Role.RoleName == "devotee" || user.Role.RoleName == "volunteer") {
		entityID, err := s.repo.FindEntityIDByUserID(user.ID)
		if err == nil && entityID != nil {
			user.EntityID = entityID
		}
	}

	// Build tokens
	accessToken, err := s.generateAccessToken(user)
	if err != nil {
		return nil, nil, err
	}
	refreshToken, err := s.generateRefreshToken(user)
	if err != nil {
		return nil, nil, err
	}

	return &TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}, user, nil
}
func (s *service) generateAccessToken(user *User) (string, error) {
	claims := jwt.MapClaims{
		"user_id": user.ID,
		"role_id": user.RoleID,
		"exp":     time.Now().Add(s.accessTTL).Unix(),
	}
	
	// Add entity_id if it exists
	if user.EntityID != nil {
		claims["entity_id"] = *user.EntityID
	}
	
	// NEW: Add assigned tenant info for standarduser/monitoringuser
	if user.Role.RoleName == "standarduser" || user.Role.RoleName == "monitoringuser" {
		assignedTenantID, err := s.repo.GetAssignedTenantID(user.ID)
		if err == nil && assignedTenantID != nil {
			claims["assigned_tenant_id"] = *assignedTenantID
			
			// Add permission type based on role
			permissionType, _ := s.repo.GetUserPermissionType(user.ID)
			claims["permission_type"] = permissionType
		}
	}
	
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.accessSecret))
}

func (s *service) generateRefreshToken(user *User) (string, error) {
	claims := jwt.MapClaims{
		"user_id": user.ID,
		"role_id": user.RoleID,
		"exp":     time.Now().Add(s.refreshTTL).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.refreshSecret))
}

// =============================
// Refresh
// =============================

func (s *service) Refresh(refreshToken string) (string, error) {
	token, err := jwt.Parse(refreshToken, func(t *jwt.Token) (interface{}, error) {
		return []byte(s.refreshSecret), nil
	})
	if err != nil || !token.Valid {
		return "", errors.New("invalid refresh token")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || claims["user_id"] == nil || claims["role_id"] == nil {
		return "", errors.New("invalid token claims")
	}

	userID := uint(claims["user_id"].(float64))
	user, err := s.repo.FindByID(userID)
	if err != nil {
		return "", errors.New("user not found")
	}

	return s.generateAccessToken(&user)
}

// =============================
// Forgot Password
// =============================

func (s *service) RequestPasswordReset(email string) error {
	user, err := s.repo.FindByEmail(email)
	if err != nil {
		return errors.New("user not found")
	}

	resetToken := generateSecureToken()
	ttl := 15 * time.Minute
	key := fmt.Sprintf("reset_token:%s", resetToken)

	// Store user ID as value
	err = utils.SetToken(key, fmt.Sprint(user.ID), ttl)
	if err != nil {
		return errors.New("could not save reset token")
	}

	// Send reset link via email
	if err := utils.SendResetLink(user.Email, resetToken); err != nil {
		return errors.New("failed to send email")
	}

	return nil
}


func (s *service) ResetPassword(token string, newPassword string) error {
	key := fmt.Sprintf("reset_token:%s", token)
	val, err := utils.GetToken(key)
	if err != nil {
		return errors.New("invalid or expired token")
	}

	// Convert userID string back to uint
	var userID uint
	_, err = fmt.Sscan(val, &userID)
	if err != nil {
		return errors.New("invalid token data")
	}

	user, err := s.repo.FindByID(userID)
	if err != nil {
		return errors.New("user not found")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	user.PasswordHash = string(hash)
	err = s.repo.Update(&user)
	if err != nil {
		return errors.New("failed to update password")
	}

	_ = utils.DeleteToken(key) // Cleanup token

	return nil
}

// =============================
// Logout
// =============================

func (s *service) Logout() error {
	// JWT is stateless — frontend should just clear token
	return nil
}

// =============================
// Get User By ID
// =============================

func (s *service) GetUserByID(userID uint) (User, error) {
	return s.repo.FindByID(userID)
}

// =============================
// Helpers (for reset tokens)
// =============================

func generateSecureToken() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func cleanPhone(raw string) (string, error) {
	re := regexp.MustCompile(`\D`)
	cleaned := re.ReplaceAllString(raw, "")

	// Strip leading 91 if present and length is 12
	if len(cleaned) == 12 && strings.HasPrefix(cleaned, "91") {
		cleaned = cleaned[2:]
	}

	if len(cleaned) != 10 {
		return "", errors.New("invalid phone number format")
	}

	return cleaned, nil
}

func (s *service) GetPublicRoles() ([]PublicRoleResponse, error) {
	roles, err := s.repo.GetPublicRoles()
	if err != nil {
		return nil, err
	}

	var publicRoles []PublicRoleResponse
	for _, role := range roles {
		publicRoles = append(publicRoles, PublicRoleResponse{
			ID:          role.ID,
			RoleName:    role.RoleName,
			Description: role.Description,
		})
	}

	return publicRoles, nil
}