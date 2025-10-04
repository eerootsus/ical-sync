package gcal

import (
	"context"
	"fmt"
	"log"
	"time"

	ics "github.com/arran4/golang-ical"
	"google.golang.org/api/calendar/v3"
)

// WriteEvents writes iCal events to Google Calendar
func WriteEvents(ctx context.Context, service *calendar.Service, calendarID string, events []*ics.VEvent, replacementSummary string) error {
	for i, event := range events {
		uid := ""
		if uidProp := event.GetProperty(ics.ComponentPropertyUniqueId); uidProp != nil {
			uid = uidProp.Value
		}

		if err := writeEvent(ctx, service, calendarID, event, replacementSummary); err != nil {
			return fmt.Errorf("failed to write event %d (UID: %s): %w", i+1, uid, err)
		}
		log.Printf("Successfully created event %d/%d (UID: %s)", i+1, len(events), uid)
	}

	return nil
}

// writeEvent writes a single iCal event to Google Calendar
func writeEvent(ctx context.Context, service *calendar.Service, calendarID string, event *ics.VEvent, replacementSummary string) error {
	gcalEvent := convertToGoogleEvent(event, replacementSummary)

	_, err := service.Events.Insert(calendarID, gcalEvent).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("unable to create event: %w", err)
	}

	return nil
}

// convertToGoogleEvent converts an iCal VEvent to a Google Calendar Event
func convertToGoogleEvent(event *ics.VEvent, replacementSummary string) *calendar.Event {
	gcalEvent := &calendar.Event{}

	// Set summary
	if replacementSummary != "" {
		gcalEvent.Summary = replacementSummary
	} else if summary := event.GetProperty(ics.ComponentPropertySummary); summary != nil {
		gcalEvent.Summary = summary.Value
	}

	// Set start time
	startTime, err := event.GetStartAt()
	if err == nil {
		gcalEvent.Start = &calendar.EventDateTime{
			DateTime: startTime.Format(time.RFC3339),
		}
	} else {
		log.Printf("Error getting start time for event: %v", err)
	}

	// Set end time
	endTime, err := event.GetEndAt()
	if err == nil {
		gcalEvent.End = &calendar.EventDateTime{
			DateTime: endTime.Format(time.RFC3339),
		}
	} else {
		log.Printf("Error getting end time for event: %v", err)
	}

	// Set UID as extended property to avoid duplicates
	if uid := event.GetProperty(ics.ComponentPropertyUniqueId); uid != nil {
		gcalEvent.ExtendedProperties = &calendar.EventExtendedProperties{
			Private: map[string]string{
				"icalUID": uid.Value,
			},
		}
	}

	return gcalEvent
}
