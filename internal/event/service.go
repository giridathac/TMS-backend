package event

import (
	"context"
	"errors"
	"time"

	"github.com/sharath018/temple-management-backend/internal/auditlog"
	"github.com/sharath018/temple-management-backend/internal/notification"
	"github.com/sharath018/temple-management-backend/middleware"
)

// Service wraps business logic for temple events
type Service struct {
	Repo     *Repository
	AuditSvc auditlog.Service // Audit service for logging
	NotifSvc notification.Service
}

// NewService initializes a new Service with audit logging
func NewService(r *Repository, auditSvc auditlog.Service) *Service {
	return &Service{
		Repo:     r,
		AuditSvc: auditSvc,
	}
}

// ===========================
// 🎯 Create Event
func (s *Service) CreateEvent(req *CreateEventRequest, accessContext middleware.AccessContext, entityID uint, ip string) error {
	// Convert entityID to pointer for audit logging
	entityIDPtr := &entityID

	// Check write permissions
	if !accessContext.CanWrite() {
		s.AuditSvc.LogAction(
			context.Background(),
			&accessContext.UserID,
			entityIDPtr,
			"EVENT_CREATED",
			map[string]interface{}{
				"title":      req.Title,
				"event_type": req.EventType,
				"error":      "write access denied",
			},
			ip,
			"failure",
		)
		return errors.New("write access denied")
	}

	// 🔄 Parse EventDate
	eventDate, err := time.Parse("2006-01-02", req.EventDate)
	if err != nil {
		// Log failed event creation attempt
		s.AuditSvc.LogAction(
			context.Background(),
			&accessContext.UserID,
			entityIDPtr,
			"EVENT_CREATED",
			map[string]interface{}{
				"title":      req.Title,
				"event_type": req.EventType,
				"error":      "invalid event_date format",
				"event_date": req.EventDate,
			},
			ip,
			"failure",
		)
		return errors.New("invalid event_date format. Use YYYY-MM-DD")
	}

	// 🔄 Parse EventTime (optional)
	var eventTimePtr *time.Time
	if req.EventTime != "" {
		parsedTime, err := time.Parse("15:04", req.EventTime)
		if err != nil {
			// Log failed event creation attempt
			s.AuditSvc.LogAction(
				context.Background(),
				&accessContext.UserID,
				entityIDPtr,
				"EVENT_CREATED",
				map[string]interface{}{
					"title":      req.Title,
					"event_type": req.EventType,
					"error":      "invalid event_time format",
					"event_time": req.EventTime,
				},
				ip,
				"failure",
			)
			return errors.New("invalid event_time format. Use HH:MM in 24-hour format")
		}
		normalizedTime := time.Date(0, 1, 1, parsedTime.Hour(), parsedTime.Minute(), 0, 0, time.UTC)
		eventTimePtr = &normalizedTime
	}

	// 🛡 Handle optional IsActive safely
	isActive := true
	if req.IsActive != nil {
		isActive = *req.IsActive
	}

	event := &Event{
		Title:       req.Title,
		Description: req.Description,
		EventDate:   eventDate,
		EventTime:   eventTimePtr,
		Location:    req.Location,
		EventType:   req.EventType,
		IsActive:    isActive,
		CreatedBy:   accessContext.UserID,
		EntityID:    entityID, // Use the passed entityID directly
	}

	// Attempt to create event in database
	err = s.Repo.CreateEvent(event)
	if err != nil {
		// Log failed event creation
		s.AuditSvc.LogAction(
			context.Background(),
			&accessContext.UserID,
			entityIDPtr,
			"EVENT_CREATED",
			map[string]interface{}{
				"title":      req.Title,
				"event_type": req.EventType,
				"event_date": req.EventDate,
				"location":   req.Location,
				"error":      err.Error(),
			},
			ip,
			"failure",
		)
		return err
	}

	// Log successful event creation
	s.AuditSvc.LogAction(
		context.Background(),
		&accessContext.UserID,
		entityIDPtr,
		"EVENT_CREATED",
		map[string]interface{}{
			"event_id":   event.ID,
			"title":      event.Title,
			"event_type": event.EventType,
			"event_date": event.EventDate.Format("2006-01-02"),
			"location":   event.Location,
			"is_active":  event.IsActive,
			"entity_id":  entityID, // Add entity_id to audit log for verification
		},
		ip,
		"success",
	)

	// In-app notifications to devotees & volunteers of the entity
	if s.NotifSvc != nil {
		_ = s.NotifSvc.CreateInAppForEntityRoles(context.Background(), entityID,
			[]string{"devotee", "volunteer"},
			"New Event",
			req.Title+" on "+eventDate.Format("2006-01-02"),
			"event",
		)
	}

	return nil
}

