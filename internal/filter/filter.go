package filter

import (
	"regexp"
	"strings"
	"time"

	ics "github.com/arran4/golang-ical"
)

func FilterEvents(events []*ics.VEvent, patterns []string, startDate, endDate *time.Time, onlyAccepted bool, attendeeEmail string) []*ics.VEvent {
	var filtered []*ics.VEvent

	for _, event := range events {
		if shouldIncludeEvent(event, patterns) && isWithinTimeframe(event, startDate, endDate) && isAccepted(event, onlyAccepted, attendeeEmail) {
			filtered = append(filtered, event)
		}
	}

	return filtered
}

// isAccepted checks whether the event is accepted by the calendar owner.
// It checks both standard iCal ATTENDEE PARTSTAT and Microsoft's
// X-MICROSOFT-CDO-BUSYSTATUS property (used by Outlook published feeds).
func isAccepted(event *ics.VEvent, onlyAccepted bool, attendeeEmail string) bool {
	if !onlyAccepted {
		return true
	}

	// Check X-MICROSOFT-CDO-BUSYSTATUS (Outlook published feeds use this
	// instead of ATTENDEE/PARTSTAT)
	for _, prop := range event.Properties {
		if prop.IANAToken == "X-MICROSOFT-CDO-BUSYSTATUS" {
			status := strings.ToUpper(prop.Value)
			return status == "BUSY" || status == "OOF" || status == "WORKINGELSEWHERE"
		}
	}

	// Fall back to standard ATTENDEE PARTSTAT
	if attendeeEmail == "" {
		return true
	}

	email := strings.ToLower(attendeeEmail)
	for _, prop := range event.Properties {
		if prop.IANAToken != string(ics.ComponentPropertyAttendee) {
			continue
		}
		if !strings.Contains(strings.ToLower(prop.Value), email) {
			continue
		}
		if partstat, ok := prop.ICalParameters["PARTSTAT"]; ok && len(partstat) > 0 {
			return strings.EqualFold(partstat[0], "ACCEPTED")
		}
		return false
	}

	return true
}

func shouldIncludeEvent(event *ics.VEvent, patterns []string) bool {
	if len(patterns) == 0 {
		return true
	}

	summary := ""
	if summaryProp := event.GetProperty(ics.ComponentPropertySummary); summaryProp != nil {
		summary = summaryProp.Value
	}
	title := strings.ToLower(summary)

	for _, pattern := range patterns {
		if matchesPattern(title, pattern) {
			return false
		}
	}

	return true
}

func isWithinTimeframe(event *ics.VEvent, startDate, endDate *time.Time) bool {
	eventStartTime, err := event.GetStartAt()
	if err != nil {
		return true
	}

	if startDate != nil {
		if eventStartTime.Before(*startDate) {
			return false
		}
	}

	if endDate != nil {
		endOfDay := endDate.Add(24*time.Hour - time.Nanosecond)
		if eventStartTime.After(endOfDay) {
			return false
		}
	}

	return true
}

func matchesPattern(title, pattern string) bool {
	pattern = strings.ToLower(strings.TrimSpace(pattern))

	if strings.HasPrefix(pattern, "/") && strings.HasSuffix(pattern, "/") {
		regexPattern := pattern[1 : len(pattern)-1]
		regex, err := regexp.Compile(regexPattern)
		if err != nil {
			return strings.Contains(title, pattern)
		}
		return regex.MatchString(title)
	}

	if strings.Contains(pattern, "*") {
		return matchesWildcard(title, pattern)
	}

	return strings.Contains(title, pattern)
}

func matchesWildcard(text, pattern string) bool {
	return wildcardMatch(text, pattern, 0, 0)
}

func wildcardMatch(text, pattern string, textIndex, patternIndex int) bool {
	if patternIndex == len(pattern) {
		return textIndex == len(text)
	}

	if textIndex == len(text) {
		for i := patternIndex; i < len(pattern); i++ {
			if pattern[i] != '*' {
				return false
			}
		}
		return true
	}

	if pattern[patternIndex] == '*' {
		return wildcardMatch(text, pattern, textIndex+1, patternIndex) ||
			wildcardMatch(text, pattern, textIndex, patternIndex+1)
	}

	if pattern[patternIndex] == '?' || pattern[patternIndex] == text[textIndex] {
		return wildcardMatch(text, pattern, textIndex+1, patternIndex+1)
	}

	return false
}
