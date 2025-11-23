package entity

import (
	"context"
	"errors"
	"log"
	"strings"
	"time"

	"github.com/sharath018/temple-management-backend/internal/auditlog"
	"github.com/sharath018/temple-management-backend/internal/auth"
)

type MembershipService interface {
	UpdateMembershipStatus(userID, entityID uint, status string) error
}

type Service struct {
	Repo              *Repository
	MembershipService MembershipService
	AuditService      auditlog.Service
}



func NewService(r *Repository, ms MembershipService, as auditlog.Service) *Service {
	return &Service{
		Repo:              r,
		MembershipService: ms,
		AuditService:      as,
	}
}

var (
	ErrMissingFields = errors.New("temple name, deity, phone, and email are required")
)

// ========== ENTITY CORE ==========

// CreateEntity - Create temple with auto-approval for superadmin (role_id = 1)
func (s *Service) CreateEntity(e *Entity, userID uint, userRoleID uint, ip string) error {
	// Validate required fields
	if strings.TrimSpace(e.Name) == "" ||
		e.MainDeity == nil || strings.TrimSpace(*e.MainDeity) == "" ||
		strings.TrimSpace(e.Phone) == "" ||
		strings.TrimSpace(e.Email) == "" {

		auditDetails := map[string]interface{}{
			"temple_name": strings.TrimSpace(e.Name),
			"email":       strings.TrimSpace(e.Email),
			"role_id":     userRoleID,
			"error":       "Missing required fields",
		}
		s.AuditService.LogAction(context.Background(), &userID, nil, "TEMPLE_CREATE_FAILED", auditDetails, ip, "failure")

		return ErrMissingFields
	}

	now := time.Now()

	// AUTO-APPROVE LOGIC: Check if creator is superadmin (role_id = 1)
	if e.Status == "" {
		if userRoleID == 1 {
			e.Status = "approved"
			log.Printf("🎉 Temple auto-approved: Created by superadmin (user_id: %d, role_id: %d)", userID, userRoleID)
		} else {
			e.Status = "pending"
			log.Printf("📝 Temple pending approval: Created by role_id: %d (user_id: %d)", userRoleID, userID)
		}
	}

	// Set metadata
	e.CreatedAt = now
	e.UpdatedAt = now
	e.CreatorRoleID = &userRoleID

	// Sanitize inputs
	e.Name = strings.TrimSpace(e.Name)
	e.Email = strings.TrimSpace(e.Email)
	e.Phone = strings.TrimSpace(e.Phone)
	e.TempleType = strings.TrimSpace(e.TempleType)
	e.Description = strings.TrimSpace(e.Description)
	e.StreetAddress = strings.TrimSpace(e.StreetAddress)
	e.City = strings.TrimSpace(e.City)
	e.State = strings.TrimSpace(e.State)
	e.District = strings.TrimSpace(e.District)
	e.Pincode = strings.TrimSpace(e.Pincode)

	if e.MainDeity != nil {
		trimmed := strings.TrimSpace(*e.MainDeity)
		e.MainDeity = &trimmed
	}

	e.RegistrationCertURL = strings.TrimSpace(e.RegistrationCertURL)
	e.TrustDeedURL = strings.TrimSpace(e.TrustDeedURL)
	e.PropertyDocsURL = strings.TrimSpace(e.PropertyDocsURL)
	e.AdditionalDocsURLs = strings.TrimSpace(e.AdditionalDocsURLs)

	// Save entity to database
	if err := s.Repo.CreateEntity(e); err != nil {
		auditDetails := map[string]interface{}{
			"temple_name": e.Name,
			"email":       e.Email,
			"role_id":     userRoleID,
			"error":       err.Error(),
		}
		s.AuditService.LogAction(context.Background(), &userID, nil, "TEMPLE_CREATE_FAILED", auditDetails, ip, "failure")

		return err
	}

	// ONLY create approval request if NOT auto-approved (i.e., status is pending)
	if e.Status == "pending" {
		req := &auth.ApprovalRequest{
			UserID:      userID,
			EntityID:    &e.ID,
			RequestType: "temple_approval",
			Status:      "pending",
			CreatedAt:   now,
			UpdatedAt:   now,
		}

		if err := s.Repo.CreateApprovalRequest(req); err != nil {
			auditDetails := map[string]interface{}{
				"temple_name": e.Name,
				"temple_id":   e.ID,
				"email":       e.Email,
				"role_id":     userRoleID,
				"error":       err.Error(),
			}
			s.AuditService.LogAction(context.Background(), &userID, &e.ID, "TEMPLE_APPROVAL_REQUEST_FAILED", auditDetails, ip, "failure")

			return err
		}

		log.Printf("✅ Approval request created for temple ID: %d", e.ID)
	} else {
		log.Printf("⚡ Skipped approval request - Temple auto-approved (ID: %d)", e.ID)
	}

	// Log successful temple creation with appropriate action type
	auditDetails := map[string]interface{}{
		"temple_name":   e.Name,
		"temple_id":     e.ID,
		"temple_type":   e.TempleType,
		"email":         e.Email,
		"phone":         e.Phone,
		"city":          e.City,
		"state":         e.State,
		"main_deity":    e.MainDeity,
		"status":        e.Status,
		"role_id":       userRoleID,
		"auto_approved": e.Status == "approved",
	}

	actionType := "TEMPLE_CREATED"
	if e.Status == "approved" {
		actionType = "TEMPLE_CREATED_AUTO_APPROVED"
	}

	s.AuditService.LogAction(context.Background(), &userID, &e.ID, actionType, auditDetails, ip, "success")

	return nil
}

