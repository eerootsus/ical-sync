package config

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"ical-sync/internal/dateparse"
)

type Config struct {
	InputURL           string     `json:"input_url"`
	OutputFile         string     `json:"output_file"`
	FilterPatterns     []string   `json:"filter_patterns"`
	StartDate          *time.Time `json:"start_date,omitempty"`
	EndDate            *time.Time `json:"end_date,omitempty"`
	ReplacementSummary string     `json:"replacement_summary"`
	OnlyAccepted       bool       `json:"only_accepted,omitempty"`
	AttendeeEmail      string     `json:"attendee_email,omitempty"`

	// Google Calendar integration
	GoogleCalendarID  string `json:"google_calendar_id,omitempty"`
	GoogleCredentials string `json:"google_credentials,omitempty"`
	GoogleToken       string `json:"google_token,omitempty"`
}

type configJSON struct {
	InputURL           string   `json:"input_url"`
	OutputFile         string   `json:"output_file"`
	FilterPatterns     []string `json:"filter_patterns"`
	StartDate          string   `json:"start_date,omitempty"`
	EndDate            string   `json:"end_date,omitempty"`
	ReplacementSummary string   `json:"replacement_summary"`
	OnlyAccepted       bool     `json:"only_accepted,omitempty"`
	AttendeeEmail      string   `json:"attendee_email,omitempty"`

	// Google Calendar integration
	GoogleCalendarID  string `json:"google_calendar_id,omitempty"`
	GoogleCredentials string `json:"google_credentials,omitempty"`
	GoogleToken       string `json:"google_token,omitempty"`
}

func LoadFromFile(filename string) (*Config, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var configData configJSON
	err = json.Unmarshal(data, &configData)
	if err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	config := &Config{
		InputURL:           configData.InputURL,
		OutputFile:         configData.OutputFile,
		FilterPatterns:     configData.FilterPatterns,
		ReplacementSummary: configData.ReplacementSummary,
		OnlyAccepted:       configData.OnlyAccepted,
		AttendeeEmail:      configData.AttendeeEmail,
		GoogleCalendarID:   configData.GoogleCalendarID,
		GoogleCredentials:  configData.GoogleCredentials,
		GoogleToken:        configData.GoogleToken,
	}

	// Parse dates with support for relative formats
	startDate, endDate, err := dateparse.ParseDateRange(configData.StartDate, configData.EndDate)
	if err != nil {
		return nil, fmt.Errorf("failed to parse date range: %w", err)
	}
	config.StartDate = startDate
	config.EndDate = endDate

	return config, nil
}

func (c *Config) SaveToFile(filename string) error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	err = os.WriteFile(filename, data, 0644)
	if err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}
