package donation

import "time"

// ==============================
// DTOs and Request/Response Models - FIXED + AUDIT SUPPORT
// ==============================

// CreateDonationRequest is sent by frontend to initiate a donation
type CreateDonationRequest struct {
	UserID       uint    `json:"-"`                             // Filled from JWT claims
	EntityID     uint    `json:"-"`                             // Set from user context
	Amount       float64 `json:"amount" binding:"required,gt=0"` // Donation amount in INR
	DonationType string  `json:"donationType" binding:"required,oneof=general seva event festival construction annadanam education maintenance"`
	ReferenceID  *uint   `json:"referenceID,omitempty"`          // Optional: SevaID or EventID
	Note         *string `json:"note,omitempty"`                 // Optional donor message
	IPAddress    string  `json:"-"`                             // ✅ NEW: For audit logging (filled from middleware)
}

// CreateDonationResponse is returned to frontend after creating Razorpay order
type CreateDonationResponse struct {
	OrderID     string  `json:"order_id"`       // Razorpay order ID
	Amount      float64 `json:"amount"`         // Donation amount in INR
	Currency    string  `json:"currency"`       // Currency, always "INR"
	RazorpayKey string  `json:"razorpay_key"`   // Razorpay key for client-side SDK
}

// VerifyPaymentRequest is used by frontend to confirm payment success
type VerifyPaymentRequest struct {
	OrderID      string `json:"orderID" binding:"required"`            // Razorpay order ID
	PaymentID    string `json:"paymentID" binding:"required"`          // Razorpay payment ID
	RazorpaySig  string `json:"razorpaySig" binding:"required"`        // Signature to verify payment
	IPAddress    string `json:"-"`                                     // ✅ NEW: For audit logging (filled from middleware)
}

// DonationWithUser includes user and entity information - FIXED FIELD MAPPING
type DonationWithUser struct {
	ID           uint      `json:"id" db:"id"`
	UserID       uint      `json:"user_id" db:"user_id"`
	EntityID     uint      `json:"entity_id" db:"entity_id"`
	Amount       float64   `json:"amount" db:"amount"`
	DonationType string    `json:"donationType" db:"donation_type"`      // FIXED: proper mapping
	ReferenceID  *uint     `json:"referenceID,omitempty" db:"reference_id"`
	Method       string    `json:"paymentMethod" db:"method"`            // FIXED: proper mapping
	Status       string    `json:"status" db:"status"`
	OrderID      string    `json:"transactionId" db:"order_id"`          // FIXED: proper mapping
	PaymentID    *string   `json:"paymentId,omitempty" db:"payment_id"`
	Note         *string   `json:"note,omitempty" db:"note"`
	DonatedAt    *time.Time `json:"donatedAt,omitempty" db:"donated_at"`  // FIXED: proper field
	CreatedAt    time.Time `json:"created_at" db:"created_at"`           // FIXED: show date properly
	UpdatedAt    time.Time `json:"updated_at" db:"updated_at"`
	
	// User information - FIXED FIELD NAMES
	UserName  string `json:"userName" db:"user_name"`                   // FIXED: proper mapping
	UserEmail string `json:"userEmail" db:"user_email"`                 // FIXED: proper mapping
	
	// Entity information - FIXED FIELD NAME
	EntityName string `json:"entityName" db:"entity_name"`              // FIXED: proper mapping
	
	// Additional computed fields for consistency
	Date           time.Time `json:"date" db:"created_at"`               // FIXED: show donation date
	Type           string    `json:"type" db:"donation_type"`           // FIXED: show donation type
	DonorName      string    `json:"donorName" db:"user_name"`          // FIXED: donor info
	DonorEmail     string    `json:"donorEmail" db:"user_email"`        // FIXED: donor info
	PaymentMethod  string    `json:"method" db:"method"`                // FIXED: payment method
}

