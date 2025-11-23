package seva

import (
    "context"
    "errors"
    "fmt"
    "time"

    "github.com/sharath018/temple-management-backend/internal/auditlog"
    "github.com/sharath018/temple-management-backend/internal/notification"
    "github.com/sharath018/temple-management-backend/middleware"
)

type Service interface {
    // Seva Core
    CreateSeva(ctx context.Context, seva *Seva, accessContext middleware.AccessContext, ip string) error
    UpdateSeva(ctx context.Context, seva *Seva, accessContext middleware.AccessContext, ip string) error
    DeleteSeva(ctx context.Context, sevaID uint, accessContext middleware.AccessContext, ip string) error
    GetSevasByEntity(ctx context.Context, entityID uint) ([]Seva, error)
    GetSevaByID(ctx context.Context, id uint) (*Seva, error)

    // Enhanced seva listing with filters for temple admin
    GetSevasWithFilters(ctx context.Context, entityID uint, sevaType, search, status string, limit, offset int) ([]Seva, int64, error)

    // Booking Core
    BookSeva(ctx context.Context, booking *SevaBooking, userRole string, userID uint, entityID uint, ip string) error
    GetBookingsForUser(ctx context.Context, userID uint) ([]SevaBooking, error)
    GetBookingsForEntity(ctx context.Context, entityID uint) ([]SevaBooking, error)
    UpdateBookingStatus(ctx context.Context, bookingID uint, newStatus string, userID uint, ip string) error

    // Composite Booking Details
    GetDetailedBookingsForEntity(ctx context.Context, entityID uint) ([]DetailedBooking, error)

    // Filters, Search, Pagination
    SearchBookings(ctx context.Context, filter BookingFilter) ([]DetailedBooking, int64, error)

    // Counts
    GetBookingCountsByStatus(ctx context.Context, entityID uint) (BookingStatusCounts, error)

    GetDetailedBookingsWithFilters(ctx context.Context, entityID uint, status, sevaType, startDate, endDate, search string, limit, offset int) ([]DetailedBooking, error)
    GetBookingByID(ctx context.Context, bookingID uint) (*SevaBooking, error)
    GetBookingStatusCounts(ctx context.Context, entityID uint) (BookingStatusCounts, error)

    GetPaginatedSevas(ctx context.Context, entityID uint, sevaType string, search string, limit int, offset int) ([]Seva, error)

    // Get approved booking counts per seva
    GetApprovedBookingCountsPerSeva(ctx context.Context, entityID uint) (map[uint]int64, error)

    SetNotifService(n notification.Service)
}

type service struct {
    repo     Repository
    auditSvc auditlog.Service
    notifSvc notification.Service
}

func NewService(repo Repository, auditSvc auditlog.Service) Service {
    return &service{
        repo:     repo,
        auditSvc: auditSvc,
    }
}

func (s *service) SetNotifService(n notification.Service) {
    s.notifSvc = n
}

