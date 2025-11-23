package auditlog

import (
	"time"
)

// AuditLog represents the audit_logs table
type AuditLog struct {
	ID        uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID    *uint     `gorm:"index" json:"user_id"`      // nullable (e.g. failed login)
	EntityID  *uint     `gorm:"index" json:"entity_id"`    // nullable (temple, event, seva, etc.)
	Action    string    `gorm:"size:100;not null;index" json:"action"`
	Details   string    `gorm:"type:jsonb" json:"details"` // freeform JSON details
	IPAddress string    `gorm:"size:45" json:"ip_address"`
	Status    string    `gorm:"size:20;not null;index" json:"status"` // success/failure
	CreatedAt time.Time `gorm:"autoCreateTime;index" json:"created_at"`
}

// TableName overrides table name for AuditLog
func (AuditLog) TableName() string {
	return "audit_logs"
}

// Request/Response DTOs

// AuditLogResponse represents the audit log response for API
type AuditLogResponse struct {
	ID        uint      `json:"id"`
	UserID    *uint     `json:"user_id"`
	EntityID  *uint     `json:"entity_id"`
	Action    string    `json:"action"`
	Details   string    `json:"details"`
	IPAddress string    `json:"ip_address"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	// Additional fields for better display
	UserName   *string `json:"user_name,omitempty"`
	EntityName *string `json:"entity_name,omitempty"`
}

// AuditLogFilter represents filters for querying audit logs
type AuditLogFilter struct {
	UserID   *uint     `json:"user_id"`
	EntityID *uint     `json:"entity_id"`
	Action   string    `json:"action"`
	Status   string    `json:"status"`
	FromDate *time.Time `json:"from_date"`
	ToDate   *time.Time `json:"to_date"`
	Page     int       `json:"page"`
	Limit    int       `json:"limit"`
}

// PaginatedAuditLogs represents paginated audit log response
type PaginatedAuditLogs struct {
	Data       []AuditLogResponse `json:"data"`
	Total      int64              `json:"total"`
	Page       int                `json:"page"`
	Limit      int                `json:"limit"`
	TotalPages int                `json:"total_pages"`
}