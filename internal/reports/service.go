package reports

import (
	"context"
	"fmt"
	"strconv"

	"github.com/sharath018/temple-management-backend/internal/auditlog"
)

// ReportService performs business logic and coordinates repo + exporter.
type ReportService interface {
	GetActivities(req ActivitiesReportRequest) (ReportData, error)
	ExportActivities(ctx context.Context, req ActivitiesReportRequest, userID *uint, ip string) ([]byte, string, string, error)

	GetTempleRegisteredReport(req TempleRegisteredReportRequest, entityIDs []string) ([]TempleRegisteredReportRow, error)
	ExportTempleRegisteredReport(ctx context.Context, req TempleRegisteredReportRequest, entityIDs []string, reportType string, userID *uint, ip string) ([]byte, string, string, error)

	GetDevoteeBirthdaysReport(req DevoteeBirthdaysReportRequest, entityIDs []string) ([]DevoteeBirthdayReportRow, error)
	ExportDevoteeBirthdaysReport(ctx context.Context, req DevoteeBirthdaysReportRequest, entityIDs []string, reportType string, userID *uint, ip string) ([]byte, string, string, error)

	GetDevoteeListReport(req DevoteeListReportRequest, entityIDs []string) ([]DevoteeListReportRow, error)
	ExportDevoteeListReport(ctx context.Context, req DevoteeListReportRequest, entityIDs []string, reportType string, userID *uint, ip string) ([]byte, string, string, error)

	GetDevoteeProfileReport(req DevoteeProfileReportRequest, entityIDs []string) ([]DevoteeProfileReportRow, error)
	ExportDevoteeProfileReport(ctx context.Context, req DevoteeProfileReportRequest, entityIDs []string, reportType string, userID *uint, ip string) ([]byte, string, string, error)

	GetAuditLogsReport(req AuditLogReportRequest, entityIDs []string) ([]AuditLogReportRow, error)
	ExportAuditLogsReport(ctx context.Context, req AuditLogReportRequest, entityIDs []string, reportType string, userID *uint, ip string) ([]byte, string, string, error)

	GetApprovalStatusReport(req ApprovalStatusReportRequest, entityIDs []string) ([]ApprovalStatusReportRow, error)
	ExportApprovalStatusReport(ctx context.Context, req ApprovalStatusReportRequest, entityIDs []string, reportType string, userID *uint, ip string) ([]byte, string, string, error)

	GetUserDetailsReport(req UserDetailReportRequest, entityIDs []string) ([]UserDetailsReportRow, error)
	ExportUserDetailsReport(ctx context.Context, req UserDetailReportRequest, entityIDs []string, reportType string, userID *uint, ip string) ([]byte, string, string, error)
}

type reportService struct {
	repo     ReportRepository
	exporter ReportExporter
	auditSvc auditlog.Service
}

func NewReportService(repo ReportRepository, exporter ReportExporter, auditSvc auditlog.Service) ReportService {
	return &reportService{
		repo:     repo,
		exporter: exporter,
		auditSvc: auditSvc,
	}
}

// ===============================
// Utility
// ===============================

func convertUintSlice(strs []string) []uint {
	out := make([]uint, 0, len(strs))
	for _, s := range strs {
		id, err := strconv.ParseUint(s, 10, 64)
		if err == nil {
			out = append(out, uint(id))
		}
	}
	return out
}

// ===============================
// Activities Reports
// ===============================

func (s *reportService) GetActivities(req ActivitiesReportRequest) (ReportData, error) {
	if req.Type != ReportTypeEvents && req.Type != ReportTypeSevas &&
		req.Type != ReportTypeBookings && req.Type != ReportTypeDonations {
		return ReportData{}, fmt.Errorf("invalid report type: %s", req.Type)
	}
	start := req.StartDate
	end := req.EndDate

	var data ReportData
	var err error
	switch req.Type {
	case ReportTypeEvents:
		data.Events, err = s.repo.GetEvents(convertUintSlice(req.EntityIDs), start, end)
	case ReportTypeSevas:
		data.Sevas, err = s.repo.GetSevas(convertUintSlice(req.EntityIDs), start, end)
	case ReportTypeBookings:
		data.Bookings, err = s.repo.GetSevaBookings(convertUintSlice(req.EntityIDs), start, end)
	case ReportTypeDonations:
		data.Donations, err = s.repo.GetDonations(convertUintSlice(req.EntityIDs), start, end)
	}
	return data, err
}

