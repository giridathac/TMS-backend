package donation

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	razorpay "github.com/razorpay/razorpay-go"
	"github.com/sharath018/temple-management-backend/config"
	"github.com/sharath018/temple-management-backend/internal/auditlog"
	"github.com/sharath018/temple-management-backend/middleware"
)

type Service interface {
	// Core donation operations (DEVOTEE - UNCHANGED)
	StartDonation(req CreateDonationRequest) (*CreateDonationResponse, error)
	VerifyAndUpdateDonation(req VerifyPaymentRequest) error
	
	// Data retrieval - UPDATED for entity-based approach
	GetDonationsByUser(userID uint) ([]DonationWithUser, error)
	GetDonationsByUserAndEntity(userID uint, entityID uint) ([]DonationWithUser, error) // NEW
	GetDonationsWithFilters(filters DonationFilters, accessContext middleware.AccessContext) ([]DonationWithUser, int, error)
	
	// Analytics and reporting - TEMPLE ADMIN (UPDATED)
	GetDashboardStats(entityID uint, accessContext middleware.AccessContext) (*DashboardStats, error)
	GetTopDonors(entityID uint, limit int, accessContext middleware.AccessContext) ([]TopDonor, error)
	GetAnalytics(entityID uint, days int, accessContext middleware.AccessContext) (*AnalyticsData, error)
	
	// Receipt and export - BOTH (UPDATED)
	GenerateReceipt(donationID uint, userID uint, accessContext *middleware.AccessContext, entityID uint) (*Receipt, error) // NEW: Added entityID
	ExportDonations(filters DonationFilters, format string, accessContext middleware.AccessContext) ([]byte, string, error)

	// Recent donations - BOTH (UPDATED)
	GetRecentDonationsByUser(ctx context.Context, userID uint, limit int) ([]RecentDonation, error)
	GetRecentDonationsByUserAndEntity(ctx context.Context, userID uint, entityID uint, limit int) ([]RecentDonation, error) // NEW
	GetRecentDonationsByEntity(ctx context.Context, entityID uint, limit int, accessContext middleware.AccessContext) ([]RecentDonation, error)
}

type service struct {
	repo       Repository
	client     *razorpay.Client
	cfg        *config.Config
	auditSvc   auditlog.Service
}

func NewService(repo Repository, cfg *config.Config, auditSvc auditlog.Service) Service {
	client := razorpay.NewClient(cfg.RazorpayKey, cfg.RazorpaySecret)
	return &service{
		repo:     repo,
		client:   client,
		cfg:      cfg,
		auditSvc: auditSvc,
	}
}

// ==============================
// Core Donation Operations (DEVOTEE - UNCHANGED)
// ==============================

// StartDonation initializes the Razorpay order and creates a pending donation entry
func (s *service) StartDonation(req CreateDonationRequest) (*CreateDonationResponse, error) {
	ctx := context.Background()
	
	// Create Razorpay order
	amountInPaise := int(req.Amount * 100)
	
	data := map[string]interface{}{
		"amount":          amountInPaise,
		"currency":        "INR",
		"payment_capture": 1,
		"notes": map[string]interface{}{
			"user_id":       req.UserID,
			"entity_id":     req.EntityID,
			"donation_type": req.DonationType,
		},
	}
	
	if req.ReferenceID != nil {
		data["notes"].(map[string]interface{})["reference_id"] = *req.ReferenceID
	}

	order, err := s.client.Order.Create(data, nil)
	if err != nil {
		s.auditSvc.LogAction(ctx, &req.UserID, &req.EntityID, "DONATION_INITIATED", map[string]interface{}{
			"amount":        req.Amount,
			"donation_type": req.DonationType,
			"error":         err.Error(),
		}, req.IPAddress, "failure")
		
		return nil, fmt.Errorf("razorpay order creation failed: %w", err)
	}

	orderID, ok := order["id"].(string)
	if !ok {
		s.auditSvc.LogAction(ctx, &req.UserID, &req.EntityID, "DONATION_INITIATED", map[string]interface{}{
			"amount":        req.Amount,
			"donation_type": req.DonationType,
			"error":         "unable to extract order_id from Razorpay response",
		}, req.IPAddress, "failure")
		
		return nil, errors.New("unable to extract order_id from Razorpay response")
	}

	// Create pending donation record
	donation := &Donation{
		UserID:       req.UserID,
		EntityID:     req.EntityID,
		Amount:       req.Amount,
		DonationType: req.DonationType,
		ReferenceID:  req.ReferenceID,
		Method:       "PENDING", // Will be updated after payment
		Status:       StatusPending,
		OrderID:      orderID,
		Note:         req.Note,
	}

	if err := s.repo.Create(context.Background(), donation); err != nil {
		s.auditSvc.LogAction(ctx, &req.UserID, &req.EntityID, "DONATION_INITIATED", map[string]interface{}{
			"amount":        req.Amount,
			"donation_type": req.DonationType,
			"order_id":      orderID,
			"error":         err.Error(),
		}, req.IPAddress, "failure")
		
		return nil, fmt.Errorf("failed to create donation record: %w", err)
	}

	s.auditSvc.LogAction(ctx, &req.UserID, &req.EntityID, "DONATION_INITIATED", map[string]interface{}{
		"amount":        req.Amount,
		"donation_type": req.DonationType,
		"order_id":      orderID,
		"reference_id":  req.ReferenceID,
	}, req.IPAddress, "success")

	return &CreateDonationResponse{
		OrderID:     orderID,
		Amount:      req.Amount,
		Currency:    "INR",
		RazorpayKey: s.cfg.RazorpayKey,
	}, nil
}