func (s *service) CreateSeva(ctx context.Context, seva *Seva, accessContext middleware.AccessContext, ip string) error {
    if !accessContext.CanWrite() {
        s.auditSvc.LogAction(ctx, &accessContext.UserID, accessContext.GetAccessibleEntityID(), "SEVA_CREATE_FAILED", map[string]interface{}{
            "reason":    "write access denied",
            "seva_name": seva.Name,
        }, ip, "failure")
        return errors.New("write access denied")
    }

    entityID := accessContext.GetAccessibleEntityID()
    if entityID == nil {
        s.auditSvc.LogAction(ctx, &accessContext.UserID, nil, "SEVA_CREATE_FAILED", map[string]interface{}{
            "reason":    "no accessible entity",
            "seva_name": seva.Name,
        }, ip, "failure")
        return errors.New("no accessible entity")
    }

    validStatuses := map[string]bool{"upcoming": true, "ongoing": true, "completed": true}
    if seva.Status == "" {
        seva.Status = "upcoming"
    } else if !validStatuses[seva.Status] {
        s.auditSvc.LogAction(ctx, &accessContext.UserID, entityID, "SEVA_CREATE_FAILED", map[string]interface{}{
            "reason":         "invalid status",
            "seva_name":      seva.Name,
            "invalid_status": seva.Status,
        }, ip, "failure")
        return errors.New("invalid status. Must be 'upcoming', 'ongoing', or 'completed'")
    }

    seva.EntityID = *entityID
    // ✅ UPDATED: Initialize slot fields
    seva.BookedSlots = 0
    seva.RemainingSlots = seva.AvailableSlots

    err := s.repo.CreateSeva(ctx, seva)
    if err != nil {
        s.auditSvc.LogAction(ctx, &accessContext.UserID, entityID, "SEVA_CREATE_FAILED", map[string]interface{}{
            "seva_name": seva.Name,
            "seva_type": seva.SevaType,
            "status":    seva.Status,
            "error":     err.Error(),
        }, ip, "failure")
        return err
    }

    s.auditSvc.LogAction(ctx, &accessContext.UserID, entityID, "SEVA_CREATED", map[string]interface{}{
        "seva_id":         seva.ID,
        "seva_name":       seva.Name,
        "seva_type":       seva.SevaType,
        "price":           seva.Price,
        "status":          seva.Status,
        "available_slots": seva.AvailableSlots,
        "booked_slots":    seva.BookedSlots,
        "remaining_slots": seva.RemainingSlots,
        "role":            accessContext.RoleName,
    }, ip, "success")

    if s.notifSvc != nil {
        _ = s.notifSvc.CreateInAppForEntityRoles(
            ctx,
            *entityID,
            []string{"devotee", "volunteer"},
            "New Seva",
            seva.Name+" has been added",
            "seva",
        )
    }

    return nil
}

func (s *service) UpdateSeva(ctx context.Context, seva *Seva, accessContext middleware.AccessContext, ip string) error {
    if !accessContext.CanWrite() {
        s.auditSvc.LogAction(ctx, &accessContext.UserID, accessContext.GetAccessibleEntityID(), "SEVA_UPDATE_FAILED", map[string]interface{}{
            "reason":  "write access denied",
            "seva_id": seva.ID,
        }, ip, "failure")
        return errors.New("write access denied")
    }

    entityID := accessContext.GetAccessibleEntityID()
    if entityID == nil {
        s.auditSvc.LogAction(ctx, &accessContext.UserID, nil, "SEVA_UPDATE_FAILED", map[string]interface{}{
            "reason":  "no accessible entity",
            "seva_id": seva.ID,
        }, ip, "failure")
        return errors.New("no accessible entity")
    }

    if seva.Status != "" {
        validStatuses := map[string]bool{"upcoming": true, "ongoing": true, "completed": true}
        if !validStatuses[seva.Status] {
            s.auditSvc.LogAction(ctx, &accessContext.UserID, entityID, "SEVA_UPDATE_FAILED", map[string]interface{}{
                "reason":         "invalid status",
                "seva_id":        seva.ID,
                "invalid_status": seva.Status,
            }, ip, "failure")
            return errors.New("invalid status. Must be 'upcoming', 'ongoing', or 'completed'")
        }
    }

    // ✅ UPDATED: Recalculate remaining slots before update
    seva.RemainingSlots = seva.AvailableSlots - seva.BookedSlots
    if seva.RemainingSlots < 0 {
        seva.RemainingSlots = 0
    }

    err := s.repo.UpdateSeva(ctx, seva)
    if err != nil {
        s.auditSvc.LogAction(ctx, &accessContext.UserID, entityID, "SEVA_UPDATE_FAILED", map[string]interface{}{
            "seva_id":   seva.ID,
            "seva_name": seva.Name,
            "error":     err.Error(),
        }, ip, "failure")
        return err
    }

    s.auditSvc.LogAction(ctx, &accessContext.UserID, entityID, "SEVA_UPDATED", map[string]interface{}{
        "seva_id":         seva.ID,
        "seva_name":       seva.Name,
        "seva_type":       seva.SevaType,
        "price":           seva.Price,
        "status":          seva.Status,
        "available_slots": seva.AvailableSlots,
        "booked_slots":    seva.BookedSlots,
        "remaining_slots": seva.RemainingSlots,
        "role":            accessContext.RoleName,
    }, ip, "success")

    if s.notifSvc != nil && accessContext.GetAccessibleEntityID() != nil {
        _ = s.notifSvc.CreateInAppForEntityRoles(
            ctx,
            *entityID,
            []string{"devotee", "volunteer"},
            "Seva Updated",
            seva.Name+" has been updated",
            "seva",
        )
    }

    return nil
}

