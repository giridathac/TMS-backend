package reports

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"strconv"
	"time"

	"github.com/jung-kurt/gofpdf"
	"github.com/xuri/excelize/v2"
)

// ReportExporter defines the interface for exporting reports in different formats
type ReportExporter interface {
	Export(reportType, format string, data ReportData) ([]byte, string, string, error)
}

type reportExporter struct{}

func NewReportExporter() ReportExporter {
	return &reportExporter{}
}

// Export method calling corrected methods
func (e *reportExporter) Export(reportType, format string, data ReportData) ([]byte, string, string, error) {
	timestamp := time.Now().Format("20060102_150405")

	switch reportType {
	case ReportTypeEvents:
		return e.exportEventsByFormat(format, timestamp, data.Events)

	case ReportTypeSevas:
		return e.exportSevasByFormat(format, timestamp, data.Sevas)

	case ReportTypeBookings:
		return e.exportBookingsByFormat(format, timestamp, data.Bookings)

	case ReportTypeDonations:
		return e.exportDonationsByFormat(format, timestamp, data.Donations)

	case ReportTypeTempleRegistered:
		return e.exportTemplesRegistered(data.TemplesRegistered)
	case ReportTypeTempleRegisteredPDF:
		return e.exportTemplesRegisteredPDF(data.TemplesRegistered)
	case ReportTypeTempleRegisteredExcel:
		return e.exportTemplesRegisteredExcel(data.TemplesRegistered)

	case ReportTypeDevoteeBirthdays:
		return e.exportDevoteeBirthdays(data.DevoteeBirthdays)
	case ReportTypeDevoteeBirthdaysPDF:
		return e.exportDevoteeBirthdaysPDF(data.DevoteeBirthdays)
	case ReportTypeDevoteeBirthdaysExcel:
		return e.exportDevoteeBirthdaysExcel(data.DevoteeBirthdays)

	case ReportTypeDevoteeList:
		return e.exportDevoteeListByFormat(format, data.DevoteeList)
	case ReportTypeDevoteeListCSV:
		return e.exportDevoteeListCSV(data.DevoteeList)
	case ReportTypeDevoteeListExcel:
		return e.exportDevoteeListExcel(data.DevoteeList)
	case ReportTypeDevoteeListPDF:
		return e.exportDevoteeListPDF(data.DevoteeList)

	case ReportTypeDevoteeProfile:
		return e.exportDevoteeProfileByFormat(format, data.DevoteeProfiles)
	case ReportTypeDevoteeProfileCSV:
		return e.exportDevoteeProfileCSV(data.DevoteeProfiles)
	case ReportTypeDevoteeProfileExcel:
		return e.exportDevoteeProfileExcel(data.DevoteeProfiles)
	case ReportTypeDevoteeProfilePDF:
		return e.exportDevoteeProfilePDF(data.DevoteeProfiles)
	case ReportTypeDevoteeProfilePDF_ext:
		return e.exportDevoteeProfilePDF_ext(data.DevoteeProfiles_ext)

	case ReportTypeAuditLogs:
		return e.exportAuditLogsByFormat(format, timestamp, data.AuditLogs)
	case ReportTypeAuditLogsExcel:
		return e.exportAuditLogsByFormat(FormatExcel, timestamp, data.AuditLogs)
	case ReportTypeAuditLogsCSV:
		return e.exportAuditLogsByFormat(FormatCSV, timestamp, data.AuditLogs)
	case ReportTypeAuditLogsPDF:
		return e.exportAuditLogsByFormat(FormatPDF, timestamp, data.AuditLogs)

	case ReportTypeApprovalStatus:
		return e.exportApprovalStatusByFormat(format, data.ApprovalStatus)
	case ReportTypeApprovalStatusCSV:
		return e.exportApprovalStatusByFormat(FormatCSV, data.ApprovalStatus)
	case ReportTypeApprovalStatusExcel:
		return e.exportApprovalStatusByFormat(FormatExcel, data.ApprovalStatus)
	case ReportTypeApprovalStatusPDF:
		return e.exportApprovalStatusByFormat(FormatPDF, data.ApprovalStatus)

	case ReportTypeUserDetails:
		return e.exportUserDetailsByFormat(format, data.UserDetails)
	case ReportTypeUserDetailsCSV:
		return e.exportUserDetailsByFormat(FormatCSV, data.UserDetails)
	case ReportTypeUserDetailsExcel:
		return e.exportUserDetailsByFormat(FormatExcel, data.UserDetails)
	case ReportTypeUserDetailsPDF:
		return e.exportUserDetailsByFormat(FormatPDF, data.UserDetails)

	default:
		return nil, "", "", fmt.Errorf("unsupported report type: %s", reportType)
	}
}

//// ============================
/// AUDIT LOGS EXPORTS
//// ============================

// exportAuditLogsByFormat chooses export format for audit logs
func (e *reportExporter) exportAuditLogsByFormat(format, timestamp string, logs []AuditLogReportRow) ([]byte, string, string, error) {
	switch format {
	case FormatExcel:
		data, err := e.exportAuditLogsExcel(logs)
		if err != nil {
			return nil, "", "", err
		}
		filename := fmt.Sprintf("audit_logs_report_%s.xlsx", timestamp)
		return data, filename, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", nil

	case FormatCSV:
		data, err := e.exportAuditLogsCSV(logs)
		if err != nil {
			return nil, "", "", err
		}
		filename := fmt.Sprintf("audit_logs_report_%s.csv", timestamp)
		return data, filename, "text/csv", nil

	case FormatPDF:
		data, err := e.exportAuditLogsPDF(logs)
		if err != nil {
			return nil, "", "", err
		}
		filename := fmt.Sprintf("audit_logs_report_%s.pdf", timestamp)
		return data, filename, "application/pdf", nil

	default:
		return nil, "", "", fmt.Errorf("unsupported format for audit logs: %s", format)
	}
}

func (e *reportExporter) exportAuditLogsCSV(logs []AuditLogReportRow) ([]byte, error) {
	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)

	headers := []string{"ID", "Entity", "User ID", "User Name", "User Role", "Action", "Status", "IP Address", "Timestamp", "Details"}
	if err := writer.Write(headers); err != nil {
		return nil, err
	}

	for _, log := range logs {
		userID := ""
		if log.UserID != nil {
			userID = strconv.FormatUint(uint64(*log.UserID), 10)
		}

		record := []string{
			strconv.FormatUint(uint64(log.ID), 10),
			log.EntityName,
			userID,
			log.UserName,
			log.UserRole,
			log.Action,
			log.Status,
			log.IPAddress,
			log.Timestamp.Format("2006-01-02 15:04:05"),
			log.Details,
		}
		if err := writer.Write(record); err != nil {
			return nil, err
		}
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// exportTemplesRegistered exports temples registered as CSV.
func (e *reportExporter) exportTemplesRegistered(rows []TempleRegisteredReportRow) ([]byte, string, string, error) {
	buf := &bytes.Buffer{}
	w := csv.NewWriter(buf)

	headers := []string{"id", "name", "created_at", "status"}
	if err := w.Write(headers); err != nil {
		return nil, "", "", err
	}

	for _, r := range rows {
		record := []string{
			fmt.Sprint(r.ID),
			r.Name,
			r.CreatedAt.Format("2006-01-02 15:04:05"),
			r.Status,
		}
		if err := w.Write(record); err != nil {
			return nil, "", "", err
		}
	}

	w.Flush()
	if err := w.Error(); err != nil {
		return nil, "", "", err
	}

	return buf.Bytes(), "temples_registered_report.csv", "text/csv", nil
}

// exportTemplesRegisteredExcel exports temples registered as Excel.
func (e *reportExporter) exportTemplesRegisteredExcel(rows []TempleRegisteredReportRow) ([]byte, string, string, error) {
	f := excelize.NewFile()
	sheet := "Temples Registered"
	index, err := f.NewSheet(sheet)
	if err != nil {
		return nil, "", "", err
	}
	f.DeleteSheet("Sheet1")
	f.SetActiveSheet(index)

	headers := []string{"id", "name", "created_at", "status"}
	for i, h := range headers {
		cell, err := excelize.CoordinatesToCellName(i+1, 1)
		if err != nil {
			return nil, "", "", err
		}
		f.SetCellValue(sheet, cell, h)
	}

	for rIdx, r := range rows {
		row := rIdx + 2
		f.SetCellValue(sheet, fmt.Sprintf("A%d", row), r.ID)
		f.SetCellValue(sheet, fmt.Sprintf("B%d", row), r.Name)
		f.SetCellValue(sheet, fmt.Sprintf("C%d", row), r.CreatedAt.Format("2006-01-02 15:04:05"))
		f.SetCellValue(sheet, fmt.Sprintf("D%d", row), r.Status)
	}

	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		return nil, "", "", err
	}

	return buf.Bytes(), "temples_registered_report.xlsx", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", nil
}

