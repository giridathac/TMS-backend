package notification

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
)

type Repository interface {
	// Templates
	CreateTemplate(ctx context.Context, template *NotificationTemplate) error
	GetTemplateByID(ctx context.Context, id uint, entityID uint) (*NotificationTemplate, error)
	GetTemplatesByEntity(ctx context.Context, entityID uint) ([]NotificationTemplate, error)
	UpdateTemplate(ctx context.Context, template *NotificationTemplate) error
	DeleteTemplate(ctx context.Context, id uint, entityID uint) error

	// Logs
	CreateNotificationLog(ctx context.Context, log *NotificationLog) error
	UpdateNotificationLog(ctx context.Context, log *NotificationLog) error
	GetNotificationsByUser(ctx context.Context, userID uint) ([]NotificationLog, error)
	MarkNotificationAsRead(ctx context.Context, notificationID uint, userID uint) error

	// In-app notifications
	CreateInApp(ctx context.Context, n *InAppNotification) error
	ListInAppByUser(ctx context.Context, userID uint, entityID *uint, limit int) ([]InAppNotification, error)
	MarkInAppAsRead(ctx context.Context, id uint, userID uint) error

	// ✅ FCM Device Tokens
	SaveDeviceToken(ctx context.Context, token *FCMDeviceToken) error
	GetUserDeviceTokens(ctx context.Context, userID uint, entityID uint) ([]string, error)
	GetDeviceTokensByEntityAndRole(ctx context.Context, entityID uint, roleNames []string) ([]string, error)
	RemoveDeviceToken(ctx context.Context, userID uint, deviceToken string) error
	DeactivateOldTokens(ctx context.Context, userID uint, keepToken string) error
}

type repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) Repository {
	return &repository{db: db}
}

// ------------------------------
// Templates
// ------------------------------

func (r *repository) CreateTemplate(ctx context.Context, template *NotificationTemplate) error {
	return r.db.WithContext(ctx).Create(template).Error
}