// GetAllEntities - Super Admin → Get all temples
func (s *Service) GetAllEntities() ([]Entity, error) {
	return s.Repo.GetAllEntities()
}

// GetEntitiesByCreator - Temple Admin → Get entities created by specific user
func (s *Service) GetEntitiesByCreator(creatorID uint) ([]Entity, error) {
	return s.Repo.GetEntitiesByCreator(creatorID)
}

// GetEntityByID - Anyone → View a temple by ID
func (s *Service) GetEntityByID(id int) (Entity, error) {
	return s.Repo.GetEntityByID(id)
}

// UpdateEntity - Temple Admin → Update own temple
// UpdateEntity - Temple Admin → Update temple with re-approval logic for rejected temples
func (s *Service) UpdateEntity(e Entity, userID uint, userRoleID uint, ip string, wasRejected bool) error {
	existingEntity, err := s.Repo.GetEntityByID(int(e.ID))
	if err != nil {
		auditDetails := map[string]interface{}{
			"temple_id": e.ID,
			"error":     "Temple not found",
		}
		s.AuditService.LogAction(context.Background(), &userID, &e.ID, "TEMPLE_UPDATE_FAILED", auditDetails, ip, "failure")
		return err
	}

	e.UpdatedAt = time.Now()

	// Update the entity in database
	if err := s.Repo.UpdateEntity(e); err != nil {
		auditDetails := map[string]interface{}{
			"temple_id":   e.ID,
			"temple_name": e.Name,
			"error":       err.Error(),
		}
		s.AuditService.LogAction(context.Background(), &userID, &e.ID, "TEMPLE_UPDATE_FAILED", auditDetails, ip, "failure")
		return err
	}

	// 🆕 CREATE NEW APPROVAL REQUEST IF TEMPLE WAS REJECTED AND NOW PENDING
	if wasRejected && e.Status == "pending" {
		now := time.Now()
		
		// First, close any old approval requests for this entity
		if err := s.Repo.CloseOldApprovalRequests(e.ID, "temple_approval"); err != nil {
			log.Printf("⚠️ Failed to close old approval requests for temple ID %d: %v", e.ID, err)
		}
		
		// Create new approval request for superadmin review
		req := &auth.ApprovalRequest{
			UserID:      userID,
			EntityID:    &e.ID,
			RequestType: "temple_reapproval", // Different type to indicate it's a re-approval
			Status:      "pending",
			CreatedAt:   now,
			UpdatedAt:   now,
		}

		if err := s.Repo.CreateApprovalRequest(req); err != nil {
			auditDetails := map[string]interface{}{
				"temple_id":   e.ID,
				"temple_name": e.Name,
				"error":       err.Error(),
				"action":      "Failed to create re-approval request",
			}
			s.AuditService.LogAction(context.Background(), &userID, &e.ID, "TEMPLE_REAPPROVAL_REQUEST_FAILED", auditDetails, ip, "failure")
			
			// Don't fail the entire update, just log the error
			log.Printf("⚠️ Failed to create re-approval request for temple ID %d: %v", e.ID, err)
		} else {
			log.Printf("✅ Re-approval request created for previously rejected temple ID: %d", e.ID)
			
			// Log successful re-approval request creation
			auditDetails := map[string]interface{}{
				"temple_id":       e.ID,
				"temple_name":     e.Name,
				"previous_status": "rejected",
				"new_status":      "pending",
				"action":          "Re-submitted for approval",
			}
			s.AuditService.LogAction(context.Background(), &userID, &e.ID, "TEMPLE_REAPPROVAL_REQUESTED", auditDetails, ip, "success")
		}
	}

	// Log successful temple update
	auditDetails := map[string]interface{}{
		"temple_id":       e.ID,
		"temple_name":     e.Name,
		"previous_name":   existingEntity.Name,
		"temple_type":     e.TempleType,
		"email":           e.Email,
		"phone":           e.Phone,
		"city":            e.City,
		"state":           e.State,
		"main_deity":      e.MainDeity,
		"description":     e.Description,
		"updated_fields":  getUpdatedFields(existingEntity, e),
		"was_rejected":    wasRejected,
		"status":          e.Status,
	}
	
	actionType := "TEMPLE_UPDATED"
	if wasRejected && e.Status == "pending" {
		actionType = "TEMPLE_UPDATED_RESUBMITTED"
	}
	
	s.AuditService.LogAction(context.Background(), &userID, &e.ID, actionType, auditDetails, ip, "success")

	return nil
}
// ToggleEntityStatus toggles the active/inactive status of an entity
// This should be added ONLY ONCE in your service.go file
// If you have this method declared twice, remove one of them
func (s *Service) ToggleEntityStatus(id int, isActive bool, userID uint, ip string) error{
	// Get existing entity
	existingEntity, err := s.Repo.GetEntityByID(id)
	if err != nil {
		auditDetails := map[string]interface{}{
			"temple_id": id,
			"error":     "Temple not found",
		}
		entityID := uint(id)
		s.AuditService.LogAction(context.Background(), &userID, &entityID, "TEMPLE_STATUS_TOGGLE_FAILED", auditDetails, ip, "failure")
		return err
	}

	// Update the status
	if err := s.Repo.UpdateEntityStatus(id, isActive); err != nil {
		auditDetails := map[string]interface{}{
			"temple_id":   id,
			"temple_name": existingEntity.Name,
			"new_status":  isActive,
			"error":       err.Error(),
		}
		entityID := uint(id)
		s.AuditService.LogAction(context.Background(), &userID, &entityID, "TEMPLE_STATUS_TOGGLE_FAILED", auditDetails, ip, "failure")
		return err
	}

	// Log successful status toggle
	statusText := "inactive"
	if isActive {
		statusText = "active"
	}

	auditDetails := map[string]interface{}{
		"temple_id":       id,
		"temple_name":     existingEntity.Name,
		"previous_status": existingEntity.IsActive,
		"new_status":      isActive,
		"status_text":     statusText,
	}
	entityID := uint(id)
	s.AuditService.LogAction(context.Background(), &userID, &entityID, "TEMPLE_STATUS_TOGGLED", auditDetails, ip, "success")

	return nil
}