// exportTemplesRegisteredPDF exports temples registered as PDF.
func (e *reportExporter) exportTemplesRegisteredPDF(rows []TempleRegisteredReportRow) ([]byte, string, string, error) {
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.AddPage()
	pdf.SetFont("Arial", "B", 12)
	pdf.Cell(40, 10, "Temples Registered Report")
	pdf.Ln(10)

	pdf.SetFont("Arial", "B", 10)
	headers := []string{"ID", "Name", "Created At", "Status"}
	widths := []float64{20, 80, 50, 40}

	// Print headers
	for i, h := range headers {
		pdf.CellFormat(widths[i], 7, h, "1", 0, "C", false, 0, "")
	}
	pdf.Ln(-1)

	// Print data rows
	pdf.SetFont("Arial", "", 10)
	for _, r := range rows {
		pdf.CellFormat(widths[0], 6, fmt.Sprint(r.ID), "1", 0, "C", false, 0, "")
		pdf.CellFormat(widths[1], 6, r.Name, "1", 0, "L", false, 0, "")
		pdf.CellFormat(widths[2], 6, r.CreatedAt.Format("2006-01-02 15:04:05"), "1", 0, "C", false, 0, "")
		pdf.CellFormat(widths[3], 6, r.Status, "1", 0, "C", false, 0, "")
		pdf.Ln(-1)
	}

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, "", "", err
	}

	return buf.Bytes(), "temples_registered_report.pdf", "application/pdf", nil
}

// exportDevoteeBirthdays exports devotee birthdays as CSV.
func (e *reportExporter) exportDevoteeBirthdays(rows []DevoteeBirthdayReportRow) ([]byte, string, string, error) {
	buf := &bytes.Buffer{}
	w := csv.NewWriter(buf)

	headers := []string{"full_name", "date_of_birth", "gender", "phone", "email", "temple_name", "member_since"}
	if err := w.Write(headers); err != nil {
		return nil, "", "", err
	}

	for _, r := range rows {
		record := []string{
			r.FullName,
			r.DateOfBirth.Format("2006-01-02"),
			r.Gender,
			r.Phone,
			r.Email,
			r.TempleName,
			r.MemberSince.Format("2006-01-02 15:04:05"),
		}
		if err := w.Write(record); err != nil {
			return nil, "", "", err
		}
	}

	w.Flush()
	if err := w.Error(); err != nil {
		return nil, "", "", err
	}

	return buf.Bytes(), "devotee_birthdays_report.csv", "text/csv", nil
}

// exportDevoteeBirthdaysExcel exports devotee birthdays as Excel.
func (e *reportExporter) exportDevoteeBirthdaysExcel(rows []DevoteeBirthdayReportRow) ([]byte, string, string, error) {
	f := excelize.NewFile()
	sheet := "Devotee Birthdays"
	index, err := f.NewSheet(sheet)
	if err != nil {
		return nil, "", "", err
	}
	f.DeleteSheet("Sheet1")
	f.SetActiveSheet(index)

	headers := []string{"Full Name", "Date of Birth", "Gender", "Phone", "Email", "Temple Name", "Member Since"}
	for i, h := range headers {
		cell, err := excelize.CoordinatesToCellName(i+1, 1)
		if err != nil {
			return nil, "", "", err
		}
		f.SetCellValue(sheet, cell, h)
	}

	for rIdx, r := range rows {
		row := rIdx + 2
		f.SetCellValue(sheet, fmt.Sprintf("A%d", row), r.FullName)
		f.SetCellValue(sheet, fmt.Sprintf("B%d", row), r.DateOfBirth.Format("2006-01-02"))
		f.SetCellValue(sheet, fmt.Sprintf("C%d", row), r.Gender)
		f.SetCellValue(sheet, fmt.Sprintf("D%d", row), r.Phone)
		f.SetCellValue(sheet, fmt.Sprintf("E%d", row), r.Email)
		f.SetCellValue(sheet, fmt.Sprintf("F%d", row), r.TempleName)
		f.SetCellValue(sheet, fmt.Sprintf("G%d", row), r.MemberSince.Format("2006-01-02 15:04:05"))
	}

	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		return nil, "", "", err
	}

	return buf.Bytes(), "devotee_birthdays_report.xlsx", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", nil
}

// exportDevoteeBirthdaysPDF exports devotee birthdays as PDF.
func (e *reportExporter) exportDevoteeBirthdaysPDF(rows []DevoteeBirthdayReportRow) ([]byte, string, string, error) {
	pdf := gofpdf.New("L", "mm", "A4", "")
	pdf.AddPage()
	pdf.SetFont("Arial", "B", 12)
	pdf.Cell(40, 10, "Devotee Birthdays Report")
	pdf.Ln(10)

	pdf.SetFont("Arial", "B", 10)
	headers := []string{"Full Name", "Date of Birth", "Gender", "Phone", "Email", "Temple", "Member Since"}
	widths := []float64{35, 22, 12, 22, 40, 30, 22}

	// Print headers
	for i, h := range headers {
		pdf.CellFormat(widths[i], 7, h, "1", 0, "C", false, 0, "")
	}
	pdf.Ln(-1)

	// Print data rows
	pdf.SetFont("Arial", "", 8)
	for _, r := range rows {
		pdf.CellFormat(widths[0], 6, r.FullName, "1", 0, "L", false, 0, "")
		pdf.CellFormat(widths[1], 6, r.DateOfBirth.Format("2006-01-02"), "1", 0, "C", false, 0, "")
		pdf.CellFormat(widths[2], 6, r.Gender, "1", 0, "C", false, 0, "")
		pdf.CellFormat(widths[3], 6, r.Phone, "1", 0, "C", false, 0, "")
		pdf.CellFormat(widths[4], 6, r.Email, "1", 0, "L", false, 0, "")
		pdf.CellFormat(widths[5], 6, r.TempleName, "1", 0, "L", false, 0, "")
		pdf.CellFormat(widths[6], 6, r.MemberSince.Format("2006-01-02"), "1", 0, "C", false, 0, "")
		pdf.Ln(-1)
	}

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, "", "", err
	}

	return buf.Bytes(), "devotee_birthdays_report.pdf", "application/pdf", nil
}

// Export Donations by format
func (e *reportExporter) exportDonationsByFormat(format, timestamp string, donations []DonationReportRow) ([]byte, string, string, error) {
	switch format {
	case FormatExcel:
		data, err := e.exportDonationsExcel(donations)
		if err != nil {
			return nil, "", "", err
		}
		filename := fmt.Sprintf("donations_report_%s.xlsx", timestamp)
		return data, filename, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", nil

	case FormatCSV:
		data, err := e.exportDonationsCSV(donations)
		if err != nil {
			return nil, "", "", err
		}
		filename := fmt.Sprintf("donations_report_%s.csv", timestamp)
		return data, filename, "text/csv", nil

	case FormatPDF:
		data, err := e.exportDonationsPDF(donations)
		if err != nil {
			return nil, "", "", err
		}
		filename := fmt.Sprintf("donations_report_%s.pdf", timestamp)
		return data, filename, "application/pdf", nil

	default:
		return nil, "", "", fmt.Errorf("unsupported format for donations: %s", format)
	}
}

