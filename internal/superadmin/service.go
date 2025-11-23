package superadmin

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"regexp"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/sharath018/temple-management-backend/internal/auditlog"
	"github.com/sharath018/temple-management-backend/internal/auth"
	"github.com/sharath018/temple-management-backend/internal/entity"
	"github.com/sharath018/temple-management-backend/utils"
	"golang.org/x/crypto/bcrypt"
)

type Service struct {
	repo         *Repository
	auditService auditlog.Service
}

func NewService(repo *Repository, auditService auditlog.Service) *Service {
	return &Service{
		repo:         repo,
		auditService: auditService,
	}
}

// ================== TENANT ==================

func (s *Service) ApproveTenant(ctx context.Context, userID uint, adminID uint, ip string) error {
	// Check existence and current status
	user, err := s.repo.GetUserByID(ctx, userID)
	if err != nil {
		// Log failed approval attempt
		s.auditService.LogAction(ctx, &adminID, nil, "TENANT_APPROVAL_FAILED", map[string]interface{}{
			"target_user_id": userID,
			"reason":         "tenant not found",
		}, ip, "failure")
		return errors.New("tenant not found")
	}

	if user.Status == "active" {
		s.auditService.LogAction(ctx, &adminID, nil, "TENANT_APPROVAL_FAILED", map[string]interface{}{
			"target_user_id":    userID,
			"target_user_email": user.Email,
			"reason":            "already approved",
		}, ip, "failure")
		return errors.New("tenant already approved")
	}
	if user.Status == "rejected" {
		s.auditService.LogAction(ctx, &adminID, nil, "TENANT_APPROVAL_FAILED", map[string]interface{}{
			"target_user_id":    userID,
			"target_user_email": user.Email,
			"reason":            "already rejected",
		}, ip, "failure")
		return errors.New("tenant already rejected")
	}

	if err := s.repo.ApproveTenant(ctx, userID, adminID); err != nil {
		s.auditService.LogAction(ctx, &adminID, nil, "TENANT_APPROVAL_FAILED", map[string]interface{}{
			"target_user_id":    userID,
			"target_user_email": user.Email,
			"reason":            "database error",
		}, ip, "failure")
		return err
	}

	if err := s.repo.MarkTenantApprovalApproved(ctx, userID, adminID); err != nil {
		s.auditService.LogAction(ctx, &adminID, nil, "TENANT_APPROVAL_FAILED", map[string]interface{}{
			"target_user_id":    userID,
			"target_user_email": user.Email,
			"reason":            "failed to mark approval",
		}, ip, "failure")
		return err
	}

	// Log successful approval
	s.auditService.LogAction(ctx, &adminID, nil, "TENANT_APPROVED", map[string]interface{}{
		"target_user_id":    userID,
		"target_user_email": user.Email,
		"target_user_name":  user.FullName,
	}, ip, "success")

	return nil
}

func (s *Service) RejectTenant(ctx context.Context, userID uint, adminID uint, reason string, ip string) error {
	if reason == "" {
		return errors.New("rejection reason is required")
	}

	user, err := s.repo.GetUserByID(ctx, userID)
	if err != nil {
		s.auditService.LogAction(ctx, &adminID, nil, "TENANT_REJECTION_FAILED", map[string]interface{}{
			"target_user_id": userID,
			"reason":         "tenant not found",
		}, ip, "failure")
		return errors.New("tenant not found")
	}

	if user.Status == "rejected" {
		s.auditService.LogAction(ctx, &adminID, nil, "TENANT_REJECTION_FAILED", map[string]interface{}{
			"target_user_id":    userID,
			"target_user_email": user.Email,
			"reason":            "already rejected",
		}, ip, "failure")
		return errors.New("tenant already rejected")
	}
	if user.Status == "active" {
		s.auditService.LogAction(ctx, &adminID, nil, "TENANT_REJECTION_FAILED", map[string]interface{}{
			"target_user_id":    userID,
			"target_user_email": user.Email,
			"reason":            "already approved",
		}, ip, "failure")
		return errors.New("tenant already approved")
	}

	if err := s.repo.RejectTenant(ctx, userID, adminID, reason); err != nil {
		s.auditService.LogAction(ctx, &adminID, nil, "TENANT_REJECTION_FAILED", map[string]interface{}{
			"target_user_id":    userID,
			"target_user_email": user.Email,
			"reason":            "database error",
		}, ip, "failure")
		return err
	}

	// Log successful rejection
	s.auditService.LogAction(ctx, &adminID, nil, "TENANT_REJECTED", map[string]interface{}{
		"target_user_id":    userID,
		"target_user_email": user.Email,
		"target_user_name":  user.FullName,
		"rejection_reason":  reason,
	}, ip, "success")

	return nil
}

func (s *Service) GetPendingTenants(ctx context.Context) ([]auth.User, error) {
	return s.repo.GetPendingTenants(ctx)
}

// GetAllTenantDetails fetches all tenant details
func (s *Service) GetAllTenantDetails(ctx context.Context) ([]TenantTempleDetails, error) {
	return s.repo.GetAllTenantDetails(ctx)
}

// GetMultipleTenantDetails fetches details for multiple tenants by IDs
func (s *Service) GetMultipleTenantDetails(ctx context.Context, tenantIDs []uint) ([]TenantTempleDetails, error) {
	if len(tenantIDs) == 0 {
		return []TenantTempleDetails{}, nil
	}

	// Optional: Add limit to prevent too many IDs at once
	if len(tenantIDs) > 100 {
		return nil, errors.New("maximum 100 tenant IDs allowed per request")
	}

	return s.repo.GetMultipleTenantDetails(ctx, tenantIDs)
}