// DeleteEntity - Super Admin → Delete temple
func (s *Service) DeleteEntity(id int, userID uint, ip string) error {
	existingEntity, err := s.Repo.GetEntityByID(id)
	if err != nil {
		auditDetails := map[string]interface{}{
			"temple_id": id,
			"error":     "Temple not found",
		}
		entityID := uint(id)
		s.AuditService.LogAction(context.Background(), &userID, &entityID, "TEMPLE_DELETE_FAILED", auditDetails, ip, "failure")

		return err
	}

	if err := s.Repo.DeleteEntity(id); err != nil {
		auditDetails := map[string]interface{}{
			"temple_id":   id,
			"temple_name": existingEntity.Name,
			"error":       err.Error(),
		}
		entityID := uint(id)
		s.AuditService.LogAction(context.Background(), &userID, &entityID, "TEMPLE_DELETE_FAILED", auditDetails, ip, "failure")

		return err
	}

	auditDetails := map[string]interface{}{
		"temple_id":   id,
		"temple_name": existingEntity.Name,
		"temple_type": existingEntity.TempleType,
		"email":       existingEntity.Email,
		"city":        existingEntity.City,
		"state":       existingEntity.State,
	}
	entityID := uint(id)
	s.AuditService.LogAction(context.Background(), &userID, &entityID, "TEMPLE_DELETED", auditDetails, ip, "success")

	return nil
}

