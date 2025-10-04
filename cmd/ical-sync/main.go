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
	if *startDate != "" {
		start, err := time.Parse("2006-01-02", *startDate)
		if err != nil {
			log.Fatalf("Invalid start date format. Use YYYY-MM-DD: %v", err)
		}
		cfg.StartDate = &start
	}
	if *endDate != "" {
		end, err := time.Parse("2006-01-02", *endDate)
		if err != nil {
			log.Fatalf("Invalid end date format. Use YYYY-MM-DD: %v", err)
		}
		cfg.EndDate = &end
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

	fmt.Printf("Found %d events\n", len(events))

	filteredEvents := filter.FilterEvents(events, cfg.FilterPatterns, cfg.StartDate, cfg.EndDate)
	fmt.Printf("After filtering: %d events\n", len(filteredEvents))

	// Write to output file if specified
	if cfg.OutputFile != "" {
		err = calendar.WriteCalendar(cfg.OutputFile, filteredEvents, cfg.ReplacementSummary)
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
		err = gcal.WriteEvents(ctx, service, cfg.GoogleCalendarID, filteredEvents, cfg.ReplacementSummary)
		if err != nil {
			log.Fatalf("Failed to write events to Google Calendar: %v", err)
		}

		fmt.Printf("Successfully wrote %d events to Google Calendar: %s\n", len(filteredEvents), cfg.GoogleCalendarID)
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
