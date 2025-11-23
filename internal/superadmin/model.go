package superadmin

import (
	"time"
	"gorm.io/gorm"
)

type TenantApprovalCount struct {
	Approved int64 `json:"approved"`
	Pending  int64 `json:"pending"`
	Rejected int64 `json:"rejected"`
}

// ================ TEMPLE APPROVAL COUNTS ================

type TempleApprovalCount struct {
	PendingApproval int64 `json:"pending_approval"`
	ActiveTemples   int64 `json:"active_temples"`
	Rejected        int64 `json:"rejected"`
	TotalDevotees   int64 `json:"total_devotees"`
}

// ================ TENANT WITH TEMPLE DETAILS ================

type TenantWithDetails struct {
	// User details
	ID        uint      `json:"id"`
	FullName  string    `json:"full_name"`
	Email     string    `json:"email"`
	Phone     string    `json:"phone"`
	RoleID    uint      `json:"role_id"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// Temple details
	TempleDetails *TenantTempleDetails `json:"temple_details,omitempty"`
}

type TenantTempleDetails struct {
	ID                uint      `json:"id"`
	TempleName        string    `json:"temple_name"`
	TemplePlace       string    `json:"temple_place"`
	TempleAddress     string    `json:"temple_address"`
	TemplePhoneNo     string    `json:"temple_phone_no"`
	TempleDescription string    `json:"temple_description"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// ================ USER MANAGEMENT ================

type CreateUserRequest struct {
	FullName          string `json:"fullName" binding:"required"`
	Email             string `json:"email" binding:"required,email"`
	Password          string `json:"password" binding:"required,min=6"`
	Phone             string `json:"phone" binding:"required"`
	Role              string `json:"role" binding:"required"`

	// Temple admin specific fields (required only for templeadmin role)
	TempleName        string `json:"templeName"`
	TemplePlace       string `json:"templePlace"`
	TempleAddress     string `json:"templeAddress"`
	TemplePhoneNo     string `json:"templePhoneNo"`
	TempleDescription string `json:"templeDescription"`
}

type UpdateUserRequest struct {
	FullName          string `json:"fullName"`
	Email             string `json:"email" binding:"email"`
	Phone             string `json:"phone"`
	TempleName        string `json:"templeName"`
	TemplePlace       string `json:"templePlace"`
	TempleAddress     string `json:"templeAddress"`
	TemplePhoneNo     string `json:"templePhoneNo"`
	TempleDescription string `json:"templeDescription"`
}

// NEW: Struct for tenant selection (different from assignment)
type TenantSelectionResponse struct {
	ID           uint   `json:"id"`
	Name         string `json:"name"`
	Email        string `json:"email"`
	TempleName   string `json:"templeName"`
	Location     string `json:"location"`
	Status       string `json:"status"`
	TemplesCount int    `json:"templesCount"`
	ImageUrl     string `json:"imageUrl,omitempty"`
}

type UserResponse struct {
	ID        uint      `json:"id"`
	FullName  string    `json:"full_name"`
	Email     string    `json:"email"`
	Phone     string    `json:"phone"`
	Role      UserRole  `json:"role"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// Assignment details exposed directly for Vue
	TenantAssigned     string     `json:"tenant_assigned,omitempty"` // now stores tenant name
	AssignedDate       *time.Time `json:"assignedDate,omitempty"`
	ReassignmentDate   *time.Time `json:"reassignmentDate,omitempty"`

	// Temple details for templeadmin users
	TempleDetails           *TenantTempleDetails     `json:"temple_details,omitempty"`
	TenantAssignmentDetails *TenantAssignmentDetails `json:"tenant_assignment_details,omitempty"`
}

type UserRole struct {
	ID                  uint   `json:"id"`
	RoleName            string `json:"role_name"`
	Description         string `json:"description"`
	CanRegisterPublicly bool   `json:"can_register_publicly"`
}

type AssignableTenant struct {
	UserID        uint   `json:"userID"`
	TenantName    string `json:"tenantName"`
	Email         string `json:"email"`
	TempleAddress string `json:"templeAddress"`
	TempleName    string `json:"templeName"`
}

// AssignRequest holds the user IDs and the single tenant ID for assignment.
type AssignRequest struct {
	UserID   uint `json:"userId" binding:"required"`
	TenantID uint `json:"tenantId" binding:"required"`
}

// AssignTenantsRequest holds the tenant IDs to be assigned to a user.
type AssignTenantsRequest struct {
	TenantIDs []uint `json:"tenant_ids" binding:"required"`
}

// TenantListResponse holds the details of a single tenant for the list view.
type TenantListResponse struct {
	ID            uint   `json:"id"`
	UserID        uint   `json:"user_id"`
	TempleName    string `json:"temple_name"`
	TempleAddress string `json:"temple_address"`
}

type TenantAssignmentDetails struct {
	TenantName string    `json:"tenant_name"`
	AssignedOn time.Time `json:"assigned_on"`
	UpdatedOn  time.Time `json:"updated_on"`
}

// ================ TEMPLE DETAILS FOR API RESPONSE ================

type TempleDetails struct {
	ID    uint   `json:"id"`
	Name  string `json:"name"`
	City  string `json:"city"`
	State string `json:"state"`
}

// TenantResponse represents a tenant with their temple details for API response
type TenantResponse struct {
	ID       uint           `json:"id"`
	FullName string         `json:"fullName"`
	Email    string         `json:"email"`
	Role     string         `json:"role"`
	Status   string         `json:"status"`
	Temple   *TempleDetails `json:"temple,omitempty"`
}

type BulkUserCSV struct {
	FullName string `csv:"Full Name" json:"full_name"`
	Email    string `csv:"Email" json:"email"`
	Phone    string `csv:"Phone" json:"phone"`
	Password string `csv:"Password" json:"password"`
	Role     string `csv:"Role" json:"role"`
	Status   string `csv:"Status" json:"status"`
}

type BulkUploadResult struct {
	TotalRows    int      `json:"total_rows"`
	SuccessCount int      `json:"success_count"`
	FailedCount  int      `json:"failed_count"`
	Errors       []string `json:"errors,omitempty"`
}

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