func (e *reportExporter) exportDonationsExcel(donations []DonationReportRow) ([]byte, error) {
	f := excelize.NewFile()
	sheetName := "Donations"
	f.SetSheetName("Sheet1", sheetName)

	// UPDATED with Temple Name
	headers := []string{"ID", "Donor Name", "Temple Name", "Donor Email", "Amount", "Donation Type", "Payment Method", "Status", "Donation Date", "Order ID", "Payment ID", "Created At", "Updated At"}
	for i, header := range headers {
		cell := fmt.Sprintf("%c1", 'A'+i)
		f.SetCellValue(sheetName, cell, header)
	}

	for i, donation := range donations {
		row := i + 2
		paymentID := ""
		if donation.PaymentID != nil {
			paymentID = *donation.PaymentID
		}

		f.SetCellValue(sheetName, fmt.Sprintf("A%d", row), donation.ID)
		f.SetCellValue(sheetName, fmt.Sprintf("B%d", row), donation.DonorName)
		f.SetCellValue(sheetName, fmt.Sprintf("C%d", row), donation.TempleName)
		f.SetCellValue(sheetName, fmt.Sprintf("D%d", row), donation.DonorEmail)
		f.SetCellValue(sheetName, fmt.Sprintf("E%d", row), donation.Amount)
		f.SetCellValue(sheetName, fmt.Sprintf("F%d", row), donation.DonationType)
		f.SetCellValue(sheetName, fmt.Sprintf("G%d", row), donation.PaymentMethod)
		f.SetCellValue(sheetName, fmt.Sprintf("H%d", row), donation.Status)
		f.SetCellValue(sheetName, fmt.Sprintf("I%d", row), donation.DonationDate.Format("2006-01-02 15:04:05"))
		f.SetCellValue(sheetName, fmt.Sprintf("J%d", row), donation.OrderID)
		f.SetCellValue(sheetName, fmt.Sprintf("K%d", row), paymentID)
		f.SetCellValue(sheetName, fmt.Sprintf("L%d", row), donation.CreatedAt.Format("2006-01-02 15:04:05"))
		f.SetCellValue(sheetName, fmt.Sprintf("M%d", row), donation.UpdatedAt.Format("2006-01-02 15:04:05"))
	}

	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (e *reportExporter) exportDonationsCSV(donations []DonationReportRow) ([]byte, error) {
	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)

	// UPDATED with Temple Name
	headers := []string{"ID", "Donor Name", "Temple Name", "Donor Email", "Amount", "Donation Type", "Payment Method", "Status", "Donation Date", "Order ID", "Payment ID", "Created At", "Updated At"}
	if err := writer.Write(headers); err != nil {
		return nil, err
	}

	for _, donation := range donations {
		paymentID := ""
		if donation.PaymentID != nil {
			paymentID = *donation.PaymentID
		}

		record := []string{
			strconv.FormatUint(uint64(donation.ID), 10),
			donation.DonorName,
			donation.TempleName,
			donation.DonorEmail,
			fmt.Sprintf("%.2f", donation.Amount),
			donation.DonationType,
			donation.PaymentMethod,
			donation.Status,
			donation.DonationDate.Format("2006-01-02 15:04:05"),
			donation.OrderID,
			paymentID,
			donation.CreatedAt.Format("2006-01-02 15:04:05"),
			donation.UpdatedAt.Format("2006-01-02 15:04:05"),
		}
		if err := writer.Write(record); err != nil {
			return nil, err
		}
	}

	// Important: Flush before getting bytes
	writer.Flush()
	if err := writer.Error(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func (e *reportExporter) exportDonationsPDF(donations []DonationReportRow) ([]byte, error) {
	pdf := gofpdf.New("L", "mm", "A4", "")
	pdf.AddPage()
	pdf.SetFont("Arial", "B", 16)
	pdf.Cell(0, 10, "Donations Report")
	pdf.Ln(20)

	pdf.SetFont("Arial", "B", 10)
	// Define column widths - UPDATED with Temple Name
	widths := []float64{35, 30, 35, 20, 25, 25, 20, 25, 35}
	headers := []string{"Donor Name", "Temple Name", "Donor Email", "Amount", "Type", "Method", "Status", "Donation Date", "Order ID"}

	// Print headers with borders
	for i, header := range headers {
		pdf.CellFormat(widths[i], 7, header, "1", 0, "C", false, 0, "")
	}
	pdf.Ln(-1)

	// Print data rows with borders
	pdf.SetFont("Arial", "", 8)
	for _, donation := range donations {
		pdf.CellFormat(widths[0], 6, donation.DonorName, "1", 0, "L", false, 0, "")
		pdf.CellFormat(widths[1], 6, donation.TempleName, "1", 0, "L", false, 0, "")
		pdf.CellFormat(widths[2], 6, donation.DonorEmail, "1", 0, "L", false, 0, "")
		pdf.CellFormat(widths[3], 6, fmt.Sprintf("%.2f", donation.Amount), "1", 0, "R", false, 0, "")
		pdf.CellFormat(widths[4], 6, donation.DonationType, "1", 0, "L", false, 0, "")
		pdf.CellFormat(widths[5], 6, donation.PaymentMethod, "1", 0, "C", false, 0, "")
		pdf.CellFormat(widths[6], 6, donation.Status, "1", 0, "C", false, 0, "")
		pdf.CellFormat(widths[7], 6, donation.DonationDate.Format("2006-01-02"), "1", 0, "C", false, 0, "")
		pdf.CellFormat(widths[8], 6, donation.OrderID, "1", 0, "L", false, 0, "")
		pdf.Ln(-1)
	}

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Devotee List Export - format switch
func (e *reportExporter) exportDevoteeListByFormat(format string, rows []DevoteeListReportRow) ([]byte, string, string, error) {
	switch format {
	case FormatCSV:
		return e.exportDevoteeListCSV(rows)
	case FormatExcel:
		return e.exportDevoteeListExcel(rows)
	case FormatPDF:
		return e.exportDevoteeListPDF(rows)
	default:
		return nil, "", "", fmt.Errorf("unsupported format for devotee list: %s", format)
	}
}

// Devotee Profile Export - format switch
func (e *reportExporter) exportDevoteeProfileByFormat(format string, rows []DevoteeProfileReportRow) ([]byte, string, string, error) {
	switch format {
	case FormatCSV:
		return e.exportDevoteeProfileCSV(rows)
	case FormatExcel:
		return e.exportDevoteeProfileExcel(rows)
	case FormatPDF:
		return e.exportDevoteeProfilePDF(rows)
	default:
		return nil, "", "", fmt.Errorf("unsupported format for devotee profile: %s", format)
	}
}

// Devotee Profile CSV export
func (e *reportExporter) exportDevoteeProfileCSV(rows []DevoteeProfileReportRow) ([]byte, string, string, error) {
	buf := &bytes.Buffer{}
	w := csv.NewWriter(buf)

	headers := []string{"User ID", "Full Name", "Temple Name", "Date of Birth", "Gender", "Full Address", "Gotra", "Nakshatra", "Rashi", "Lagna"}
	if err := w.Write(headers); err != nil {
		return nil, "", "", err
	}

	for _, r := range rows {
		record := []string{
			r.UserID,
			r.FullName,
			r.TempleName,
			r.DOB.Format("2006-01-02"),
			r.Gender,
			r.FullAddress,
			r.Gotra,
			r.Nakshatra,
			r.Rashi,
			r.Lagna,
		}
		if err := w.Write(record); err != nil {
			return nil, "", "", err
		}
	}

	w.Flush()
	if err := w.Error(); err != nil {
		return nil, "", "", err
	}

	return buf.Bytes(), "devotee_profile_report.csv", "text/csv", nil
}

// Devotee Profile Excel export
func (e *reportExporter) exportDevoteeProfileExcel(rows []DevoteeProfileReportRow) ([]byte, string, string, error) {
	f := excelize.NewFile()
	sheet := "Devotee Profile"
	index, err := f.NewSheet(sheet)
	if err != nil {
		return nil, "", "", err
	}
	f.DeleteSheet("Sheet1")
	f.SetActiveSheet(index)

	headers := []string{"User ID", "Full Name", "Temple Name", "Date of Birth", "Gender", "Full Address", "Gotra", "Nakshatra", "Rashi", "Lagna"}
	for i, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		f.SetCellValue(sheet, cell, h)
	}

	for rIdx, r := range rows {
		row := rIdx + 2
		f.SetCellValue(sheet, fmt.Sprintf("A%d", row), r.UserID)
		f.SetCellValue(sheet, fmt.Sprintf("B%d", row), r.FullName)
		f.SetCellValue(sheet, fmt.Sprintf("C%d", row), r.TempleName)
		f.SetCellValue(sheet, fmt.Sprintf("D%d", row), r.DOB.Format("2006-01-02"))
		f.SetCellValue(sheet, fmt.Sprintf("E%d", row), r.Gender)
		f.SetCellValue(sheet, fmt.Sprintf("F%d", row), r.FullAddress)
		f.SetCellValue(sheet, fmt.Sprintf("G%d", row), r.Gotra)
		f.SetCellValue(sheet, fmt.Sprintf("H%d", row), r.Nakshatra)
		f.SetCellValue(sheet, fmt.Sprintf("I%d", row), r.Rashi)
		f.SetCellValue(sheet, fmt.Sprintf("J%d", row), r.Lagna)
	}

	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		return nil, "", "", err
	}

	return buf.Bytes(), "devotee_profile_report.xlsx", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", nil
}

// Devotee Profile PDF export
func (e *reportExporter) exportDevoteeProfilePDF(rows []DevoteeProfileReportRow) ([]byte, string, string, error) {
	pdf := gofpdf.New("L", "mm", "A4", "") // Landscape for more columns
	pdf.AddPage()
	pdf.SetFont("Arial", "B", 12)
	pdf.Cell(40, 10, "Devotee Profile Report")
	pdf.Ln(10)

	pdf.SetFont("Arial", "B", 8) // Smaller font for headers
	headers := []string{"User ID", "Full Name", "Temple Name", "DOB", "Gender", "Address", "Gotra", "Nakshatra", "Rashi", "Lagna"}
	widths := []float64{20, 30, 35, 20, 15, 45, 18, 22, 18, 18}

	for i, h := range headers {
		pdf.CellFormat(widths[i], 7, h, "1", 0, "C", false, 0, "")
	}
	pdf.Ln(-1)

	pdf.SetFont("Arial", "", 7) // Even smaller font for data
	for _, r := range rows {
		pdf.CellFormat(widths[0], 6, r.UserID, "1", 0, "C", false, 0, "")
		pdf.CellFormat(widths[1], 6, r.FullName, "1", 0, "L", false, 0, "")
		pdf.CellFormat(widths[2], 6, r.TempleName, "1", 0, "L", false, 0, "")
		pdf.CellFormat(widths[3], 6, r.DOB.Format("2006-01-02"), "1", 0, "C", false, 0, "")
		pdf.CellFormat(widths[4], 6, r.Gender, "1", 0, "C", false, 0, "")

		// Truncate address if too long for PDF cell
		address := r.FullAddress
		if len(address) > 30 {
			address = address[:27] + "..."
		}
		pdf.CellFormat(widths[5], 6, address, "1", 0, "L", false, 0, "")

		pdf.CellFormat(widths[6], 6, r.Gotra, "1", 0, "C", false, 0, "")
		pdf.CellFormat(widths[7], 6, r.Nakshatra, "1", 0, "C", false, 0, "")
		pdf.CellFormat(widths[8], 6, r.Rashi, "1", 0, "C", false, 0, "")
		pdf.CellFormat(widths[9], 6, r.Lagna, "1", 0, "C", false, 0, "")
		pdf.Ln(-1)
	}

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, "", "", err
	}

	return buf.Bytes(), "devotee_profile_report.pdf", "application/pdf", nil
}

