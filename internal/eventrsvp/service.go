package eventrsvp

import (
	"errors"

	"github.com/sharath018/temple-management-backend/internal/event"
)

// Service handles business logic related to RSVPs
type Service struct {
	Repo         *Repository
	EventService *event.Service
}

// NewService initializes the RSVP service with repository and event dependency
func NewService(repo *Repository, eventService *event.Service) *Service {
	return &Service{
		Repo:         repo,
		EventService: eventService,
	}
}

// ✅ CreateRSVP adds a new RSVP entry (after validation)
func (s *Service) CreateRSVP(rsvp *RSVP) error {
	// Validate Status before inserting (cheap early check)
	if rsvp.Status != "attending" && rsvp.Status != "maybe" && rsvp.Status != "not_attending" {
		return errors.New("invalid RSVP status")
	}

	return s.Repo.CreateRSVP(rsvp)
}

// 📦 GetMyRSVPs fetches RSVPs made by the current user
func (s *Service) GetMyRSVPs(userID uint) ([]RSVP, error) {
	return s.Repo.GetMyRSVPs(userID)
}

// 📦 GetRSVPsByEvent returns all RSVPs for an event
func (s *Service) GetRSVPsByEvent(eventID uint) ([]RSVP, error) {
	return s.Repo.GetRSVPsByEvent(eventID)
}

// 🔁 UpdateRSVPStatus updates RSVP status if it already exists
func (s *Service) UpdateRSVPStatus(eventID, userID uint, status, notes string) error {
	// Validate Status before updating
	if status != "attending" && status != "maybe" && status != "not_attending" {
		return errors.New("invalid RSVP status")
	}
	return s.Repo.UpdateRSVPStatus(eventID, userID, status, notes)
}
