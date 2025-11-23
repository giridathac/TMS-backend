package reports

import (
	"fmt"
	"time"

	"gorm.io/gorm"
)

// ReportRepository defines the database operations required by the reports service.
type ReportRepository interface {
	// GetEntitiesByTenant returns entity IDs created by the given tenant (temple admin user)
	GetEntitiesByTenant(userID uint) ([]uint, error)

	// Added for superadmin and tenant-based access
	GetAllEntityIDs() ([]uint, error)
	GetEntitiesByTenantID(tenantID uint) ([]uint, error)

	GetEvents(entityIDs []uint, start, end time.Time) ([]EventReportRow, error)
	GetSevas(entityIDs []uint, start, end time.Time) ([]SevaReportRow, error)
	GetSevaBookings(entityIDs []uint, start, end time.Time) ([]SevaBookingReportRow, error)
	GetTemplesRegistered(entityIDs []uint, start, end time.Time, status string) ([]TempleRegisteredReportRow, error)
	GetDevoteeBirthdays(entityIDs []uint, start, end time.Time) ([]DevoteeBirthdayReportRow, error)
	GetDonations(entityIDs []uint, start, end time.Time) ([]DonationReportRow, error)
	GetDevoteeList(entityIDs []uint, start, end time.Time, status string) ([]DevoteeListReportRow, error)
	GetDevoteeProfiles(entityIDs []uint, start, end time.Time, status string) ([]DevoteeProfileReportRow, error)
	GetDevoteeProfiles_ext(entityIDs []uint, start, end time.Time, status string, all string) ([]DevoteeProfileReportRow_ext, error)
	GetAuditLogs(entityIDs []uint, start, end time.Time, actionTypes []string, status string) ([]AuditLogReportRow, error)
	GetApprovalStatus(entityIDs []uint, start, end time.Time, role, status string) ([]ApprovalStatusReportRow, error)
	GetUserDetails(entityIDs []uint, start, end time.Time, role, status string) ([]UserDetailsReportRow, error)
}

type repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) ReportRepository {
	return &repository{db: db}
}

// ======================
// Entity Fetch Methods
// ======================

func (r *repository) GetEntitiesByTenant(userID uint) ([]uint, error) {
	var ids []uint
	err := r.db.Table("entities").
		Select("id").
		Where("created_by = ?", userID).
		Scan(&ids).Error
	fmt.Println("GetEntitiesByTenant ids:", ids, userID)

	return ids, err
}

// Get all entities (for superadmin)
func (r *repository) GetAllEntityIDs() ([]uint, error) {
	var ids []uint
	err := r.db.Table("entities").
		Select("id").
		Scan(&ids).Error
	return ids, err
}

// Get entities by tenant ID (for tenant-level users)
func (r *repository) GetEntitiesByTenantID(tenantID uint) ([]uint, error) {
	var ids []uint
	err := r.db.Table("entities").
		Select("id").
		Where("tenant_id = ?", tenantID).
		Scan(&ids).Error
	fmt.Println("GetEntitiesByTenantID:", ids)
	return ids, err
}

// ======================
// Reports
// ======================