func (s *reportService) ExportActivities(ctx context.Context, req ActivitiesReportRequest, userID *uint, ip string) ([]byte, string, string, error) {
	data, err := s.GetActivities(req)
	if err != nil {
		details := map[string]interface{}{
			"report_type": req.Type,
			"format":      req.Format,
			"error":       err.Error(),
		}
		s.auditSvc.LogAction(ctx, userID, nil, "TEMPLE_ACTIVITIES_REPORT_DOWNLOAD_FAILED", details, ip, "failure")
		return nil, "", "", err
	}

	bytes, filename, mimeType, err := s.exporter.Export(req.Type, req.Format, data)
	if err != nil {
		details := map[string]interface{}{
			"report_type": req.Type,
			"format":      req.Format,
			"error":       err.Error(),
		}
		s.auditSvc.LogAction(ctx, userID, nil, "TEMPLE_ACTIVITIES_REPORT_DOWNLOAD_FAILED", details, ip, "failure")
		return nil, "", "", err
	}

	details := map[string]interface{}{
		"report_type": req.Type,
		"format":      req.Format,
		"filename":    filename,
		"entity_ids":  req.EntityIDs,
		"date_range":  req.DateRange,
	}
	s.auditSvc.LogAction(ctx, userID, nil, "TEMPLE_ACTIVITIES_REPORT_DOWNLOADED", details, ip, "success")

	return bytes, filename, mimeType, nil
}

// ===============================
// Temple Registered Reports
// ===============================

func (s *reportService) GetTempleRegisteredReport(req TempleRegisteredReportRequest, entityIDs []string) ([]TempleRegisteredReportRow, error) {
	return s.repo.GetTemplesRegistered(convertUintSlice(entityIDs), req.StartDate, req.EndDate, req.Status)
}

func (s *reportService) ExportTempleRegisteredReport(ctx context.Context, req TempleRegisteredReportRequest, entityIDs []string, reportType string, userID *uint, ip string) ([]byte, string, string, error) {
	rows, err := s.GetTempleRegisteredReport(req, entityIDs)
	if err != nil {
		details := map[string]interface{}{
			"report_type": "temple_registered",
			"format":      req.Format,
			"error":       err.Error(),
		}
		s.auditSvc.LogAction(ctx, userID, nil, "TEMPLE_REGISTER_REPORT_DOWNLOAD_FAILED", details, ip, "failure")
		return nil, "", "", err
	}

	data := ReportData{TemplesRegistered: rows}
	bytes, filename, mimeType, err := s.exporter.Export(reportType, req.Format, data)
	if err != nil {
		details := map[string]interface{}{
			"report_type": "temple_registered",
			"format":      req.Format,
			"error":       err.Error(),
		}
		s.auditSvc.LogAction(ctx, userID, nil, "TEMPLE_REGISTER_REPORT_DOWNLOAD_FAILED", details, ip, "failure")
		return nil, "", "", err
	}

	details := map[string]interface{}{
		"report_type":  "temple_registered",
		"format":       req.Format,
		"filename":     filename,
		"entity_ids":   entityIDs,
		"status":       req.Status,
		"date_range":   req.DateRange,
		"record_count": len(rows),
	}
	s.auditSvc.LogAction(ctx, userID, nil, "TEMPLE_REGISTER_REPORT_DOWNLOADED", details, ip, "success")

	return bytes, filename, mimeType, nil
}

// ===============================
// Devotee Birthdays Reports - FIXED
// ===============================

func (s *reportService) GetDevoteeBirthdaysReport(req DevoteeBirthdaysReportRequest, entityIDs []string) ([]DevoteeBirthdayReportRow, error) {
	// ✅ FIX: Convert string IDs to uint BEFORE calling repo
	entityUintIDs := convertUintSlice(entityIDs)
	
	if len(entityUintIDs) == 0 {
		fmt.Println("⚠️ Service: No valid entity IDs after conversion")
		return []DevoteeBirthdayReportRow{}, nil
	}
	
	fmt.Printf("\n🔄 Service Layer: GetDevoteeBirthdaysReport\n")
	fmt.Printf("   Input (strings): %v\n", entityIDs)
	fmt.Printf("   Converted (uints): %v\n", entityUintIDs)
	fmt.Printf("   Date Range: %s to %s\n", 
		req.StartDate.Format("2006-01-02"), 
		req.EndDate.Format("2006-01-02"))
	
	// ✅ Call repository with proper uint slice
	return s.repo.GetDevoteeBirthdays(entityUintIDs, req.StartDate, req.EndDate)
}