// VerifyAndUpdateDonation securely verifies Razorpay signature and updates payment status (DEVOTEE - UNCHANGED)
func (s *service) VerifyAndUpdateDonation(req VerifyPaymentRequest) error {
	ctx := context.Background()
	
	// Step 1: Verify HMAC Signature
	expected := hmac.New(sha256.New, []byte(s.cfg.RazorpaySecret))
	expected.Write([]byte(req.OrderID + "|" + req.PaymentID))
	computedSignature := hex.EncodeToString(expected.Sum(nil))

	if computedSignature != req.RazorpaySig {
		s.auditSvc.LogAction(ctx, nil, nil, "DONATION_VERIFICATION_FAILED", map[string]interface{}{
			"order_id":   req.OrderID,
			"payment_id": req.PaymentID,
			"reason":     "invalid payment signature",
		}, req.IPAddress, "failure")
		
		return fmt.Errorf("invalid payment signature")
	}

	// Step 2: Fetch payment details from Razorpay
	payment, err := s.client.Payment.Fetch(req.PaymentID, nil, nil)
	if err != nil {
		s.auditSvc.LogAction(ctx, nil, nil, "DONATION_VERIFICATION_FAILED", map[string]interface{}{
			"order_id":   req.OrderID,
			"payment_id": req.PaymentID,
			"reason":     "razorpay payment fetch failed",
			"error":      err.Error(),
		}, req.IPAddress, "failure")
		
		return fmt.Errorf("razorpay payment fetch failed: %w", err)
	}

	status, ok := payment["status"].(string)
	if !ok {
		s.auditSvc.LogAction(ctx, nil, nil, "DONATION_VERIFICATION_FAILED", map[string]interface{}{
			"order_id":   req.OrderID,
			"payment_id": req.PaymentID,
			"reason":     "invalid payment status format",
		}, req.IPAddress, "failure")
		
		return errors.New("invalid payment status format")
	}

	// Step 3: Get donation record
	donation, err := s.repo.GetByOrderID(context.Background(), req.OrderID)
	if err != nil {
		s.auditSvc.LogAction(ctx, nil, nil, "DONATION_VERIFICATION_FAILED", map[string]interface{}{
			"order_id":   req.OrderID,
			"payment_id": req.PaymentID,
			"reason":     "donation record not found",
		}, req.IPAddress, "failure")
		
		return errors.New("donation record not found for given order ID")
	}

	if donation.Status == StatusSuccess {
		s.auditSvc.LogAction(ctx, &donation.UserID, &donation.EntityID, "DONATION_ALREADY_PROCESSED", map[string]interface{}{
			"order_id":   req.OrderID,
			"payment_id": req.PaymentID,
			"amount":     donation.Amount,
		}, req.IPAddress, "success")
		
		return nil // Already processed
	}

	// Step 4: Update donation status
	var amount float64
	switch val := payment["amount"].(type) {
	case float64:
		amount = val / 100
	case json.Number:
		amountPaise, _ := val.Float64()
		amount = amountPaise / 100
	default:
		s.auditSvc.LogAction(ctx, &donation.UserID, &donation.EntityID, "DONATION_VERIFICATION_FAILED", map[string]interface{}{
			"order_id":   req.OrderID,
			"payment_id": req.PaymentID,
			"reason":     "unsupported amount type",
			"amount_type": fmt.Sprintf("%T", val),
		}, req.IPAddress, "failure")
		
		return fmt.Errorf("unsupported amount type: %T", val)
	}

	newStatus := StatusFailed
	var donatedAt *time.Time
	auditAction := "DONATION_FAILED"
	auditStatus := "failure"
	
	if status == "captured" {
		newStatus = StatusSuccess
		now := time.Now()
		donatedAt = &now
		auditAction = "DONATION_SUCCESS"
		auditStatus = "success"
	}

	// Extract payment method
	method := "UNKNOWN"
	if paymentMethod, ok := payment["method"].(string); ok {
		method = paymentMethod
	}

	// Update the donation record
	err = s.repo.UpdatePaymentDetails(context.Background(), req.OrderID, UpdatePaymentDetailsParams{
		Status:    newStatus,
		PaymentID: &req.PaymentID,
		Method:    method,
		Amount:    amount,
		DonatedAt: donatedAt,
	})

	if err != nil {
		s.auditSvc.LogAction(ctx, &donation.UserID, &donation.EntityID, "DONATION_UPDATE_FAILED", map[string]interface{}{
			"order_id":   req.OrderID,
			"payment_id": req.PaymentID,
			"amount":     amount,
			"method":     method,
			"error":      err.Error(),
		}, req.IPAddress, "failure")
		
		return err
	}

	s.auditSvc.LogAction(ctx, &donation.UserID, &donation.EntityID, auditAction, map[string]interface{}{
		"order_id":       req.OrderID,
		"payment_id":     req.PaymentID,
		"amount":         amount,
		"donation_type":  donation.DonationType,
		"method":         method,
		"razorpay_status": status,
		"reference_id":   donation.ReferenceID,
	}, req.IPAddress, auditStatus)

	return nil
}