func (r *repository) GetEvents(entityIDs []uint, start, end time.Time) ([]EventReportRow, error) {
	var out []EventReportRow
	if len(entityIDs) == 0 {
		return out, nil
	}

	err := r.db.Table("events e").
		Select(`
			e.title,
			ent.name as temple_name,
			e.description,
			e.event_type,
			e.event_date,
			TO_CHAR(e.event_time, 'HH24:MI') as event_time,
			e.location,
			e.created_by,
			e.is_active,
			e.created_at,
			e.updated_at
		`).
		Joins("LEFT JOIN entities ent ON e.entity_id = ent.id").
		Where("e.entity_id IN ?", entityIDs).
		Where("e.event_date BETWEEN ? AND ?", start, end).
		Order("e.event_date DESC").
		Scan(&out).Error
	return out, err
}
func (r *repository) GetSevas(entityIDs []uint, start, end time.Time) ([]SevaReportRow, error) {
	var out []SevaReportRow
	if len(entityIDs) == 0 {
		return out, nil
	}

	err := r.db.Table("sevas s").
		Select(`
			s.name,
			ent.name as temple_name,
			s.seva_type,
			s.description,
			s.price,
			s.created_at as date,
			s.start_time,
			s.end_time,
			s.duration,
			s.status,
			s.is_active,
			s.created_at,
			s.updated_at
		`).
		Joins("LEFT JOIN entities ent ON s.entity_id = ent.id").
		Where("s.entity_id IN ?", entityIDs).
		Where("s.created_at BETWEEN ? AND ?", start, end).
		Order("s.created_at DESC").
		Scan(&out).Error

	return out, err
}

func (r *repository) GetSevaBookings(entityIDs []uint, start, end time.Time) ([]SevaBookingReportRow, error) {
	var out []SevaBookingReportRow
	if len(entityIDs) == 0 {
		return out, nil
	}

	err := r.db.Table("seva_bookings sb").
		Select(`
			s.name as seva_name,
			ent.name as temple_name,
			s.seva_type,
			u.full_name as devotee_name,
			u.phone as devotee_phone,
			sb.booking_time,
			sb.status,
			sb.created_at,
			sb.updated_at
		`).
		Joins("LEFT JOIN sevas s ON sb.seva_id = s.id").
		Joins("LEFT JOIN entities ent ON sb.entity_id = ent.id").
		Joins("LEFT JOIN users u ON sb.user_id = u.id").
		Where("sb.entity_id IN ?", entityIDs).
		Where("sb.created_at BETWEEN ? AND ?", start, end).
		Order("sb.created_at DESC").
		Scan(&out).Error
	return out, err
}

func (r *repository) GetDonations(entityIDs []uint, start, end time.Time) ([]DonationReportRow, error) {
	var out []DonationReportRow
	if len(entityIDs) == 0 {
		return out, nil
	}

	err := r.db.Table("donations d").
		Select(`
			d.id,
			COALESCE(NULLIF(u.full_name, ''), u.email, 'Anonymous') as donor_name,
			ent.name as temple_name,
			COALESCE(u.email, '') as donor_email,
			d.amount,
			d.donation_type,
			d.method as payment_method,
			d.status,
			COALESCE(d.donated_at, d.created_at) as donation_date,
			d.order_id,
			d.payment_id,
			d.created_at,
			d.updated_at
		`).
		Joins("LEFT JOIN users u ON d.user_id = u.id").
		Joins("LEFT JOIN entities ent ON d.entity_id = ent.id").
		Where("d.entity_id IN ?", entityIDs).
		Where("d.created_at BETWEEN ? AND ?", start, end).
		Order("d.created_at DESC").
		Scan(&out).Error
	return out, err
}

func (r *repository) GetTemplesRegistered(entityIDs []uint, start, end time.Time, status string) ([]TempleRegisteredReportRow, error) {
	var rows []TempleRegisteredReportRow
	if len(entityIDs) == 0 {
		return rows, nil
	}

	query := r.db.Table("entities").
		Select("id, name, created_at, status").
		Where("id IN ?", entityIDs).
		Where("created_at BETWEEN ? AND ?", start, end)

	if status != "" {
		query = query.Where("status = ?", status)
	}

	err := query.Order("created_at DESC").Scan(&rows).Error
	return rows, err
}