// ===========================
// 🔍 Get Event by ID - FIXED with proper entity validation
func (s *Service) GetEventByID(id uint, accessContext middleware.AccessContext) (*Event, error) {
	// Get accessible entity ID
	entityID := accessContext.GetAccessibleEntityID()
	if entityID == nil {
		return nil, errors.New("no accessible entity")
	}

	// Check read permissions
	if !accessContext.CanRead() {
		return nil, errors.New("read access denied")
	}

	event, err := s.Repo.GetEventByID(id)
	if err != nil {
		return nil, err
	}

	// Verify the event belongs to the accessible entity
	if event.EntityID != *entityID {
		return nil, errors.New("event not found")
	}

	// Get RSVP count using the entity-specific method
	count, _ := s.Repo.CountRSVPsByEntity(event.ID, *entityID)
	event.RSVPCount = count

	return event, nil
}

// ===========================
// 📆 Get Upcoming Events - FIXED with proper entity filtering
func (s *Service) GetUpcomingEvents(accessContext middleware.AccessContext) ([]Event, error) {
	// Get accessible entity ID
	entityID := accessContext.GetAccessibleEntityID()
	if entityID == nil {
		return nil, errors.New("no accessible entity")
	}

	// Check read permissions - We need to ensure devotees have access
	if !accessContext.CanRead() && accessContext.RoleName != "devotee" && accessContext.RoleName != "volunteer" {
		return nil, errors.New("read access denied")
	}

	// Get upcoming events for the specific entity
	events, err := s.Repo.GetUpcomingEvents(*entityID)
	if err != nil {
		return nil, err
	}

	// Ensure RSVP counts are calculated correctly for each event
	for i := range events {
		count, _ := s.Repo.CountRSVPsByEntity(events[i].ID, *entityID)
		events[i].RSVPCount = count
	}

	return events, nil
}

// ===========================
// 📄 List Events with Pagination - FIXED with proper entity filtering
func (s *Service) ListEventsByEntity(accessContext middleware.AccessContext, limit, offset int, search string) ([]Event, error) {
	// Get accessible entity ID
	entityID := accessContext.GetAccessibleEntityID()
	if entityID == nil {
		return nil, errors.New("no accessible entity")
	}

	// Allow devotees and volunteers to see events without explicit read permission
	if !(accessContext.RoleName == "devotee" || accessContext.RoleName == "volunteer") && !accessContext.CanRead() {
		return nil, errors.New("read access denied")
	}

	events, err := s.Repo.ListEventsByEntity(*entityID, limit, offset, search)
	if err != nil {
		return nil, err
	}

	// Ensure RSVP counts are calculated correctly for the specific entity
	for i := range events {
		count, _ := s.Repo.CountRSVPsByEntity(events[i].ID, *entityID)
		events[i].RSVPCount = count
	}

	return events, nil
}

// ===========================
// 📄 List Events with Pagination by Explicit Entity ID - FIXED
func (s *Service) ListEventsByEntityID(accessContext middleware.AccessContext, entityID uint, limit, offset int, search string) ([]Event, error) {
	// Check permissions
	if !(accessContext.RoleName == "devotee" || accessContext.RoleName == "volunteer") && !accessContext.CanRead() {
		return nil, errors.New("read access denied")
	}

	// Pass the explicit entityID to the repository
	events, err := s.Repo.ListEventsByEntity(entityID, limit, offset, search)
	if err != nil {
		return nil, err
	}

	// Ensure RSVP counts are calculated correctly for the specific entity
	for i := range events {
		count, _ := s.Repo.CountRSVPsByEntity(events[i].ID, entityID)
		events[i].RSVPCount = count
	}

	return events, nil
}

// ===========================
// 📊 Dashboard Stats - FIXED with proper entity filtering
func (s *Service) GetEventStats(accessContext middleware.AccessContext) (*EventStatsResponse, error) {
	// Get accessible entity ID from access context
	entityID := accessContext.GetAccessibleEntityID()
	if entityID == nil {
		return nil, errors.New("no accessible entity")
	}

	// Check read permissions
	if !accessContext.CanRead() {
		return nil, errors.New("read access denied")
	}

	// This will now use the corrected repository method with proper entity filtering
	return s.Repo.GetEventStats(*entityID)
}