func (s *reportService) ExportDevoteeBirthdaysReport(
	ctx context.Context,
	req DevoteeBirthdaysReportRequest,
	entityIDs []string,
	reportType string,
	userID *uint,
	ip string,
) ([]byte, string, string, error) {

	//fmt.Println("\n" + "="*60)
	fmt.Println("📤 EXPORT Service: Devotee Birthdays Report")
	//fmt.Println("="*60)
	fmt.Printf("📋 Entity IDs: %v\n", entityIDs)
	fmt.Printf("📅 Date Range: %s to %s (%s)\n", 
		req.StartDate.Format("2006-01-02"), 
		req.EndDate.Format("2006-01-02"),
		req.DateRange)
	fmt.Printf("📄 Report Type: %s\n", reportType)

	// ✅ FIX: Use GetDevoteeBirthdaysReport which now handles conversion properly
	rows, err := s.GetDevoteeBirthdaysReport(req, entityIDs)
	if err != nil {
		fmt.Printf("❌ Error fetching birthdays: %v\n", err)
		details := map[string]interface{}{
			"report_type": "devotee_birthdays",
			"format":      req.Format,
			"error":       err.Error(),
		}
		s.auditSvc.LogAction(ctx, userID, nil, "DEVOTEE_BIRTHDAYS_REPORT_DOWNLOAD_FAILED", details, ip, "failure")
		return nil, "", "", err
	}

	fmt.Printf("✅ Fetched %d birthday records\n", len(rows))

	if len(rows) == 0 {
		fmt.Println("⚠️ No birthdays found - this might be expected if no birthdays fall in the date range")
	}

	// Prepare data for export
	data := ReportData{DevoteeBirthdays: rows}
	bytes, filename, mimeType, err := s.exporter.Export(reportType, req.Format, data)
	if err != nil {
		fmt.Printf("❌ Export failed: %v\n", err)
		details := map[string]interface{}{
			"report_type": "devotee_birthdays",
			"format":      req.Format,
			"error":       err.Error(),
		}
		s.auditSvc.LogAction(ctx, userID, nil, "DEVOTEE_BIRTHDAYS_REPORT_DOWNLOAD_FAILED", details, ip, "failure")
		return nil, "", "", err
	}

	// Log success
	details := map[string]interface{}{
		"report_type":  "devotee_birthdays",
		"format":       req.Format,
		"filename":     filename,
		"entity_ids":   entityIDs,
		"date_range":   req.DateRange,
		"record_count": len(rows),
	}
	s.auditSvc.LogAction(ctx, userID, nil, "DEVOTEE_BIRTHDAYS_REPORT_DOWNLOADED", details, ip, "success")

	fmt.Printf("✅ Export successful: %s (%d records)\n", filename, len(rows))
	//fmt.Println("="*60 + "\n")

	return bytes, filename, mimeType, nil
}

// ===============================
// Devotee List Reports
// ===============================

func (s *reportService) GetDevoteeListReport(req DevoteeListReportRequest, entityIDs []string) ([]DevoteeListReportRow, error) {
	return s.repo.GetDevoteeList(convertUintSlice(entityIDs), req.StartDate, req.EndDate, req.Status)
}

func (s *reportService) ExportDevoteeListReport(ctx context.Context, req DevoteeListReportRequest, entityIDs []string, reportType string, userID *uint, ip string) ([]byte, string, string, error) {
	rows, err := s.GetDevoteeListReport(req, entityIDs)
	if err != nil {
		details := map[string]interface{}{
			"report_type": "devotee_list",
			"format":      req.Format,
			"error":       err.Error(),
			"status":      req.Status,
		}
		s.auditSvc.LogAction(ctx, userID, nil, "DEVOTEE_LIST_REPORT_DOWNLOAD_FAILED", details, ip, "failure")
		return nil, "", "", err
	}

	data := ReportData{DevoteeList: rows}
	bytes, filename, mimeType, err := s.exporter.Export(reportType, req.Format, data)
	if err != nil {
		details := map[string]interface{}{
			"report_type": "devotee_list",
			"format":      req.Format,
			"error":       err.Error(),
			"status":      req.Status,
		}
		s.auditSvc.LogAction(ctx, userID, nil, "DEVOTEE_LIST_REPORT_DOWNLOAD_FAILED", details, ip, "failure")
		return nil, "", "", err
	}

	details := map[string]interface{}{
		"report_type":  "devotee_list",
		"format":       req.Format,
		"filename":     filename,
		"entity_ids":   entityIDs,
		"status":       req.Status,
		"date_range":   req.DateRange,
		"record_count": len(rows),
	}
	s.auditSvc.LogAction(ctx, userID, nil, "DEVOTEE_LIST_REPORT_DOWNLOADED", details, ip, "success")

	return bytes, filename, mimeType, nil
}