func (s *service) DeleteSeva(ctx context.Context, sevaID uint, accessContext middleware.AccessContext, ip string) error {
    if !accessContext.CanWrite() {
        s.auditSvc.LogAction(ctx, &accessContext.UserID, accessContext.GetAccessibleEntityID(), "SEVA_DELETE_FAILED", map[string]interface{}{
            "reason":  "write access denied",
            "seva_id": sevaID,
        }, ip, "failure")
        return errors.New("write access denied")
    }

    entityID := accessContext.GetAccessibleEntityID()
    if entityID == nil {
        s.auditSvc.LogAction(ctx, &accessContext.UserID, nil, "SEVA_DELETE_FAILED", map[string]interface{}{
            "reason":  "no accessible entity",
            "seva_id": sevaID,
        }, ip, "failure")
        return errors.New("no accessible entity")
    }

    seva, err := s.repo.GetSevaByID(ctx, sevaID)
    if err != nil {
        s.auditSvc.LogAction(ctx, &accessContext.UserID, entityID, "SEVA_DELETE_FAILED", map[string]interface{}{
            "seva_id": sevaID,
            "reason":  "seva not found",
            "error":   err.Error(),
        }, ip, "failure")
        return err
    }

    if seva.EntityID != *entityID {
        s.auditSvc.LogAction(ctx, &accessContext.UserID, entityID, "SEVA_DELETE_FAILED", map[string]interface{}{
            "seva_id": sevaID,
            "reason":  "access denied to this seva",
        }, ip, "failure")
        return errors.New("access denied to this seva")
    }

    bookings, err := s.repo.ListBookingsByEntityID(ctx, *entityID)
    if err == nil {
        hasBookings := false
        for _, booking := range bookings {
            if booking.SevaID == sevaID {
                hasBookings = true
                break
            }
        }

        if hasBookings {
            s.auditSvc.LogAction(ctx, &accessContext.UserID, entityID, "SEVA_DELETE_FAILED", map[string]interface{}{
                "seva_id":   sevaID,
                "seva_name": seva.Name,
                "reason":    "seva has existing bookings",
            }, ip, "failure")
            return errors.New("cannot delete seva with existing bookings")
        }
    }

    err = s.repo.DeleteSeva(ctx, sevaID)
    if err != nil {
        s.auditSvc.LogAction(ctx, &accessContext.UserID, entityID, "SEVA_DELETE_FAILED", map[string]interface{}{
            "seva_id":   sevaID,
            "seva_name": seva.Name,
            "error":     err.Error(),
        }, ip, "failure")
        return err
    }

    s.auditSvc.LogAction(ctx, &accessContext.UserID, entityID, "SEVA_DELETED_PERMANENTLY", map[string]interface{}{
        "seva_id":   sevaID,
        "seva_name": seva.Name,
        "seva_type": seva.SevaType,
        "price":     seva.Price,
        "status":    seva.Status,
        "role":      accessContext.RoleName,
    }, ip, "success")

    if s.notifSvc != nil {
        _ = s.notifSvc.CreateInAppForEntityRoles(
            ctx,
            *entityID,
            []string{"devotee", "volunteer"},
            "Seva Deleted",
            seva.Name+" has been removed",
            "seva",
        )
    }

    return nil
}

