package gcal

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	ics "github.com/arran4/golang-ical"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/googleapi"
)

// SyncResult contains the results of syncing events to Google Calendar
type SyncResult struct {
	CreatedCount int
	UpdatedCount int
	DeletedCount int
	UnchangedCount int
	FailureCount int
	Errors       []EventError
}

// EventError represents an error that occurred while syncing a specific event
type EventError struct {
	EventIndex int
	UID        string
	Error      error
}

// SyncEvents performs a full sync of iCal events to Google Calendar:
// creates new events, updates changed ones, and deletes events no longer in the source.
func SyncEvents(ctx context.Context, service *calendar.Service, calendarID string, events []*ics.VEvent, replacementSummary string, timeMin, timeMax time.Time) (*SyncResult, error) {
	result := &SyncResult{
		Errors: make([]EventError, 0),
	}

	// Fetch all existing events from gcal in the time range
	existing, err := listExistingEvents(ctx, service, calendarID, timeMin, timeMax)
	if err != nil {
		return nil, fmt.Errorf("failed to list existing events: %w", err)
	}
	log.Printf("Found %d existing events in Google Calendar", len(existing))

	// Build map of iCalUID → gcal event for quick lookup
	gcalByUID := make(map[string]*calendar.Event, len(existing))
	for _, e := range existing {
		if e.ICalUID != "" {
			gcalByUID[e.ICalUID] = e
		}
	}

	// Track which gcal UIDs are still present in source
	sourceUIDs := make(map[string]bool, len(events))

	for i, event := range events {
		uid := ""
		if uidProp := event.GetProperty(ics.ComponentPropertyUniqueId); uidProp != nil {
			uid = uidProp.Value
		}

		if uid == "" {
			result.FailureCount++
			result.Errors = append(result.Errors, EventError{
				EventIndex: i + 1,
				UID:        uid,
				Error:      fmt.Errorf("event missing UID, cannot sync to Google Calendar"),
			})
			log.Printf("Failed event %d/%d: missing UID", i+1, len(events))
			continue
		}

		sourceUIDs[uid] = true
		desired := convertToGoogleEvent(event, replacementSummary)

		if gcalEvent, exists := gcalByUID[uid]; exists {
			// Event exists — check if it needs updating
			if eventNeedsUpdate(gcalEvent, desired) {
				if err := updateEvent(ctx, service, calendarID, gcalEvent.Id, desired); err != nil {
					result.FailureCount++
					result.Errors = append(result.Errors, EventError{
						EventIndex: i + 1,
						UID:        uid,
						Error:      err,
					})
					log.Printf("Failed to update event %d/%d (UID: %s): %v", i+1, len(events), uid, err)
				} else {
					result.UpdatedCount++
					log.Printf("Updated event %d/%d (UID: %s)", i+1, len(events), uid)
				}
			} else {
				result.UnchangedCount++
			}
		} else {
			// Event doesn't exist in our time-range query — try to create it
			if err := createEvent(ctx, service, calendarID, desired); err != nil {
				// Handle 409 duplicate: event exists outside our query window
				if isDuplicateError(err) {
					if gcalEvent, lookupErr := findEventByUID(ctx, service, calendarID, uid); lookupErr == nil && gcalEvent != nil {
						if updateErr := updateEvent(ctx, service, calendarID, gcalEvent.Id, desired); updateErr != nil {
							result.FailureCount++
							result.Errors = append(result.Errors, EventError{
								EventIndex: i + 1,
								UID:        uid,
								Error:      updateErr,
							})
							log.Printf("Failed to update duplicate event %d/%d (UID: %s): %v", i+1, len(events), uid, updateErr)
						} else {
							result.UpdatedCount++
							log.Printf("Updated duplicate event %d/%d (UID: %s)", i+1, len(events), uid)
						}
					} else {
						result.FailureCount++
						result.Errors = append(result.Errors, EventError{
							EventIndex: i + 1,
							UID:        uid,
							Error:      err,
						})
						log.Printf("Failed to create event %d/%d (UID: %s): %v (and lookup failed)", i+1, len(events), uid, err)
					}
				} else {
					result.FailureCount++
					result.Errors = append(result.Errors, EventError{
						EventIndex: i + 1,
						UID:        uid,
						Error:      err,
					})
					log.Printf("Failed to create event %d/%d (UID: %s): %v", i+1, len(events), uid, err)
				}
			} else {
				result.CreatedCount++
				log.Printf("Created event %d/%d (UID: %s)", i+1, len(events), uid)
			}
		}
	}

	// Delete gcal events that are no longer in the source (only if managed by ical-sync)
	for uid, gcalEvent := range gcalByUID {
		if sourceUIDs[uid] {
			continue
		}
		if !isManagedEvent(gcalEvent) {
			continue
		}
		if err := service.Events.Delete(calendarID, gcalEvent.Id).Context(ctx).Do(); err != nil {
			result.FailureCount++
			result.Errors = append(result.Errors, EventError{
				UID:   uid,
				Error: fmt.Errorf("failed to delete event: %w", err),
			})
			log.Printf("Failed to delete event (UID: %s): %v", uid, err)
		} else {
			result.DeletedCount++
			log.Printf("Deleted event (UID: %s) — no longer in source", uid)
		}
	}

	return result, nil
}

