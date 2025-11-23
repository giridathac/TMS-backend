package eventrsvp

import (
	"errors"

	"gorm.io/gorm"
)

// Repository encapsulates database operations for RSVP
type Repository struct {
	DB *gorm.DB
}

// NewRepository returns a new instance of the RSVP repository
func NewRepository(db *gorm.DB) *Repository {
	return &Repository{DB: db}
}

// ✅ CreateRSVP inserts a new RSVP into the DB
func (r *Repository) CreateRSVP(rsvp *RSVP) error {
	return r.DB.Create(rsvp).Error
}

// ✅ GetMyRSVPs lists all RSVPs made by a specific user
func (r *Repository) GetMyRSVPs(userID uint) ([]RSVP, error) {
	var rsvps []RSVP
	err := r.DB.
		Where("user_id = ?", userID).
		Order("rsvp_date DESC").
		Find(&rsvps).Error
	return rsvps, err
}

// ✅ GetRSVPsByEvent lists all RSVPs for a given event (temple admin use)
func (r *Repository) GetRSVPsByEvent(eventID uint) ([]RSVP, error) {
	var rsvps []RSVP
	err := r.DB.
		Where("event_id = ?", eventID).
		Order("rsvp_date ASC").
		Find(&rsvps).Error
	return rsvps, err
}

// ✅ UpdateRSVPStatus updates an existing RSVP record
func (r *Repository) UpdateRSVPStatus(eventID, userID uint, status, notes string) error {
	result := r.DB.Model(&RSVP{}).
		Where("event_id = ? AND user_id = ?", eventID, userID).
		Updates(map[string]interface{}{
			"status": status,
			"notes":  notes,
		})

	if result.RowsAffected == 0 {
		return errors.New("no existing RSVP found for update")
	}
	return result.Error
}