// Devotee Profile PDF export with extended fields (including Temple Name)
func (e *reportExporter) exportDevoteeProfilePDF_ext(rows []DevoteeProfileReportRow_ext) ([]byte, string, string, error) {
	pdf := gofpdf.New("L", "mm", "A4", "") // Landscape for more columns
	pdf.AddPage()
	pdf.SetFont("Arial", "B", 12)
	pdf.Cell(40, 10, "Devotee Profile Report")
	pdf.Ln(10)

	pdf.SetFont("Arial", "B", 8) // Smaller font for headers
	headers := []string{"User ID", "Full Name", "Temple Name", "DOB", "Gender", "Address", "Gotra", "Nakshatra", "Rashi", "Lagna"}
	widths := []float64{20, 30, 35, 20, 15, 45, 18, 22, 18, 18}

	for i, h := range headers {
		pdf.CellFormat(widths[i], 7, h, "1", 0, "C", false, 0, "")
	}
	pdf.Ln(-1)

	pdf.SetFont("Arial", "", 7) // Even smaller font for data
	for _, r := range rows {
		pdf.CellFormat(widths[0], 6, r.UserID, "1", 0, "C", false, 0, "")
		pdf.CellFormat(widths[1], 6, r.FullName, "1", 0, "L", false, 0, "")
		pdf.CellFormat(widths[2], 6, r.TempleName, "1", 0, "L", false, 0, "")
		pdf.CellFormat(widths[3], 6, r.DOB.Format("2006-01-02"), "1", 0, "C", false, 0, "")
		pdf.CellFormat(widths[4], 6, r.Gender, "1", 0, "C", false, 0, "")

		// Truncate address if too long for PDF cell
		address := r.FullAddress
		if len(address) > 30 {
			address = address[:27] + "..."
		}
		pdf.CellFormat(widths[5], 6, address, "1", 0, "L", false, 0, "")

		pdf.CellFormat(widths[6], 6, r.Gotra, "1", 0, "C", false, 0, "")
		pdf.CellFormat(widths[7], 6, r.Nakshatra, "1", 0, "C", false, 0, "")
		pdf.CellFormat(widths[8], 6, r.Rashi, "1", 0, "C", false, 0, "")
		pdf.CellFormat(widths[9], 6, r.Lagna, "1", 0, "C", false, 0, "")
		pdf.Ln(-1)
	}

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, "", "", err
	}

	return buf.Bytes(), "devotee_profile_report.pdf", "application/pdf", nil
}
// Devotee List CSV export
func (e *reportExporter) exportDevoteeListCSV(rows []DevoteeListReportRow) ([]byte, string, string, error) {
	buf := &bytes.Buffer{}
	w := csv.NewWriter(buf)

	headers := []string{"User ID", "Devotee Name", "Temple Name", "Devotee Status", "Joined At", "Created At"}
	if err := w.Write(headers); err != nil {
		return nil, "", "", err
	}

	for _, r := range rows {
		record := []string{
			r.UserID,
			r.DevoteeName,
			r.TempleName,
			r.DevoteeStatus,
			r.JoinedAt.Format("2006-01-02 15:04:05"),
			r.CreatedAt.Format("2006-01-02 15:04:05"),
		}
		if err := w.Write(record); err != nil {
			return nil, "", "", err
		}
	}

	w.Flush()
	if err := w.Error(); err != nil {
		return nil, "", "", err
	}

	return buf.Bytes(), "devotee_list_report.csv", "text/csv", nil
}

// Devotee List Excel export
func (e *reportExporter) exportDevoteeListExcel(rows []DevoteeListReportRow) ([]byte, string, string, error) {
	f := excelize.NewFile()
	sheet := "Devotee List"
	index, err := f.NewSheet(sheet)
	if err != nil {
		return nil, "", "", err
	}
	f.DeleteSheet("Sheet1")
	f.SetActiveSheet(index)

	headers := []string{"User ID", "Devotee Name", "Temple Name", "Devotee Status", "Joined At", "Created At"}
	for i, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		f.SetCellValue(sheet, cell, h)
	}

	for rIdx, r := range rows {
		row := rIdx + 2
		f.SetCellValue(sheet, fmt.Sprintf("A%d", row), r.UserID)
		f.SetCellValue(sheet, fmt.Sprintf("B%d", row), r.DevoteeName)
		f.SetCellValue(sheet, fmt.Sprintf("C%d", row), r.TempleName)
		f.SetCellValue(sheet, fmt.Sprintf("D%d", row), r.DevoteeStatus)
		f.SetCellValue(sheet, fmt.Sprintf("E%d", row), r.JoinedAt.Format("2006-01-02 15:04:05"))
		f.SetCellValue(sheet, fmt.Sprintf("F%d", row), r.CreatedAt.Format("2006-01-02 15:04:05"))
	}

	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		return nil, "", "", err
	}

	return buf.Bytes(), "devotee_list_report.xlsx", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", nil
}

