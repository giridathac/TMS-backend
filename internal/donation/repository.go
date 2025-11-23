package donation

import (
	"context"
	"time"

	"gorm.io/gorm"
)

type Repository interface {
	// Basic CRUD operations
	Create(ctx context.Context, donation *Donation) error
	GetByOrderID(ctx context.Context, orderID string) (*Donation, error)
	GetByIDWithUser(ctx context.Context, donationID uint) (*DonationWithUser, error)
	UpdatePaymentDetails(ctx context.Context, orderID string, params UpdatePaymentDetailsParams) error

	// Data retrieval with filtering
	ListByUserID(ctx context.Context, userID uint) ([]DonationWithUser, error)
	ListByUserIDAndEntity(ctx context.Context, userID uint, entityID uint) ([]DonationWithUser, error) // NEW: Entity-based filtering for users
	ListWithFilters(ctx context.Context, filters DonationFilters) ([]DonationWithUser, int, error)

	// Analytics queries
	GetTotalStats(ctx context.Context, entityID uint) (*StatsResult, error)
	GetStatsInDateRange(ctx context.Context, entityID uint, from, to time.Time) (*StatsResult, error)
	GetUniqueDonorCount(ctx context.Context, entityID uint) (int, error)
	GetTopDonors(ctx context.Context, entityID uint, limit int) ([]TopDonor, error)
	GetDonationTrends(ctx context.Context, entityID uint, days int) ([]TrendData, error)
	GetDonationsByType(ctx context.Context, entityID uint) ([]TypeData, error)
	GetDonationsByMethod(ctx context.Context, entityID uint) ([]MethodData, error)
	
	// Recent donations - BOTH USER AND ENTITY with enhanced filtering
	GetRecentDonationsByUser(ctx context.Context, userID uint, limit int) ([]RecentDonation, error)
	GetRecentDonationsByUserAndEntity(ctx context.Context, userID uint, entityID uint, limit int) ([]RecentDonation, error) // NEW: User donations within specific entity
	GetRecentDonationsByEntity(ctx context.Context, entityID uint, limit int) ([]RecentDonation, error)
}

type repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) Repository {
	return &repository{db: db}
}

// ==============================
// Basic CRUD Operations - UNCHANGED
// ==============================

func (r *repository) Create(ctx context.Context, donation *Donation) error {
	return r.db.WithContext(ctx).Create(donation).Error
}

func (r *repository) GetByOrderID(ctx context.Context, orderID string) (*Donation, error) {
	var donation Donation
	err := r.db.WithContext(ctx).
		Where("order_id = ?", orderID).
		First(&donation).Error
	if err != nil {
		return nil, err
	}
	return &donation, nil
}

func (r *repository) GetByIDWithUser(ctx context.Context, donationID uint) (*DonationWithUser, error) {
	var result DonationWithUser
	err := r.db.WithContext(ctx).
		Table("donations d").
		Select(`
			d.id, d.user_id, d.entity_id, d.amount, d.donation_type, d.reference_id,
			d.method, d.status, d.order_id, d.payment_id, d.note, d.donated_at,
			d.created_at, d.updated_at,
			COALESCE(NULLIF(u.full_name, ''), u.email, 'Anonymous') as user_name, 
			COALESCE(u.email, '') as user_email,
			COALESCE(e.name, '') as entity_name
		`).
		Joins("LEFT JOIN users u ON d.user_id = u.id").
		Joins("LEFT JOIN entities e ON d.entity_id = e.id").
		Where("d.id = ?", donationID).
		First(&result).Error

	if err != nil {
		return nil, err
	}
	return &result, nil
}

func (r *repository) UpdatePaymentDetails(ctx context.Context, orderID string, params UpdatePaymentDetailsParams) error {
	updates := map[string]interface{}{
		"status":     params.Status,
		"payment_id": params.PaymentID,
		"method":     params.Method,
		"amount":     params.Amount,
	}

	if params.DonatedAt != nil {
		updates["donated_at"] = params.DonatedAt
	}

	return r.db.WithContext(ctx).
		Model(&Donation{}).
		Where("order_id = ?", orderID).
		Updates(updates).Error
}

// ==============================
// Data Retrieval with Filtering - ENHANCED with entity-based filtering
// ==============================

