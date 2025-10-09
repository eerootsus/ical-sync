package gcal

import (
	"context"
	"fmt"
	"log"
	"time"

	ics "github.com/arran4/golang-ical"
	"google.golang.org/api/calendar/v3"
)

// WriteEventsResult contains the results of writing events to Google Calendar
type WriteEventsResult struct {
	SuccessCount int
	FailureCount int
	SkippedCount int
	Errors       []EventError
}

// EventError represents an error that occurred while writing a specific event
type EventError struct {
	EventIndex int
	UID        string
	Error      error
}

// WriteEvents writes iCal events to Google Calendar
// Returns a result containing success/failure counts and any errors encountered
func WriteEvents(ctx context.Context, service *calendar.Service, calendarID string, events []*ics.VEvent, replacementSummary string) (*WriteEventsResult, error) {
	result := &WriteEventsResult{
		Errors: make([]EventError, 0),
	}

	for i, event := range events {
		uid := ""
		if uidProp := event.GetProperty(ics.ComponentPropertyUniqueId); uidProp != nil {
			uid = uidProp.Value
		}

		// Events without UID cannot be deduplicated
		if uid == "" {
			result.FailureCount++
			result.Errors = append(result.Errors, EventError{
				EventIndex: i + 1,
				UID:        uid,
				Error:      fmt.Errorf("event missing UID, cannot write to Google Calendar"),
			})
			log.Printf("Failed event %d/%d: missing UID", i+1, len(events))
			continue
		}

		// Check if event already exists (debug enabled for first event only)
		exists, err := eventExists(ctx, service, calendarID, uid)
		if err != nil {
			result.FailureCount++
			result.Errors = append(result.Errors, EventError{
				EventIndex: i + 1,
				UID:        uid,
				Error:      fmt.Errorf("failed to check if event exists: %w", err),
			})
			log.Printf("Failed to check event %d/%d (UID: %s): %v", i+1, len(events), uid, err)
			continue
		}

		if exists {
			result.SkippedCount++
			log.Printf("Skipping event %d/%d (UID: %s) - already exists", i+1, len(events), uid)
			continue
		}

		if err := writeEvent(ctx, service, calendarID, event, replacementSummary); err != nil {
			result.FailureCount++
			result.Errors = append(result.Errors, EventError{
				EventIndex: i + 1,
				UID:        uid,
				Error:      err,
			})
			log.Printf("Failed to create event %d/%d (UID: %s): %v", i+1, len(events), uid, err)
		} else {
			result.SuccessCount++
			log.Printf("Successfully created event %d/%d (UID: %s)", i+1, len(events), uid)
		}
	}

	return result, nil
}

// eventExists checks if an event with the given icalUID already exists in the calendar
func eventExists(ctx context.Context, service *calendar.Service, calendarID, icalUID string) (bool, error) {
	if icalUID == "" {
		return false, nil
	}

	events, err := service.Events.List(calendarID).
		ICalUID(icalUID).
		ShowDeleted(true).
		Context(ctx).
		MaxResults(1).
		Do()

	if err != nil {
		return false, fmt.Errorf("unable to query events: %w", err)
	}

	return len(events.Items) > 0, nil
}

// writeEvent writes a single iCal event to Google Calendar
func writeEvent(ctx context.Context, service *calendar.Service, calendarID string, event *ics.VEvent, replacementSummary string) error {
	gcalEvent := convertToGoogleEvent(event, replacementSummary)

	createdEvent, err := service.Events.Insert(calendarID, gcalEvent).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("unable to create event: %w", err)
	}

	log.Printf("Created event ID: %s", createdEvent.Id)

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

	// Set UID to avoid duplicates
	if uid := event.GetProperty(ics.ComponentPropertyUniqueId); uid != nil {
		gcalEvent.ICalUID = uid.Value
	}

	return gcalEvent
}
