package auditlog

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
)

type Service interface {
	LogAction(ctx context.Context, userID *uint, entityID *uint, action string, details map[string]interface{}, ip string, status string) error
	GetAuditLogs(ctx context.Context, filter AuditLogFilter) (*PaginatedAuditLogs, error)
	GetAuditLogByID(ctx context.Context, id uint) (*AuditLogResponse, error)
}

type service struct {
	repo Repository
}

func NewService(repo Repository) Service {
	return &service{repo: repo}
}

// LogAction creates a new audit log entry
func (s *service) LogAction(ctx context.Context, userID *uint, entityID *uint, action string, details map[string]interface{}, ip string, status string) error {
	// Handle nil details
	if details == nil {
		details = make(map[string]interface{})
	}

	// Convert details to JSON string
	detailsJSON, err := json.Marshal(details)
	if err != nil {
		detailsJSON = []byte("{}")
	}

	// Create audit log entry
	log := &AuditLog{
		UserID:    userID,
		EntityID:  entityID,
		Action:    action,
		Details:   string(detailsJSON),
		IPAddress: ip,
		Status:    status,
	}

	return s.repo.Create(ctx, log)
}

// GetAuditLogs retrieves paginated audit logs with filters
func (s *service) GetAuditLogs(ctx context.Context, filter AuditLogFilter) (*PaginatedAuditLogs, error) {
	logs, total, err := s.repo.GetByFilter(ctx, filter)
	if err != nil {
		return nil, err
	}

	// Calculate pagination info
	totalPages := int(math.Ceil(float64(total) / float64(filter.Limit)))
	if filter.Limit == 0 {
		totalPages = 0
	}

	return &PaginatedAuditLogs{
		Data:       logs,
		Total:      total,
		Page:       filter.Page,
		Limit:      filter.Limit,
		TotalPages: totalPages,
	}, nil
}

// GetAuditLogByID retrieves a specific audit log by ID
func (s *service) GetAuditLogByID(ctx context.Context, id uint) (*AuditLogResponse, error) {
	log, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("audit log not found: %w", err)
	}
	return log, nil
}

// Helper functions for common audit actions

// LogAuthAction logs authentication related actions
func (s *service) LogAuthAction(ctx context.Context, userID *uint, action string, email string, ip string, status string) error {
	details := map[string]interface{}{
		"email": email,
	}
	return s.LogAction(ctx, userID, nil, action, details, ip, status)
}

// LogEntityAction logs entity management actions
func (s *service) LogEntityAction(ctx context.Context, userID *uint, entityID *uint, action string, entityName string, ip string, status string) error {
	details := map[string]interface{}{
		"entity_name": entityName,
	}
	return s.LogAction(ctx, userID, entityID, action, details, ip, status)
}

// LogUserManagementAction logs user management actions
func (s *service) LogUserManagementAction(ctx context.Context, adminUserID *uint, targetUserID *uint, action string, targetUserEmail string, ip string, status string) error {
	details := map[string]interface{}{
		"target_user_id":    targetUserID,
		"target_user_email": targetUserEmail,
	}
	return s.LogAction(ctx, adminUserID, nil, action, details, ip, status)
}