func (r *repository) ListByUserID(ctx context.Context, userID uint) ([]DonationWithUser, error) {
	var donations []DonationWithUser
	err := r.db.WithContext(ctx).
		Table("donations d").
		Select(`
			d.id, d.user_id, d.entity_id, d.amount, d.donation_type, d.reference_id,
			d.method, d.status, d.order_id, d.payment_id, d.note, d.donated_at,
			d.created_at, d.updated_at,
			COALESCE(NULLIF(u.full_name, ''), u.email, 'Anonymous') as user_name, 
			COALESCE(u.email, '') as user_email,
			COALESCE(e.name, '') as entity_name
		`).
		Joins("LEFT JOIN users u ON d.user_id = u.id").
		Joins("LEFT JOIN entities e ON d.entity_id = e.id").
		Where("d.user_id = ?", userID).
		Order("d.created_at DESC").
		Find(&donations).Error

	return donations, err
}

// NEW: Get donations by user filtered by specific entity - SAME PATTERN AS EVENTS
func (r *repository) ListByUserIDAndEntity(ctx context.Context, userID uint, entityID uint) ([]DonationWithUser, error) {
	var donations []DonationWithUser
	err := r.db.WithContext(ctx).
		Table("donations d").
		Select(`
			d.id, d.user_id, d.entity_id, d.amount, d.donation_type, d.reference_id,
			d.method, d.status, d.order_id, d.payment_id, d.note, d.donated_at,
			d.created_at, d.updated_at,
			COALESCE(NULLIF(u.full_name, ''), u.email, 'Anonymous') as user_name, 
			COALESCE(u.email, '') as user_email,
			COALESCE(e.name, '') as entity_name
		`).
		Joins("LEFT JOIN users u ON d.user_id = u.id").
		Joins("LEFT JOIN entities e ON d.entity_id = e.id").
		Where("d.user_id = ? AND d.entity_id = ?", userID, entityID). // NEW: Entity-based filtering
		Order("d.created_at DESC").
		Find(&donations).Error

	return donations, err
}

func (r *repository) ListWithFilters(ctx context.Context, filters DonationFilters) ([]DonationWithUser, int, error) {
	var donations []DonationWithUser
	var total int64

	// Build base query with proper field selection
	query := r.db.WithContext(ctx).
		Table("donations d").
		Select(`
			d.id, d.user_id, d.entity_id, d.amount, d.donation_type, d.reference_id,
			d.method, d.status, d.order_id, d.payment_id, d.note, d.donated_at,
			d.created_at, d.updated_at,
			COALESCE(NULLIF(u.full_name, ''), u.email, 'Anonymous') as user_name, 
			COALESCE(u.email, '') as user_email,
			COALESCE(e.name, '') as entity_name
		`).
		Joins("LEFT JOIN users u ON d.user_id = u.id").
		Joins("LEFT JOIN entities e ON d.entity_id = e.id").
		Where("d.entity_id = ?", filters.EntityID) // CRITICAL: Entity-based filtering

	// Apply filters
	query = r.applyFilters(query, filters)

	// Count total records
	countQuery := r.db.WithContext(ctx).
		Table("donations d").
		Joins("LEFT JOIN users u ON d.user_id = u.id").
		Where("d.entity_id = ?", filters.EntityID) // CRITICAL: Entity-based filtering for count too
	countQuery = r.applyFilters(countQuery, filters)
	countQuery.Count(&total)

	// Apply pagination and ordering
	if filters.Page > 0 && filters.Limit > 0 {
		offset := (filters.Page - 1) * filters.Limit
		query = query.Offset(offset).Limit(filters.Limit)
	}

	err := query.Order("d.created_at DESC").Find(&donations).Error
	return donations, int(total), err
}