// GetTenantDetails with better error handling
func (s *Service) GetTenantDetails(ctx context.Context, tenantID uint) (*TenantTempleDetails, error) {
	if tenantID == 0 {
		return nil, errors.New("invalid tenant ID")
	}

	details, err := s.repo.GetTenantDetails(ctx, tenantID)
	if err != nil {
		// Check if it's a record not found error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("tenant not found")
		}
		return nil, errors.New("failed to fetch tenant details")
	}

	return details, nil
}

func (s *Service) GetTenantsWithFilters(ctx context.Context, status string, limit, page int) ([]TenantWithDetails, int64, error) {
	return s.repo.GetTenantsWithFilters(ctx, status, limit, page)
}

func (s *Service) UpdateTenantApprovalStatus(ctx context.Context, userID, adminID uint, action string, reason string) error {
	switch action {
	case "approve":
		return s.ApproveTenant(ctx, userID, adminID, "")
	case "reject":
		return s.RejectTenant(ctx, userID, adminID, reason, "")
	default:
		return errors.New("invalid action: must be approve or reject")
	}
}

// ================== ENTITY ==================

func (s *Service) GetPendingEntities(ctx context.Context) ([]entity.Entity, error) {
	return s.repo.GetPendingEntities(ctx)
}

func (s *Service) GetEntitiesWithFilters(ctx context.Context, status string, limit, page int) ([]entity.Entity, int64, error) {
	return s.repo.GetEntitiesWithFilters(ctx, status, limit, page)
}

func (s *Service) ApproveEntity(ctx context.Context, entityID uint, adminID uint, ip string) error {
	ent, err := s.repo.GetEntityByID(ctx, entityID)
	if err != nil {
		s.auditService.LogAction(ctx, &adminID, &entityID, "ENTITY_APPROVAL_FAILED", map[string]interface{}{
			"entity_id": entityID,
			"reason":    "entity not found",
		}, ip, "failure")
		return errors.New("entity not found")
	}

	if ent.Status == "approved" {
		s.auditService.LogAction(ctx, &adminID, &entityID, "ENTITY_APPROVAL_FAILED", map[string]interface{}{
			"entity_id":   entityID,
			"entity_name": ent.Name,
			"reason":      "already approved",
		}, ip, "failure")
		return errors.New("entity already approved")
	}
	if ent.Status == "rejected" {
		s.auditService.LogAction(ctx, &adminID, &entityID, "ENTITY_APPROVAL_FAILED", map[string]interface{}{
			"entity_id":   entityID,
			"entity_name": ent.Name,
			"reason":      "already rejected",
		}, ip, "failure")
		return errors.New("entity already rejected")
	}

	if err := s.repo.ApproveEntity(ctx, entityID, adminID); err != nil {
		s.auditService.LogAction(ctx, &adminID, &entityID, "ENTITY_APPROVAL_FAILED", map[string]interface{}{
			"entity_id":   entityID,
			"entity_name": ent.Name,
			"reason":      "database error",
		}, ip, "failure")
		return err
	}

	if err := s.repo.LinkEntityToUser(ctx, ent.CreatedBy, ent.ID); err != nil {
		s.auditService.LogAction(ctx, &adminID, &entityID, "ENTITY_APPROVAL_FAILED", map[string]interface{}{
			"entity_id":   entityID,
			"entity_name": ent.Name,
			"reason":      "failed to link entity to user",
		}, ip, "failure")
		return err
	}

	// Log successful approval
	s.auditService.LogAction(ctx, &adminID, &entityID, "ENTITY_APPROVED", map[string]interface{}{
		"entity_id":   entityID,
		"entity_name": ent.Name,
		"entity_type": ent.TempleType,
		"created_by":  ent.CreatedBy,
	}, ip, "success")

	return nil
}

func (s *Service) RejectEntity(ctx context.Context, entityID uint, adminID uint, reason string, ip string) error {
	if reason == "" {
		return errors.New("rejection reason is required")
	}

	ent, err := s.repo.GetEntityByID(ctx, entityID)
	if err != nil {
		s.auditService.LogAction(ctx, &adminID, &entityID, "ENTITY_REJECTION_FAILED", map[string]interface{}{
			"entity_id": entityID,
			"reason":    "entity not found",
		}, ip, "failure")
		return errors.New("entity not found")
	}

	if ent.Status == "rejected" {
		s.auditService.LogAction(ctx, &adminID, &entityID, "ENTITY_REJECTION_FAILED", map[string]interface{}{
			"entity_id":   entityID,
			"entity_name": ent.Name,
			"reason":      "already rejected",
		}, ip, "failure")
		return errors.New("entity already rejected")
	}
	if ent.Status == "approved" {
		s.auditService.LogAction(ctx, &adminID, &entityID, "ENTITY_REJECTION_FAILED", map[string]interface{}{
			"entity_id":   entityID,
			"entity_name": ent.Name,
			"reason":      "already approved",
		}, ip, "failure")
		return errors.New("entity already approved")
	}

	rejectedAt := time.Now()
	if err := s.repo.RejectEntity(ctx, entityID, adminID, reason, rejectedAt); err != nil {
		s.auditService.LogAction(ctx, &adminID, &entityID, "ENTITY_REJECTION_FAILED", map[string]interface{}{
			"entity_id":   entityID,
			"entity_name": ent.Name,
			"reason":      "database error",
		}, ip, "failure")
		return err
	}

	// Log successful rejection
	s.auditService.LogAction(ctx, &adminID, &entityID, "ENTITY_REJECTED", map[string]interface{}{
		"entity_id":        entityID,
		"entity_name":      ent.Name,
		"entity_type":      ent.TempleType,
		"created_by":       ent.CreatedBy,
		"rejection_reason": reason,
	}, ip, "success")

	return nil
}

