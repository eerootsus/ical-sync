package calendar

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	ics "github.com/arran4/golang-ical"
	"github.com/teambition/rrule-go"
)

func FetchEvents(url string) ([]*ics.VEvent, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch calendar: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch calendar: HTTP %d", resp.StatusCode)
	}

	cal, err := ics.ParseCalendar(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to parse calendar: %w", err)
	}

	return cal.Events(), nil
}

func CleanEventProperties(events []*ics.VEvent, replacementSummary string) {
	for _, event := range events {
		event.RemoveProperty(ics.ComponentPropertyDescription)
		event.RemoveProperty(ics.ComponentPropertyLocation)
		event.RemoveProperty(ics.ComponentPropertyClass)
		event.RemoveProperty(ics.ComponentPropertyPriority)
		event.RemoveProperty(ics.ComponentPropertySequence)
		event.RemoveProperty(ics.ComponentPropertyTransp)
		event.RemoveProperty(ics.ComponentPropertyStatus)

		RemoveXProperties(event)

		if replacementSummary != "" {
			event.SetProperty(ics.ComponentPropertySummary, replacementSummary)
		}
	}
}

func RemoveXProperties(event *ics.VEvent) {
	var keptProperties []ics.IANAProperty
	for i := range event.Properties {
		if !strings.HasPrefix(event.Properties[i].IANAToken, "X-") {
			keptProperties = append(keptProperties, event.Properties[i])
		}
	}
	event.Properties = keptProperties
}

// windowsToIANATimezones maps Windows timezone identifiers to IANA timezone identifiers
// See https://gist.github.com/alejzeis/ad5827eb14b5c22109ba652a1a267af5 for other possible replacements
var windowsToIANATimezones = map[string]string{
	"FLE Standard Time":       "Europe/Tallinn",
	"Greenwich Standard Time": "Europe/London",
}

// NormalizeTimezones converts Windows timezone identifiers to IANA timezone identifiers
func NormalizeTimezones(events []*ics.VEvent) {
	for _, event := range events {
		for i := range event.Properties {
			prop := &event.Properties[i]

			// Check if this property has a TZID parameter
			if tzid, ok := prop.ICalParameters["TZID"]; ok && len(tzid) > 0 {
				// Check if this is a Windows timezone that needs conversion
				if ianaTimezone, found := windowsToIANATimezones[tzid[0]]; found {
					prop.ICalParameters["TZID"] = []string{ianaTimezone}
				}
			}
		}
	}
}

// ExpandRecurringEvents expands events with RRULE into individual instances
// within the given time window. Non-recurring events are passed through as-is.
// Each expanded instance gets a unique UID ({original}_{occurrence_datetime}).
// RECURRENCE-ID overrides (single-instance exceptions) are handled: the
// overridden occurrence is excluded from expansion and the override event
// is kept with a unique UID.
func ExpandRecurringEvents(events []*ics.VEvent, windowStart, windowEnd time.Time) []*ics.VEvent {
	// First pass: collect RECURRENCE-ID override times grouped by UID.
	// These represent single-instance exceptions to a recurring series.
	overrides := make(map[string]map[int64]bool)
	for _, event := range events {
		recIDProp := event.GetProperty(ics.ComponentPropertyRecurrenceId)
		if recIDProp == nil {
			continue
		}
		uid := ""
		if p := event.GetProperty(ics.ComponentPropertyUniqueId); p != nil {
			uid = p.Value
		}
		if uid == "" {
			continue
		}
		t, err := parsePropertyTime(recIDProp)
		if err != nil {
			continue
		}
		if overrides[uid] == nil {
			overrides[uid] = make(map[int64]bool)
		}
		overrides[uid][t.UTC().Unix()] = true
	}

	// Second pass: expand recurring events and handle overrides
	var result []*ics.VEvent
	for _, event := range events {
		// RECURRENCE-ID override events: give them a unique UID for dedup
		if recIDProp := event.GetProperty(ics.ComponentPropertyRecurrenceId); recIDProp != nil {
			if uidProp := event.GetProperty(ics.ComponentPropertyUniqueId); uidProp != nil {
				if t, err := parsePropertyTime(recIDProp); err == nil {
					event.SetProperty(ics.ComponentPropertyUniqueId,
						fmt.Sprintf("%s_%s", uidProp.Value, t.UTC().Format("20060102T150405")))
				}
			}
			event.RemoveProperty(ics.ComponentPropertyRecurrenceId)
			result = append(result, event)
			continue
		}

		rruleProp := event.GetProperty(ics.ComponentPropertyRrule)
		if rruleProp == nil {
			result = append(result, event)
			continue
		}

		uid := ""
		if p := event.GetProperty(ics.ComponentPropertyUniqueId); p != nil {
			uid = p.Value
		}
		summary := ""
		if p := event.GetProperty(ics.ComponentPropertySummary); p != nil {
			summary = p.Value
		}

		expanded, err := expandRecurringEvent(event, rruleProp.Value, windowStart, windowEnd)
		if err != nil {
			log.Printf("Warning: failed to expand recurring event %q, keeping original: %v", summary, err)
			result = append(result, event)
			continue
		}

		// Filter out occurrences that have RECURRENCE-ID overrides
		uidOverrides := overrides[uid]
		var kept []*ics.VEvent
		for _, exp := range expanded {
			startTime, err := exp.GetStartAt()
			if err == nil && uidOverrides != nil && uidOverrides[startTime.UTC().Unix()] {
				continue
			}
			kept = append(kept, exp)
		}

		if len(kept) > 0 {
			log.Printf("Expanded recurring event %q into %d instances", summary, len(kept))
		} else {
			log.Printf("Recurring event %q has no instances in window, skipping", summary)
		}
		result = append(result, kept...)
	}

	return result
}