// ========== DEVOTEE MANAGEMENT ==========


// GetDevotees - Temple Admin → Get devotees for specific entity
func (s *Service) GetDevotees(entityID uint) ([]DevoteeDTO, error) {
	return s.Repo.GetDevoteesByEntityID(entityID)
}
// GetDevoteeStats - Temple Admin → Get devotee statistics for entity
func (s *Service) GetDevoteeStats(entityID uint) (DevoteeStats, error) {
	return s.Repo.GetDevoteeStats(entityID)
}

// DashboardSummary is the structured JSON response
type DashboardSummary struct {
	RegisteredDevotees struct {
		Total     int64 `json:"total"`
		ThisMonth int64 `json:"this_month"`
	} `json:"registered_devotees"`

	SevaBookings struct {
		Today     int64 `json:"today"`
		ThisMonth int64 `json:"this_month"`
	} `json:"seva_bookings"`

	MonthDonations struct {
		Amount        float64 `json:"amount"`
		PercentChange float64 `json:"percent_change"`
	} `json:"month_donations"`

	UpcomingEvents struct {
		Total    int64 `json:"total"`
		ThisWeek int64 `json:"this_week"`
	} `json:"upcoming_events"`
}

// GetDashboardSummary - Temple Admin → Dashboard Summary
func (s *Service) GetDashboardSummary(entityID uint) (DashboardSummary, error) {
	var summary DashboardSummary

	totalDevotees, err := s.Repo.CountDevotees(entityID)
	if err != nil {
		return summary, err
	}
	thisMonthDevotees, err := s.Repo.CountDevoteesThisMonth(entityID)
	if err != nil {
		return summary, err
	}
	summary.RegisteredDevotees.Total = totalDevotees
	summary.RegisteredDevotees.ThisMonth = thisMonthDevotees

	todaySevas, err := s.Repo.CountSevaBookingsToday(entityID)
	if err != nil {
		return summary, err
	}
	monthSevas, err := s.Repo.CountSevaBookingsThisMonth(entityID)
	if err != nil {
		return summary, err
	}
	summary.SevaBookings.Today = todaySevas
	summary.SevaBookings.ThisMonth = monthSevas

	monthDonationAmount, percentChange, err := s.Repo.GetMonthDonationsWithChange(entityID)
	if err != nil {
		return summary, err
	}
	summary.MonthDonations.Amount = monthDonationAmount
	summary.MonthDonations.PercentChange = percentChange

	totalUpcoming, err := s.Repo.CountUpcomingEvents(entityID)
	if err != nil {
		return summary, err
	}
	thisWeekUpcoming, err := s.Repo.CountUpcomingEventsThisWeek(entityID)
	if err != nil {
		return summary, err
	}
	summary.UpcomingEvents.Total = totalUpcoming
	summary.UpcomingEvents.ThisWeek = thisWeekUpcoming

	return summary, nil
}
/*func (s *Service) GetVolunteersByEntityID(entityID uint) ([]UserEntityMembership, error) {
	return s.Repo.GetVolunteersByEntityID(entityID)
}
*/
// Helper function to track what fields were updated
func getUpdatedFields(old, new Entity) []string {
	var updatedFields []string

	if old.Name != new.Name {
		updatedFields = append(updatedFields, "name")
	}
	if (old.MainDeity == nil && new.MainDeity != nil) ||
		(old.MainDeity != nil && new.MainDeity == nil) ||
		(old.MainDeity != nil && new.MainDeity != nil && *old.MainDeity != *new.MainDeity) {
		updatedFields = append(updatedFields, "main_deity")
	}
	if old.TempleType != new.TempleType {
		updatedFields = append(updatedFields, "temple_type")
	}
	if old.Email != new.Email {
		updatedFields = append(updatedFields, "email")
	}
	if old.Phone != new.Phone {
		updatedFields = append(updatedFields, "phone")
	}
	if old.Description != new.Description {
		updatedFields = append(updatedFields, "description")
	}
	if old.StreetAddress != new.StreetAddress {
		updatedFields = append(updatedFields, "street_address")
	}
	if old.City != new.City {
		updatedFields = append(updatedFields, "city")
	}
	if old.State != new.State {
		updatedFields = append(updatedFields, "state")
	}
	if old.District != new.District {
		updatedFields = append(updatedFields, "district")
	}
	if old.Pincode != new.Pincode {
		updatedFields = append(updatedFields, "pincode")
	}

	return updatedFields
}