func (s *Service) UpdateEntityApprovalStatus(ctx context.Context, entityID, adminID uint, action string, reason string) error {
	switch action {
	case "approve":
		return s.ApproveEntity(ctx, entityID, adminID, "")
	case "reject":
		return s.RejectEntity(ctx, entityID, adminID, reason, "")
	default:
		return errors.New("invalid action: must be approve or reject")
	}
}

// ================== METRIC COUNTS ==================

// Tenant approval counts for SuperAdmin dashboard
func (s *Service) GetTenantApprovalCounts(ctx context.Context) (*TenantApprovalCount, error) {
	approved, err := s.repo.CountTenantsByStatus(ctx, "active")
	if err != nil {
		return nil, err
	}

	pending, err := s.repo.CountTenantsByStatus(ctx, "pending")
	if err != nil {
		return nil, err
	}

	rejected, err := s.repo.CountTenantsByStatus(ctx, "rejected")
	if err != nil {
		return nil, err
	}

	return &TenantApprovalCount{
		Approved: approved,
		Pending:  pending,
		Rejected: rejected,
	}, nil
}

// Temple (entity) approval counts for dashboard
func (s *Service) GetTempleApprovalCounts(ctx context.Context) (*TempleApprovalCount, error) {
	pending, err := s.repo.CountEntitiesByStatus(ctx, "PENDING")
	if err != nil {
		return nil, err
	}

	active, err := s.repo.CountEntitiesByStatus(ctx, "APPROVED")
	if err != nil {
		return nil, err
	}

	rejected, err := s.repo.CountEntitiesByStatus(ctx, "REJECTED")
	if err != nil {
		return nil, err
	}

	totalDevotees, err := s.repo.CountUsersByRole(ctx, "devotee")
	if err != nil {
		return nil, err
	}

	return &TempleApprovalCount{
		PendingApproval: pending,
		ActiveTemples:   active,
		Rejected:        rejected,
		TotalDevotees:   totalDevotees,
	}, nil
}

// ================== USER MANAGEMENT ==================

// Create user (admin-created users)
func (s *Service) CreateUser(ctx context.Context, req CreateUserRequest, adminID uint, ip string) error {
	// Validate role exists
	role, err := s.repo.FindRoleByName(ctx, strings.ToLower(req.Role))
	if err != nil {
		s.auditService.LogAction(ctx, &adminID, nil, "USER_CREATE_FAILED", map[string]interface{}{
			"target_email": req.Email,
			"target_role":  req.Role,
			"reason":       "invalid role",
		}, ip, "failure")
		return errors.New("invalid role")
	}

	// Check if email already exists
	exists, err := s.repo.UserExistsByEmail(ctx, req.Email)
	if err != nil {
		s.auditService.LogAction(ctx, &adminID, nil, "USER_CREATE_FAILED", map[string]interface{}{
			"target_email": req.Email,
			"reason":       "failed to check email availability",
		}, ip, "failure")
		return errors.New("failed to check email availability")
	}
	if exists {
		s.auditService.LogAction(ctx, &adminID, nil, "USER_CREATE_FAILED", map[string]interface{}{
			"target_email": req.Email,
			"reason":       "email already exists",
		}, ip, "failure")
		return errors.New("email already exists")
	}

	// Clean and validate phone
	phone, err := cleanPhone(req.Phone)
	if err != nil {
		s.auditService.LogAction(ctx, &adminID, nil, "USER_CREATE_FAILED", map[string]interface{}{
			"target_email": req.Email,
			"reason":       "invalid phone number",
		}, ip, "failure")
		return err
	}

	// Hash password
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		s.auditService.LogAction(ctx, &adminID, nil, "USER_CREATE_FAILED", map[string]interface{}{
			"target_email": req.Email,
			"reason":       "failed to hash password",
		}, ip, "failure")
		return errors.New("failed to hash password")
	}

	// Create user - admin-created users are active by default
	user := &auth.User{
		FullName:     req.FullName,
		Email:        req.Email,
		PasswordHash: string(hash),
		Phone:        phone,
		RoleID:       role.ID,
		Status:       "active",
	}

	if err := s.repo.CreateUser(ctx, user); err != nil {
		s.auditService.LogAction(ctx, &adminID, nil, "USER_CREATE_FAILED", map[string]interface{}{
			"target_email": req.Email,
			"target_role":  req.Role,
			"reason":       "failed to create user",
		}, ip, "failure")
		return errors.New("failed to create user")
	}

	// If templeadmin role, create tenant details
	if strings.ToLower(req.Role) == "templeadmin" {
		tenantDetails := &auth.TenantDetails{
			UserID:            user.ID,
			TempleName:        req.TempleName,
			TemplePlace:       req.TemplePlace,
			TempleAddress:     req.TempleAddress,
			TemplePhoneNo:     req.TemplePhoneNo,
			TempleDescription: req.TempleDescription,
		}

		if err := s.repo.CreateTenantDetails(ctx, tenantDetails); err != nil {
			s.auditService.LogAction(ctx, &adminID, nil, "USER_CREATE_FAILED", map[string]interface{}{
				"target_user_id": user.ID,
				"target_email":   req.Email,
				"reason":         "failed to create temple details",
			}, ip, "failure")
			return errors.New("failed to create temple details")
		}
	}

	// Log successful user creation
	s.auditService.LogAction(ctx, &adminID, nil, "USER_CREATED", map[string]interface{}{
		"target_user_id":     user.ID,
		"target_email":       req.Email,
		"target_name":        req.FullName,
		"target_role":        req.Role,
		"has_temple_details": strings.ToLower(req.Role) == "templeadmin",
	}, ip, "success")

	return nil
}

