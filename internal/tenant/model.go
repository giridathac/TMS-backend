package tenant

import (
    "time"
    "gorm.io/gorm"
)

// User represents a user in the system
type User struct {
    ID                   uint           `gorm:"primaryKey" json:"id"`
    FullName             string         `gorm:"column:full_name;size:100;not null" json:"name"`
    Email                string         `gorm:"size:100;uniqueIndex;not null" json:"email"`
    Phone                string         `gorm:"size:20" json:"phone"`
    PasswordHash         string         `gorm:"column:password_hash;size:255;not null" json:"-"` // stored hashed, hidden from JSON
    RoleID               uint           `gorm:"column:role_id" json:"-"`
    EntityID             uint           `gorm:"column:entity_id" json:"-"`
    Status               string         `gorm:"default:active" json:"status"`
    EmailVerified        bool           `gorm:"column:email_verified;default:false" json:"email_verified"`
    EmailVerifiedAt      *time.Time     `gorm:"column:email_verified_at" json:"email_verified_at"`
    ForgotPasswordToken  string         `gorm:"column:forgot_password_token" json:"-"`
    ForgotPasswordExpiry *time.Time     `gorm:"column:forgot_password_expiry" json:"-"`
    CreatedAt            time.Time      `json:"created_at"`
    UpdatedAt            time.Time      `json:"updated_at"`
    DeletedAt            gorm.DeletedAt `gorm:"index" json:"-"`
    CreatedBy            string         `gorm:"column:created_by" json:"-"`
}

// TableName specifies the table name for the User model
func (User) TableName() string {
    return "users"
}

// TenantUserAssignment represents the association between a tenant and a user
type TenantUserAssignment struct {
    ID        uint      `gorm:"primaryKey" json:"id"`
    UserID    uint      `gorm:"column:user_id;not null" json:"user_id"`
    TenantID  uint      `gorm:"column:tenant_id;not null" json:"tenant_id"`
    CreatedBy uint      `gorm:"column:created_by;not null" json:"created_by"`
    Status    string    `gorm:"column:status;size:20;default:active" json:"status"`
    CreatedAt time.Time `json:"created_at"`
    UpdatedAt time.Time `json:"updated_at"`
}

// TableName specifies the table name for the TenantUserAssignment model
func (TenantUserAssignment) TableName() string {
    return "tenant_user_assignments"
}

// UserInput represents the data received from the frontend
// UserInput represents the data received from the frontend
type UserInput struct {
    Name     string `json:"Name" binding:"required"`
    Email    string `json:"Email" binding:"required,email"`
    Phone    string `json:"Phone" binding:"required"`
    Password string `json:"Password"` // Remove required tag for updates
    Role     string `json:"Role" binding:"required"`
    Status   string `json:"Status"`   // Add status field
}

// UserResponse represents the response sent back to the frontend
type UserResponse struct {
    ID        uint      `json:"id"`
    Name      string    `json:"name"`
    Email     string    `json:"email"`
    Phone     string    `json:"phone"`
    Status    string    `json:"status"`
    CreatedAt time.Time `json:"created_at"`
    Role      string    `json:"role,omitempty"` // Added for frontend compatibility
}