// ==============================
// Data Retrieval - UPDATED for entity-based approach
// ==============================

func (s *service) GetDonationsByUser(userID uint) ([]DonationWithUser, error) {
	donations, err := s.repo.ListByUserID(context.Background(), userID)
	if err != nil {
		return nil, err
	}

	for i := range donations {
		donations[i].Date = donations[i].CreatedAt
		donations[i].Type = donations[i].DonationType
		donations[i].DonorName = donations[i].UserName
		donations[i].DonorEmail = donations[i].UserEmail
		donations[i].PaymentMethod = donations[i].Method
		
		if donations[i].DonatedAt == nil {
			donations[i].DonatedAt = &donations[i].CreatedAt
		}
	}

	return donations, nil
}

// NEW: Get donations by user filtered by entity
func (s *service) GetDonationsByUserAndEntity(userID uint, entityID uint) ([]DonationWithUser, error) {
	donations, err := s.repo.ListByUserIDAndEntity(context.Background(), userID, entityID)
	if err != nil {
		return nil, err
	}

	for i := range donations {
		donations[i].Date = donations[i].CreatedAt
		donations[i].Type = donations[i].DonationType
		donations[i].DonorName = donations[i].UserName
		donations[i].DonorEmail = donations[i].UserEmail
		donations[i].PaymentMethod = donations[i].Method
		
		if donations[i].DonatedAt == nil {
			donations[i].DonatedAt = &donations[i].CreatedAt
		}
	}

	return donations, nil
}

func (s *service) GetDonationsWithFilters(filters DonationFilters, accessContext middleware.AccessContext) ([]DonationWithUser, int, error) {
	// Check permissions
	if !accessContext.CanRead() {
		return nil, 0, errors.New("read access denied")
	}

	// Verify entity access
	entityID := accessContext.GetAccessibleEntityID()
	if entityID == nil || *entityID != filters.EntityID {
		return nil, 0, errors.New("access denied to requested entity")
	}

	donations, total, err := s.repo.ListWithFilters(context.Background(), filters)
	if err != nil {
		return nil, 0, err
	}

	for i := range donations {
		donations[i].Date = donations[i].CreatedAt
		donations[i].Type = donations[i].DonationType
		donations[i].DonorName = donations[i].UserName
		donations[i].DonorEmail = donations[i].UserEmail
		donations[i].PaymentMethod = donations[i].Method
		
		if donations[i].DonatedAt == nil {
			donations[i].DonatedAt = &donations[i].CreatedAt
		}
	}

	return donations, total, nil
}