// Get users with pagination and filters
func (s *Service) GetUsers(ctx context.Context, limit, page int, search, roleFilter, statusFilter string) ([]UserResponse, int64, error) {
	return s.repo.GetUsers(ctx, limit, page, search, roleFilter, statusFilter)
}

// Get user by ID
func (s *Service) GetUserByID(ctx context.Context, userID uint) (*UserResponse, error) {
	return s.repo.GetUserWithDetails(ctx, userID)
}

// Update user
func (s *Service) UpdateUser(ctx context.Context, userID uint, req UpdateUserRequest, adminID uint, ip string) error {
	// Get existing user to check if it exists
	existingUser, err := s.repo.GetUserWithDetails(ctx, userID)
	if err != nil {
		s.auditService.LogAction(ctx, &adminID, nil, "USER_UPDATE_FAILED", map[string]interface{}{
			"target_user_id": userID,
			"reason":         "user not found",
		}, ip, "failure")
		return errors.New("user not found")
	}

	// Check if email is being changed and if new email already exists
	if req.Email != "" && req.Email != existingUser.Email {
		exists, err := s.repo.UserExistsByEmail(ctx, req.Email)
		if err != nil {
			s.auditService.LogAction(ctx, &adminID, nil, "USER_UPDATE_FAILED", map[string]interface{}{
				"target_user_id": userID,
				"target_email":   existingUser.Email,
				"new_email":      req.Email,
				"reason":         "failed to check email availability",
			}, ip, "failure")
			return errors.New("failed to check email availability")
		}
		if exists {
			s.auditService.LogAction(ctx, &adminID, nil, "USER_UPDATE_FAILED", map[string]interface{}{
				"target_user_id": userID,
				"target_email":   existingUser.Email,
				"new_email":      req.Email,
				"reason":         "email already exists",
			}, ip, "failure")
			return errors.New("email already exists")
		}
	}

	// Prepare user updates
	userUpdates := &auth.User{}
	changes := make(map[string]interface{})

	if req.FullName != "" {
		userUpdates.FullName = req.FullName
		changes["full_name"] = req.FullName
	}
	if req.Email != "" {
		userUpdates.Email = req.Email
		changes["email"] = req.Email
	}
	if req.Phone != "" {
		phone, err := cleanPhone(req.Phone)
		if err != nil {
			s.auditService.LogAction(ctx, &adminID, nil, "USER_UPDATE_FAILED", map[string]interface{}{
				"target_user_id": userID,
				"target_email":   existingUser.Email,
				"reason":         "invalid phone number",
			}, ip, "failure")
			return err
		}
		userUpdates.Phone = phone
		changes["phone"] = phone
	}

	// Update user
	if err := s.repo.UpdateUser(ctx, userID, userUpdates); err != nil {
		s.auditService.LogAction(ctx, &adminID, nil, "USER_UPDATE_FAILED", map[string]interface{}{
			"target_user_id": userID,
			"target_email":   existingUser.Email,
			"reason":         "failed to update user",
		}, ip, "failure")
		return errors.New("failed to update user")
	}

	// Update temple details if user is templeadmin and temple details provided
	if existingUser.Role.RoleName == "templeadmin" &&
		(req.TempleName != "" || req.TemplePlace != "" || req.TempleAddress != "" ||
			req.TemplePhoneNo != "" || req.TempleDescription != "") {

		tenantDetails := &auth.TenantDetails{}
		templeChanges := make(map[string]interface{})

		if req.TempleName != "" {
			tenantDetails.TempleName = req.TempleName
			templeChanges["temple_name"] = req.TempleName
		}
		if req.TemplePlace != "" {
			tenantDetails.TemplePlace = req.TemplePlace
			templeChanges["temple_place"] = req.TemplePlace
		}
		if req.TempleAddress != "" {
			tenantDetails.TempleAddress = req.TempleAddress
			templeChanges["temple_address"] = req.TempleAddress
		}
		if req.TemplePhoneNo != "" {
			tenantDetails.TemplePhoneNo = req.TemplePhoneNo
			templeChanges["temple_phone"] = req.TemplePhoneNo
		}
		if req.TempleDescription != "" {
			tenantDetails.TempleDescription = req.TempleDescription
			templeChanges["temple_description"] = req.TempleDescription
		}

		if err := s.repo.UpdateTenantDetails(ctx, userID, tenantDetails); err != nil {
			s.auditService.LogAction(ctx, &adminID, nil, "USER_UPDATE_FAILED", map[string]interface{}{
				"target_user_id": userID,
				"target_email":   existingUser.Email,
				"reason":         "failed to update temple details",
			}, ip, "failure")
			return errors.New("failed to update temple details")
		}

		changes["temple_details"] = templeChanges
	}

	// Log successful user update
	s.auditService.LogAction(ctx, &adminID, nil, "USER_UPDATED", map[string]interface{}{
		"target_user_id": userID,
		"target_email":   existingUser.Email,
		"target_name":    existingUser.FullName,
		"changes":        changes,
	}, ip, "success")

	return nil
}

