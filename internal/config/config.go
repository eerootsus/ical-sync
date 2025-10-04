package config

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

type Config struct {
	InputURL           string     `json:"input_url"`
	OutputFile         string     `json:"output_file"`
	FilterPatterns     []string   `json:"filter_patterns"`
	StartDate          *time.Time `json:"start_date,omitempty"`
	EndDate            *time.Time `json:"end_date,omitempty"`
	ReplacementSummary string     `json:"replacement_summary"`

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

	var config Config
	err = json.Unmarshal(data, &config)
	if err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return &config, nil
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