// ===========================
// 📆 Get Upcoming Events by Explicit Entity ID - FIXED
func (s *Service) GetUpcomingEventsByEntityID(accessContext middleware.AccessContext, entityID uint) ([]Event, error) {
	// Check permissions
	if !(accessContext.RoleName == "devotee" || accessContext.RoleName == "volunteer") && !accessContext.CanRead() {
		return nil, errors.New("read access denied")
	}

	// Pass the explicit entityID to the repository
	events, err := s.Repo.GetUpcomingEvents(entityID)
	if err != nil {
		return nil, err
	}

	// Ensure RSVP counts are calculated correctly for the specific entity
	for i := range events {
		count, _ := s.Repo.CountRSVPsByEntity(events[i].ID, entityID)
		events[i].RSVPCount = count
	}

	return events, nil
}

// ===========================
// 🛠 Update Event (with ownership check and audit logging) - FIXED
func (s *Service) UpdateEvent(id uint, req *UpdateEventRequest, accessContext middleware.AccessContext, ip string) error {
	// Get accessible entity ID
	entityID := accessContext.GetAccessibleEntityID()
	if entityID == nil {
		return errors.New("no accessible entity")
	}

	// Check write permissions
	if !accessContext.CanWrite() {
		s.AuditSvc.LogAction(
			context.Background(),
			&accessContext.UserID,
			entityID,
			"EVENT_UPDATED",
			map[string]interface{}{
				"event_id": id,
				"error":    "write access denied",
			},
			ip,
			"failure",
		)
		return errors.New("write access denied")
	}

	// First check if event exists and user has permission
	event, err := s.Repo.GetEventByID(id)
	if err != nil {
		// Log failed update attempt - event not found
		s.AuditSvc.LogAction(
			context.Background(),
			&accessContext.UserID,
			entityID,
			"EVENT_UPDATED",
			map[string]interface{}{
				"event_id": id,
				"error":    "event not found",
			},
			ip,
			"failure",
		)
		return err
	}

	if event.EntityID != *entityID {
		// Log unauthorized update attempt
		s.AuditSvc.LogAction(
			context.Background(),
			&accessContext.UserID,
			entityID,
			"EVENT_UPDATED",
			map[string]interface{}{
				"event_id":     id,
				"event_title":  event.Title,
				"error":        "unauthorized access",
				"event_entity": event.EntityID,
			},
			ip,
			"failure",
		)
		return errors.New("unauthorized: cannot update this event")
	}

	// Store original values for audit log
	originalTitle := event.Title
	originalEventType := event.EventType
	originalEventDate := event.EventDate.Format("2006-01-02")
	originalLocation := event.Location
	originalIsActive := event.IsActive

	// 🔄 Parse and update EventDate
	eventDate, err := time.Parse("2006-01-02", req.EventDate)
	if err != nil {
		// Log failed update due to invalid date
		s.AuditSvc.LogAction(
			context.Background(),
			&accessContext.UserID,
			entityID,
			"EVENT_UPDATED",
			map[string]interface{}{
				"event_id":     id,
				"event_title":  event.Title,
				"error":        "invalid event_date format",
				"invalid_date": req.EventDate,
			},
			ip,
			"failure",
		)
		return errors.New("invalid event_date format. Use YYYY-MM-DD")
	}
	event.EventDate = eventDate

	// 🔄 Parse and update EventTime (or nil)
	if req.EventTime != "" {
		parsedTime, err := time.Parse("15:04", req.EventTime)
		if err != nil {
			// Log failed update due to invalid time
			s.AuditSvc.LogAction(
				context.Background(),
				&accessContext.UserID,
				entityID,
				"EVENT_UPDATED",
				map[string]interface{}{
					"event_id":     id,
					"event_title":  event.Title,
					"error":        "invalid event_time format",
					"invalid_time": req.EventTime,
				},
				ip,
				"failure",
			)
			return errors.New("invalid event_time format. Use HH:MM in 24-hour format")
		}
		normalizedTime := time.Date(0, 1, 1, parsedTime.Hour(), parsedTime.Minute(), 0, 0, time.UTC)
		event.EventTime = &normalizedTime
	} else {
		event.EventTime = nil
	}

	// 🔄 Other fields
	event.Title = req.Title
	event.Description = req.Description
	event.Location = req.Location
	event.EventType = req.EventType
	if req.IsActive != nil {
		event.IsActive = *req.IsActive
	}

	// ✅ Now update using parsed `*Event`
	err = s.Repo.UpdateEvent(event)
	if err != nil {
		// Log failed update attempt
		s.AuditSvc.LogAction(
			context.Background(),
			&accessContext.UserID,
			entityID,
			"EVENT_UPDATED",
			map[string]interface{}{
				"event_id":    id,
				"event_title": originalTitle,
				"error":       err.Error(),
			},
			ip,
			"failure",
		)
		return err
	}

	// Log successful event update with changes
	changes := make(map[string]interface{})
	if originalTitle != event.Title {
		changes["title_changed"] = map[string]string{"from": originalTitle, "to": event.Title}
	}
	if originalEventType != event.EventType {
		changes["event_type_changed"] = map[string]string{"from": originalEventType, "to": event.EventType}
	}
	if originalEventDate != event.EventDate.Format("2006-01-02") {
		changes["event_date_changed"] = map[string]string{"from": originalEventDate, "to": event.EventDate.Format("2006-01-02")}
	}
	if originalLocation != event.Location {
		changes["location_changed"] = map[string]string{"from": originalLocation, "to": event.Location}
	}
	if originalIsActive != event.IsActive {
		changes["status_changed"] = map[string]bool{"from": originalIsActive, "to": event.IsActive}
	}

	s.AuditSvc.LogAction(
		context.Background(),
		&accessContext.UserID,
		entityID,
		"EVENT_UPDATED",
		map[string]interface{}{
			"event_id":    event.ID,
			"event_title": event.Title,
			"changes":     changes,
		},
		ip,
		"success",
	)

	if s.NotifSvc != nil {
		_ = s.NotifSvc.CreateInAppForEntityRoles(context.Background(), *entityID,
			[]string{"devotee", "volunteer"},
			"Event Updated",
			event.Title+" updated for "+event.EventDate.Format("2006-01-02"),
			"event",
		)
	}

	return nil
}

