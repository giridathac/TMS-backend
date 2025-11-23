package reports

import (
	"time"
)

// Add to existing constants
const (
	// Existing report types
	ReportTypeEvents   = "events"
	ReportTypeSevas    = "sevas"
	ReportTypeBookings = "bookings"

	// New donation report type
	ReportTypeDonations = "donations"

	// Date range constants
	DateRangeDaily   = "daily"
	DateRangeWeekly  = "weekly"
	DateRangeMonthly = "monthly"
	DateRangeYearly  = "yearly"
	DateRangeCustom  = "custom"

	// Report format constants
	FormatCSV   = "csv"
	FormatExcel = "excel"
	FormatPDF   = "pdf"

	// Temple registered report types
	ReportTypeTempleRegistered      = "temple-registered"
	ReportTypeTempleRegisteredExcel = "temple-registered-excel"
	ReportTypeTempleRegisteredPDF   = "temple-registered-pdf"

	// Devotee birthdays report types
	ReportTypeDevoteeBirthdays      = "devotee-birthdays"
	ReportTypeDevoteeBirthdaysExcel = "devotee-birthdays-excel"
	ReportTypeDevoteeBirthdaysPDF   = "devotee-birthdays-pdf"

	// Donation report types
	ReportTypeDonationsExcel = "donations-excel"
	ReportTypeDonationsPDF   = "donations-pdf"

	// Devotee list report types
	ReportTypeDevoteeList      = "devotee-list"
	ReportTypeDevoteeListCSV   = "devotee-list-csv"
	ReportTypeDevoteeListExcel = "devotee-list-excel"
	ReportTypeDevoteeListPDF   = "devotee-list-pdf"

	// Devotee profile report types
	ReportTypeDevoteeProfile        = "devotee-profile"
	ReportTypeDevoteeProfileCSV     = "devotee-profile-csv"
	ReportTypeDevoteeProfileExcel   = "devotee-profile-excel"
	ReportTypeDevoteeProfilePDF     = "devotee-profile-pdf"
	ReportTypeDevoteeProfilePDF_ext = "devotee-profile_ext-pdf"

	// Audit log report types
	ReportTypeAuditLogs      = "audit-logs"
	ReportTypeAuditLogsCSV   = "audit-logs-csv"
	ReportTypeAuditLogsExcel = "audit-logs-excel"
	ReportTypeAuditLogsPDF   = "audit-logs-pdf"

	// Approval Status report types
	ReportTypeApprovalStatus      = "approval-status"
	ReportTypeApprovalStatusCSV   = "approval-status-csv"
	ReportTypeApprovalStatusExcel = "approval-status-excel"
	ReportTypeApprovalStatusPDF   = "approval-status-pdf"

	// User Detail report types
	ReportTypeUserDetails      = "user-details"
	ReportTypeUserDetailsCSV   = "user-details-csv"
	ReportTypeUserDetailsExcel = "user-details-excel"
	ReportTypeUserDetailsPDF   = "user-details-pdf"
)

// ActivitiesReportRequest represents request parameters for temple activities report
type ActivitiesReportRequest struct {
	EntityID  string    `json:"entity_id"`
	EntityIDs []string  `json:"entity_ids"`
	Type      string    `json:"type"`
	DateRange string    `json:"date_range"`
	StartDate time.Time `json:"start_date"`
	EndDate   time.Time `json:"end_date"`
	Format    string    `json:"format"`
}

// ReportData struct with all report types
type ReportData struct {
	Events              []EventReportRow              `json:"events,omitempty"`
	Sevas               []SevaReportRow               `json:"sevas,omitempty"`
	Bookings            []SevaBookingReportRow        `json:"bookings,omitempty"`
	Donations           []DonationReportRow           `json:"donations,omitempty"`
	TemplesRegistered   []TempleRegisteredReportRow   `json:"temples_registered,omitempty"`
	DevoteeBirthdays    []DevoteeBirthdayReportRow    `json:"devotee_birthdays,omitempty"`
	DevoteeList         []DevoteeListReportRow        `json:"devotee_list,omitempty"`
	DevoteeProfiles     []DevoteeProfileReportRow     `json:"devotee_profiles,omitempty"`
	DevoteeProfiles_ext []DevoteeProfileReportRow_ext `json:"devotee_profiles_ext,omitempty"`
	AuditLogs           []AuditLogReportRow           `json:"audit_logs,omitempty"`
	UserDetails         []UserDetailsReportRow        `json:"user_details,omitempty"`
	ApprovalStatus      []ApprovalStatusReportRow     `json:"approval_status,omitempty"`
}