// DonationFilters for filtering and pagination
type DonationFilters struct {
	EntityID  uint       `json:"entity_id"`
	Status    string     `json:"status,omitempty"`
	Type      string     `json:"type,omitempty"`
	Method    string     `json:"method,omitempty"`
	From      *time.Time `json:"from,omitempty"`
	To        *time.Time `json:"to,omitempty"`
	MinAmount *float64   `json:"min_amount,omitempty"`
	MaxAmount *float64   `json:"max_amount,omitempty"`
	Search    string     `json:"search,omitempty"`
	Page      int        `json:"page"`
	Limit     int        `json:"limit"`
}

// UpdatePaymentDetailsParams for updating payment information
type UpdatePaymentDetailsParams struct {
	Status    string     `json:"status"`
	PaymentID *string    `json:"payment_id"`
	Method    string     `json:"method"`
	Amount    float64    `json:"amount"`
	DonatedAt *time.Time `json:"donated_at"`
}

// ==============================
// Analytics and Reporting Models
// ==============================

// DashboardStats represents overall donation statistics
type DashboardStats struct {
	TotalAmount    float64 `json:"totalAmount"`
	TotalCount     int     `json:"total_count"`
	CompletedCount int     `json:"completed"`
	PendingCount   int     `json:"pending"`
	FailedCount    int     `json:"failed"`
	ThisMonth      float64 `json:"thisMonth"`
	Today          float64 `json:"today"`
	TotalDonors    int     `json:"totalDonors"`
	AverageAmount  float64 `json:"averageAmount"`
}

// StatsResult for database aggregation queries
type StatsResult struct {
	Amount         float64 `json:"amount"`
	Count          int     `json:"count"`
	CompletedCount int     `json:"completed_count"`
	PendingCount   int     `json:"pending_count"`
	FailedCount    int     `json:"failed_count"`
}

// TopDonor represents a top donor
type TopDonor struct {
	Name          string  `json:"name"`
	Email         string  `json:"email"`
	TotalAmount   float64 `json:"total_amount"`
	DonationCount int     `json:"donation_count"`
}

// TrendData for donation trends over time
type TrendData struct {
	Date   time.Time `json:"date"`
	Amount float64   `json:"amount"`
	Count  int       `json:"count"`
}

// TypeData for donations by type
type TypeData struct {
	Type   string  `json:"type"`
	Amount float64 `json:"amount"`
	Count  int     `json:"count"`
}

// MethodData for donations by payment method
type MethodData struct {
	Method string  `json:"method"`
	Amount float64 `json:"amount"`
	Count  int     `json:"count"`
}

// AnalyticsData combines all analytics information
type AnalyticsData struct {
	Trends   []TrendData  `json:"trends"`
	ByType   []TypeData   `json:"byType"`
	ByMethod []MethodData `json:"byMethod"`
}

// Receipt represents a donation receipt
type Receipt struct {
	ID             uint      `json:"id"`
	DonationAmount float64   `json:"donationAmount"`
	DonationType   string    `json:"donationType"`
	DonorName      string    `json:"donorName"`
	DonorEmail     string    `json:"donorEmail"`
	TransactionID  string    `json:"transactionId"`
	DonatedAt      time.Time `json:"donatedAt"`
	Method         string    `json:"method"`
	EntityName     string    `json:"entityName"`
	ReceiptNumber  string    `json:"receiptNumber"`
	GeneratedAt    time.Time `json:"generatedAt"`
}

// DonationListResponse represents paginated donation list response
type DonationListResponse struct {
	Data       []DonationWithUser `json:"data"`
	Total      int                `json:"total"`
	Page       int                `json:"page"`
	Limit      int                `json:"limit"`
	TotalPages int                `json:"total_pages"`
	Success    bool               `json:"success"`
}

// RecentDonation represents recent donation info - FIXED FOR USER-SPECIFIC DATA
type RecentDonation struct {
	Amount       float64   `json:"amount" db:"amount"`
	DonationType string    `json:"donation_type" db:"donation_type"`
	Method       string    `json:"method" db:"method"`
	Status       string    `json:"status" db:"status"`
	DonatedAt    time.Time `json:"donated_at" db:"donated_at"`
	UserName     string    `json:"user_name" db:"user_name"`      // FIXED: Include user info
	EntityName   string    `json:"entity_name" db:"entity_name"`  // FIXED: Include entity info
}