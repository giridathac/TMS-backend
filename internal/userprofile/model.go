package userprofile

import (
	"time"

	"gorm.io/gorm"
)

// ============================
// 🔷 Devotee Profile Model
type DevoteeProfile struct {
	ID                         uint           `gorm:"primaryKey" json:"id"`
	UserID                     uint           `gorm:"not null;index" json:"user_id"`     // Injected from token
	EntityID                   uint           `gorm:"not null;index" json:"entity_id"`   // Injected from token

	// SECTION 1: Personal Details
	FullName                   *string        `json:"full_name,omitempty"`
	DOB                        *time.Time     `json:"dob,omitempty"`
	Gender                     *string        `json:"gender,omitempty"`
	StreetAddress              *string        `json:"street_address,omitempty"`
	City                       *string        `json:"city,omitempty"`
	State                      *string        `json:"state,omitempty"`
	Pincode                    *string        `json:"pincode,omitempty"`
	Country                    *string        `json:"country,omitempty"`

	// SECTION 2: Spiritual Info
	Gotra                      *string        `json:"gotra,omitempty"`
	Nakshatra                  *string        `json:"nakshatra,omitempty"`
	Rashi                      *string        `json:"rashi,omitempty"`
	Lagna                      *string        `json:"lagna,omitempty"`
	VedaShaka                  *string        `json:"veda_shaka,omitempty"`

	// SECTION 3: Family Lineage
	FatherName                 *string        `json:"father_name,omitempty"`
	FatherGotra                *string        `json:"father_gotra,omitempty"`
	FatherNativePlace          *string        `json:"father_native_place,omitempty"`
	FatherVedaShaka            *string        `json:"father_veda_shaka,omitempty"`

	MotherName                 *string        `json:"mother_name,omitempty"`
	MaidenGotra                *string        `json:"maiden_gotra,omitempty"`
	MotherNativePlace          *string        `json:"mother_native_place,omitempty"`
	MaternalGrandfatherName    *string        `json:"maternal_grandfather_name,omitempty"`

	PaternalGrandfatherName    *string        `json:"paternal_grandfather_name,omitempty"`
	PaternalGrandmotherName    *string        `json:"paternal_grandmother_name,omitempty"`
	MaternalGrandmotherName    *string        `json:"maternal_grandmother_name,omitempty"`

	// SECTION 4: Seva Preferences
	SevaAbhisheka              *bool          `json:"seva_abhisheka,omitempty"`
	SevaArti                   *bool          `json:"seva_arti,omitempty"`
	SevaAnnadana               *bool          `json:"seva_annadana,omitempty"`
	SevaArchana                *bool          `json:"seva_archana,omitempty"`
	SevaKalyanam               *bool          `json:"seva_kalyanam,omitempty"`
	SevaHomam                  *bool          `json:"seva_homam,omitempty"`

	DonateTempleMaintenance    *bool          `json:"donate_temple_maintenance,omitempty"`
	DonateAnnadanaProgram      *bool          `json:"donate_annadana_program,omitempty"`
	DonateFestivalCelebrations *bool          `json:"donate_festival_celebrations,omitempty"`
	DonateReligiousEducation   *bool          `json:"donate_religious_education,omitempty"`
	DonateTempleConstruction   *bool          `json:"donate_temple_construction,omitempty"`
	DonateGeneral              *bool          `json:"donate_general,omitempty"`

	SpecialInterestsOrNotes    *string        `json:"special_interests_or_notes,omitempty"`

	// SECTION 5: Family Members
	SpouseName                 *string        `json:"spouse_name,omitempty"`
	SpouseEmail                *string        `json:"spouse_email,omitempty"`
	SpousePhone                *string        `json:"spouse_phone,omitempty"`
	SpouseDOB                  *time.Time     `json:"spouse_dob,omitempty"`
	SpouseGotra                *string        `json:"spouse_gotra,omitempty"`
	SpouseNakshatra            *string        `json:"spouse_nakshatra,omitempty"`

	Children                   []*Child       `gorm:"foreignKey:ProfileID" json:"children,omitempty"`
	EmergencyContacts          []*EmergencyContact `gorm:"foreignKey:ProfileID" json:"emergency_contacts,omitempty"`

	// SECTION 6: Health, Sankalpa & Notes
	HealthNotes                *string        `json:"health_notes,omitempty"`
	AllergiesOrConditions      *string        `json:"allergies_or_conditions,omitempty"`
	DietaryRestrictions        *string        `json:"dietary_restrictions,omitempty"`

	PersonalSankalpa           *string        `json:"personal_sankalpa,omitempty"`
	AdditionalNotes            *string        `json:"additional_notes,omitempty"`

	// Profile Completion
	ProfileCompletionPercentage int           `json:"profile_completion_percentage"`

	CreatedAt                  time.Time      `json:"created_at"`
	UpdatedAt                  time.Time      `json:"updated_at"`
	DeletedAt                  gorm.DeletedAt `gorm:"index" json:"-"`
}

// ============================
// 🔷 Children Model
type Child struct {
	ID              uint       `gorm:"primaryKey" json:"id"`
	ProfileID       uint       `gorm:"not null;index" json:"-"`
	ChildName       *string    `json:"child_name,omitempty"`
	ChildDOB        *time.Time `json:"child_dob,omitempty"`
	ChildGender     *string    `json:"child_gender,omitempty"`
	ChildEducation  *string    `json:"child_education,omitempty"`
	ChildInterests  *string    `json:"child_interests,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

// ============================
// 🔷 Emergency Contact Model
type EmergencyContact struct {
	ID                  uint       `gorm:"primaryKey" json:"id"`
	ProfileID           uint       `gorm:"not null;index" json:"-"`
	ContactName         *string    `json:"contact_name,omitempty"`
	ContactRelationship *string    `json:"contact_relationship,omitempty"`
	ContactPhone        *string    `json:"contact_phone,omitempty"`
	ContactAddress      *string    `json:"contact_address,omitempty"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
}

// ==========================
// 🔗 Membership Mapping Table: Temple Join Logic
type UserEntityMembership struct {
	ID        uint      `gorm:"primaryKey"`
	UserID    uint      `gorm:"not null;index" json:"user_id"`
	EntityID  uint      `gorm:"not null;index" json:"entity_id"`
	JoinedAt  time.Time `gorm:"autoCreateTime" json:"joined_at"`
	Status    string    `gorm:"default:'active'" json:"status"`
	CreatedAt time.Time `json:"created_at"`
}