// ===========================
// ❌ Delete Event (with ownership check and audit logging) - FIXED
func (s *Service) DeleteEvent(id uint, accessContext middleware.AccessContext, ip string) error {
	// Get accessible entity ID
	entityID := accessContext.GetAccessibleEntityID()
	if entityID == nil {
		return errors.New("no accessible entity")
	}

	// Check write permissions
	if !accessContext.CanWrite() {
		s.AuditSvc.LogAction(
			context.Background(),
			&accessContext.UserID,
			entityID,
			"EVENT_DELETED",
			map[string]interface{}{
				"event_id": id,
				"error":    "write access denied",
			},
			ip,
			"failure",
		)
		return errors.New("write access denied")
	}

	// First check if event exists and user has permission
	event, err := s.Repo.GetEventByID(id)
	if err != nil {
		// Log failed delete attempt - event not found
		s.AuditSvc.LogAction(
			context.Background(),
			&accessContext.UserID,
			entityID,
			"EVENT_DELETED",
			map[string]interface{}{
				"event_id": id,
				"error":    "event not found",
			},
			ip,
			"failure",
		)
		return err
	}

	if event.EntityID != *entityID {
		// Log unauthorized delete attempt
		s.AuditSvc.LogAction(
			context.Background(),
			&accessContext.UserID,
			entityID,
			"EVENT_DELETED",
			map[string]interface{}{
				"event_id":     id,
				"event_title":  event.Title,
				"error":        "unauthorized access",
				"event_entity": event.EntityID,
			},
			ip,
			"failure",
		)
		return errors.New("unauthorized: cannot delete this event")
	}

	// Store event details for audit log before deletion
	eventTitle := event.Title
	eventType := event.EventType
	eventDate := event.EventDate.Format("2006-01-02")
	location := event.Location

	// Attempt to delete the event
	err = s.Repo.DeleteEvent(id, *entityID)
	if err != nil {
		// Log failed delete attempt
		s.AuditSvc.LogAction(
			context.Background(),
			&accessContext.UserID,
			entityID,
			"EVENT_DELETED",
			map[string]interface{}{
				"event_id":    id,
				"event_title": eventTitle,
				"error":       err.Error(),
			},
			ip,
			"failure",
		)
		return err
	}

	// Log successful event deletion
	s.AuditSvc.LogAction(
		context.Background(),
		&accessContext.UserID,
		entityID,
		"EVENT_DELETED",
		map[string]interface{}{
			"event_id":    id,
			"event_title": eventTitle,
			"event_type":  eventType,
			"event_date":  eventDate,
			"location":    location,
		},
		ip,
		"success",
	)

	if s.NotifSvc != nil {
		_ = s.NotifSvc.CreateInAppForEntityRoles(context.Background(), *entityID,
			[]string{"devotee", "volunteer"},
			"Event Deleted",
			eventTitle+" has been removed",
			"event",
		)
	}

	return nil
}
