package auth

import (
	"strings"
	"time"

	"gorm.io/gorm"
)

type Repository interface {
	Create(user *User) error
	FindByEmail(email string) (*User, error)
	FindByID(userID uint) (User, error)
	FindRoleByName(name string) (*UserRole, error)
	FindEntityIDByUserID(userID uint) (*uint, error)
	CreateApprovalRequest(userID uint, requestType string) error
	UpdateEntityID(userID uint, entityID uint) error
	GetUserEmailsByRole(roleName string, entityID uint) ([]string, error)
	GetUserIDsByRole(roleName string, entityID uint) ([]uint, error)

	// Password reset methods
	SetForgotPasswordToken(userID uint, token string, expiry time.Time) error
	GetByResetToken(token string) (*User, error)
	ClearResetToken(userID uint) error
	Update(user *User) error
	CreateTenantDetails(t *TenantDetails) error
	
	// NEW: Public roles method
	GetPublicRoles() ([]UserRole, error)

		// New methods for tenant assignment
	GetAssignedTenantID(userID uint) (*uint, error)
	GetUserPermissionType(userID uint) (string, error)
}

type repository struct{ db *gorm.DB }

func NewRepository(db *gorm.DB) Repository {
	return &repository{db}
}

// Create a new user
func (r *repository) Create(user *User) error {
	return r.db.Create(user).Error
}

// Find user by email (used in login & password reset)
func (r *repository) FindByEmail(email string) (*User, error) {
	var u User
	err := r.db.Preload("Role").Where("email = ?", email).First(&u).Error
	return &u, err
}

// Find user by ID (with role preload)
func (r *repository) FindByID(userID uint) (User, error) {
	var user User
	err := r.db.Preload("Role").First(&user, userID).Error
	if err != nil {
		return user, err
	}

	// Dynamically resolve EntityID if nil and user is temple-related
	roleName := strings.ToLower(user.Role.RoleName)
	if user.EntityID == nil && (roleName == "templeadmin" || roleName == "devotee" || roleName == "volunteer") {
		entityID, err := r.FindEntityIDByUserID(user.ID)
		if err == nil && entityID != nil {
			user.EntityID = entityID
		}
	}

	return user, nil
}

// Find user role by name
func (r *repository) FindRoleByName(name string) (*UserRole, error) {
	var role UserRole
	err := r.db.Where("role_name = ?", name).First(&role).Error
	return &role, err
}

// Find user's approved EntityID (either via approval or membership)
func (r *repository) FindEntityIDByUserID(userID uint) (*uint, error) {

    // 1. Check templeadmin approval
    var req ApprovalRequest
    err := r.db.
        Where("user_id = ? AND status = ?", userID, "approved").
        Order("id DESC").
        First(&req).Error
    if err == nil && req.EntityID != nil {
        return req.EntityID, nil
    }

    // 2. Check devotee/volunteer membership
    type membership struct {
        EntityID uint
    }

    var m membership
    err = r.db.
        Table("user_entity_memberships").
        Select("entity_id").
        Where("user_id = ?", userID).
        Order("joined_at DESC").
        First(&m).Error
    if err == nil {
        return &m.EntityID, nil
    }

    // 3. NEW: Check if the user is the creator of the entity
    type entity struct {
        ID        uint
        CreatedBy uint
    }

    var e entity
    err = r.db.
        Table("entities").
        Select("id, created_by").
        Where("created_by = ?", userID).
        Order("created_at DESC").
        First(&e).Error
    if err == nil {
        return &e.ID, nil
    }

    return nil, gorm.ErrRecordNotFound
}


// Create approval request for templeadmin
func (r *repository) CreateApprovalRequest(userID uint, requestType string) error {
	req := ApprovalRequest{
		UserID:      userID,
		RequestType: requestType,
		Status:      "pending",
	}
	return r.db.Create(&req).Error
}

// Update user's associated EntityID
func (r *repository) UpdateEntityID(userID uint, entityID uint) error {
	return r.db.Model(&User{}).Where("id = ?", userID).Update("entity_id", entityID).Error
}

