package eventrsvp

import "time"

const (
	RSVPStatusAttending    = "attending"
	RSVPStatusMaybe        = "maybe"
	RSVPStatusNotAttending = "not_attending"
)

// RSVP represents a user's response to an event invitation
type RSVP struct {
	ID       uint      `gorm:"primaryKey" json:"id"`
	EventID  uint      `gorm:"not null;index:idx_event_user,unique" json:"event_id"` // Composite Unique Index
	UserID   uint      `gorm:"not null;index:idx_event_user,unique" json:"user_id"`  // Composite Unique Index
	Status   string    `gorm:"type:varchar(20);default:'attending'" json:"status"`   // Controlled via code, not enum
	Notes    string    `gorm:"type:text" json:"notes,omitempty"`                     // Optional Notes
	RSVPDate time.Time `gorm:"autoCreateTime" json:"rsvp_date"`                      // Auto-filled timestamp
}
