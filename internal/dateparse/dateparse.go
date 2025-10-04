package dateparse

import (
	"fmt"
	"time"
)

// ParseDateRange parses start and end date strings with support for relative formats.
// The end date is calculated relative to the start date if both are specified.
func ParseDateRange(startStr, endStr string) (*time.Time, *time.Time, error) {
	now := time.Now()
	var start, end *time.Time

	// Parse start date
	if startStr != "" {
		startTime, err := ParseDate(startStr, now)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid start date: %w", err)
		}
		start = &startTime
	}

	// Parse end date (may be relative to start date)
	if endStr != "" {
		referenceDate := now
		if start != nil {
			referenceDate = *start
		}
		endTime, err := ParseDate(endStr, referenceDate)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid end date: %w", err)
		}
		end = &endTime
	}

	return start, end, nil
}

// ParseDate parses a date string with support for multiple formats:
// - Relative dates: "today", "tomorrow", "yesterday"
// - Day offsets: "+N" or "-N" (relative to referenceDate)
// - Absolute dates: "YYYY-MM-DD" or RFC3339 format
func ParseDate(dateStr string, referenceDate time.Time) (time.Time, error) {
	// Handle relative dates
	switch dateStr {
	case "today":
		return time.Date(referenceDate.Year(), referenceDate.Month(), referenceDate.Day(), 0, 0, 0, 0, referenceDate.Location()), nil
	case "tomorrow":
		tomorrow := referenceDate.AddDate(0, 0, 1)
		return time.Date(tomorrow.Year(), tomorrow.Month(), tomorrow.Day(), 0, 0, 0, 0, tomorrow.Location()), nil
	case "yesterday":
		yesterday := referenceDate.AddDate(0, 0, -1)
		return time.Date(yesterday.Year(), yesterday.Month(), yesterday.Day(), 0, 0, 0, 0, yesterday.Location()), nil
	}

	// Try parsing as relative day count (+N or -N)
	if len(dateStr) > 0 && (dateStr[0] == '+' || dateStr[0] == '-') {
		var days int
		_, err := fmt.Sscanf(dateStr, "%d", &days)
		if err == nil {
			result := referenceDate.AddDate(0, 0, days)
			return time.Date(result.Year(), result.Month(), result.Day(), 0, 0, 0, 0, result.Location()), nil
		}
	}

	// Try parsing as RFC3339 (for backwards compatibility with config files)
	parsed, err := time.Parse(time.RFC3339, dateStr)
	if err == nil {
		return parsed, nil
	}

	// Try parsing as YYYY-MM-DD
	parsed, err = time.Parse("2006-01-02", dateStr)
	if err != nil {
		return time.Time{}, fmt.Errorf("use RFC3339, YYYY-MM-DD format, relative dates (today, tomorrow, yesterday), or day offsets (+N, -N): %w", err)
	}

	return parsed, nil
}