// ✅ GetUserEmailsByRole fetches all user emails by role and entity
func (r *repository) GetUserEmailsByRole(roleName string, entityID uint) ([]string, error) {
	var emails []string

	// Join with user_entity_memberships for devotees/volunteers
	err := r.db.
		Table("users").
		Select("users.email").
		Joins("JOIN user_roles ON users.role_id = user_roles.id").
		Joins("JOIN user_entity_memberships ON users.id = user_entity_memberships.user_id").
		Where("user_roles.role_name = ? AND user_entity_memberships.entity_id = ? AND users.status = ? AND user_entity_memberships.status = ?",
			roleName, entityID, "active", "active").
		Scan(&emails).Error

	return emails, err
}

// GetUserIDsByRole fetches all user IDs by role and entity
func (r *repository) GetUserIDsByRole(roleName string, entityID uint) ([]uint, error) {
	var ids []uint
	type row struct{ ID uint }
	var rows []row
	err := r.db.
		Table("users").
		Select("users.id as id").
		Joins("JOIN user_roles ON users.role_id = user_roles.id").
		Joins("JOIN user_entity_memberships ON users.id = user_entity_memberships.user_id").
		Where("user_roles.role_name = ? AND user_entity_memberships.entity_id = ? AND users.status = ? AND user_entity_memberships.status = ?",
			roleName, entityID, "active", "active").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, r := range rows {
		ids = append(ids, r.ID)
	}
	return ids, nil
}

// ✅ Set Forgot Password Token and expiry
func (r *repository) SetForgotPasswordToken(userID uint, token string, expiry time.Time) error {
	return r.db.Model(&User{}).Where("id = ?", userID).Updates(map[string]interface{}{
		"forgot_password_token":  token,
		"forgot_password_expiry": expiry,
	}).Error
}

// ✅ Get user by forgot password token (must not be expired)
func (r *repository) GetByResetToken(token string) (*User, error) {
	var user User
	err := r.db.
		Where("forgot_password_token = ? AND forgot_password_expiry > ?", token, time.Now()).
		First(&user).Error
	return &user, err
}

// ✅ Clear forgot password token (after successful reset)
func (r *repository) ClearResetToken(userID uint) error {
	return r.db.Model(&User{}).Where("id = ?", userID).Updates(map[string]interface{}{
		"forgot_password_token":  nil,
		"forgot_password_expiry": nil,
	}).Error
}
func (r *repository) CreateTenantDetails(t *TenantDetails) error {
	return r.db.Create(t).Error
}

func (r *repository) Update(user *User) error {
	return r.db.Save(user).Error
}

func (r *repository) GetPublicRoles() ([]UserRole, error) {
	var roles []UserRole
	err := r.db.Where("can_register_publicly = ?", true).Find(&roles).Error
	return roles, err
}

// Add these methods to your auth/repository.go

// GetAssignedTenantID returns the assigned tenant ID for a user
func (r *repository) GetAssignedTenantID(userID uint) (*uint, error) {
	var assignment struct {
		TenantID uint
	}
	
	err := r.db.Table("tenant_user_assignments").
		Select("tenant_id").
		Where("user_id = ? AND status = ?", userID, "active").
		First(&assignment).Error
		
	if err != nil {
		return nil, err
	}
	
	return &assignment.TenantID, nil
}

// GetUserPermissionType returns the permission type based on user role
func (r *repository) GetUserPermissionType(userID uint) (string, error) {
	var user struct {
		RoleName string
	}
	
	err := r.db.Table("users").
		Select("user_roles.role_name").
		Joins("JOIN user_roles ON users.role_id = user_roles.id").
		Where("users.id = ?", userID).
		First(&user).Error
		
	if err != nil {
		return "", err
	}
	
	// Set permission type based on role
	switch user.RoleName {
	case "standarduser":
		return "full", nil
	case "monitoringuser":
		return "readonly", nil
	default:
		return "full", nil
	}
}