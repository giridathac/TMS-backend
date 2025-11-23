package auth

import (
	"log"
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

func SeedUserRoles(db *gorm.DB) error {
	roles := []UserRole{
		{RoleName: "superadmin", Description: "Super Admin", CanRegisterPublicly: false, Status: "active"},
		{RoleName: "templeadmin", Description: "Temple Admin", CanRegisterPublicly: false, Status: "active"},
		{RoleName: "devotee", Description: "Devotee", CanRegisterPublicly: true, Status: "active"},
		{RoleName: "volunteer", Description: "Volunteer", CanRegisterPublicly: true, Status: "active"},
		{RoleName: "standarduser", Description: "Standard User", CanRegisterPublicly: false, Status: "active"},
		{RoleName: "monitoringuser", Description: "Monitoring User", CanRegisterPublicly: false, Status: "active"},
	}

	for _, role := range roles {
		var existing UserRole
		err := db.Where("role_name = ?", role.RoleName).First(&existing).Error
		if err != nil {
			if err := db.Create(&role).Error; err != nil {
				return err
			}
			log.Printf("🌱 Created role: %s", role.RoleName)
		} else {
			log.Printf("ℹ️  Role already exists: %s", role.RoleName)
		}
	}

	return nil
}

// ⚠️ WARNING: This is for local development only.
// Remove or disable before deploying to production.
func SeedSuperAdminUser(db *gorm.DB) error {
	email := "superadmin@example.com"
	password := "admin123"

	var existing User
	if err := db.Where("email = ?", email).First(&existing).Error; err == nil {
		log.Println("ℹ️  Super Admin already exists")
		return nil
	}

	var role UserRole
	if err := db.Where("role_name = ?", "superadmin").First(&role).Error; err != nil {
		return err
	}

	hash, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)

	user := User{
		FullName:      "Super Admin",
		Email:         email,
		Phone:         "9999888877",
		PasswordHash:  string(hash),
		RoleID:        role.ID,
		EmailVerified: true,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	if err := db.Create(&user).Error; err != nil {
		return err
	}

	log.Printf("🌱 Super Admin created: %s / %s", email, password)
	return nil
}