package tenant

import (
    "errors"
    "time"
    "golang.org/x/crypto/bcrypt"
    "log"
)

// Service provides tenant user management functionality
type Service struct {
    repo *Repository
}

// NewService creates a new service instance
func NewService(repo *Repository) *Service {
    return &Service{repo: repo}
}

// GetTenantUsers fetches users assigned to a tenant
func (s *Service) GetTenantUsers(tenantID uint, role string) ([]UserResponse, error) {
    log.Printf("SERVICE: Getting users for tenant ID %d", tenantID)
    users, err := s.repo.GetTenantUsers(tenantID, role)
    if err != nil {
        log.Printf("Service: Error getting users: %v", err)
        return nil, err
    }
    
    // Ensure we always return an empty array instead of nil
    if users == nil {
        log.Printf("Service: No users found, returning empty array")
        return []UserResponse{}, nil
    }
    
    // Add role for frontend compatibility
    for i := range users {
        // Only set default role if not available from DB
        if users[i].Role == "" {
            users[i].Role = "StandardUser"
        } else {
            // Convert role names to PascalCase for frontend compatibility
            switch users[i].Role {
            case "monitoringuser":
                users[i].Role = "MonitoringUser"
            case "standarduser":
                users[i].Role = "StandardUser"
            }
        }
    }
    
    log.Printf("Service: Returning %d users", len(users))
    return users, nil
}


// UpdateUser updates a user's details and/or status
// UpdateUser updates a user's details and/or status
// UpdateUser updates a user's details and/or status
func (s *Service) UpdateUser(tenantID, userID uint, input UserInput, status string) (*UserResponse, error) {
    log.Printf("🔵 SERVICE: Updating user %d for tenant %d", userID, tenantID)
    
    // First verify that the user belongs to the tenant
    exists, err := s.repo.CheckUserBelongsToTenant(userID, tenantID)
    if err != nil {
        return nil, err
    }
    if !exists {
        return nil, errors.New("user does not belong to this tenant")
    }
    
    // Get the user before update to preserve data
    currentUser, err := s.repo.GetUserByID(userID)
    if err != nil {
        return nil, err
    }
    
    // Update user details in the user table
    err = s.repo.UpdateUserDetails(userID, input)
    if err != nil {
        return nil, err
    }
    
    // Use the provided status or keep the current one
    userStatus := status
    if userStatus == "" {
        userStatus = currentUser.Status
    }
    
    // Update status if provided
    if status != "" {
        err = s.repo.UpdateUserStatus(userID, tenantID, status)
        if err != nil {
            return nil, err
        }
    }
    
    // Get the updated user
    user, err := s.repo.GetUserByID(userID)
    if err != nil {
        return nil, err
    }
    
    // Get role ID or name if available
    roleName := input.Role
    if roleName == "" {
        // If role wasn't provided, try to determine from user's role_id
        // This is simplified - you may need a more complex mapping
        switch user.RoleID {
        case 5:
            roleName = "StandardUser"
        case 6:
            roleName = "MonitoringUser"
        default:
            roleName = "StandardUser" // Default
        }
    }
    
    // Construct response
    response := &UserResponse{
        ID:        userID,
        Name:      user.FullName,
        Email:     user.Email,
        Phone:     user.Phone,
        Status:    userStatus, // Use preserved status
        CreatedAt: user.CreatedAt,
        Role:      roleName,
    }
    
    return response, nil
}

// CreateOrUpdateTenantUser creates a new user or updates an existing user's tenant assignment
// CreateOrUpdateTenantUser creates a new user or updates an existing user's tenant assignment
func (s *Service) CreateOrUpdateTenantUser(tenantID uint, input UserInput, creatorID uint) (*UserResponse, error) {
    log.Printf("🔴 SERVICE: Creating/updating user for tenant %d: %s (%s) by creator %d", 
               tenantID, input.Name, input.Email, creatorID)
    
    // Check if user exists
    existingUser, err := s.repo.GetUserByEmail(input.Email)
    if err != nil {
        log.Printf("Error checking for existing user: %v", err)
        return nil, err
    }
    
    var userID uint
    
    if existingUser != nil {
        // User exists, use their ID
        log.Printf("User already exists (ID: %d), will update assignment", existingUser.ID)
        userID = existingUser.ID
    } else {
        // Create new user
        log.Printf("User does not exist, creating new user")
        hashedPassword, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
        if err != nil {
            log.Printf("Failed to hash password: %v", err)
            return nil, errors.New("failed to hash password")
        }
        
        // Get role ID from name
        roleID, err := s.repo.GetRoleIDByName(input.Role)
        if err != nil {
            log.Printf("Invalid role '%s': %v", input.Role, err)
            roleID = 5 // Default to standarduser if lookup fails
        }
        
        newUser := User{
            FullName:     input.Name,
            Email:        input.Email,
            Phone:        input.Phone,
            PasswordHash: string(hashedPassword),
            RoleID:       roleID,
            Status:       "active",
            CreatedAt:    time.Now(),
            UpdatedAt:    time.Now(),
            CreatedBy:    "system",
        }
        
        if err := s.repo.CreateUser(&newUser); err != nil {
            log.Printf("Failed to create user: %v", err)
            return nil, errors.New("failed to create user: " + err.Error())
        }
        
        userID = newUser.ID
        log.Printf("New user created with ID: %d", userID)
    }
    
    // Create or update tenant user assignment - explicitly passing tenantID and creatorID parameters
    log.Printf("🔴 SERVICE: Passing tenant ID %d and creator ID %d to repository", tenantID, creatorID)
    err = s.repo.UpdateTenantUserAssignment(userID, tenantID, creatorID)
    if err != nil {
        log.Printf("Failed to assign user to tenant: %v", err)
        return nil, errors.New("failed to assign user to tenant: " + err.Error())
    }
    
    log.Printf("User successfully assigned to tenant %d by creator %d", tenantID, creatorID)
    
    // Construct user response
    response := &UserResponse{
        ID:        userID,
        Name:      input.Name,
        Email:     input.Email,
        Phone:     input.Phone,
        Status:    "active",
        CreatedAt: time.Now(),
        Role:      input.Role,
    }
    
    return response, nil
}

