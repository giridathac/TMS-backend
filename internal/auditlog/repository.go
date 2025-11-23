package auditlog

import (
	"context"

	"gorm.io/gorm"
)

type Repository interface {
	Create(ctx context.Context, log *AuditLog) error
	GetByFilter(ctx context.Context, filter AuditLogFilter) ([]AuditLogResponse, int64, error)
	GetByID(ctx context.Context, id uint) (*AuditLogResponse, error)
}

type repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) Repository {
	return &repository{db: db}
}

// Create inserts a new audit log entry
func (r *repository) Create(ctx context.Context, log *AuditLog) error {
	return r.db.WithContext(ctx).Create(log).Error
}

// GetByFilter retrieves audit logs with filtering and pagination
func (r *repository) GetByFilter(ctx context.Context, filter AuditLogFilter) ([]AuditLogResponse, int64, error) {
	var logs []AuditLogResponse
	var total int64

	// Build the query
	query := r.db.WithContext(ctx).
		Table("audit_logs al").
		Select(`
			al.id, al.user_id, al.entity_id, al.action, 
			al.details, al.ip_address, al.status, al.created_at,
			u.full_name as user_name,
			e.name as entity_name
		`).
		Joins("LEFT JOIN users u ON al.user_id = u.id").
		Joins("LEFT JOIN entities e ON al.entity_id = e.id")

	// Apply filters
	if filter.UserID != nil {
		query = query.Where("al.user_id = ?", *filter.UserID)
	}
	if filter.EntityID != nil {
		query = query.Where("al.entity_id = ?", *filter.EntityID)
	}
	if filter.Action != "" {
		query = query.Where("al.action ILIKE ?", "%"+filter.Action+"%")
	}
	if filter.Status != "" {
		query = query.Where("al.status = ?", filter.Status)
	}
	if filter.FromDate != nil {
		query = query.Where("al.created_at >= ?", *filter.FromDate)
	}
	if filter.ToDate != nil {
		query = query.Where("al.created_at <= ?", *filter.ToDate)
	}

	// Get total count
	countQuery := query
	if err := countQuery.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Apply pagination
	if filter.Limit <= 0 {
		filter.Limit = 20 // default limit
	}
	if filter.Page <= 0 {
		filter.Page = 1 // default page
	}

	offset := (filter.Page - 1) * filter.Limit
	query = query.Order("al.created_at DESC").
		Limit(filter.Limit).
		Offset(offset)

	// Execute query
	if err := query.Find(&logs).Error; err != nil {
		return nil, 0, err
	}

	return logs, total, nil
}

// GetByID retrieves a specific audit log by ID
func (r *repository) GetByID(ctx context.Context, id uint) (*AuditLogResponse, error) {
	var log AuditLogResponse

	err := r.db.WithContext(ctx).
		Table("audit_logs al").
		Select(`
			al.id, al.user_id, al.entity_id, al.action, 
			al.details, al.ip_address, al.status, al.created_at,
			u.full_name as user_name,
			e.name as entity_name
		`).
		Joins("LEFT JOIN users u ON al.user_id = u.id").
		Joins("LEFT JOIN entities e ON al.entity_id = e.id").
		Where("al.id = ?", id).
		First(&log).Error

	if err != nil {
		return nil, err
	}

	return &log, nil
}