// Devotee List PDF export
func (e *reportExporter) exportDevoteeListPDF(rows []DevoteeListReportRow) ([]byte, string, string, error) {
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.AddPage()
	pdf.SetFont("Arial", "B", 12)
	pdf.Cell(40, 10, "Devotee List Report")
	pdf.Ln(10)

	pdf.SetFont("Arial", "B", 10)
	headers := []string{"User ID", "Devotee Name", "Temple Name", "Status", "Joined At", "Created At"}
	widths := []float64{20, 30, 30, 20, 30, 40}

	for i, h := range headers {
		pdf.CellFormat(widths[i], 7, h, "1", 0, "C", false, 0, "")
	}
	pdf.Ln(-1)

	pdf.SetFont("Arial", "", 9)
	for _, r := range rows {
		pdf.CellFormat(widths[0], 6, r.UserID, "1", 0, "C", false, 0, "")
		pdf.CellFormat(widths[1], 6, r.DevoteeName, "1", 0, "L", false, 0, "")
		pdf.CellFormat(widths[2], 6, r.TempleName, "1", 0, "L", false, 0, "")
		pdf.CellFormat(widths[3], 6, r.DevoteeStatus, "1", 0, "C", false, 0, "")
		pdf.CellFormat(widths[4], 6, r.JoinedAt.Format("2006-01-02 15:04:05"), "1", 0, "C", false, 0, "")
		pdf.CellFormat(widths[5], 6, r.CreatedAt.Format("2006-01-02 15:04:05"), "1", 0, "C", false, 0, "")
		pdf.Ln(-1)
	}

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, "", "", err
	}

	return buf.Bytes(), "devotee_list_report.pdf", "application/pdf", nil
}