// EventReportRow represents a single row in the events report
type EventReportRow struct {
	Title       string    `json:"title"`
	TempleName  string    `json:"temple_name"`
	Description string    `json:"description"`
	EventType   string    `json:"event_type"`
	EventDate   time.Time `json:"event_date"`
	EventTime   string    `json:"event_time"`
	Location    string    `json:"location"`
	CreatedBy   uint      `json:"created_by"`
	IsActive    bool      `json:"is_active"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// SevaReportRow represents a single row in the sevas report
type SevaReportRow struct {
	Name              string    `json:"name"`
	TempleName        string    `json:"temple_name"`
	SevaType          string    `json:"seva_type"`
	Description       string    `json:"description"`
	Price             float64   `json:"price"`
	Date              time.Time `json:"date"`
	StartTime         string    `json:"start_time"`
	EndTime           string    `json:"end_time"`
	Duration          int       `json:"duration"`
	MaxBookingsPerDay int       `json:"max_bookings_per_day"`
	Status            string    `json:"status"`
	IsActive          bool      `json:"is_active"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// SevaBookingReportRow represents a single row in the seva bookings report
type SevaBookingReportRow struct {
	SevaName     string    `json:"seva_name"`
	TempleName   string    `json:"temple_name"`
	SevaType     string    `json:"seva_type"`
	DevoteeName  string    `json:"devotee_name"`
	DevoteePhone string    `json:"devotee_phone"`
	BookingTime  time.Time `json:"booking_time"`
	Status       string    `json:"status"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// DonationReportRow represents a single row in the donations report
type DonationReportRow struct {
	ID            uint      `json:"id"`
	DonorName     string    `json:"donor_name"`
	TempleName    string    `json:"temple_name"`
	DonorEmail    string    `json:"donor_email"`
	Amount        float64   `json:"amount"`
	DonationType  string    `json:"donation_type"`
	PaymentMethod string    `json:"payment_method"`
	Status        string    `json:"status"`
	DonationDate  time.Time `json:"donation_date"`
	OrderID       string    `json:"order_id"`
	PaymentID     *string   `json:"payment_id"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// TempleRegisteredReportRequest represents request parameters for temple registered report
type TempleRegisteredReportRequest struct {
	EntityID  string    `json:"entity_id"`
	Status    string    `json:"status"`
	DateRange string    `json:"date_range"`
	StartDate time.Time `json:"start_date"`
	EndDate   time.Time `json:"end_date"`
	Format    string    `json:"format"`
}

// TempleRegisteredReportRow represents a single row in the temples registered report
type TempleRegisteredReportRow struct {
	ID        uint      `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
	Status    string    `json:"status"`
}

// DevoteeBirthdaysReportRequest represents request parameters for devotee birthdays report
type DevoteeBirthdaysReportRequest struct {
	EntityID  string    `json:"entity_id"`
	DateRange string    `json:"date_range"`
	StartDate time.Time `json:"start_date"`
	EndDate   time.Time `json:"end_date"`
	Format    string    `json:"format"`
}

// DevoteeBirthdayReportRow represents a single row in the devotee birthdays report
type DevoteeBirthdayReportRow struct {
	FullName    string    `json:"full_name"`
	DateOfBirth time.Time `json:"date_of_birth"`
	Gender      string    `json:"gender"`
	Phone       string    `json:"phone"`
	Email       string    `json:"email"`
	TempleName  string    `json:"temple_name"`
	MemberSince time.Time `json:"member_since"`
}

// DevoteeListReportRequest represents request parameters for devotee list report
type DevoteeListReportRequest struct {
	EntityID  string    `json:"entity_id"`
	DateRange string    `json:"date_range"`
	StartDate time.Time `json:"start_date"`
	EndDate   time.Time `json:"end_date"`
	Status    string    `json:"status"` // active, inactive, blocked, etc.
	Format    string    `json:"format"`
}

// DevoteeListReportRow represents a single row in the devotee list report
type DevoteeListReportRow struct {
	UserID        string    `json:"user_id"`
	DevoteeName   string    `json:"devotee_name"`
	TempleName    string    `json:"temple_name"`
	JoinedAt      time.Time `json:"joined_at"`
	DevoteeStatus string    `json:"devotee_status"`
	CreatedAt     time.Time `json:"created_at"`
}

// DevoteeProfileReportRequest represents request parameters for devotee profile report
type DevoteeProfileReportRequest struct {
	EntityID  string    `json:"entity_id"`
	DateRange string    `json:"date_range"`
	StartDate time.Time `json:"start_date"`
	EndDate   time.Time `json:"end_date"`
	Status    string    `json:"status"` // active, inactive, blocked, etc.
	Format    string    `json:"format"`
}

// DevoteeProfileReportRow represents a single row in the devotee profile report
type DevoteeProfileReportRow struct {
	UserID      string    `json:"user_id"`
	FullName    string    `json:"full_name"`
	TempleName  string    `json:"temple_name"`
	DOB         time.Time `json:"dob"`
	Gender      string    `json:"gender"`
	FullAddress string    `json:"full_address"`
	Gotra       string    `json:"gotra"`
	Nakshatra   string    `json:"nakshatra"`
	Rashi       string    `json:"rashi"`
	Lagna       string    `json:"lagna"`
}

// DevoteeProfileReportRow_ext represents an extended row with temple name
type DevoteeProfileReportRow_ext struct {
	UserID      string    `json:"user_id"`
	FullName    string    `json:"full_name"`
	TempleName  string    `json:"temple_name"`
	DOB         time.Time `json:"dob"`
	Gender      string    `json:"gender"`
	FullAddress string    `json:"full_address"`
	Gotra       string    `json:"gotra"`
	Nakshatra   string    `json:"nakshatra"`
	Rashi       string    `json:"rashi"`
	Lagna       string    `json:"lagna"`
}

// AuditLogReportRequest represents request parameters for audit logs report
type AuditLogReportRequest struct {
	EntityID  string    `json:"entity_id"`
	Action    string    `json:"action"`
	Status    string    `json:"status"`
	DateRange string    `json:"date_range"`
	StartDate time.Time `json:"start_date"`
	EndDate   time.Time `json:"end_date"`
	Format    string    `json:"format"`
}

type AuditLogReportRow struct {
	ID         uint      `json:"id"`
	UserID     *uint     `json:"user_id"`
	UserName   string    `json:"user_name"`
	UserRole   string    `json:"user_role"`
	EntityID   uint      `json:"entity_id"`
	EntityName string    `json:"entity_name"`
	Action     string    `json:"action"`
	Status     string    `json:"status"`
	IPAddress  string    `json:"ip_address"`
	Timestamp  time.Time `json:"timestamp"`
	Details    string    `json:"details"`
	CreatedAt  time.Time `json:"created_at"`
}

// ApprovalStatusReportRequest defines filters for approval status report
type ApprovalStatusReportRequest struct {
	Role      string    // "tenantadmin" or "templeadmin" or empty (both)
	Status    string    // "approved", "rejected", "pending"
	DateRange string    // "weekly", "monthly", etc.
	StartDate time.Time
	EndDate   time.Time
	Format    string // "excel", "csv", "pdf", or empty for JSON
	UserID    uint   // Current user requesting the report
}

// ApprovalStatusReportRow represents a single row in the approval status report
type ApprovalStatusReportRow struct {
	TenantID   string     `json:"tenant_id"`
	TenantName string     `json:"tenant_name"`
	EntityID   string     `json:"entity_id"`
	EntityName string     `json:"entity_name"`
	Status     string     `json:"status"`
	CreatedAt  time.Time  `json:"created_at"`
	ApprovedAt *time.Time `json:"approved_at"`
	Email      string     `json:"email"`
	//Role       string     `json:"role"` // "tenantadmin" or "templeadmin"
}
type UserDetailsReportRow = UserDetailReportRow

type UserDetailReportRequest struct {
	EntityID  string    `json:"entity_id"`
	Role      string    `json:"role"`
	Status    string    `json:"status"`
	DateRange string    `json:"date_range"`
	StartDate time.Time `json:"start_date"`
	EndDate   time.Time `json:"end_date"`
	Format    string    `json:"format"`
	UserID    uint      `json:"user_id"`
}

type UserDetailReportRow struct {
	ID         uint      `json:"id"`
	Name       string    `json:"name"`
	EntityName string    `json:"entity_name"`
	Email      string    `json:"email"`
	Role       string    `json:"role"`
	Status     string    `json:"status"`
	CreatedAt  time.Time `json:"created_at"`
}