package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"ical-sync/internal/calendar"
	"ical-sync/internal/config"
	"ical-sync/internal/dateparse"
	"ical-sync/internal/filter"
	"ical-sync/internal/gcal"
)

func main() {
	var (
		configFile         = flag.String("config", "", "Path to configuration file")
		inputURL           = flag.String("input", "", "Input iCal URL")
		outputFile         = flag.String("output", "", "Output iCal file")
		filters            = flag.String("filter", "", "Comma-separated list of title patterns to exclude")
		startDate          = flag.String("start", "", "Start date for events (YYYY-MM-DD)")
		endDate            = flag.String("end", "", "End date for events (YYYY-MM-DD)")
		replacementSummary = flag.String("summary", "", "Replacement summary text for all events")

		// Google Calendar flags
		googleCalendarID  = flag.String("gcal-id", "", "Google Calendar ID to write events to")
		googleCredentials = flag.String("gcal-creds", "", "Path to Google Calendar credentials file")
		googleToken       = flag.String("gcal-token", "", "Path to Google Calendar token file")
	)
	flag.Parse()

	var cfg *config.Config
	var err error

	if *configFile != "" {
		cfg, err = config.LoadFromFile(*configFile)
		if err != nil && !os.IsNotExist(err) {
			log.Fatalf("Failed to load config: %v", err)
		}
	}

	if cfg == nil {
		cfg = &config.Config{}
	}

	if *inputURL != "" {
		cfg.InputURL = *inputURL
	}
	if *outputFile != "" {
		cfg.OutputFile = *outputFile
	}
	if *filters != "" {
		cfg.FilterPatterns = parseFilters(*filters)
	}
	if *startDate != "" || *endDate != "" {
		start, end, err := dateparse.ParseDateRange(*startDate, *endDate)
		if err != nil {
			log.Fatalf("Failed to parse date range: %v", err)
		}
		cfg.StartDate = start
		cfg.EndDate = end
	}
	if *replacementSummary != "" {
		cfg.ReplacementSummary = *replacementSummary
	}
	if *googleCalendarID != "" {
		cfg.GoogleCalendarID = *googleCalendarID
	}
	if *googleCredentials != "" {
		cfg.GoogleCredentials = *googleCredentials
	}
	if *googleToken != "" {
		cfg.GoogleToken = *googleToken
	}

	if cfg.InputURL == "" {
		fmt.Println("Usage: ical-sync -input <iCal URL> [-output <output file>] [-filter <patterns>] [-start <date>] [-end <date>]")
		flag.PrintDefaults()
		os.Exit(1)
	}

	fmt.Printf("Reading calendar from: %s\n", cfg.InputURL)
	events, err := calendar.FetchEvents(cfg.InputURL)
	if err != nil {
		log.Fatalf("Failed to fetch events: %v", err)
	}

	calendar.NormalizeTimezones(events)

	// Expand recurring events into individual instances
	expandStart := time.Now()
	expandEnd := expandStart.AddDate(0, 0, 30)
	if cfg.StartDate != nil {
		expandStart = *cfg.StartDate
	}
	if cfg.EndDate != nil {
		expandEnd = cfg.EndDate.Add(24*time.Hour - time.Nanosecond)
	}
	events = calendar.ExpandRecurringEvents(events, expandStart, expandEnd)

	fmt.Printf("Found %d events (after expanding recurring)\n", len(events))

	// Log the time range being used for filtering
	if cfg.StartDate != nil && cfg.EndDate != nil {
		fmt.Printf("Filtering events from %s to %s\n", cfg.StartDate.Format("2006-01-02"), cfg.EndDate.Format("2006-01-02"))
	} else if cfg.StartDate != nil {
		fmt.Printf("Filtering events from %s onwards\n", cfg.StartDate.Format("2006-01-02"))
	} else if cfg.EndDate != nil {
		fmt.Printf("Filtering events up to %s\n", cfg.EndDate.Format("2006-01-02"))
	} else {
		fmt.Printf("No time range filtering applied\n")
	}

	filteredEvents := filter.FilterEvents(events, cfg.FilterPatterns, cfg.StartDate, cfg.EndDate, cfg.OnlyAccepted, cfg.AttendeeEmail)
	fmt.Printf("After filtering: %d events\n", len(filteredEvents))

	calendar.CleanEventProperties(filteredEvents, cfg.ReplacementSummary)
	calendar.ApplyCacheBuster(filteredEvents, cfg.CacheBuster)

	// Write to output file if specified
	if cfg.OutputFile != "" {
		err = calendar.WriteCalendar(cfg.OutputFile, filteredEvents)
		if err != nil {
			log.Fatalf("Failed to write calendar: %v", err)
		}
		fmt.Printf("Created calendar with %d events: %s\n", len(filteredEvents), cfg.OutputFile)
	}

	// Write to Google Calendar if specified
	if cfg.GoogleCalendarID != "" {
		if cfg.GoogleCredentials == "" {
			log.Fatal("Google Calendar credentials file path required (use -gcal-creds)")
		}
		if cfg.GoogleToken == "" {
			log.Fatal("Google Calendar token file path required (use -gcal-token)")
		}

		ctx := context.Background()

		// Authenticate and get service
		service, err := gcal.GetCalendarService(ctx, cfg.GoogleCredentials, cfg.GoogleToken)
		if err != nil {
			log.Fatalf("Failed to authenticate with Google Calendar: %v", err)
		}

		// Write events to Google Calendar
		fmt.Printf("Writing %d events to Google Calendar...\n", len(filteredEvents))
		result, err := gcal.WriteEvents(ctx, service, cfg.GoogleCalendarID, filteredEvents, cfg.ReplacementSummary)
		if err != nil {
			log.Fatalf("Failed to write events to Google Calendar: %v", err)
		}

		// Report results
		fmt.Printf("Google Calendar sync complete:\n")
		fmt.Printf("  Successfully written: %d events\n", result.SuccessCount)
		if result.SkippedCount > 0 {
			fmt.Printf("  Skipped (already exist): %d events\n", result.SkippedCount)
		}
		if result.FailureCount > 0 {
			fmt.Printf("  Failed: %d events\n", result.FailureCount)
			fmt.Printf("\nFailed events:\n")
			for _, eventErr := range result.Errors {
				fmt.Printf("  - Event #%d (UID: %s): %v\n", eventErr.EventIndex, eventErr.UID, eventErr.Error)
			}
			os.Exit(1)
		}
	}

	if cfg.OutputFile == "" && cfg.GoogleCalendarID == "" {
		log.Fatal("No output specified. Use -output for file output or -gcal-id for Google Calendar output")
	}
}

func parseFilters(filterStr string) []string {
	if filterStr == "" {
		return nil
	}

	var filters []string
	current := ""
	for _, char := range filterStr {
		if char == ',' {
			if current != "" {
				filters = append(filters, current)
				current = ""
			}
		} else {
			current += string(char)
		}
	}
	if current != "" {
		filters = append(filters, current)
	}

	return filters
}
