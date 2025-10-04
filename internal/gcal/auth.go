package gcal

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"
)

// GetCalendarService creates and returns a Google Calendar API service
func GetCalendarService(ctx context.Context, credentialsPath, tokenPath string) (*calendar.Service, error) {
	config, err := getOAuthConfig(credentialsPath)
	if err != nil {
		return nil, err
	}

	token, err := getToken(config, tokenPath)
	if err != nil {
		return nil, err
	}

	client := config.Client(ctx, token)
	service, err := calendar.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return nil, fmt.Errorf("unable to create calendar service: %w", err)
	}

	return service, nil
}

// getOAuthConfig reads OAuth2 credentials from a file
func getOAuthConfig(credentialsPath string) (*oauth2.Config, error) {
	b, err := os.ReadFile(credentialsPath)
	if err != nil {
		return nil, fmt.Errorf("unable to read credentials file: %w", err)
	}

	config, err := google.ConfigFromJSON(b, calendar.CalendarScope)
	if err != nil {
		return nil, fmt.Errorf("unable to parse credentials: %w", err)
	}

	return config, nil
}

// getToken retrieves a token from a file, or requests a new one if needed
func getToken(config *oauth2.Config, tokenPath string) (*oauth2.Token, error) {
	token, err := tokenFromFile(tokenPath)
	if err == nil {
		return token, nil
	}

	// If token doesn't exist, request a new one
	token, err = getTokenFromWeb(config)
	if err != nil {
		return nil, err
	}

	// Save the token for future use
	if err := saveToken(tokenPath, token); err != nil {
		return nil, err
	}

	return token, nil
}

// getTokenFromWeb requests a token from the web and prompts the user to authorize
func getTokenFromWeb(config *oauth2.Config) (*oauth2.Token, error) {
	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	fmt.Printf("Go to the following link in your browser:\n%v\n\n", authURL)
	fmt.Print("Enter the authorization code: ")

	var authCode string
	if _, err := fmt.Scan(&authCode); err != nil {
		return nil, fmt.Errorf("unable to read authorization code: %w", err)
	}

	token, err := config.Exchange(context.Background(), authCode)
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve token from web: %w", err)
	}

	return token, nil
}

// tokenFromFile retrieves a token from a local file
func tokenFromFile(file string) (*oauth2.Token, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	token := &oauth2.Token{}
	err = json.NewDecoder(f).Decode(token)
	return token, err
}

// saveToken saves a token to a file
func saveToken(path string, token *oauth2.Token) error {
	fmt.Printf("Saving credential file to: %s\n", path)
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("unable to cache oauth token: %w", err)
	}
	defer f.Close()

	return json.NewEncoder(f).Encode(token)
}