// Delete user
func (s *Service) DeleteUser(ctx context.Context, userID uint, adminID uint, ip string) error {
	// Get existing user to check role
	existingUser, err := s.repo.GetUserWithDetails(ctx, userID)
	if err != nil {
		s.auditService.LogAction(ctx, &adminID, nil, "USER_DELETE_FAILED", map[string]interface{}{
			"target_user_id": userID,
			"reason":         "user not found",
		}, ip, "failure")
		return errors.New("user not found")
	}

	// Keep this restriction for safety - prevent deleting superadmin users
	if existingUser.Role.RoleName == "superadmin" {
		s.auditService.LogAction(ctx, &adminID, nil, "USER_DELETE_FAILED", map[string]interface{}{
			"target_user_id": userID,
			"target_email":   existingUser.Email,
			"reason":         "cannot delete superadmin user",
		}, ip, "failure")
		return errors.New("cannot delete superadmin user")
	}

	// Prevent self-deletion
	if userID == adminID {
		s.auditService.LogAction(ctx, &adminID, nil, "USER_DELETE_FAILED", map[string]interface{}{
			"target_user_id": userID,
			"target_email":   existingUser.Email,
			"reason":         "cannot delete own account",
		}, ip, "failure")
		return errors.New("cannot delete your own account")
	}

	if err := s.repo.DeleteUser(ctx, userID); err != nil {
		s.auditService.LogAction(ctx, &adminID, nil, "USER_DELETE_FAILED", map[string]interface{}{
			"target_user_id": userID,
			"target_email":   existingUser.Email,
			"reason":         "database error",
		}, ip, "failure")
		return err
	}

	// Log successful user deletion
	s.auditService.LogAction(ctx, &adminID, nil, "USER_DELETED", map[string]interface{}{
		"target_user_id": userID,
		"target_email":   existingUser.Email,
		"target_name":    existingUser.FullName,
		"target_role":    existingUser.Role.RoleName,
	}, ip, "success")

	return nil
}

// Update user status
func (s *Service) UpdateUserStatus(ctx context.Context, userID uint, status string, adminID uint, ip string) error {
	// Get existing user first
	existingUser, err := s.repo.GetUserWithDetails(ctx, userID)
	if err != nil {
		s.auditService.LogAction(ctx, &adminID, nil, "USER_STATUS_UPDATE_FAILED", map[string]interface{}{
			"target_user_id": userID,
			"new_status":     status,
			"reason":         "user not found",
		}, ip, "failure")
		return errors.New("user not found")
	}

	// Only keep the self-deactivation check
	if userID == adminID && status == "inactive" {
		s.auditService.LogAction(ctx, &adminID, nil, "USER_STATUS_UPDATE_FAILED", map[string]interface{}{
			"target_user_id": userID,
			"target_email":   existingUser.Email,
			"new_status":     status,
			"reason":         "cannot deactivate own account",
		}, ip, "failure")
		return errors.New("cannot deactivate your own account")
	}

	if err := s.repo.UpdateUserStatus(ctx, userID, status); err != nil {
		s.auditService.LogAction(ctx, &adminID, nil, "USER_STATUS_UPDATE_FAILED", map[string]interface{}{
			"target_user_id": userID,
			"target_email":   existingUser.Email,
			"new_status":     status,
			"reason":         "database error",
		}, ip, "failure")
		return err
	}

	// Log successful status update
	s.auditService.LogAction(ctx, &adminID, nil, "USER_STATUS_UPDATED", map[string]interface{}{
		"target_user_id": userID,
		"target_email":   existingUser.Email,
		"target_name":    existingUser.FullName,
		"old_status":     existingUser.Status,
		"new_status":     status,
	}, ip, "success")

	return nil
}

// Get all user roles
func (s *Service) GetUserRoles(ctx context.Context) ([]UserRole, error) {
	return s.repo.GetUserRoles(ctx)
}

// ================== HELPERS ==================

func cleanPhone(raw string) (string, error) {
	re := regexp.MustCompile(`\D`)
	cleaned := re.ReplaceAllString(raw, "")

	// Strip leading 91 if present and length is 12
	if len(cleaned) == 12 && strings.HasPrefix(cleaned, "91") {
		cleaned = cleaned[2:]
	}

	if len(cleaned) != 10 {
		return "", errors.New("invalid phone number format")
	}

	return cleaned, nil
}

// ================== USER ROLES ==================

// CreateRole handles the creation of a new user role
func (s *Service) CreateRole(ctx context.Context, req *auth.CreateRoleRequest, adminID uint, ip string) error {
	// 1. Basic validation from the DTO
	if req.RoleName == "" || req.Description == "" {
		s.auditService.LogAction(ctx, &adminID, nil, "ROLE_CREATE_FAILED", map[string]interface{}{
			"role_name": req.RoleName,
			"reason":    "role name and description are required",
		}, ip, "failure")
		return errors.New("role name and description are required")
	}

	// 2. Check for uniqueness using the new repository method
	exists, err := s.repo.CheckIfRoleNameExists(ctx, req.RoleName)
	if err != nil {
		s.auditService.LogAction(ctx, &adminID, nil, "ROLE_CREATE_FAILED", map[string]interface{}{
			"role_name": req.RoleName,
			"reason":    "failed to check role uniqueness",
		}, ip, "failure")
		return err
	}
	if exists {
		s.auditService.LogAction(ctx, &adminID, nil, "ROLE_CREATE_FAILED", map[string]interface{}{
			"role_name": req.RoleName,
			"reason":    "role name already exists",
		}, ip, "failure")
		return errors.New("role name already exists")
	}

	// 3. Create the UserRole model instance
	newRole := &auth.UserRole{
		RoleName:            req.RoleName,
		Description:         req.Description,
		CanRegisterPublicly: false,
		Status:              "active",
	}

	// 4. Save to the database via the repository
	if err := s.repo.CreateUserRole(ctx, newRole); err != nil {
		s.auditService.LogAction(ctx, &adminID, nil, "ROLE_CREATE_FAILED", map[string]interface{}{
			"role_name": req.RoleName,
			"reason":    "database error",
		}, ip, "failure")
		return err
	}

	// Log successful role creation
	s.auditService.LogAction(ctx, &adminID, nil, "ROLE_CREATED", map[string]interface{}{
		"role_id":               newRole.ID,
		"role_name":             req.RoleName,
		"description":           req.Description,
		"can_register_publicly": false,
	}, ip, "success")

	return nil
}

