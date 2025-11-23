package auth

import (
	"time"

	"gorm.io/gorm"
)

// UserRole represents the user_roles table
type UserRole struct {
	ID                  uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	RoleName            string    `gorm:"size:50;unique;not null" json:"role_name"` // templeadmin, devotee, volunteer, super_admin
	Description         string    `gorm:"type:text" json:"description"`
	CanRegisterPublicly bool      `gorm:"default:true" json:"can_register_publicly"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
	Status              string                 `gorm:"type:varchar(20);default:'active'" json:"status"`
}

// TableName overrides table name for UserRole
func (UserRole) TableName() string {
	return "user_roles"
}

// User represents the users table
type User struct {
	ID                   uint           `gorm:"primaryKey;autoIncrement" json:"id"`
	FullName             string         `gorm:"size:255;not null" json:"full_name"`
	Email                string         `gorm:"size:255;unique;not null" json:"email"`
	PasswordHash         string         `gorm:"size:255;not null" json:"-"`
	Phone                string         `gorm:"size:20;not null" json:"phone"`
	RoleID               uint           `gorm:"not null" json:"role_id"`
	Role                 UserRole       `gorm:"foreignKey:RoleID;references:ID" json:"role"`
	
	EntityID             *uint          `gorm:"index" json:"entity_id,omitempty"`
	Status               string         `gorm:"size:20;default:active" json:"status"` // active, pending, rejected, inactive
	EmailVerified        bool           `gorm:"default:false" json:"email_verified"`
	EmailVerifiedAt      *time.Time     `json:"email_verified_at,omitempty"`
	ForgotPasswordToken  *string        `gorm:"size:255" json:"-"`
	ForgotPasswordExpiry *time.Time     `json:"-"`
	CreatedAt            time.Time      `json:"created_at"`
	UpdatedAt            time.Time      `json:"updated_at"`
	DeletedAt            gorm.DeletedAt `gorm:"index" json:"-"`
	CreatedBy string `gorm:"size:50" json:"created_by"`

}

// TableName overrides table name for User
func (User) TableName() string {
	return "users"
}

// ApprovalRequest represents approval_requests table
type ApprovalRequest struct {
	ID          uint       `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID      uint       `gorm:"not null" json:"user_id"`
	User        User       `gorm:"foreignKey:UserID;references:ID" json:"user"`
	RequestType string     `gorm:"size:50;not null" json:"request_type"` // tenant_approval, temple_approval
	EntityID    *uint      `json:"entity_id,omitempty"`
	Status      string     `gorm:"size:20;default:pending" json:"status"` // pending, approved, rejected
	AdminNotes  *string    `gorm:"type:text" json:"admin_notes,omitempty"`
	ApprovedBy  *uint      `json:"approved_by,omitempty"`
	ApprovedAt  *time.Time `json:"approved_at,omitempty"`
	RejectedAt  *time.Time `json:"rejected_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// TableName overrides table name for ApprovalRequest
func (ApprovalRequest) TableName() string {
	return "approval_requests"
}

// TenantDetails represents the tenant_details table
type TenantDetails struct {
	ID                uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID            uint      `gorm:"not null" json:"user_id"`
	User              User      `gorm:"foreignKey:UserID;references:ID" json:"user"`
	TempleName        string    `gorm:"size:255;not null" json:"temple_name"`
	TemplePlace       string    `gorm:"size:255;not null" json:"temple_place"`
	TempleAddress     string    `gorm:"type:text;not null" json:"temple_address"`
	TemplePhoneNo     string    `gorm:"size:20;not null" json:"temple_phone_no"`
	TempleDescription string    `gorm:"type:text;not null" json:"temple_description"`
	CreatedAt         time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt         time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

// TableName overrides table name for TenantDetails
func (TenantDetails) TableName() string {
	return "tenant_details"
}

type PublicRoleResponse struct {
	ID          uint   `json:"id"`
	RoleName    string `json:"role_name"`
	Description string `json:"description"`
}

// 🆕 Add these structs to your internal/auth/model.go file
// CreateRoleRequest is the DTO for creating a new role.
// It matches the data sent by the UI form.
type CreateRoleRequest struct {
	RoleName    string `json:"role_name" binding:"required"`
	Description string `json:"description" binding:"required"`
	CanRegisterPublicly bool   `json:"can_register_publicly"`
}

// RoleResponse is the DTO for retrieving a list of roles.
// It matches the data needed by the "Available Roles" table.
type RoleResponse struct {
	ID                  uint   `json:"id"`
	RoleName            string `json:"role_name"`
	Description         string `json:"description"`
	Status              string `json:"status"`
	CanRegisterPublicly bool   `json:"can_register_publicly"`
}
// UpdateRoleRequest is the DTO for updating an existing role.
type UpdateRoleRequest struct {
    RoleName            string `json:"role_name"`
    Description         string `json:"description"`
    Status              string `json:"status"` // 👈 This is what you need
    CanRegisterPublicly *bool  `json:"can_register_publicly"`
}

type UpdateRoleStatusRequest struct {
	Status string `json:"status" binding:"required"`
}

type TenantUserAssignment struct {
	ID        uint `gorm:"primarykey"`
	CreatedAt time.Time
	UpdatedAt time.Time

	UserID    uint `gorm:"uniqueIndex:idx_user_tenant;not null"`
	TenantID  uint `gorm:"uniqueIndex:idx_user_tenant;not null"`
	CreatedBy uint `gorm:"not null"`

	// New Status field
	Status string `gorm:"type:varchar(20);not null;default:'active'"`
}

// TableName overrides the default table name for GORM.
func (TenantUserAssignment) TableName() string {
	return "tenant_user_assignments"
}