func (e *reportExporter) exportAuditLogsPDF(logs []AuditLogReportRow) ([]byte, error) {
	pdf := gofpdf.New("L", "mm", "A4", "")
	pdf.AddPage()
	pdf.SetFont("Arial", "B", 16)
	pdf.Cell(0, 10, "Audit Logs Report")
	pdf.Ln(20)

	pdf.SetFont("Arial", "B", 9)
	widths := []float64{12, 25, 20, 30, 25, 25, 20, 25, 30, 50}
	headers := []string{"ID", "Entity", "User ID", "User Name", "User Role", "Action", "Status", "IP Address", "Timestamp", "Details"}

	for i, h := range headers {
		pdf.CellFormat(widths[i], 7, h, "1", 0, "C", false, 0, "")
	}
	pdf.Ln(-1)

	pdf.SetFont("Arial", "", 8)
	for _, log := range logs {
		userID := ""
		if log.UserID != nil {
			userID = strconv.FormatUint(uint64(*log.UserID), 10)
		}

		values := []string{
			strconv.FormatUint(uint64(log.ID), 10),
			log.EntityName,
			userID,
			log.UserName,
			log.UserRole,
			log.Action,
			log.Status,
			log.IPAddress,
			log.Timestamp.Format("2006-01-02 15:04:05"),
			log.Details,
		}

		for i, v := range values {
			pdf.CellFormat(widths[i], 6, v, "1", 0, "L", false, 0, "")
		}
		pdf.Ln(-1)
	}

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (e *reportExporter) exportAuditLogsExcel(logs []AuditLogReportRow) ([]byte, error) {
	f := excelize.NewFile()
	sheetName := "Audit Logs"
	f.SetSheetName("Sheet1", sheetName)

	headers := []string{"ID", "Entity", "User ID", "User Name", "User Role", "Action", "Status", "IP Address", "Timestamp", "Details"}
	for i, header := range headers {
		cell := fmt.Sprintf("%c1", 'A'+i)
		f.SetCellValue(sheetName, cell, header)
	}

	for i, log := range logs {
		row := i + 2
		userID := ""
		if log.UserID != nil {
			userID = strconv.FormatUint(uint64(*log.UserID), 10)
		}

		f.SetCellValue(sheetName, fmt.Sprintf("A%d", row), log.ID)
		f.SetCellValue(sheetName, fmt.Sprintf("B%d", row), log.EntityName)
		f.SetCellValue(sheetName, fmt.Sprintf("C%d", row), userID)
		f.SetCellValue(sheetName, fmt.Sprintf("D%d", row), log.UserName)
		f.SetCellValue(sheetName, fmt.Sprintf("E%d", row), log.UserRole)
		f.SetCellValue(sheetName, fmt.Sprintf("F%d", row), log.Action)
		f.SetCellValue(sheetName, fmt.Sprintf("G%d", row), log.Status)
		f.SetCellValue(sheetName, fmt.Sprintf("H%d", row), log.IPAddress)
		f.SetCellValue(sheetName, fmt.Sprintf("I%d", row), log.Timestamp.Format("2006-01-02 15:04:05"))
		f.SetCellValue(sheetName, fmt.Sprintf("J%d", row), log.Details)
	}

	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

//// ============================
/// APPROVAL STATUS EXPORTS
//// ============================
// exportApprovalStatusByFormat chooses export format for approval status
func (e *reportExporter) exportApprovalStatusByFormat(format string, rows []ApprovalStatusReportRow) ([]byte, string, string, error) {
	timestamp := time.Now().Format("20060102_150405")
	
	switch format {
	case FormatExcel:
		data, err := e.exportApprovalStatusExcel(rows)
		if err != nil {
			return nil, "", "", err
		}
		filename := fmt.Sprintf("approval_status_report_%s.xlsx", timestamp)
		return data, filename, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", nil
		
	case FormatCSV:
		data, err := e.exportApprovalStatusCSV(rows)
		if err != nil {
			return nil, "", "", err
		}
		filename := fmt.Sprintf("approval_status_report_%s.csv", timestamp)
		return data, filename, "text/csv", nil
		
	case FormatPDF:
		data, err := e.exportApprovalStatusPDF(rows)
		if err != nil {
			return nil, "", "", err
		}
		filename := fmt.Sprintf("approval_status_report_%s.pdf", timestamp)
		return data, filename, "application/pdf", nil
		
	default:
		return nil, "", "", fmt.Errorf("unsupported format for approval status: %s", format)
	}
}

// exportApprovalStatusCSV exports approval status report to CSV with all fields
func (e *reportExporter) exportApprovalStatusCSV(rows []ApprovalStatusReportRow) ([]byte, error) {
	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)

	// Updated headers with all new fields
	headers := []string{
		"Tenant ID", 
		"Tenant Name", 
		"Entity ID", 
		"Entity Name", 
		"Status", 
		"Requested At", 
		"Approved At", 
		"Email", 
		"Role",
	}
	if err := writer.Write(headers); err != nil {
		return nil, err
	}

	for _, row := range rows {
		// Handle nullable ApprovedAt field
		approvedAt := ""
		if row.ApprovedAt != nil {
			approvedAt = row.ApprovedAt.Format("2006-01-02 15:04:05")
		}

		record := []string{
			row.TenantID,
			row.TenantName,
			row.EntityID,
			row.EntityName,
			row.Status,
			row.CreatedAt.Format("2006-01-02 15:04:05"),
			approvedAt,
			row.Email,
			//row.Role,
		}
		if err := writer.Write(record); err != nil {
			return nil, err
		}
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// exportApprovalStatusExcel exports approval status report to Excel with all fields
func (e *reportExporter) exportApprovalStatusExcel(rows []ApprovalStatusReportRow) ([]byte, error) {
	f := excelize.NewFile()
	sheetName := "Approval Status"
	f.SetSheetName("Sheet1", sheetName)

	// Updated headers with all new fields
	headers := []string{
		"Tenant ID", 
		"Tenant Name", 
		"Entity ID", 
		"Entity Name", 
		"Status", 
		"Requested At", 
		"Approved At", 
		"Email", 
		//"Role",
	}
	for i, header := range headers {
		cell := fmt.Sprintf("%c1", 'A'+i)
		f.SetCellValue(sheetName, cell, header)
	}

	for i, row := range rows {
		rowNum := i + 2
		
		// Handle nullable ApprovedAt field
		approvedAt := ""
		if row.ApprovedAt != nil {
			approvedAt = row.ApprovedAt.Format("2006-01-02 15:04:05")
		}

		f.SetCellValue(sheetName, fmt.Sprintf("A%d", rowNum), row.TenantID)
		f.SetCellValue(sheetName, fmt.Sprintf("B%d", rowNum), row.TenantName)
		f.SetCellValue(sheetName, fmt.Sprintf("C%d", rowNum), row.EntityID)
		f.SetCellValue(sheetName, fmt.Sprintf("D%d", rowNum), row.EntityName)
		f.SetCellValue(sheetName, fmt.Sprintf("E%d", rowNum), row.Status)
		f.SetCellValue(sheetName, fmt.Sprintf("F%d", rowNum), row.CreatedAt.Format("2006-01-02 15:04:05"))
		f.SetCellValue(sheetName, fmt.Sprintf("G%d", rowNum), approvedAt)
		f.SetCellValue(sheetName, fmt.Sprintf("H%d", rowNum), row.Email)
		//f.SetCellValue(sheetName, fmt.Sprintf("I%d", rowNum), row.Role)
	}

	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// exportApprovalStatusPDF exports approval status report to PDF with all fields
func (e *reportExporter) exportApprovalStatusPDF(rows []ApprovalStatusReportRow) ([]byte, error) {
	pdf := gofpdf.New("L", "mm", "A4", "")
	pdf.AddPage()
	pdf.SetFont("Arial", "B", 16)
	pdf.Cell(0, 10, "Approval Status Report")
	pdf.Ln(20)

	pdf.SetFont("Arial", "B", 8)
	// Updated column widths for all fields (total: ~280mm fits landscape A4)
	widths := []float64{20, 35, 20, 40, 20, 35, 35, 50, 25}
	headers := []string{
		"Tenant ID", 
		"Tenant Name", 
		"Entity ID", 
		"Entity Name", 
		"Status", 
		"Requested At", 
		"Approved At", 
		"Email", 
		//"Role",
	}

	for i, h := range headers {
		pdf.CellFormat(widths[i], 7, h, "1", 0, "C", false, 0, "")
	}
	pdf.Ln(-1)

	pdf.SetFont("Arial", "", 7)
	for _, row := range rows {
		// Handle nullable ApprovedAt field
		approvedAt := "N/A"
		if row.ApprovedAt != nil {
			approvedAt = row.ApprovedAt.Format("2006-01-02 15:04")
		}

		values := []string{
			row.TenantID,
			row.TenantName,
			row.EntityID,
			row.EntityName,
			row.Status,
			row.CreatedAt.Format("2006-01-02 15:04"),
			approvedAt,
			row.Email,
			//row.Role,
		}

		for i, v := range values {
			// Truncate long values for PDF display to fit in cells
			maxLen := 30
			switch i {
case 1, 3: // Tenant Name or Entity Name
				maxLen = 25
			case 7: // Email
				maxLen = 35
			}
			
			if len(v) > maxLen {
				v = v[:maxLen-3] + "..."
			}
			
			pdf.CellFormat(widths[i], 6, v, "1", 0, "L", false, 0, "")
		}
		pdf.Ln(-1)
	}

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
//// ============================
/// USER DETAILS EXPORTS
//// ============================

func (e *reportExporter) exportUserDetailsByFormat(format string, rows []UserDetailsReportRow) ([]byte, string, string, error) {
	timestamp := time.Now().Format("20060102_150405")

	switch format {
	case FormatExcel:
		data, err := e.exportUserDetailsExcel(rows)
		if err != nil {
			return nil, "", "", err
		}
		filename := fmt.Sprintf("user_details_report_%s.xlsx", timestamp)
		return data, filename, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", nil

	case FormatCSV:
		data, err := e.exportUserDetailsCSV(rows)
		if err != nil {
			return nil, "", "", err
		}
		filename := fmt.Sprintf("user_details_report_%s.csv", timestamp)
		return data, filename, "text/csv", nil

	case FormatPDF:
		data, err := e.exportUserDetailsPDF(rows)
		if err != nil {
			return nil, "", "", err
		}
		filename := fmt.Sprintf("user_details_report_%s.pdf", timestamp)
		return data, filename, "application/pdf", nil

	default:
		return nil, "", "", fmt.Errorf("unsupported format for user details: %s", format)
	}
}

func (e *reportExporter) exportUserDetailsCSV(rows []UserDetailsReportRow) ([]byte, error) {
	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)

	headers := []string{"ID", "User Name", "Entity Name", "Email", "Role", "Status", "Created At"}
	if err := writer.Write(headers); err != nil {
		return nil, err
	}

	for _, row := range rows {
		record := []string{
			strconv.FormatUint(uint64(row.ID), 10),
			row.Name,
			row.EntityName,
			row.Email,
			row.Role,
			row.Status,
			row.CreatedAt.Format("2006-01-02 15:04:05"),
		}
		if err := writer.Write(record); err != nil {
			return nil, err
		}
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func (e *reportExporter) exportUserDetailsExcel(rows []UserDetailsReportRow) ([]byte, error) {
	f := excelize.NewFile()
	sheetName := "User Details"
	f.SetSheetName("Sheet1", sheetName)

	headers := []string{"ID", "User Name", "Entity Name", "Email", "Role", "Status", "Created At"}
	for i, header := range headers {
		cell := fmt.Sprintf("%c1", 'A'+i)
		f.SetCellValue(sheetName, cell, header)
	}

	for i, row := range rows {
		rowNum := i + 2
		f.SetCellValue(sheetName, fmt.Sprintf("A%d", rowNum), row.ID)
		f.SetCellValue(sheetName, fmt.Sprintf("B%d", rowNum), row.Name)
		f.SetCellValue(sheetName, fmt.Sprintf("C%d", rowNum), row.EntityName)
		f.SetCellValue(sheetName, fmt.Sprintf("D%d", rowNum), row.Email)
		f.SetCellValue(sheetName, fmt.Sprintf("E%d", rowNum), row.Role)
		f.SetCellValue(sheetName, fmt.Sprintf("F%d", rowNum), row.Status)
		f.SetCellValue(sheetName, fmt.Sprintf("G%d", rowNum), row.CreatedAt.Format("2006-01-02 15:04:05"))
	}

	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (e *reportExporter) exportUserDetailsPDF(rows []UserDetailsReportRow) ([]byte, error) {
	pdf := gofpdf.New("L", "mm", "A4", "")
	pdf.AddPage()
	pdf.SetFont("Arial", "B", 16)
	pdf.Cell(0, 10, "User Details Report")
	pdf.Ln(20)

	pdf.SetFont("Arial", "B", 10)
	widths := []float64{15, 40, 40, 50, 30, 25, 35}
	headers := []string{"ID", "User Name", "Entity Name", "Email", "Role", "Status", "Created At"}

	for i, h := range headers {
		pdf.CellFormat(widths[i], 7, h, "1", 0, "C", false, 0, "")
	}
	pdf.Ln(-1)

	pdf.SetFont("Arial", "", 8)
	for _, row := range rows {
		values := []string{
			strconv.FormatUint(uint64(row.ID), 10),
			row.Name,
			row.EntityName,
			row.Email,
			row.Role,
			row.Status,
			row.CreatedAt.Format("2006-01-02 15:04:05"),
		}

		for i, v := range values {
			pdf.CellFormat(widths[i], 6, v, "1", 0, "L", false, 0, "")
		}
		pdf.Ln(-1)
	}

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Export Events by format
func (e *reportExporter) exportEventsByFormat(format, timestamp string, events []EventReportRow) ([]byte, string, string, error) {
	switch format {
	case FormatExcel:
		data, err := e.exportEventsExcel(events)
		if err != nil {
			return nil, "", "", err
		}
		filename := fmt.Sprintf("events_report_%s.xlsx", timestamp)
		return data, filename, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", nil

	case FormatCSV:
		data, err := e.exportEventsCSV(events)
		if err != nil {
			return nil, "", "", err
		}
		filename := fmt.Sprintf("events_report_%s.csv", timestamp)
		return data, filename, "text/csv", nil

	case FormatPDF:
		data, err := e.exportEventsPDF(events)
		if err != nil {
			return nil, "", "", err
		}
		filename := fmt.Sprintf("events_report_%s.pdf", timestamp)
		return data, filename, "application/pdf", nil

	default:
		return nil, "", "", fmt.Errorf("unsupported format for events: %s", format)
	}
}

// Export Sevas by format
func (e *reportExporter) exportSevasByFormat(format, timestamp string, sevas []SevaReportRow) ([]byte, string, string, error) {
	switch format {
	case FormatExcel:
		data, err := e.exportSevasExcel(sevas)
		if err != nil {
			return nil, "", "", err
		}
		filename := fmt.Sprintf("sevas_report_%s.xlsx", timestamp)
		return data, filename, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", nil

	case FormatCSV:
		data, err := e.exportSevasCSV(sevas)
		if err != nil {
			return nil, "", "", err
		}
		filename := fmt.Sprintf("sevas_report_%s.csv", timestamp)
		return data, filename, "text/csv", nil

	case FormatPDF:
		data, err := e.exportSevasPDF(sevas)
		if err != nil {
			return nil, "", "", err
		}
		filename := fmt.Sprintf("sevas_report_%s.pdf", timestamp)
		return data, filename, "application/pdf", nil

	default:
		return nil, "", "", fmt.Errorf("unsupported format for sevas: %s", format)
	}
}

// Export Bookings by format
func (e *reportExporter) exportBookingsByFormat(format, timestamp string, bookings []SevaBookingReportRow) ([]byte, string, string, error) {
	switch format {
	case FormatExcel:
		data, err := e.exportBookingsExcel(bookings)
		if err != nil {
			return nil, "", "", err
		}
		filename := fmt.Sprintf("bookings_report_%s.xlsx", timestamp)
		return data, filename, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", nil

	case FormatCSV:
		data, err := e.exportBookingsCSV(bookings)
		if err != nil {
			return nil, "", "", err
		}
		filename := fmt.Sprintf("bookings_report_%s.csv", timestamp)
		return data, filename, "text/csv", nil

	case FormatPDF:
		data, err := e.exportBookingsPDF(bookings)
		if err != nil {
			return nil, "", "", err
		}
		filename := fmt.Sprintf("bookings_report_%s.pdf", timestamp)
		return data, filename, "application/pdf", nil

	default:
		return nil, "", "", fmt.Errorf("unsupported format for bookings: %s", format)
	}
}

func (e *reportExporter) exportEventsExcel(events []EventReportRow) ([]byte, error) {
	f := excelize.NewFile()
	sheetName := "Events"
	f.SetSheetName("Sheet1", sheetName)

	// Headers - UPDATED with Temple Name
	headers := []string{"Title", "Temple Name", "Description", "Event Type", "Event Date", "Event Time", "Location", "Created By", "Created At", "Updated At"}
	for i, header := range headers {
		cell := fmt.Sprintf("%c1", 'A'+i)
		f.SetCellValue(sheetName, cell, header)
	}

	// Data
	for i, event := range events {
		row := i + 2
		f.SetCellValue(sheetName, fmt.Sprintf("A%d", row), event.Title)
		f.SetCellValue(sheetName, fmt.Sprintf("B%d", row), event.TempleName)
		f.SetCellValue(sheetName, fmt.Sprintf("C%d", row), event.Description)
		f.SetCellValue(sheetName, fmt.Sprintf("D%d", row), event.EventType)
		f.SetCellValue(sheetName, fmt.Sprintf("E%d", row), event.EventDate.Format("2006-01-02"))
		f.SetCellValue(sheetName, fmt.Sprintf("F%d", row), event.EventTime)
		f.SetCellValue(sheetName, fmt.Sprintf("G%d", row), event.Location)
		f.SetCellValue(sheetName, fmt.Sprintf("H%d", row), event.CreatedBy)
		f.SetCellValue(sheetName, fmt.Sprintf("I%d", row), event.CreatedAt.Format("2006-01-02 15:04:05"))
		f.SetCellValue(sheetName, fmt.Sprintf("J%d", row), event.UpdatedAt.Format("2006-01-02 15:04:05"))
		//f.SetCellValue(sheetName, fmt.Sprintf("K%d", row), event.IsActive)
	}

	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (e *reportExporter) exportEventsCSV(events []EventReportRow) ([]byte, error) {
	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)

	// Headers - UPDATED with Temple Name
	headers := []string{"Title", "Temple Name", "Description", "Event Type", "Event Date", "Event Time", "Location", "Created By", "Created At", "Updated At"}
	if err := writer.Write(headers); err != nil {
		return nil, err
	}

	// Data
	for _, event := range events {
		record := []string{
			event.Title,
			event.TempleName,
			event.Description,
			event.EventType,
			event.EventDate.Format("2006-01-02"),
			event.EventTime,
			event.Location,
			strconv.FormatUint(uint64(event.CreatedBy), 10),
			event.CreatedAt.Format("2006-01-02 15:04:05"),
			event.UpdatedAt.Format("2006-01-02 15:04:05"),
			//strconv.FormatBool(event.IsActive),
		}
		if err := writer.Write(record); err != nil {
			return nil, err
		}
	}

	// Important: Flush before getting bytes
	writer.Flush()
	if err := writer.Error(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func (e *reportExporter) exportEventsPDF(events []EventReportRow) ([]byte, error) {
	pdf := gofpdf.New("L", "mm", "A4", "")
	pdf.AddPage()
	pdf.SetFont("Arial", "B", 16)
	pdf.Cell(0, 10, "Events Report")
	pdf.Ln(20)

	pdf.SetFont("Arial", "B", 10)
	// Define column widths - UPDATED with Temple Name
	widths := []float64{40, 30, 30, 25, 20, 30, 25, 15}
	headers := []string{"Title", "Temple Name", "Event Type", "Date", "Time", "Location", "Created At"}

	// Print headers with borders
	for i, header := range headers {
		pdf.CellFormat(widths[i], 7, header, "1", 0, "C", false, 0, "")
	}
	pdf.Ln(-1)

	// Print data rows with borders
	pdf.SetFont("Arial", "", 8)
	for _, event := range events {
		pdf.CellFormat(widths[0], 6, event.Title, "1", 0, "L", false, 0, "")
		pdf.CellFormat(widths[1], 6, event.TempleName, "1", 0, "L", false, 0, "")
		pdf.CellFormat(widths[2], 6, event.EventType, "1", 0, "L", false, 0, "")
		pdf.CellFormat(widths[3], 6, event.EventDate.Format("02-01-06"), "1", 0, "L", false, 0, "")
		pdf.CellFormat(widths[4], 6, event.EventTime, "1", 0, "L", false, 0, "")
		pdf.CellFormat(widths[5], 6, event.Location, "1", 0, "L", false, 0, "")
		pdf.CellFormat(widths[6], 6, event.CreatedAt.Format("02-01-06"), "1", 0, "C", false, 0, "")
		//pdf.CellFormat(widths[7], 6, strconv.FormatBool(event.IsActive), "1", 0, "C", false, 0, "")
		pdf.Ln(-1)
	}

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (e *reportExporter) exportSevasExcel(sevas []SevaReportRow) ([]byte, error) {
	f := excelize.NewFile()
	sheetName := "Sevas"
	f.SetSheetName("Sheet1", sheetName)

	// UPDATED with Temple Name
	headers := []string{"Name", "Temple Name", "Seva Type", "Description", "Price", "Date", "Start Time", "End Time", "Duration", "Max Bookings", "Status", "Is Active", "Created At", "Updated At"}
	for i, header := range headers {
		cell := fmt.Sprintf("%c1", 'A'+i)
		f.SetCellValue(sheetName, cell, header)
	}

	for i, seva := range sevas {
		row := i + 2
		f.SetCellValue(sheetName, fmt.Sprintf("A%d", row), seva.Name)
		f.SetCellValue(sheetName, fmt.Sprintf("B%d", row), seva.TempleName)
		f.SetCellValue(sheetName, fmt.Sprintf("C%d", row), seva.SevaType)
		f.SetCellValue(sheetName, fmt.Sprintf("D%d", row), seva.Description)
		f.SetCellValue(sheetName, fmt.Sprintf("E%d", row), seva.Price)
		f.SetCellValue(sheetName, fmt.Sprintf("F%d", row), seva.Date.Format("2006-01-02"))
		f.SetCellValue(sheetName, fmt.Sprintf("G%d", row), seva.StartTime)
		f.SetCellValue(sheetName, fmt.Sprintf("H%d", row), seva.EndTime)
		f.SetCellValue(sheetName, fmt.Sprintf("I%d", row), seva.Duration)
		f.SetCellValue(sheetName, fmt.Sprintf("J%d", row), seva.MaxBookingsPerDay)
		f.SetCellValue(sheetName, fmt.Sprintf("K%d", row), seva.Status)
		f.SetCellValue(sheetName, fmt.Sprintf("L%d", row), seva.IsActive)
		f.SetCellValue(sheetName, fmt.Sprintf("M%d", row), seva.CreatedAt.Format("2006-01-02 15:04:05"))
		f.SetCellValue(sheetName, fmt.Sprintf("N%d", row), seva.UpdatedAt.Format("2006-01-02 15:04:05"))
	}

	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (e *reportExporter) exportSevasCSV(sevas []SevaReportRow) ([]byte, error) {
	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)

	headers := []string{"Name", "Temple Name", "Seva Type", "Description", "Price", "Date", "Start Time", "End Time", "Duration", "Max Bookings", "Status", "Is Active", "Created At", "Updated At"}
	if err := writer.Write(headers); err != nil {
		return nil, err
	}

	for _, seva := range sevas {
		record := []string{
			seva.Name,
			seva.TempleName,
			seva.SevaType,
			seva.Description,
			fmt.Sprintf("%.2f", seva.Price),
			seva.Date.Format("2006-01-02"),
			seva.StartTime,
			seva.EndTime,
			strconv.Itoa(seva.Duration),
			strconv.Itoa(seva.MaxBookingsPerDay),
			seva.Status,
			strconv.FormatBool(seva.IsActive),
			seva.CreatedAt.Format("2006-01-02 15:04:05"),
			seva.UpdatedAt.Format("2006-01-02 15:04:05"),
		}
		if err := writer.Write(record); err != nil {
			return nil, err
		}
	}

	// Important: Flush before getting bytes
	writer.Flush()
	if err := writer.Error(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func (e *reportExporter) exportSevasPDF(sevas []SevaReportRow) ([]byte, error) {
	fmt.Println("Sevas:-", sevas)
	pdf := gofpdf.New("L", "mm", "A4", "")
	pdf.AddPage()
	pdf.SetFont("Arial", "B", 16)
	pdf.Cell(0, 10, "Sevas Report")
	pdf.Ln(20)

	pdf.SetFont("Arial", "B", 10)
	// Define column widths - UPDATED with Temple Name
	widths := []float64{40, 40, 20, 20, 25, 25, 15, 20, 15}
	headers := []string{"Name", "Temple Name", "Type", "Price", "Start Time", "End Time", "Duration", "Status", "Active"}

	// Print headers with borders
	for i, header := range headers {
		pdf.CellFormat(widths[i], 7, header, "1", 0, "C", false, 0, "")
	}
	pdf.Ln(-1)

	// Print data rows with borders
	pdf.SetFont("Arial", "", 8)
	for _, seva := range sevas {
		pdf.CellFormat(widths[0], 6, seva.Name, "1", 0, "L", false, 0, "")
		pdf.CellFormat(widths[1], 6, seva.TempleName, "1", 0, "L", false, 0, "")
		pdf.CellFormat(widths[2], 6, seva.SevaType, "1", 0, "L", false, 0, "")
		pdf.CellFormat(widths[3], 6, fmt.Sprintf("%.2f", seva.Price), "1", 0, "R", false, 0, "")
		pdf.CellFormat(widths[4], 6, seva.StartTime, "1", 0, "C", false, 0, "")
		pdf.CellFormat(widths[5], 6, seva.EndTime, "1", 0, "C", false, 0, "")
		pdf.CellFormat(widths[6], 6, strconv.Itoa(seva.Duration), "1", 0, "C", false, 0, "")
		pdf.CellFormat(widths[7], 6, seva.Status, "1", 0, "C", false, 0, "")
		pdf.CellFormat(widths[8], 6, strconv.FormatBool(seva.IsActive), "1", 0, "C", false, 0, "")
		pdf.Ln(-1)
	}

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (e *reportExporter) exportBookingsExcel(bookings []SevaBookingReportRow) ([]byte, error) {
	f := excelize.NewFile()
	sheetName := "Bookings"
	f.SetSheetName("Sheet1", sheetName)

	// UPDATED with Temple Name
	headers := []string{"Seva Name", "Temple Name", "Seva Type", "Devotee Name", "Devotee Phone", "Booking Time", "Status", "Created At", "Updated At"}
	for i, header := range headers {
		cell := fmt.Sprintf("%c1", 'A'+i)
		f.SetCellValue(sheetName, cell, header)
	}

	for i, booking := range bookings {
		row := i + 2
		f.SetCellValue(sheetName, fmt.Sprintf("A%d", row), booking.SevaName)
		f.SetCellValue(sheetName, fmt.Sprintf("B%d", row), booking.TempleName)
		f.SetCellValue(sheetName, fmt.Sprintf("C%d", row), booking.SevaType)
		f.SetCellValue(sheetName, fmt.Sprintf("D%d", row), booking.DevoteeName)
		f.SetCellValue(sheetName, fmt.Sprintf("E%d", row), booking.DevoteePhone)
		f.SetCellValue(sheetName, fmt.Sprintf("F%d", row), booking.BookingTime.Format("2006-01-02 15:04:05"))
		f.SetCellValue(sheetName, fmt.Sprintf("G%d", row), booking.Status)
		f.SetCellValue(sheetName, fmt.Sprintf("H%d", row), booking.CreatedAt.Format("2006-01-02 15:04:05"))
		f.SetCellValue(sheetName, fmt.Sprintf("I%d", row), booking.UpdatedAt.Format("2006-01-02 15:04:05"))
	}

	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (e *reportExporter) exportBookingsCSV(bookings []SevaBookingReportRow) ([]byte, error) {
	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)

	// UPDATED with Temple Name
	headers := []string{"Seva Name", "Temple Name", "Seva Type", "Devotee Name", "Devotee Phone", "Booking Time", "Status", "Created At", "Updated At"}
	if err := writer.Write(headers); err != nil {
		return nil, err
	}

	for _, booking := range bookings {
		record := []string{
			booking.SevaName,
			booking.TempleName,
			booking.SevaType,
			booking.DevoteeName,
			booking.DevoteePhone,
			booking.BookingTime.Format("2006-01-02 15:04:05"),
			booking.Status,
			booking.CreatedAt.Format("2006-01-02 15:04:05"),
			booking.UpdatedAt.Format("2006-01-02 15:04:05"),
		}
		if err := writer.Write(record); err != nil {
			return nil, err
		}
	}

	// Important: Flush before getting bytes
	writer.Flush()
	if err := writer.Error(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func (e *reportExporter) exportBookingsPDF(bookings []SevaBookingReportRow) ([]byte, error) {
	pdf := gofpdf.New("L", "mm", "A4", "")
	pdf.AddPage()
	pdf.SetFont("Arial", "B", 16)
	pdf.Cell(0, 10, "Seva Bookings Report")
	pdf.Ln(20)

	pdf.SetFont("Arial", "B", 10)
	// Define column widths - UPDATED with Temple Name
	widths := []float64{40, 40, 25, 35, 30, 35, 20}
	headers := []string{"Seva Name", "Temple Name", "Seva Type", "Devotee Name", "Phone", "Booking Time", "Status"}

	// Print headers with borders
	for i, header := range headers {
		pdf.CellFormat(widths[i], 7, header, "1", 0, "C", false, 0, "")
	}
	pdf.Ln(-1)

	// Print data rows with borders
	pdf.SetFont("Arial", "", 8)
	for _, booking := range bookings {
		pdf.CellFormat(widths[0], 6, booking.SevaName, "1", 0, "L", false, 0, "")
		pdf.CellFormat(widths[1], 6, booking.TempleName, "1", 0, "L", false, 0, "")
		pdf.CellFormat(widths[2], 6, booking.SevaType, "1", 0, "L", false, 0, "")
		pdf.CellFormat(widths[3], 6, booking.DevoteeName, "1", 0, "L", false, 0, "")
		pdf.CellFormat(widths[4], 6, booking.DevoteePhone, "1", 0, "C", false, 0, "")
		pdf.CellFormat(widths[5], 6, booking.BookingTime.Format("02-01-06 15:04"), "1", 0, "C", false, 0, "")
		pdf.CellFormat(widths[6], 6, booking.Status, "1", 0, "C", false, 0, "")
		pdf.Ln(-1)
	}

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}