// GetRoles fetches all active roles for the UI
func (s *Service) GetRoles(ctx context.Context) ([]auth.RoleResponse, error) {
	// 1. Fetch all active roles from the repository
	roles, err := s.repo.GetAllUserRoles(ctx)
	if err != nil {
		return nil, err
	}

	// 2. Convert the database models to the response DTOs
	var roleResponses []auth.RoleResponse
	for _, role := range roles {
		roleResponses = append(roleResponses, auth.RoleResponse{
			ID:                  role.ID,
			RoleName:            role.RoleName,
			Description:         role.Description,
			Status:              role.Status,
			CanRegisterPublicly: role.CanRegisterPublicly,
		})
	}

	return roleResponses, nil
}

// UpdateRole updates an existing user role's details
func (s *Service) UpdateRole(ctx context.Context, roleID uint, req *auth.UpdateRoleRequest, adminID uint, ip string) error {
	role, err := s.repo.GetUserRoleByID(ctx, roleID)
	if err != nil {
		s.auditService.LogAction(ctx, &adminID, nil, "ROLE_UPDATE_FAILED", map[string]interface{}{
			"role_id": roleID,
			"reason":  "database error",
		}, ip, "failure")
		return err
	}
	if role == nil {
		s.auditService.LogAction(ctx, &adminID, nil, "ROLE_UPDATE_FAILED", map[string]interface{}{
			"role_id": roleID,
			"reason":  "role not found",
		}, ip, "failure")
		return errors.New("role not found")
	}

	changes := make(map[string]interface{})

	// Update only if provided
	if req.RoleName != "" && req.RoleName != role.RoleName {
		exists, err := s.repo.CheckIfRoleNameExists(ctx, req.RoleName)
		if err != nil {
			s.auditService.LogAction(ctx, &adminID, nil, "ROLE_UPDATE_FAILED", map[string]interface{}{
				"role_id":   roleID,
				"role_name": role.RoleName,
				"reason":    "failed to check role uniqueness",
			}, ip, "failure")
			return err
		}
		if exists {
			s.auditService.LogAction(ctx, &adminID, nil, "ROLE_UPDATE_FAILED", map[string]interface{}{
				"role_id":   roleID,
				"role_name": role.RoleName,
				"new_name":  req.RoleName,
				"reason":    "role name already exists",
			}, ip, "failure")
			return errors.New("role name already exists")
		}
		changes["role_name"] = map[string]string{"old": role.RoleName, "new": req.RoleName}
		role.RoleName = req.RoleName
	}
	if req.Description != "" {
		changes["description"] = map[string]string{"old": role.Description, "new": req.Description}
		role.Description = req.Description
	}
	if req.CanRegisterPublicly != nil {
		changes["can_register_publicly"] = map[string]bool{"old": role.CanRegisterPublicly, "new": *req.CanRegisterPublicly}
		role.CanRegisterPublicly = *req.CanRegisterPublicly
	}

	role.UpdatedAt = time.Now()

	if err := s.repo.UpdateUserRole(ctx, role); err != nil {
		s.auditService.LogAction(ctx, &adminID, nil, "ROLE_UPDATE_FAILED", map[string]interface{}{
			"role_id":   roleID,
			"role_name": role.RoleName,
			"reason":    "database error",
		}, ip, "failure")
		return err
	}

	// Log successful role update
	s.auditService.LogAction(ctx, &adminID, nil, "ROLE_UPDATED", map[string]interface{}{
		"role_id":   roleID,
		"role_name": role.RoleName,
		"changes":   changes,
	}, ip, "success")

	return nil
}

// ToggleRoleStatus specifically handles updating only the status
func (s *Service) ToggleRoleStatus(ctx context.Context, roleID uint, status string, adminID uint, ip string) error {
	role, err := s.repo.GetUserRoleByID(ctx, roleID)
	if err != nil {
		s.auditService.LogAction(ctx, &adminID, nil, "ROLE_STATUS_UPDATE_FAILED", map[string]interface{}{
			"role_id":    roleID,
			"new_status": status,
			"reason":     "database error",
		}, ip, "failure")
		return err
	}
	if role == nil {
		s.auditService.LogAction(ctx, &adminID, nil, "ROLE_STATUS_UPDATE_FAILED", map[string]interface{}{
			"role_id":    roleID,
			"new_status": status,
			"reason":     "role not found",
		}, ip, "failure")
		return errors.New("role not found")
	}

	// Check if the status is a valid value
	if status != "active" && status != "inactive" {
		s.auditService.LogAction(ctx, &adminID, nil, "ROLE_STATUS_UPDATE_FAILED", map[string]interface{}{
			"role_id":    roleID,
			"role_name":  role.RoleName,
			"new_status": status,
			"reason":     "invalid status provided",
		}, ip, "failure")
		return errors.New("invalid status provided")
	}

	oldStatus := role.Status
	role.Status = status
	role.UpdatedAt = time.Now()

	if err := s.repo.UpdateUserRole(ctx, role); err != nil {
		s.auditService.LogAction(ctx, &adminID, nil, "ROLE_STATUS_UPDATE_FAILED", map[string]interface{}{
			"role_id":    roleID,
			"role_name":  role.RoleName,
			"new_status": status,
			"reason":     "database error",
		}, ip, "failure")
		return err
	}

	// Log successful status update
	s.auditService.LogAction(ctx, &adminID, nil, "ROLE_STATUS_UPDATED", map[string]interface{}{
		"role_id":    roleID,
		"role_name":  role.RoleName,
		"old_status": oldStatus,
		"new_status": status,
	}, ip, "success")

	return nil
}