// ==============================
// Analytics and Reporting - TEMPLE ADMIN (UPDATED)
// ==============================

func (s *service) GetDashboardStats(entityID uint, accessContext middleware.AccessContext) (*DashboardStats, error) {
	// Check permissions
	if !accessContext.CanRead() {
		return nil, errors.New("read access denied")
	}

	// Verify entity access
	accessibleEntityID := accessContext.GetAccessibleEntityID()
	if accessibleEntityID == nil || *accessibleEntityID != entityID {
		return nil, errors.New("access denied to requested entity")
	}

	ctx := context.Background()
	
	// Get overall stats
	totalStats, err := s.repo.GetTotalStats(ctx, entityID)
	if err != nil {
		return nil, err
	}

	// Get this month stats
	now := time.Now()
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	monthStats, err := s.repo.GetStatsInDateRange(ctx, entityID, monthStart, now)
	if err != nil {
		return nil, err
	}

	// Get today stats
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	todayStats, err := s.repo.GetStatsInDateRange(ctx, entityID, todayStart, now)
	if err != nil {
		return nil, err
	}

	// Get unique donor count
	donorCount, err := s.repo.GetUniqueDonorCount(ctx, entityID)
	if err != nil {
		return nil, err
	}

	return &DashboardStats{
		TotalAmount:    totalStats.Amount,
		TotalCount:     totalStats.Count,
		CompletedCount: totalStats.CompletedCount,
		PendingCount:   totalStats.PendingCount,
		FailedCount:    totalStats.FailedCount,
		ThisMonth:      monthStats.Amount,
		Today:          todayStats.Amount,
		TotalDonors:    donorCount,
		AverageAmount:  func() float64 {
			if totalStats.CompletedCount > 0 {
				return totalStats.Amount / float64(totalStats.CompletedCount)
			}
			return 0
		}(),
	}, nil
}

func (s *service) GetTopDonors(entityID uint, limit int, accessContext middleware.AccessContext) ([]TopDonor, error) {
	// Check permissions
	if !accessContext.CanRead() {
		return nil, errors.New("read access denied")
	}

	// Verify entity access
	accessibleEntityID := accessContext.GetAccessibleEntityID()
	if accessibleEntityID == nil || *accessibleEntityID != entityID {
		return nil, errors.New("access denied to requested entity")
	}

	return s.repo.GetTopDonors(context.Background(), entityID, limit)
}

func (s *service) GetAnalytics(entityID uint, days int, accessContext middleware.AccessContext) (*AnalyticsData, error) {
	// Check permissions
	if !accessContext.CanRead() {
		return nil, errors.New("read access denied")
	}

	// Verify entity access
	accessibleEntityID := accessContext.GetAccessibleEntityID()
	if accessibleEntityID == nil || *accessibleEntityID != entityID {
		return nil, errors.New("access denied to requested entity")
	}

	ctx := context.Background()
	
	// Get donation trends
	trends, err := s.repo.GetDonationTrends(ctx, entityID, days)
	if err != nil {
		return nil, err
	}

	// Get donation by type
	byType, err := s.repo.GetDonationsByType(ctx, entityID)
	if err != nil {
		return nil, err
	}

	// Get donation by method
	byMethod, err := s.repo.GetDonationsByMethod(ctx, entityID)
	if err != nil {
		return nil, err
	}

	return &AnalyticsData{
		Trends:    trends,
		ByType:    byType,
		ByMethod:  byMethod,
	}, nil
}

// ==============================
// Receipt and Export - BOTH (UPDATED)
// ==============================

// NEW: Updated to include entity validation
func (s *service) GenerateReceipt(donationID uint, userID uint, accessContext *middleware.AccessContext, entityID uint) (*Receipt, error) {
	ctx := context.Background()
	
	donation, err := s.repo.GetByIDWithUser(ctx, donationID)
	if err != nil {
		return nil, err
	}

	// Check access permissions
	hasAccess := false
	
	// For devotees - can only access their own donations within their entity
	if donation.UserID == userID && donation.EntityID == entityID {
		hasAccess = true
	}
	
	// For temple admins - can access donations for their entity
	if accessContext != nil {
		accessibleEntityID := accessContext.GetAccessibleEntityID()
		if accessibleEntityID != nil && *accessibleEntityID == donation.EntityID && accessContext.CanRead() {
			hasAccess = true
		}
	}

	if !hasAccess {
		return nil, errors.New("unauthorized to access this donation")
	}

	if donation.Status != StatusSuccess {
		return nil, errors.New("receipt can only be generated for successful donations")
	}

	transactionID := donation.OrderID
	if donation.PaymentID != nil {
		transactionID = *donation.PaymentID
	}

	donatedAt := donation.CreatedAt
	if donation.DonatedAt != nil {
		donatedAt = *donation.DonatedAt
	}

	return &Receipt{
		ID:              donation.ID,
		DonationAmount:  donation.Amount,
		DonationType:    donation.DonationType,
		DonorName:       donation.UserName,
		DonorEmail:      donation.UserEmail,
		TransactionID:   transactionID,
		DonatedAt:       donatedAt,
		Method:          donation.Method,
		EntityName:      donation.EntityName,
		ReceiptNumber:   fmt.Sprintf("RCP-%d-%d", donation.EntityID, donation.ID),
		GeneratedAt:     time.Now(),
	}, nil
}

