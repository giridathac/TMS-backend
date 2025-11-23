
package entity

import (
	"time"
)

type Entity struct {
	ID uint `gorm:"primaryKey" json:"id"`

	// Step 1: Temple Basic Information
	Name            string  `gorm:"not null" json:"name"`
	MainDeity       *string `json:"main_deity"`
	TempleType      string  `gorm:"not null" json:"temple_type"`
	EstablishedYear *uint   `json:"established_year"`
	Email           string  `gorm:"unique;not null" json:"email"`
	Phone           string  `gorm:"not null" json:"phone"`
	Description     string  `json:"description"`

	// Step 2: Address Information
	StreetAddress string `gorm:"not null" json:"street_address"`
	Landmark      string `json:"landmark"`
	City          string `gorm:"not null" json:"city"`
	District      string `gorm:"not null" json:"district"`
	State         string `gorm:"not null" json:"state"`
	Pincode       string `gorm:"not null" json:"pincode"`
	MapLink       string `json:"map_link"`

	// Step 3: Document Uploads (URLs to stored files)
	RegistrationCertURL string `json:"registration_cert_url"`
	TrustDeedURL        string `json:"trust_deed_url"`
	PropertyDocsURL     string `json:"property_docs_url"`
	AdditionalDocsURLs  string `json:"additional_docs_urls"` // JSON string of array

	// File metadata (new fields to store file information)
	RegistrationCertInfo string `json:"registration_cert_info"` // JSON metadata
	TrustDeedInfo        string `json:"trust_deed_info"`        // JSON metadata
	PropertyDocsInfo     string `json:"property_docs_info"`     // JSON metadata
	AdditionalDocsInfo   string `json:"additional_docs_info"`   // JSON metadata

	// Terms and Verification
	AcceptedTerms bool   `gorm:"default:false" json:"accepted_terms"`
	Status        string `gorm:"default:'pending'" json:"status"`
	CreatedBy     uint   `gorm:"not null" json:"created_by"`
	
	// 🆕 NEW FIELD: Track the role_id of the user who created this temple
	// This is used for auto-approval logic (role_id = 1 for superadmin)
	CreatorRoleID *uint  `json:"creator_role_id" gorm:"index"` // Role ID of creator (1=superadmin for auto-approval)

	// 🆕 NEW FIELD: Active/Inactive status
	IsActive bool `gorm:"default:true" json:"isactive"` // Active/Inactive toggle
	ApprovedAt      *time.Time `json:"approved_at" gorm:"column:approved_at"`
    RejectedAt      *time.Time `json:"rejected_at" gorm:"column:rejected_at"`
    RejectionReason string     `json:"rejection_reason" gorm:"column:rejection_reason;type:text"`
    

	// Meta
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

// FileInfo represents metadata about an uploaded file
type FileInfo struct {
	FileName     string    `json:"file_name"`
	FileURL      string    `json:"file_url"`
	FileSize     int64     `json:"file_size"`
	FileType     string    `json:"file_type"`
	UploadedAt   time.Time `json:"uploaded_at"`
	OriginalName string    `json:"original_name"`
}

/*
type UserEntityMembership struct {
	ID       uint      `gorm:"primaryKey"`
	UserID   uint      `gorm:"not null;index" json:"user_id"`
	EntityID uint      `gorm:"not null;index" json:"entity_id"`
	JoinedAt time.Time `gorm:"autoCreateTime" json:"joined_at"`
	Status   string    `gorm:"default:'active'" json:"status"`
	CreatedAt time.Time `json:"created_at"`
}*/
// TableName specifies the table name for the Entity model
func (Entity) TableName() string {
	return "entities"
}