// ================== PASSWORD RESET ==================

// SearchUserByEmail searches for a user by email
func (s *Service) SearchUserByEmail(ctx context.Context, email string) (*UserResponse, error) {
	// Check if email exists
	exists, err := s.repo.UserExistsByEmail(ctx, email)
	if err != nil {
		return nil, errors.New("failed to search for user")
	}
	if !exists {
		return nil, errors.New("user not found")
	}

	// Get user by email
	user, err := s.repo.GetUserByEmail(ctx, email)
	if err != nil {
		return nil, errors.New("user not found")
	}

	// Get full user details
	userResponse, err := s.repo.GetUserWithDetails(ctx, user.ID)
	if err != nil {
		return nil, errors.New("failed to get user details")
	}

	return userResponse, nil
}

// ResetUserPassword resets a user's password and sends notification
func (s *Service) ResetUserPassword(ctx context.Context, userID uint, newPassword string, adminID uint) error {
	// Get existing user to check if it exists
	user, err := s.repo.GetUserByID(ctx, userID)
	if err != nil {
		return errors.New("user not found")
	}

	// Hash the new password
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return errors.New("failed to hash password")
	}

	// Update the password
	if err := s.repo.UpdateUserPassword(ctx, userID, string(hash)); err != nil {
		return errors.New("failed to update password")
	}

	// Get admin info for the notification
	admin, err := s.repo.GetUserByID(ctx, adminID)
	if err != nil {
		// Don't fail the password reset if we can't get admin details
		utils.SendPasswordResetNotification(user.Email, user.FullName, "Admin", newPassword)
	} else {
		utils.SendPasswordResetNotification(user.Email, user.FullName, admin.FullName, newPassword)
	}

	return nil
}

// ================== USER ASSIGNMENT ==================

// GetTenantsForAssignment fetches a list of approved temple admins
func (s *Service) GetTenantsForAssignment(ctx context.Context, limit, page int) ([]AssignableTenant, int64, error) {
	// Check for valid pagination parameters
	if limit <= 0 {
		limit = 10
	}
	if page <= 0 {
		page = 1
	}

	tenants, total, err := s.repo.GetAssignableTenants(ctx, limit, page)
	if err != nil {
		return nil, 0, errors.New("failed to fetch assignable tenants")
	}

	return tenants, total, nil
}

// AssignUsersToTenant assigns a user to a specific tenant
func (s *Service) AssignUsersToTenant(ctx context.Context, userID uint, tenantID uint, adminID uint) error {
	tx := s.repo.db.WithContext(ctx).Begin()
	if tx.Error != nil {
		return errors.New("failed to start database transaction")
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// 1. Validate the user to be assigned
	var userToAssign auth.User
	err := tx.WithContext(ctx).Model(&auth.User{}).Preload("Role").Where("id = ?", userID).First(&userToAssign).Error
	if err != nil {
		tx.Rollback()
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("user with ID %d not found", userID)
		}
		return fmt.Errorf("failed to retrieve user with ID %d", userID)
	}

	roleNameLower := strings.ToLower(userToAssign.Role.RoleName)
	if roleNameLower != "standarduser" && roleNameLower != "monitoringuser" {
		tx.Rollback()
		return fmt.Errorf("user with ID %d has an unassignable role ('%s')", userID, userToAssign.Role.RoleName)
	}

	// 2. Clear all existing assignments for the user
	if err := tx.WithContext(ctx).Where("user_id = ?", userID).Delete(&auth.TenantUserAssignment{}).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to delete existing assignments for user %d: %w", userID, err)
	}

	// 3. Validate the selected tenant
	var tenantUser auth.User
	err = tx.WithContext(ctx).Model(&auth.User{}).Preload("Role").Where("id = ?", tenantID).First(&tenantUser).Error
	if err != nil {
		tx.Rollback()
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("selected tenant user (temple admin) not found")
		}
		return errors.New("failed to retrieve selected tenant user details")
	}
	if strings.ToLower(tenantUser.Role.RoleName) != "templeadmin" || tenantUser.Status != "active" {
		tx.Rollback()
		return errors.New("the selected user (temple admin) is not an active temple admin and cannot be a tenant")
	}

	// 4. Create new assignment
	newAssignment := auth.TenantUserAssignment{
		UserID:    userID,
		TenantID:  tenantID,
		CreatedBy: adminID,
	}

	if err := tx.WithContext(ctx).Create(&newAssignment).Error; err != nil {
		fmt.Printf("Database error during CREATE tenant assignment: %v\n", err)
		tx.Rollback()
		return fmt.Errorf("failed to create user assignment: %w", err)
	}

	// 5. Commit the transaction
	return tx.Commit().Error
}

// GetTenantsForSelection - Get tenants based on user role
func (s *Service) GetTenantsForSelection(ctx context.Context, userID uint, userRole string) ([]TenantSelectionResponse, error) {
	roleNameLower := strings.ToLower(userRole)

	switch roleNameLower {
	case "superadmin":
		// SuperAdmin can see all active temple admins
		return s.repo.GetTenantsForSelection(ctx)

	case "standarduser", "monitoringuser":
		// StandardUser and MonitoringUser can only see assigned tenants
		return s.repo.GetAssignedTenantsForUser(ctx, userID)

	default:
		return nil, errors.New("unauthorized: invalid role for tenant selection")
	}
}