func (r *repository) applyFilters(query *gorm.DB, filters DonationFilters) *gorm.DB {
	// Status filter
	if filters.Status != "" && filters.Status != "all" {
		query = query.Where("LOWER(d.status) = LOWER(?)", filters.Status)
	}

	// Type filter
	if filters.Type != "" && filters.Type != "all" {
		query = query.Where("LOWER(d.donation_type) = LOWER(?)", filters.Type)
	}

	// Method filter
	if filters.Method != "" && filters.Method != "all" {
		query = query.Where("LOWER(d.method) = LOWER(?)", filters.Method)
	}

	// Date range filters
	if filters.From != nil {
		query = query.Where("d.created_at >= ?", filters.From)
	}
	if filters.To != nil {
		query = query.Where("d.created_at <= ?", filters.To)
	}

	// Amount range filters
	if filters.MinAmount != nil {
		query = query.Where("d.amount >= ?", *filters.MinAmount)
	}
	if filters.MaxAmount != nil {
		query = query.Where("d.amount <= ?", *filters.MaxAmount)
	}

	// Search filter
	if filters.Search != "" {
		searchTerm := "%" + filters.Search + "%"
		query = query.Where(`
			COALESCE(NULLIF(u.full_name, ''), u.email, 'Anonymous') ILIKE ? OR 
			u.email ILIKE ? OR 
			d.payment_id ILIKE ? OR 
			d.order_id ILIKE ?
		`, searchTerm, searchTerm, searchTerm, searchTerm)
	}

	return query
}

// ==============================
// Analytics Queries - UNCHANGED (Already entity-based)
// ==============================

func (r *repository) GetTotalStats(ctx context.Context, entityID uint) (*StatsResult, error) {
	var result StatsResult
	err := r.db.WithContext(ctx).
		Table("donations").
		Select(`
			COALESCE(SUM(CASE WHEN LOWER(status) = 'success' THEN amount ELSE 0 END), 0) as amount,
			COUNT(*) as count,
			COALESCE(SUM(CASE WHEN LOWER(status) = 'success' THEN 1 ELSE 0 END), 0) as completed_count,
			COALESCE(SUM(CASE WHEN LOWER(status) = 'pending' THEN 1 ELSE 0 END), 0) as pending_count,
			COALESCE(SUM(CASE WHEN LOWER(status) = 'failed' THEN 1 ELSE 0 END), 0) as failed_count
		`).
		Where("entity_id = ?", entityID).
		Scan(&result).Error

	return &result, err
}

func (r *repository) GetStatsInDateRange(ctx context.Context, entityID uint, from, to time.Time) (*StatsResult, error) {
	var result StatsResult
	err := r.db.WithContext(ctx).
		Table("donations").
		Select(`
			COALESCE(SUM(CASE WHEN LOWER(status) = 'success' THEN amount ELSE 0 END), 0) as amount,
			COUNT(*) as count,
			COALESCE(SUM(CASE WHEN LOWER(status) = 'success' THEN 1 ELSE 0 END), 0) as completed_count,
			COALESCE(SUM(CASE WHEN LOWER(status) = 'pending' THEN 1 ELSE 0 END), 0) as pending_count,
			COALESCE(SUM(CASE WHEN LOWER(status) = 'failed' THEN 1 ELSE 0 END), 0) as failed_count
		`).
		Where("entity_id = ? AND created_at >= ? AND created_at <= ?", entityID, from, to).
		Scan(&result).Error

	return &result, err
}

func (r *repository) GetUniqueDonorCount(ctx context.Context, entityID uint) (int, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Table("donations").
		Where("entity_id = ? AND LOWER(status) = 'success'", entityID).
		Distinct("user_id").
		Count(&count).Error

	return int(count), err
}

func (r *repository) GetTopDonors(ctx context.Context, entityID uint, limit int) ([]TopDonor, error) {
	var donors []TopDonor
	err := r.db.WithContext(ctx).
		Table("donations d").
		Select(`
			COALESCE(NULLIF(u.full_name, ''), u.email, 'Anonymous') as name, 
			COALESCE(u.email, '') as email, 
			SUM(d.amount) as total_amount, 
			COUNT(d.id) as donation_count
		`).
		Joins("JOIN users u ON d.user_id = u.id").
		Where("d.entity_id = ? AND LOWER(d.status) = 'success'", entityID).
		Group("u.id, u.full_name, u.email").
		Order("total_amount DESC").
		Limit(limit).
		Scan(&donors).Error

	return donors, err
}

func (r *repository) GetDonationTrends(ctx context.Context, entityID uint, days int) ([]TrendData, error) {
	var trends []TrendData

	// Get date range
	endDate := time.Now()
	startDate := endDate.AddDate(0, 0, -days)

	err := r.db.WithContext(ctx).
		Table("donations").
		Select(`
			DATE(created_at) as date,
			COALESCE(SUM(CASE WHEN LOWER(status) = 'success' THEN amount ELSE 0 END), 0) as amount,
			COUNT(*) as count
		`).
		Where("entity_id = ? AND created_at >= ? AND created_at <= ?", entityID, startDate, endDate).
		Group("DATE(created_at)").
		Order("date ASC").
		Scan(&trends).Error

	return trends, err
}