// ===============================
// Devotee Profile Reports
// ===============================

func (s *reportService) GetDevoteeProfileReport(req DevoteeProfileReportRequest, entityIDs []string) ([]DevoteeProfileReportRow, error) {
	return s.repo.GetDevoteeProfiles(convertUintSlice(entityIDs), req.StartDate, req.EndDate, req.Status)
}

func (s *reportService) ExportDevoteeProfileReport(ctx context.Context, req DevoteeProfileReportRequest, entityIDs []string, reportType string, userID *uint, ip string) ([]byte, string, string, error) {
	rows, err := s.GetDevoteeProfileReport(req, entityIDs)
	if err != nil {
		details := map[string]interface{}{
			"report_type": "devotee_profile",
			"format":      req.Format,
			"error":       err.Error(),
			"status":      req.Status,
		}
		s.auditSvc.LogAction(ctx, userID, nil, "DEVOTEE_PROFILE_REPORT_DOWNLOAD_FAILED", details, ip, "failure")
		return nil, "", "", err
	}

	data := ReportData{DevoteeProfiles: rows}
	bytes, filename, mimeType, err := s.exporter.Export(reportType, req.Format, data)
	if err != nil {
		details := map[string]interface{}{
			"report_type": "devotee_profile",
			"format":      req.Format,
			"error":       err.Error(),
			"status":      req.Status,
		}
		s.auditSvc.LogAction(ctx, userID, nil, "DEVOTEE_PROFILE_REPORT_DOWNLOAD_FAILED", details, ip, "failure")
		return nil, "", "", err
	}

	details := map[string]interface{}{
		"report_type":  "devotee_profile",
		"format":       req.Format,
		"filename":     filename,
		"entity_ids":   entityIDs,
		"status":       req.Status,
		"date_range":   req.DateRange,
		"record_count": len(rows),
	}
	s.auditSvc.LogAction(ctx, userID, nil, "DEVOTEE_PROFILE_REPORT_DOWNLOADED", details, ip, "success")

	return bytes, filename, mimeType, nil
}

// ===============================
// Audit Logs Reports
// ===============================

func (s *reportService) GetAuditLogsReport(req AuditLogReportRequest, entityIDs []string) ([]AuditLogReportRow, error) {
	ids := convertUintSlice(entityIDs)

	var actionFilters []string
	if req.Action != "" {
		actionFilters = append(actionFilters, req.Action)
	}

	return s.repo.GetAuditLogs(ids, req.StartDate, req.EndDate, actionFilters, req.Status)
}

func (s *reportService) ExportAuditLogsReport(ctx context.Context, req AuditLogReportRequest, entityIDs []string, reportType string, userID *uint, ip string) ([]byte, string, string, error) {
	rows, err := s.GetAuditLogsReport(req, entityIDs)
	if err != nil {
		details := map[string]interface{}{
			"report_type": "audit_logs",
			"format":      req.Format,
			"error":       err.Error(),
			"action":      req.Action,
			"status":      req.Status,
		}
		s.auditSvc.LogAction(ctx, userID, nil, "AUDIT_LOGS_REPORT_DOWNLOAD_FAILED", details, ip, "failure")
		return nil, "", "", err
	}

	data := ReportData{AuditLogs: rows}
	bytes, filename, mimeType, err := s.exporter.Export(reportType, req.Format, data)
	if err != nil {
		details := map[string]interface{}{
			"report_type": "audit_logs",
			"format":      req.Format,
			"error":       err.Error(),
			"action":      req.Action,
			"status":      req.Status,
		}
		s.auditSvc.LogAction(ctx, userID, nil, "AUDIT_LOGS_REPORT_DOWNLOAD_FAILED", details, ip, "failure")
		return nil, "", "", err
	}

	details := map[string]interface{}{
		"report_type":  "audit_logs",
		"format":       req.Format,
		"filename":     filename,
		"entity_ids":   entityIDs,
		"action":       req.Action,
		"status":       req.Status,
		"date_range":   req.DateRange,
		"record_count": len(rows),
	}
	s.auditSvc.LogAction(ctx, userID, nil, "AUDIT_LOGS_REPORT_DOWNLOADED", details, ip, "success")

	return bytes, filename, mimeType, nil
}