// GetTenantsWithTempleDetails fetches tenants with their temple details
func (s *Service) GetTenantsWithTempleDetails(ctx context.Context, role, status string) ([]TenantResponse, error) {
	return s.repo.GetTenantsWithTempleDetails(ctx, role, status)
}

// ================== BULK UPLOAD ==================

// BulkUploadUsers parses CSV and inserts users
func (s *Service) BulkUploadUsers(ctx context.Context, file multipart.File, adminID uint, ip string) (*BulkUploadResult, error) {
	fmt.Println("In BulkUploadUsers function")
	reader := csv.NewReader(file)
	reader.TrimLeadingSpace = true

	// Skip header
	if _, err := reader.Read(); err != nil {
		return nil, errors.New("invalid CSV format or missing header")
	}

	var successCount, failCount int
	var errorsList []string
	totalRows := 0

	removeInvisible := func(str string) string {
		return strings.Map(func(r rune) rune {
			if r == '\u200B' || r == '\uFEFF' {
				return -1
			}
			return r
		}, str)
	}

	for {
		record, err := reader.Read()
		fmt.Println("record:", record)
		if err == io.EOF {
			break
		}
		totalRows++

		if err != nil {
			failCount++
			errorsList = append(errorsList, fmt.Sprintf("row read error: %v", err))
			continue
		}

		// Minimum 6 columns required
		if len(record) < 6 {
			failCount++
			errorsList = append(errorsList, "row missing required fields")
			continue
		}

		// Parse base columns
		fullName := strings.TrimSpace(record[0])
		email := strings.TrimSpace(record[1])
		phone := strings.TrimSpace(record[2])
		password := strings.TrimSpace(record[3])

		// Clean hidden characters from Role
		rawRole := record[4]
		cleanRole := removeInvisible(rawRole)
		roleName := strings.ToLower(strings.TrimSpace(cleanRole))

		status := strings.ToLower(strings.TrimSpace(record[5]))

		if fullName == "" || email == "" {
			failCount++
			errorsList = append(errorsList, fmt.Sprintf("invalid row: %v", record))
			continue
		}

		// Validate role (using DB)
		role, err := s.repo.FindRoleByName(ctx, roleName)
		if err != nil || role == nil {
			failCount++
			errorsList = append(errorsList, fmt.Sprintf("invalid role for email %s (role given: '%s')", email, roleName))
			continue
		}

		// Clean phone
		cleanPhoneNo, err := cleanPhone(phone)
		if err != nil {
			failCount++
			errorsList = append(errorsList, fmt.Sprintf("invalid phone for email %s", email))
			continue
		}

		// Handle password hashing
		var passwordHash string
		if password != "" {
			hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
			if err != nil {
				failCount++
				errorsList = append(errorsList, fmt.Sprintf("error hashing password for %s", email))
				continue
			}
			passwordHash = string(hash)
		} else {
			hash, _ := bcrypt.GenerateFromPassword([]byte("Default@123"), bcrypt.DefaultCost)
			passwordHash = string(hash)
		}

		user := auth.User{
			FullName:     fullName,
			Email:        email,
			Phone:        cleanPhoneNo,
			RoleID:       role.ID,
			Status:       status,
			PasswordHash: passwordHash,
		}

		// Save user
		if err := s.repo.CreateUser(ctx, &user); err != nil {
			failCount++
			errorsList = append(errorsList, fmt.Sprintf("failed to create user %s", email))
			continue
		}

		// Handle temple details for templeadmin
		if strings.ToLower(role.RoleName) == "templeadmin" {
			if len(record) < 11 {
				failCount++
				errorsList = append(errorsList, fmt.Sprintf("missing temple details for %s", email))
				continue
			}

			td := &auth.TenantDetails{
				UserID:            user.ID,
				TempleName:        strings.TrimSpace(record[6]),
				TemplePlace:       strings.TrimSpace(record[7]),
				TempleAddress:     strings.TrimSpace(record[8]),
				TemplePhoneNo:     strings.TrimSpace(record[9]),
				TempleDescription: strings.TrimSpace(record[10]),
			}

			if td.TempleName == "" || td.TemplePlace == "" || td.TempleAddress == "" ||
				td.TemplePhoneNo == "" || td.TempleDescription == "" {
				failCount++
				errorsList = append(errorsList, fmt.Sprintf("missing temple details for %s", email))
				continue
			}

			fmt.Println("td.TempleName: ", td.TempleName)
			fmt.Println("td.TemplePlace: ", td.TemplePlace)
			fmt.Println("td.TempleAddress: ", td.TempleAddress)
			fmt.Println("td.TemplePhone: ", td.TemplePhoneNo)
			fmt.Println("td.TempleDescription: ", td.TempleDescription)

			if err := s.repo.CreateApprovalRequest(user.ID, "tenant_approval", adminID); err != nil {
				failCount++
				errorsList = append(errorsList, fmt.Sprintf("failed to create temple approval request for %s", email))
				continue
			}

			if err := s.repo.CreateTenantDetails(ctx, td); err != nil {
				failCount++
				errorsList = append(errorsList, fmt.Sprintf("failed to save temple details for %s", email))
				continue
			}
		}

		successCount++
		fmt.Printf("Created user: %s (Role: %s)\n", email, roleName)
	}

	return &BulkUploadResult{
		TotalRows:    totalRows,
		SuccessCount: successCount,
		FailedCount:  failCount,
		Errors:       errorsList,
	}, nil
}