// GetDevoteeBirthdays - Fixed version with proper date handling
func (r *repository) GetDevoteeBirthdays(entityIDs []uint, start, end time.Time) ([]DevoteeBirthdayReportRow, error) {
	var rows []DevoteeBirthdayReportRow
	if len(entityIDs) == 0 {
		return rows, nil
	}

	// Format start and end dates to MM-DD format for birthday matching
	startMMDD := start.Format("01-02")
	endMMDD := end.Format("01-02")

	fmt.Printf("🔍 Fetching birthdays for entities: %v\n", entityIDs)
	fmt.Printf("📅 Date range: %s to %s (MM-DD format: %s to %s)\n",
		start.Format("2006-01-02"), end.Format("2006-01-02"), startMMDD, endMMDD)

	// Base query - join users with devotee profiles and entity memberships
	query := r.db.Table("users u").
		Select(`
			u.id as user_id,
			u.full_name,
			dp.dob as date_of_birth,
			dp.gender,
			u.phone,
			u.email,
			e.name as temple_name,
			uem.joined_at as member_since
		`).
		Joins("INNER JOIN user_entity_memberships uem ON u.id = uem.user_id").
		Joins("INNER JOIN entities e ON uem.entity_id = e.id").
		Joins("INNER JOIN devotee_profiles dp ON u.id = dp.user_id").
		Where("u.role_id = ?", 3). // Role ID 3 is devotee
		Where("uem.status = ?", "active").
		Where("uem.entity_id IN ?", entityIDs).
		Where("dp.dob IS NOT NULL")

	// Handle year wrap-around (e.g., Dec 25 to Jan 5)
	if startMMDD > endMMDD {
		// Birthday range crosses year boundary
		query = query.Where(
			"(TO_CHAR(dp.dob, 'MM-DD') >= ? OR TO_CHAR(dp.dob, 'MM-DD') <= ?)",
			startMMDD, endMMDD,
		)
	} else {
		// Normal date range within same year
		query = query.Where(
			"TO_CHAR(dp.dob, 'MM-DD') BETWEEN ? AND ?",
			startMMDD, endMMDD,
		)
	}

	query = query.Order("TO_CHAR(dp.dob, 'MM-DD') ASC")

	// Execute query with debug output
	err := query.Debug().Scan(&rows).Error
	if err != nil {
		fmt.Printf("❌ Error fetching birthdays: %v\n", err)
		return nil, err
	}

	fmt.Printf("✅ Found %d birthdays\n", len(rows))

	// Debug: Print first few results
	for i, row := range rows {
		if i < 3 { // Print first 3 for debugging
			fmt.Printf("  - %s (DOB: %v, Temple: %s)\n",
				row.FullName, row.DateOfBirth, row.TempleName)
		}
	}

	return rows, nil
}

func (r *repository) GetDevoteeList(entityIDs []uint, start, end time.Time, status string) ([]DevoteeListReportRow, error) {
	var rows []DevoteeListReportRow
	if len(entityIDs) == 0 {
		return rows, nil
	}

	query := r.db.Table("users u").
		Select(`
			u.id as user_id,
			u.full_name as devotee_name,
			en.name as temple_name,
			uem.joined_at,
			uem.status as devotee_status,
			u.created_at
		`).
		Joins("INNER JOIN user_entity_memberships uem ON u.id = uem.user_id").
		Joins("INNER JOIN entities en ON en.id = uem.entity_id").
		Where("uem.entity_id IN ?", entityIDs)

	if status != "" {
		query = query.Where("uem.status = ?", status)
	}

	query = query.Where("uem.joined_at BETWEEN ? AND ?", start, end).Order("uem.joined_at DESC")
	err := query.Scan(&rows).Error
	return rows, err
}

