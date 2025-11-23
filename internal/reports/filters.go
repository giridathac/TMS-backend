package reports

import (
	"errors"
	"time"
)

// GetDateRange returns start and end time for the given preset or custom (startStr/endStr required for custom).
// startStr/endStr expected in "2006-01-02" format when dateRange == DateRangeCustom.
func GetDateRange(dateRange, startStr, endStr string) (time.Time, time.Time, error) {
	now := time.Now()
	loc := now.Location()

	switch dateRange {
	case DateRangeDaily:
		start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
		end := start.Add(24*time.Hour - time.Second)
		return start, end, nil
	case DateRangeWeekly:
		// last 7 days (including today)
		end := time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 0, loc)
		start := end.AddDate(0, 0, -6).Truncate(24 * time.Hour)
		return start, end, nil
	case DateRangeMonthly:
		start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, loc)
		end := start.AddDate(0, 1, 0).Add(-time.Second)
		return start, end, nil
	case DateRangeYearly:
		start := time.Date(now.Year(), 1, 1, 0, 0, 0, 0, loc)
		end := time.Date(now.Year(), 12, 31, 23, 59, 59, 0, loc)
		return start, end, nil
	case DateRangeCustom:
		if startStr == "" || endStr == "" {
			return time.Time{}, time.Time{}, errors.New("start_date and end_date required for custom range")
		}
		start, err := time.ParseInLocation("2006-01-02", startStr, loc)
		if err != nil {
			return time.Time{}, time.Time{}, err
		}
		end, err := time.ParseInLocation("2006-01-02", endStr, loc)
		if err != nil {
			return time.Time{}, time.Time{}, err
		}
		// include entire end day
		end = end.Add(23*time.Hour + 59*time.Minute + 59*time.Second)
		if start.After(end) {
			return time.Time{}, time.Time{}, errors.New("start_date must be before end_date")
		}
		return start, end, nil
	default:
		// default to last 7 days
		return GetDateRange(DateRangeWeekly, "", "")
	}
}