func (s *service) GetSevasByEntity(ctx context.Context, entityID uint) ([]Seva, error) {
    return s.repo.ListSevasByEntityID(ctx, entityID)
}

func (s *service) GetSevaByID(ctx context.Context, id uint) (*Seva, error) {
    return s.repo.GetSevaByID(ctx, id)
}

func (s *service) GetSevasWithFilters(ctx context.Context, entityID uint, sevaType, search, status string, limit, offset int) ([]Seva, int64, error) {
    return s.repo.GetSevasWithFilters(ctx, entityID, sevaType, search, status, limit, offset)
}

// ✅ UPDATED: BookSeva with slot availability check using RemainingSlots
func (s *service) BookSeva(ctx context.Context, booking *SevaBooking, userRole string, userID uint, entityID uint, ip string) error {
    if userRole != "devotee" {
        s.auditSvc.LogAction(ctx, &userID, &entityID, "SEVA_BOOKING_FAILED", map[string]interface{}{
            "reason":  "unauthorized access",
            "seva_id": booking.SevaID,
        }, ip, "failure")
        return errors.New("unauthorized: only devotee can book sevas")
    }

    // Validate Seva exists and is bookable
    seva, err := s.repo.GetSevaByID(ctx, booking.SevaID)
    if err != nil {
        s.auditSvc.LogAction(ctx, &userID, &entityID, "SEVA_BOOKING_FAILED", map[string]interface{}{
            "seva_id": booking.SevaID,
            "reason":  "seva not found",
            "error":   err.Error(),
        }, ip, "failure")
        return err
    }

    // Check if seva is bookable
    if seva.Status != "upcoming" && seva.Status != "ongoing" {
        s.auditSvc.LogAction(ctx, &userID, &entityID, "SEVA_BOOKING_FAILED", map[string]interface{}{
            "seva_id":     booking.SevaID,
            "seva_name":   seva.Name,
            "seva_status": seva.Status,
            "reason":      "seva is not bookable",
        }, ip, "failure")
        return errors.New("seva is not available for booking")
    }

    // ✅ CRITICAL: Check remaining slots (booking will be pending, not yet approved)
    // We only check if there are available slots, approval will increment BookedSlots
    if seva.AvailableSlots > 0 && seva.RemainingSlots <= 0 {
        s.auditSvc.LogAction(ctx, &userID, &entityID, "SEVA_BOOKING_FAILED", map[string]interface{}{
            "seva_id":         booking.SevaID,
            "seva_name":       seva.Name,
            "reason":          "no slots available",
            "available_slots": seva.AvailableSlots,
            "booked_slots":    seva.BookedSlots,
            "remaining_slots": seva.RemainingSlots,
        }, ip, "failure")
        return errors.New("no slots available for this seva")
    }

    booking.UserID = userID
    booking.EntityID = entityID
    booking.BookingTime = time.Now()
    booking.Status = "pending"

    // Create booking
    err = s.repo.BookSeva(ctx, booking)
    if err != nil {
        s.auditSvc.LogAction(ctx, &userID, &entityID, "SEVA_BOOKING_FAILED", map[string]interface{}{
            "seva_id":   booking.SevaID,
            "seva_name": seva.Name,
            "error":     err.Error(),
        }, ip, "failure")
        return err
    }

    s.auditSvc.LogAction(ctx, &userID, &entityID, "SEVA_BOOKED", map[string]interface{}{
        "booking_id":      booking.ID,
        "seva_id":         booking.SevaID,
        "seva_name":       seva.Name,
        "seva_type":       seva.SevaType,
        "seva_status":     seva.Status,
        "booking_status":  booking.Status,
        "available_slots": seva.AvailableSlots,
        "booked_slots":    seva.BookedSlots,
        "remaining_slots": seva.RemainingSlots,
    }, ip, "success")

    if s.notifSvc != nil {
        _ = s.notifSvc.CreateInAppForEntityRoles(
            ctx,
            entityID,
            []string{"templeadmin", "standarduser"},
            "New Seva Booking",
            "A new booking was created for "+seva.Name,
            "seva",
        )
        _ = s.notifSvc.CreateInAppNotification(
            ctx,
            userID,
            entityID,
            "Booking Created",
            "Your seva booking has been submitted",
            "seva",
        )
    }

    return nil
}