func (r *repository) GetDevoteeProfiles(entityIDs []uint, start, end time.Time, status string) ([]DevoteeProfileReportRow, error) {
	var rows []DevoteeProfileReportRow
	if len(entityIDs) == 0 {
		return rows, nil
	}

	query := r.db.Table("users u").
		Select(`
			u.id as user_id,
			u.full_name,
			en.name as temple_name,  -- ADDED THIS LINE
			dp.dob,
			dp.gender,
			CONCAT(
				COALESCE(dp.street_address, ''), ' ',
				COALESCE(dp.city, ''), ' ',
				COALESCE(dp.state, ''), ' ',
				COALESCE(dp.country, ''), ' ',
				COALESCE(dp.pincode, '')
			) as full_address,
			COALESCE(dp.gotra, '') as gotra,
			COALESCE(dp.nakshatra, '') as nakshatra,
			COALESCE(dp.rashi, '') as rashi,
			COALESCE(dp.lagna, '') as lagna
		`).
		Joins("INNER JOIN user_entity_memberships uem ON u.id = uem.user_id").
		Joins("INNER JOIN entities en ON uem.entity_id = en.id"). // ADDED THIS JOIN
		Joins("INNER JOIN devotee_profiles dp ON u.id = dp.user_id").
		Where("u.role_id = ?", 3).
		Where("uem.entity_id IN ?", entityIDs)

	if status != "" {
		query = query.Where("uem.status = ?", status)
	}

	query = query.Where("uem.joined_at BETWEEN ? AND ?", start, end).Order("u.full_name ASC")
	err := query.Scan(&rows).Error

	return rows, err
}

func (r *repository) GetDevoteeProfiles_ext(entityIDs []uint, start, end time.Time, status string, all string) ([]DevoteeProfileReportRow_ext, error) {
	var rows []DevoteeProfileReportRow_ext
	if len(entityIDs) == 0 {
		return rows, nil
	}

	query := r.db.Table("users u").
		Select(`
			u.id as user_id,
			u.full_name,
			en.name as temple_name,
			dp.dob,
			dp.gender,
			CONCAT(
				COALESCE(dp.street_address, ''), ' ',
				COALESCE(dp.city, ''), ' ',
				COALESCE(dp.state, ''), ' ',
				COALESCE(dp.country, ''), ' ',
				COALESCE(dp.pincode, '')
			) as full_address,
			COALESCE(dp.gotra, '') as gotra,
			COALESCE(dp.nakshatra, '') as nakshatra,
			COALESCE(dp.rashi, '') as rashi,
			COALESCE(dp.lagna, '') as lagna
		`).
		Joins("INNER JOIN user_entity_memberships AS uem ON uem.user_id = u.id").
		Joins("INNER JOIN devotee_profiles AS dp ON dp.user_id = u.id").
		Joins("INNER JOIN entities AS en ON en.id = uem.entity_id").
		Where("u.role_id = ?", 3).
		Where("uem.entity_id IN ?", entityIDs)

	if status != "" {
		query = query.Where("uem.status = ?", status)
	}

	query = query.Where("uem.joined_at BETWEEN ? AND ?", start, end).Order("u.full_name ASC")

	err := query.Scan(&rows).Error

	return rows, err
}

func (r *repository) GetAuditLogs(entityIDs []uint, start, end time.Time, actionTypes []string, status string) ([]AuditLogReportRow, error) {
	var rows []AuditLogReportRow
	if len(entityIDs) == 0 {
		return rows, nil
	}

	query := r.db.Table("audit_logs al").
		Select(`
			al.id,
			al.entity_id,
			e.name AS entity_name,
			al.user_id,
			u.full_name AS user_name,
			COALESCE(ur.role_name, '') AS user_role,
			al.action,
			al.status,
			al.ip_address,
			al.created_at AS timestamp,
			al.created_at,
			COALESCE(al.details::text, '') AS details
		`).
		Joins("LEFT JOIN users u ON al.user_id = u.id").
		Joins("LEFT JOIN entities e ON al.entity_id = e.id").
		Joins("LEFT JOIN user_roles ur ON u.role_id = ur.id").
		Where("al.entity_id IN ?", entityIDs).
		Where("al.created_at BETWEEN ? AND ?", start, end)

	if len(actionTypes) > 0 {
		query = query.Where("al.action IN ?", actionTypes)
	}

	if status != "" {
		query = query.Where("al.status = ?", status)
	}

	err := query.Order("al.created_at DESC").Scan(&rows).Error
	return rows, err
}