func (s *service) ExportDonations(filters DonationFilters, format string, accessContext middleware.AccessContext) ([]byte, string, error) {
	// Check permissions
	if !accessContext.CanRead() {
		return nil, "", errors.New("read access denied")
	}

	// Verify entity access
	entityID := accessContext.GetAccessibleEntityID()
	if entityID == nil || *entityID != filters.EntityID {
		return nil, "", errors.New("access denied to requested entity")
	}

	ctx := context.Background()
	
	// Get all donations matching filters
	donations, _, err := s.repo.ListWithFilters(ctx, filters)
	if err != nil {
		return nil, "", err
	}

	switch format {
	case "csv":
		return s.exportAsCSV(donations)
	default:
		return nil, "", errors.New("unsupported export format")
	}
}

func (s *service) exportAsCSV(donations []DonationWithUser) ([]byte, string, error) {
	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)

	// Write header
	header := []string{
		"ID", "Date", "Donor Name", "Donor Email", "Amount", "Type", 
		"Method", "Status", "Transaction ID", "Reference ID", "Note",
	}
	if err := writer.Write(header); err != nil {
		return nil, "", err
	}

	// Write data
	for _, donation := range donations {
		donatedAt := donation.CreatedAt
		if donation.DonatedAt != nil {
			donatedAt = *donation.DonatedAt
		}

		record := []string{
			strconv.FormatUint(uint64(donation.ID), 10),
			donatedAt.Format("2006-01-02 15:04:05"),
			donation.UserName,
			donation.UserEmail,
			fmt.Sprintf("%.2f", donation.Amount),
			donation.DonationType,
			donation.Method,
			donation.Status,
			func() string {
				if donation.PaymentID != nil {
					return *donation.PaymentID
				}
				return donation.OrderID
			}(),
			func() string {
				if donation.ReferenceID != nil {
					return strconv.FormatUint(uint64(*donation.ReferenceID), 10)
				}
				return ""
			}(),
			func() string {
				if donation.Note != nil {
					return *donation.Note
				}
				return ""
			}(),
		}
		if err := writer.Write(record); err != nil {
			return nil, "", err
		}
	}

	writer.Flush()
    if err := writer.Error(); err != nil {
        return nil, "", err
    }
    filename := fmt.Sprintf("donations_%d.csv", time.Now().Unix())
    return buf.Bytes(), filename, nil
}

// ==============================
// Recent Donations - BOTH (UPDATED)
// ==============================
func (s *service) GetRecentDonationsByUser(ctx context.Context, userID uint, limit int) ([]RecentDonation, error) {
	return s.repo.GetRecentDonationsByUser(ctx, userID, limit)
}

// NEW: Get recent donations by user filtered by entity
func (s *service) GetRecentDonationsByUserAndEntity(ctx context.Context, userID uint, entityID uint, limit int) ([]RecentDonation, error) {
	return s.repo.GetRecentDonationsByUserAndEntity(ctx, userID, entityID, limit)
}

func (s *service) GetRecentDonationsByEntity(ctx context.Context, entityID uint, limit int, accessContext middleware.AccessContext) ([]RecentDonation, error) {
	// Check permissions
	if !accessContext.CanRead() {
		return nil, errors.New("read access denied")
	}

	// Verify entity access
	accessibleEntityID := accessContext.GetAccessibleEntityID()
	if accessibleEntityID == nil || *accessibleEntityID != entityID {
		return nil, errors.New("access denied to requested entity")
	}

	return s.repo.GetRecentDonationsByEntity(ctx, entityID, limit)
}