func (s *service) GetBookingsForUser(ctx context.Context, userID uint) ([]SevaBooking, error) {
    return s.repo.ListBookingsByUserID(ctx, userID)
}

func (s *service) GetBookingsForEntity(ctx context.Context, entityID uint) ([]SevaBooking, error) {
    return s.repo.ListBookingsByEntityID(ctx, entityID)
}

// ✅ UPDATED: UpdateBookingStatus - Updates BookedSlots and RemainingSlots in Seva
func (s *service) UpdateBookingStatus(ctx context.Context, bookingID uint, newStatus string, userID uint, ip string) error {
    booking, err := s.repo.GetBookingByID(ctx, bookingID)
    if err != nil {
        s.auditSvc.LogAction(ctx, &userID, nil, "SEVA_BOOKING_STATUS_UPDATE_FAILED", map[string]interface{}{
            "booking_id": bookingID,
            "new_status": newStatus,
            "reason":     "booking not found",
            "error":      err.Error(),
        }, ip, "failure")
        return err
    }

    seva, err := s.repo.GetSevaByID(ctx, booking.SevaID)
    if err != nil {
        s.auditSvc.LogAction(ctx, &userID, &booking.EntityID, "SEVA_BOOKING_STATUS_UPDATE_FAILED", map[string]interface{}{
            "booking_id": bookingID,
            "new_status": newStatus,
            "reason":     "seva not found",
            "error":      err.Error(),
        }, ip, "failure")
        return err
    }

    oldStatus := booking.Status

    // ✅ CRITICAL: Handle slot management based on status transitions
    // Case 1: Approving a booking (pending/rejected -> approved)
    if newStatus == "approved" && oldStatus != "approved" {
        // Check if slots are available
        if seva.RemainingSlots <= 0 {
            s.auditSvc.LogAction(ctx, &userID, &booking.EntityID, "SEVA_BOOKING_STATUS_UPDATE_FAILED", map[string]interface{}{
                "booking_id":      bookingID,
                "new_status":      newStatus,
                "reason":          "no slots available",
                "available_slots": seva.AvailableSlots,
                "booked_slots":    seva.BookedSlots,
                "remaining_slots": seva.RemainingSlots,
            }, ip, "failure")
            return errors.New("no slots available for this seva")
        }

        // Increment booked slots and decrement remaining slots
        if err := s.repo.IncrementBookedSlots(ctx, booking.SevaID); err != nil {
            s.auditSvc.LogAction(ctx, &userID, &booking.EntityID, "SEVA_BOOKING_STATUS_UPDATE_FAILED", map[string]interface{}{
                "booking_id": bookingID,
                "new_status": newStatus,
                "reason":     "failed to update slots",
                "error":      err.Error(),
            }, ip, "failure")
            return fmt.Errorf("failed to update slots: %v", err)
        }
    }

    // Case 2: Rejecting/Canceling an approved booking (approved -> rejected/pending)
    if oldStatus == "approved" && newStatus != "approved" {
        // Decrement booked slots and increment remaining slots
        if err := s.repo.DecrementBookedSlots(ctx, booking.SevaID); err != nil {
            s.auditSvc.LogAction(ctx, &userID, &booking.EntityID, "SEVA_BOOKING_STATUS_UPDATE_FAILED", map[string]interface{}{
                "booking_id": bookingID,
                "new_status": newStatus,
                "reason":     "failed to update slots",
                "error":      err.Error(),
            }, ip, "failure")
            return fmt.Errorf("failed to update slots: %v", err)
        }
    }

    // Update booking status
    err = s.repo.UpdateBookingStatus(ctx, bookingID, newStatus)
    if err != nil {
        s.auditSvc.LogAction(ctx, &userID, &booking.EntityID, "SEVA_BOOKING_STATUS_UPDATE_FAILED", map[string]interface{}{
            "booking_id": bookingID,
            "seva_id":    booking.SevaID,
            "new_status": newStatus,
            "error":      err.Error(),
        }, ip, "failure")
        return err
    }

    action := "SEVA_BOOKING_STATUS_UPDATED"
    switch newStatus {
    case "approved":
        action = "SEVA_BOOKING_APPROVED"
    case "rejected":
        action = "SEVA_BOOKING_REJECTED"
    }

    auditDetails := map[string]interface{}{
        "booking_id": bookingID,
        "seva_id":    booking.SevaID,
        "devotee_id": booking.UserID,
        "old_status": oldStatus,
        "new_status": newStatus,
    }

    if seva != nil {
        auditDetails["seva_name"] = seva.Name
        auditDetails["seva_type"] = seva.SevaType
        auditDetails["seva_status"] = seva.Status
        
        // Fetch updated slot info
        updatedSeva, _ := s.repo.GetSevaByID(ctx, booking.SevaID)
        if updatedSeva != nil {
            auditDetails["available_slots"] = updatedSeva.AvailableSlots
            auditDetails["booked_slots"] = updatedSeva.BookedSlots
            auditDetails["remaining_slots"] = updatedSeva.RemainingSlots
        }
    }

    s.auditSvc.LogAction(ctx, &userID, &booking.EntityID, action, auditDetails, ip, "success")

    if s.notifSvc != nil {
        _ = s.notifSvc.CreateInAppNotification(
            ctx,
            booking.UserID,
            booking.EntityID,
            "Seva Booking "+newStatus,
            "Your booking status is now "+newStatus,
            "seva",
        )
    }

    return nil
}