// listExistingEvents fetches all events from the calendar within the given time range,
// handling pagination.
func listExistingEvents(ctx context.Context, service *calendar.Service, calendarID string, timeMin, timeMax time.Time) ([]*calendar.Event, error) {
	var all []*calendar.Event
	pageToken := ""

	for {
		call := service.Events.List(calendarID).
			TimeMin(timeMin.Format(time.RFC3339)).
			TimeMax(timeMax.Format(time.RFC3339)).
			SingleEvents(true).
			ShowDeleted(false).
			MaxResults(250).
			Context(ctx)

		if pageToken != "" {
			call = call.PageToken(pageToken)
		}

		resp, err := call.Do()
		if err != nil {
			return nil, fmt.Errorf("unable to list events: %w", err)
		}

		all = append(all, resp.Items...)

		if resp.NextPageToken == "" {
			break
		}
		pageToken = resp.NextPageToken
	}

	return all, nil
}

// eventNeedsUpdate checks whether the existing gcal event differs from the desired state.
func eventNeedsUpdate(existing, desired *calendar.Event) bool {
	if existing.Summary != desired.Summary {
		return true
	}
	if !eventTimesEqual(existing.Start, desired.Start) {
		return true
	}
	if !eventTimesEqual(existing.End, desired.End) {
		return true
	}
	return false
}

func eventTimesEqual(a, b *calendar.EventDateTime) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return a.DateTime == b.DateTime && a.Date == b.Date
}

// createEvent inserts a new event into Google Calendar.
func createEvent(ctx context.Context, service *calendar.Service, calendarID string, event *calendar.Event) error {
	created, err := service.Events.Insert(calendarID, event).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("unable to create event: %w", err)
	}
	log.Printf("Created event ID: %s", created.Id)
	return nil
}

// updateEvent patches an existing Google Calendar event.
func updateEvent(ctx context.Context, service *calendar.Service, calendarID, eventID string, event *calendar.Event) error {
	_, err := service.Events.Update(calendarID, eventID, event).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("unable to update event: %w", err)
	}
	return nil
}

// isDuplicateError checks if the error is a Google API 409 Conflict (duplicate).
func isDuplicateError(err error) bool {
	var apiErr *googleapi.Error
	if ok := errors.As(err, &apiErr); ok {
		return apiErr.Code == http.StatusConflict
	}
	// Also check wrapped errors
	return false
}

// findEventByUID looks up a single event by its iCalUID across the entire calendar.
// It also checks deleted events since a cancelled event still reserves the UID.
// For expanded recurring instance UIDs (containing _YYYYMMDD), it also tries the base UID.
func findEventByUID(ctx context.Context, service *calendar.Service, calendarID, uid string) (*calendar.Event, error) {
	uidsToTry := []string{uid}
	// If this is an expanded recurring instance UID, also try the base UID
	if idx := strings.LastIndex(uid, "_"); idx > 0 {
		baseUID := uid[:idx]
		uidsToTry = append(uidsToTry, baseUID)
	}

	for _, tryUID := range uidsToTry {
		for _, showDeleted := range []bool{false, true} {
			resp, err := service.Events.List(calendarID).
				ICalUID(tryUID).
				ShowDeleted(showDeleted).
				MaxResults(1).
				Context(ctx).
				Do()
			if err != nil {
				continue
			}
			if len(resp.Items) > 0 {
				return resp.Items[0], nil
			}
		}
	}

	return nil, fmt.Errorf("event not found by UID %s", uid)
}

// managedPropertyKey is the extended property key used to mark events managed by ical-sync.
// Only events with this property will be updated or deleted during sync.
const managedPropertyKey = "ical-sync-managed"

// isManagedEvent returns true if the event was created/updated by ical-sync.
func isManagedEvent(event *calendar.Event) bool {
	if event.ExtendedProperties == nil {
		return false
	}
	return event.ExtendedProperties.Shared[managedPropertyKey] == "true"
}

// convertToGoogleEvent converts an iCal VEvent to a Google Calendar Event
func convertToGoogleEvent(event *ics.VEvent, replacementSummary string) *calendar.Event {
	gcalEvent := &calendar.Event{
		ExtendedProperties: &calendar.EventExtendedProperties{
			Shared: map[string]string{
				managedPropertyKey: "true",
			},
		},
	}

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

	// Set UID
	if uid := event.GetProperty(ics.ComponentPropertyUniqueId); uid != nil {
		gcalEvent.ICalUID = uid.Value
	}

	return gcalEvent
}