func (r *repository) GetTemplateByID(ctx context.Context, id uint, entityID uint) (*NotificationTemplate, error) {
	var t NotificationTemplate
	err := r.db.WithContext(ctx).
		Where("id = ? AND entity_id = ?", id, entityID).
		First(&t).Error
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (r *repository) GetTemplatesByEntity(ctx context.Context, entityID uint) ([]NotificationTemplate, error) {
	var templates []NotificationTemplate
	err := r.db.WithContext(ctx).
		Where("entity_id = ?", entityID).
		Order("created_at DESC").
		Find(&templates).Error
	return templates, err
}

func (r *repository) UpdateTemplate(ctx context.Context, template *NotificationTemplate) error {
	return r.db.WithContext(ctx).
		Model(&NotificationTemplate{}).
		Where("id = ? AND entity_id = ?", template.ID, template.EntityID).
		Updates(template).Error
}

func (r *repository) DeleteTemplate(ctx context.Context, id uint, entityID uint) error {
	res := r.db.WithContext(ctx).
		Where("id = ? AND entity_id = ?", id, entityID).
		Delete(&NotificationTemplate{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return errors.New("template not found or unauthorized")
	}
	return nil
}

// ------------------------------
// Logs
// ------------------------------

func (r *repository) CreateNotificationLog(ctx context.Context, log *NotificationLog) error {
	return r.db.WithContext(ctx).Create(log).Error
}

func (r *repository) UpdateNotificationLog(ctx context.Context, log *NotificationLog) error {
	return r.db.WithContext(ctx).
		Model(&NotificationLog{}).
		Where("id = ?", log.ID).
		Updates(log).Error
}

func (r *repository) GetNotificationsByUser(ctx context.Context, userID uint) ([]NotificationLog, error) {
	var logs []NotificationLog
	err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("created_at DESC").
		Find(&logs).Error
	return logs, err
}

func (r *repository) MarkNotificationAsRead(ctx context.Context, notificationID uint, userID uint) error {
	return r.db.WithContext(ctx).
		Model(&NotificationLog{}).
		Where("id = ? AND user_id = ?", notificationID, userID).
		Update("is_read", true).Error
}

// ------------------------------
// In-App Notifications
// ------------------------------

func (r *repository) CreateInApp(ctx context.Context, n *InAppNotification) error {
	return r.db.WithContext(ctx).Create(n).Error
}

func (r *repository) ListInAppByUser(ctx context.Context, userID uint, entityID *uint, limit int) ([]InAppNotification, error) {
	var items []InAppNotification
	q := r.db.WithContext(ctx).Where("user_id = ?", userID)
	if entityID != nil {
		q = q.Where("entity_id = ?", *entityID)
	}
	if limit <= 0 {
		limit = 20
	}
	err := q.Order("created_at DESC").Limit(limit).Find(&items).Error
	return items, err
}

func (r *repository) MarkInAppAsRead(ctx context.Context, id uint, userID uint) error {
	return r.db.WithContext(ctx).
		Model(&InAppNotification{}).
		Where("id = ? AND user_id = ?", id, userID).
		Update("is_read", true).Error
}

// ------------------------------
// ✅ FCM Device Tokens
// ------------------------------

// SaveDeviceToken creates or updates a device token
func (r *repository) SaveDeviceToken(ctx context.Context, token *FCMDeviceToken) error {
	// Check if token already exists
	var existing FCMDeviceToken
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND device_token = ?", token.UserID, token.DeviceToken).
		First(&existing).Error

	if err == gorm.ErrRecordNotFound {
		// Create new token
		token.LastUsedAt = time.Now()
		return r.db.WithContext(ctx).Create(token).Error
	}

	if err != nil {
		return err
	}

	// Update existing token
	existing.IsActive = true
	existing.LastUsedAt = time.Now()
	existing.DeviceType = token.DeviceType
	existing.DeviceName = token.DeviceName
	existing.EntityID = token.EntityID

	return r.db.WithContext(ctx).Save(&existing).Error
}

// GetUserDeviceTokens retrieves all active device tokens for a user
func (r *repository) GetUserDeviceTokens(ctx context.Context, userID uint, entityID uint) ([]string, error) {
	var tokens []FCMDeviceToken
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND entity_id = ? AND is_active = ?", userID, entityID, true).
		Find(&tokens).Error

	if err != nil {
		return nil, err
	}

	result := make([]string, len(tokens))
	for i, t := range tokens {
		result[i] = t.DeviceToken
	}

	return result, nil
}

// GetDeviceTokensByEntityAndRole gets tokens for users with specific roles in an entity
func (r *repository) GetDeviceTokensByEntityAndRole(ctx context.Context, entityID uint, roleNames []string) ([]string, error) {
	var tokens []string

	// This requires a join with users and roles tables
	// Adjust based on your auth schema
	query := `
	SELECT DISTINCT fdt.device_token
FROM fcm_device_tokens fdt
INNER JOIN users u ON fdt.user_id = u.id
INNER JOIN user_roles r ON u.role_id = r.id
WHERE fdt.entity_id = ?
AND fdt.is_active = true
AND r.name IN (?)

	`

	err := r.db.WithContext(ctx).Raw(query, entityID, roleNames).Scan(&tokens).Error
	return tokens, err
}

// RemoveDeviceToken deactivates a specific device token
func (r *repository) RemoveDeviceToken(ctx context.Context, userID uint, deviceToken string) error {
	return r.db.WithContext(ctx).
		Model(&FCMDeviceToken{}).
		Where("user_id = ? AND device_token = ?", userID, deviceToken).
		Update("is_active", false).Error
}

// DeactivateOldTokens deactivates all tokens for a user except the specified one
func (r *repository) DeactivateOldTokens(ctx context.Context, userID uint, keepToken string) error {
	return r.db.WithContext(ctx).
		Model(&FCMDeviceToken{}).
		Where("user_id = ? AND device_token != ?", userID, keepToken).
		Update("is_active", false).Error
}