func (r *repository) GetDonationsByType(ctx context.Context, entityID uint) ([]TypeData, error) {
	var typeData []TypeData
	err := r.db.WithContext(ctx).
		Table("donations").
		Select(`
			donation_type as type,
			COALESCE(SUM(CASE WHEN LOWER(status) = 'success' THEN amount ELSE 0 END), 0) as amount,
			COUNT(*) as count
		`).
		Where("entity_id = ?", entityID).
		Group("donation_type").
		Order("amount DESC").
		Scan(&typeData).Error

	return typeData, err
}

func (r *repository) GetDonationsByMethod(ctx context.Context, entityID uint) ([]MethodData, error) {
	var methodData []MethodData
	err := r.db.WithContext(ctx).
		Table("donations").
		Select(`
			method,
			COALESCE(SUM(CASE WHEN LOWER(status) = 'success' THEN amount ELSE 0 END), 0) as amount,
			COUNT(*) as count
		`).
		Where("entity_id = ? AND LOWER(status) = 'success'", entityID).
		Group("method").
		Order("amount DESC").
		Scan(&methodData).Error

	return methodData, err
}

// ==============================
// Recent Donations - ENHANCED with entity-based filtering
// ==============================

func (r *repository) GetRecentDonationsByUser(ctx context.Context, userID uint, limit int) ([]RecentDonation, error) {
	var recent []RecentDonation
	err := r.db.WithContext(ctx).
		Table("donations d").
		Select(`
			d.amount, d.donation_type, d.method, d.status, 
			COALESCE(d.donated_at, d.created_at) as donated_at,
			COALESCE(NULLIF(u.full_name, ''), u.email, 'Anonymous') as user_name,
			COALESCE(e.name, '') as entity_name
		`).
		Joins("LEFT JOIN users u ON d.user_id = u.id").
		Joins("LEFT JOIN entities e ON d.entity_id = e.id").
		Where("d.user_id = ?", userID).
		Order("COALESCE(d.donated_at, d.created_at) DESC").
		Limit(limit).
		Scan(&recent).Error
	return recent, err
}

// NEW: Get recent donations by user filtered by entity - SAME PATTERN AS EVENTS
func (r *repository) GetRecentDonationsByUserAndEntity(ctx context.Context, userID uint, entityID uint, limit int) ([]RecentDonation, error) {
	var recent []RecentDonation
	err := r.db.WithContext(ctx).
		Table("donations d").
		Select(`
			d.amount, d.donation_type, d.method, d.status, 
			COALESCE(d.donated_at, d.created_at) as donated_at,
			COALESCE(NULLIF(u.full_name, ''), u.email, 'Anonymous') as user_name,
			COALESCE(e.name, '') as entity_name
		`).
		Joins("LEFT JOIN users u ON d.user_id = u.id").
		Joins("LEFT JOIN entities e ON d.entity_id = e.id").
		Where("d.user_id = ? AND d.entity_id = ?", userID, entityID). // NEW: Entity-based filtering
		Order("COALESCE(d.donated_at, d.created_at) DESC").
		Limit(limit).
		Scan(&recent).Error
	return recent, err
}

func (r *repository) GetRecentDonationsByEntity(ctx context.Context, entityID uint, limit int) ([]RecentDonation, error) {
	var recent []RecentDonation
	err := r.db.WithContext(ctx).
		Table("donations d").
		Select(`
			d.amount, d.donation_type, d.method, d.status, 
			COALESCE(d.donated_at, d.created_at) as donated_at,
			COALESCE(NULLIF(u.full_name, ''), u.email, 'Anonymous') as user_name,
			COALESCE(e.name, '') as entity_name
		`).
		Joins("LEFT JOIN users u ON d.user_id = u.id").
		Joins("LEFT JOIN entities e ON d.entity_id = e.id").
		Where("d.entity_id = ?", entityID). // Filter by entity instead of user
		Order("COALESCE(d.donated_at, d.created_at) DESC").
		Limit(limit).
		Scan(&recent).Error
	return recent, err
}