func (r *repository) GetApprovalStatus(entityIDs []uint, start, end time.Time, role, status string) ([]ApprovalStatusReportRow, error) {
	var rows []ApprovalStatusReportRow

applyDateFilter := !start.IsZero() && !end.IsZero() && start.Year() > 1
	// Check role names
	var roleCheck []struct {
		RoleName string
		Count    int64
	}
	r.db.Table("users u").
		Select("ur.role_name, COUNT(*) as count").
		Joins("INNER JOIN user_roles ur ON u.role_id = ur.id").
		Group("ur.role_name").
		Scan(&roleCheck)
	
	fmt.Printf("   📊 Users by role in database:\n")
	for _, rc := range roleCheck {
		fmt.Printf("      - %s: %d users\n", rc.RoleName, rc.Count)
	}
	var tenantRows []ApprovalStatusReportRow
	
	if role == "" || role == "tenantadmin" {
		fmt.Printf("\n    Fetching Tenant Admin Approvals...\n")
		
		// Query for BOTH tenantadmin and templeadmin roles
		// because in your system, templeadmin users are the tenant-level admins
		tenantQuery := r.db.Table("users u").
			Select(`
				CAST(u.id AS VARCHAR) as tenant_id,
				u.full_name as tenant_name,
				'N/A' as entity_id,
				'N/A' as entity_name,
				COALESCE(u.status, 'pending') as status,
				u.created_at,
				CASE 
					WHEN u.status = 'approved' THEN u.updated_at
					ELSE NULL
				END as approved_at,
				u.email,
				ur.role_name as role
			`).
			Joins("INNER JOIN user_roles ur ON u.role_id = ur.id").
			Where("ur.role_name IN (?)", []string{"tenantadmin", "templeadmin"})

		// Apply date filter if valid
		if applyDateFilter {
			
			tenantQuery = tenantQuery.Where("u.created_at BETWEEN ? AND ?", start, end)
		} else {
			
		}

		// Apply status filter
		if status != "" {
			fmt.Printf("      🎯 Applying status filter: %s\n", status)
			tenantQuery = tenantQuery.Where("u.status = ?", status)
		}

		// Execute tenant query
		if err := tenantQuery.Order("u.created_at DESC").Scan(&tenantRows).Error; err != nil {
			fmt.Printf("❌ Error fetching tenant admins: %v\n", err)
			return nil, err
		}

		// Mark these as 'tenantadmin' for the report
		
		fmt.Printf("   ✅ Fetched %d tenant admin records\n", len(tenantRows))
		
		// Debug: Print tenant records
		if len(tenantRows) > 0 {
			fmt.Printf("   📋 Tenant admin records:\n")
			for i, row := range tenantRows {
				if i < 5 {
					fmt.Printf("      %d. TenantID=%s, Name=%s, Status=%s, Email=%s, Created=%s\n",
						i+1, row.TenantID, row.TenantName, row.Status, row.Email, 
						row.CreatedAt.Format("2006-01-02"))
				}
			}
		} else {
			fmt.Printf("   ⚠️ No tenant admin records found!\n")
		}
	}

	// ============================================
	// STEP 2: FETCH TEMPLE (ENTITY) APPROVALS
	// These are the temples/entities created by the tenant admins
	// ============================================
	var templeRows []ApprovalStatusReportRow
	
	if role == "" || role == "templeadmin" {
		
		
		templeQuery := r.db.Table("entities e").
			Select(`
				CAST(e.created_by AS VARCHAR) as tenant_id,
				u.full_name as tenant_name,
				CAST(e.id AS VARCHAR) as entity_id,
				e.name as entity_name,
				COALESCE(e.status, 'pending') as status,
				e.created_at,
				CASE 
					WHEN e.status = 'approved' THEN e.updated_at
					ELSE NULL
				END as approved_at,
				u.email,
				'templeadmin' as role
			`).
			Joins("INNER JOIN users u ON e.created_by = u.id")

		// Apply entity filter if provided (for non-superadmin users)
		if len(entityIDs) > 0 {
			
			templeQuery = templeQuery.Where("e.id IN ?", entityIDs)
		}

		// Apply date filter if valid
		if applyDateFilter {
			fmt.Printf("      ⏰ Applying date filter for entities: %s to %s\n", 
				start.Format("2006-01-02"), end.Format("2006-01-02"))
			templeQuery = templeQuery.Where("e.created_at BETWEEN ? AND ?", start, end)
		} else {
			fmt.Printf("      ⚠️ No date filter - fetching ALL entities\n")
		}

		// Apply status filter (entity status)
		if status != "" {
		
			templeQuery = templeQuery.Where("e.status = ?", status)
		}

		// Execute temple query
		if err := templeQuery.Order("e.created_at DESC").Scan(&templeRows).Error; err != nil {
			fmt.Printf("❌ Error fetching temple/entity records: %v\n", err)
			return nil, err
		}

	
		
		// Debug: Print temple records
		if len(templeRows) > 0 {
		
			for i, row := range templeRows {
				if i < 3 {
					fmt.Printf("      %d. TenantID=%s, TenantName=%s, EntityID=%s, EntityName=%s, Status=%s\n",
						i+1, row.TenantID, row.TenantName, row.EntityID, row.EntityName, row.Status)
				}
			}
		}
	}

	// ============================================
	// STEP 3: COMBINE RESULTS
	// ============================================
	rows = append(tenantRows, templeRows...)


	// Post-process to ensure no empty/null values
	for i := range rows {
		if rows[i].Status == "" {
			rows[i].Status = "pending"
		}
		if rows[i].TenantID == "" || rows[i].TenantID == "0" {
			rows[i].TenantID = "N/A"
		}
		if rows[i].TenantName == "" {
			rows[i].TenantName = "N/A"
		}
		if rows[i].EntityID == "" || rows[i].EntityID == "0" {
			rows[i].EntityID = "N/A"
		}
		if rows[i].EntityName == "" {
			rows[i].EntityName = "N/A"
		}
		if rows[i].Email == "" {
			rows[i].Email = "N/A"
		}
	}

	// Sort by created_at DESC to show most recent first
	if len(rows) > 1 {
		for i := 0; i < len(rows)-1; i++ {
			for j := i + 1; j < len(rows); j++ {
				if rows[i].CreatedAt.Before(rows[j].CreatedAt) {
					rows[i], rows[j] = rows[j], rows[i]
				}
			}
		}
	}

	fmt.Printf("✅ Returning %d approval status records\n", len(rows))
	return rows, nil
}
func (r *repository) GetUserDetails(entityIDs []uint, start, end time.Time, role, status string) ([]UserDetailsReportRow, error) {
	var rows []UserDetailsReportRow

	query := r.db.Table("users u").
		Select(`
			u.id,
			u.full_name as name,
			COALESCE(e.name, 'N/A') as entity_name,
			u.email,
			ur.role_name as role,
			COALESCE(uem.status, 'Active') as status,
			u.created_at
		`).
		Joins("LEFT JOIN user_entity_memberships uem ON u.id = uem.user_id").
		Joins("LEFT JOIN entities e ON uem.entity_id = e.id").
		Joins("LEFT JOIN user_roles ur ON u.role_id = ur.id")

	if len(entityIDs) > 0 {
		query = query.Where("uem.entity_id IN ?", entityIDs)
	}

	if !start.IsZero() && !end.IsZero() {
		query = query.Where("u.created_at BETWEEN ? AND ?", start, end)
	}

	if role != "" {
		query = query.Where("ur.role_name = ?", role)
	}

	if status != "" {
		query = query.Where("uem.status = ?", status)
	}

	err := query.Order("u.created_at DESC").Scan(&rows).Error
	return rows, err
}