func (s *reportService) GetApprovalStatusReport(req ApprovalStatusReportRequest, entityIDs []string) ([]ApprovalStatusReportRow, error) {
	ids := convertUintSlice(entityIDs)
	
	fmt.Printf("\n📋 Service: GetApprovalStatusReport\n")
	fmt.Printf("   Role: '%s'\n", req.Role)
	fmt.Printf("   Status: '%s'\n", req.Status)
	fmt.Printf("   Entity IDs: %v\n", ids)
	
	return s.repo.GetApprovalStatus(ids, req.StartDate, req.EndDate, req.Role, req.Status)
}

func (s *reportService) ExportApprovalStatusReport(ctx context.Context, req ApprovalStatusReportRequest, entityIDs []string, reportType string, userID *uint, ip string) ([]byte, string, string, error) {
	rows, err := s.GetApprovalStatusReport(req, entityIDs)
	if err != nil {
		s.auditSvc.LogAction(ctx, userID, nil, "APPROVAL_STATUS_REPORT_DOWNLOAD_FAILED", map[string]interface{}{
			"report_type": "approval_status",
			"format":      req.Format,
			"error":       err.Error(),
			"role":        req.Role,
			"status":      req.Status,
		}, ip, "failure")
		return nil, "", "", err
	}

	data := ReportData{ApprovalStatus: rows}
	bytes, filename, mimeType, err := s.exporter.Export(reportType, req.Format, data)
	if err != nil {
		s.auditSvc.LogAction(ctx, userID, nil, "APPROVAL_STATUS_REPORT_DOWNLOAD_FAILED", map[string]interface{}{
			"report_type": "approval_status",
			"format":      req.Format,
			"error":       err.Error(),
			"role":        req.Role,
			"status":      req.Status,
		}, ip, "failure")
		return nil, "", "", err
	}

	s.auditSvc.LogAction(ctx, userID, nil, "APPROVAL_STATUS_REPORT_DOWNLOADED", map[string]interface{}{
		"report_type":  "approval_status",
		"format":       req.Format,
		"filename":     filename,
		"entity_ids":   entityIDs,
		"role":         req.Role,
		"status":       req.Status,
		"date_range":   req.DateRange,
		"record_count": len(rows),
	}, ip, "success")

	return bytes, filename, mimeType, nil
}

// ===============================
// User Details Reports
// ===============================

func (s *reportService) GetUserDetailsReport(req UserDetailReportRequest, entityIDs []string) ([]UserDetailsReportRow, error) {
	var ids []uint
	if len(entityIDs) > 0 {
		ids = convertUintSlice(entityIDs)
	} else {
		ids = nil
	}

	return s.repo.GetUserDetails(ids, req.StartDate, req.EndDate, req.Role, req.Status)
}

func (s *reportService) ExportUserDetailsReport(ctx context.Context, req UserDetailReportRequest, entityIDs []string, reportType string, userID *uint, ip string) ([]byte, string, string, error) {
	rows, err := s.GetUserDetailsReport(req, entityIDs)
	if err != nil {
		s.auditSvc.LogAction(ctx, userID, nil, "USER_DETAILS_REPORT_DOWNLOAD_FAILED", map[string]interface{}{
			"report_type": "user_details",
			"format":      req.Format,
			"error":       err.Error(),
			"role":        req.Role,
			"status":      req.Status,
		}, ip, "failure")
		return nil, "", "", err
	}

	data := ReportData{UserDetails: rows}
	bytes, filename, mimeType, err := s.exporter.Export(reportType, req.Format, data)
	if err != nil {
		s.auditSvc.LogAction(ctx, userID, nil, "USER_DETAILS_REPORT_DOWNLOAD_FAILED", map[string]interface{}{
			"report_type": "user_details",
			"format":      req.Format,
			"error":       err.Error(),
			"role":        req.Role,
			"status":      req.Status,
		}, ip, "failure")
		return nil, "", "", err
	}

	s.auditSvc.LogAction(ctx, userID, nil, "USER_DETAILS_REPORT_DOWNLOADED", map[string]interface{}{
		"report_type":  "user_details",
		"format":       req.Format,
		"filename":     filename,
		"entity_ids":   entityIDs,
		"role":         req.Role,
		"status":       req.Status,
		"date_range":   req.DateRange,
		"record_count": len(rows),
	}, ip, "success")

	return bytes, filename, mimeType, nil
}