func (s *service) GetDetailedBookingsForEntity(ctx context.Context, entityID uint) ([]DetailedBooking, error) {
    return s.repo.ListBookingsWithDetails(ctx, entityID)
}

func (s *service) GetBookingByID(ctx context.Context, bookingID uint) (*SevaBooking, error) {
    return s.repo.GetBookingByID(ctx, bookingID)
}

func (s *service) SearchBookings(ctx context.Context, filter BookingFilter) ([]DetailedBooking, int64, error) {
    return s.repo.SearchBookingsWithFilters(ctx, filter)
}

func (s *service) GetBookingCountsByStatus(ctx context.Context, entityID uint) (BookingStatusCounts, error) {
    return s.repo.CountBookingsByStatus(ctx, entityID)
}

func (s *service) GetDetailedBookingsWithFilters(ctx context.Context, entityID uint, status, sevaType, startDate, endDate, search string, limit, offset int) ([]DetailedBooking, error) {
    filter := BookingFilter{
        EntityID:  entityID,
        Status:    status,
        SevaType:  sevaType,
        StartDate: startDate,
        EndDate:   endDate,
        Search:    search,
        Limit:     limit,
        Offset:    offset,
    }
    bookings, _, err := s.repo.SearchBookingsWithFilters(ctx, filter)
    return bookings, err
}

func (s *service) GetBookingStatusCounts(ctx context.Context, entityID uint) (BookingStatusCounts, error) {
    return s.repo.CountBookingsByStatus(ctx, entityID)
}

func (s *service) GetPaginatedSevas(ctx context.Context, entityID uint, sevaType string, search string, limit int, offset int) ([]Seva, error) {
    return s.repo.ListPaginatedSevas(ctx, entityID, sevaType, search, limit, offset)
}

// Get approved booking counts per seva
func (s *service) GetApprovedBookingCountsPerSeva(ctx context.Context, entityID uint) (map[uint]int64, error) {
    return s.repo.GetApprovedBookingsCountPerSeva(ctx, entityID)
}