package calendar

import (
	"fmt"
	"net/http"
	"os"
	"strings"

	ics "github.com/arran4/golang-ical"
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
	"FLE Standard Time": "Europe/Tallinn",
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