// parsePropertyTime parses an iCal property value as a time, respecting its TZID parameter.
func parsePropertyTime(prop *ics.IANAProperty) (time.Time, error) {
	value := prop.Value

	// Check for TZID parameter
	if tzids, ok := prop.ICalParameters["TZID"]; ok && len(tzids) > 0 {
		loc, err := time.LoadLocation(tzids[0])
		if err != nil {
			return parseICalDateTime(value)
		}
		t, err := time.ParseInLocation("20060102T150405", value, loc)
		if err != nil {
			return parseICalDateTime(value)
		}
		return t, nil
	}

	return parseICalDateTime(value)
}

func expandRecurringEvent(event *ics.VEvent, rruleStr string, windowStart, windowEnd time.Time) ([]*ics.VEvent, error) {
	eventStart, err := event.GetStartAt()
	if err != nil {
		return nil, fmt.Errorf("failed to get event start time: %w", err)
	}

	var duration time.Duration
	eventEnd, err := event.GetEndAt()
	if err == nil {
		duration = eventEnd.Sub(eventStart)
	} else {
		duration = time.Hour
	}

	// Build rrule set from DTSTART and RRULE
	setStr := fmt.Sprintf("DTSTART:%s\nRRULE:%s",
		eventStart.UTC().Format("20060102T150405Z"),
		rruleStr)

	set, err := rrule.StrToRRuleSet(setStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse RRULE %q: %w", rruleStr, err)
	}

	// Add EXDATE exclusions
	exdateProps := event.GetProperties(ics.ComponentPropertyExdate)
	for _, prop := range exdateProps {
		for _, dateStr := range strings.Split(prop.Value, ",") {
			dateStr = strings.TrimSpace(dateStr)
			if t, err := parseICalDateTime(dateStr); err == nil {
				set.ExDate(t)
			}
		}
	}

	occurrences := set.Between(windowStart, windowEnd, true)

	uid := ""
	if uidProp := event.GetProperty(ics.ComponentPropertyUniqueId); uidProp != nil {
		uid = uidProp.Value
	}

	var expanded []*ics.VEvent
	for _, occ := range occurrences {
		newEvent := cloneVEvent(event)

		newEvent.SetProperty(ics.ComponentPropertyDtStart, occ.UTC().Format("20060102T150405Z"))
		newEvent.SetProperty(ics.ComponentPropertyDtEnd, occ.Add(duration).UTC().Format("20060102T150405Z"))

		if uid != "" {
			newEvent.SetProperty(ics.ComponentPropertyUniqueId,
				fmt.Sprintf("%s_%s", uid, occ.UTC().Format("20060102T150405")))
		}

		newEvent.RemoveProperty(ics.ComponentPropertyRrule)
		newEvent.RemoveProperty(ics.ComponentPropertyRdate)
		newEvent.RemoveProperty(ics.ComponentPropertyExdate)
		newEvent.RemoveProperty(ics.ComponentPropertyExrule)

		expanded = append(expanded, newEvent)
	}

	return expanded, nil
}

func cloneVEvent(event *ics.VEvent) *ics.VEvent {
	newEvent := &ics.VEvent{}
	for _, prop := range event.Properties {
		newProp := ics.IANAProperty{
			BaseProperty: ics.BaseProperty{
				IANAToken: prop.IANAToken,
				Value:     prop.Value,
			},
		}
		if prop.ICalParameters != nil {
			newProp.ICalParameters = make(map[string][]string)
			for k, v := range prop.ICalParameters {
				newV := make([]string, len(v))
				copy(newV, v)
				newProp.ICalParameters[k] = newV
			}
		}
		newEvent.Properties = append(newEvent.Properties, newProp)
	}
	return newEvent
}

func parseICalDateTime(s string) (time.Time, error) {
	formats := []string{
		"20060102T150405Z",
		"20060102T150405",
		"20060102",
	}
	for _, format := range formats {
		if t, err := time.Parse(format, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("unable to parse iCal date-time: %s", s)
}

func WriteCalendar(filename string, events []*ics.VEvent) error {
	cal := ics.NewCalendar()
	cal.SetMethod(ics.MethodPublish)
	cal.SetProductId("-//ical-sync//EN")

	for _, event := range events {
		cal.AddVEvent(event)
	}

	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer file.Close()

	_, err = file.WriteString(cal.Serialize())
	return err
}
