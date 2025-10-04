# iCal Sync

A Go application that pulls events from an iCal calendar, filters them based on title patterns and timeframes, and creates a new iCal calendar with the remaining events.

**This has been generated with Claude**

## Features

- Fetch events from any iCal URL
- Filter events using various pattern matching methods:
  - Simple text matching
  - Wildcard patterns (`*` and `?`)
  - Regular expressions (enclosed in `/`)
- Filter events by timeframe (start and end dates)
- Replace event summaries with configurable text
- Generate a new iCal calendar with filtered events (excludes location and description)
- **Write events directly to Google Calendar**
- Support for both command-line arguments and configuration files

## Installation

```bash
go build -o ical-sync ./cmd/ical-sync
```

## Usage

### Command Line

```bash
# Basic usage
./ical-sync -input "https://example.com/calendar.ics" -output "filtered.ics"

# With filtering
./ical-sync -input "https://example.com/calendar.ics" -output "filtered.ics" -filter "meeting,standup,*private*"

# With timeframe filtering
./ical-sync -input "https://example.com/calendar.ics" -start "2024-01-01" -end "2024-12-31"

# Combined filtering
./ical-sync -input "https://example.com/calendar.ics" -filter "meeting" -start "2024-06-01" -end "2024-06-30"

# Replace all event summaries with custom text
./ical-sync -input "https://example.com/calendar.ics" -summary "Busy"

# Write to Google Calendar
./ical-sync -input "https://example.com/calendar.ics" -gcal-id "your-calendar-id@group.calendar.google.com" -gcal-creds "credentials.json" -gcal-token "token.json"

# Using a config file
./ical-sync -config config.json
```

### Configuration File

Create a `config.json` file:

```json
{
  "input_url": "https://example.com/calendar.ics",
  "output_file": "filtered.ics",
  "filter_patterns": [
    "meeting",
    "standup",
    "*private*",
    "/^urgent.*/i"
  ],
  "start_date": "today",
  "end_date": "+30",
  "replacement_summary": "Busy",
  "google_calendar_id": "your-calendar-id@group.calendar.google.com",
  "google_credentials": "credentials.json",
  "google_token": "token.json"
}
```

**Note**: The `start_date` and `end_date` fields support multiple formats:
- Relative dates: `"today"`, `"tomorrow"`, `"yesterday"`
- Day offsets: `"+7"`, `"-30"`
- Absolute dates: `"2024-01-01"` or `"2024-01-01T00:00:00Z"`

## Filtering Options

### Title Pattern Filtering

The application supports several types of filter patterns:

1. **Simple text matching**: `"meeting"` - excludes events containing "meeting" in the title
2. **Wildcard patterns**: `"*private*"` - excludes events with "private" anywhere in the title
3. **Regular expressions**: `"/^urgent.*/i"` - excludes events starting with "urgent" (case-insensitive)

### Timeframe Filtering

Filter events by date range using flexible date formats:

- **Start date**: `-start "2024-01-01"` - only include events starting from this date
- **End date**: `-end "2024-12-31"` - only include events up to this date

#### Date Format Options

The application supports multiple date formats for command-line arguments:

1. **Absolute dates**: `YYYY-MM-DD` format
   - Example: `-start "2024-01-01" -end "2024-12-31"`

2. **Relative dates**: Named shortcuts
   - `today` - current date
   - `tomorrow` - next day
   - `yesterday` - previous day
   - Example: `-start today -end tomorrow`

3. **Day offsets**: Relative day counts using `+N` or `-N`
   - `+N` - N days from reference date
   - `-N` - N days before reference date
   - Example: `-start today -end +7` (next 7 days from today)
   - Example: `-start -30 -end today` (last 30 days)

**Note**: When using relative dates, the end date is calculated relative to the start date if both are specified. For example:
- `-start today -end +7` = today through 7 days from today
- `-start 2024-06-01 -end +30` = June 1st through 30 days after June 1st

**Config format**: Configuration files support all the same date formats (RFC3339, YYYY-MM-DD, relative dates, and day offsets)

### Summary Replacement

Replace event titles with custom text:

- **CLI argument**: `-summary "Busy"` - replaces all event summaries with "Busy"
- **Config field**: `"replacement_summary": "Busy"` - same functionality in config files
- **Output format**: Location and description fields are automatically excluded from output events

## Project Structure

```
ical-sync/
├── cmd/ical-sync/
│   └── main.go           # Main application entry point
├── internal/
│   ├── calendar/
│   │   └── calendar.go   # iCal parsing and generation
│   ├── config/
│   │   └── config.go     # Configuration handling
│   └── filter/
│       └── filter.go     # Event filtering logic
├── go.mod
└── README.md
```

## Examples

### Filter out all meetings and private events:
```bash
./ical-sync -input "https://calendar.example.com/cal.ics" -filter "meeting,*private*"
```

### Use regex to filter out events starting with "Test":
```bash
./ical-sync -input "https://calendar.example.com/cal.ics" -filter "/^test/i"
```

### Combine multiple filter types:
```bash
./ical-sync -input "https://calendar.example.com/cal.ics" -filter "standup,*holiday*,/.*review$/i"
```

### Filter events for a specific month:
```bash
./ical-sync -input "https://calendar.example.com/cal.ics" -start "2024-06-01" -end "2024-06-30"
```

### Filter events for the next 7 days:
```bash
./ical-sync -input "https://calendar.example.com/cal.ics" -start today -end +7
```

### Filter events for the last 30 days:
```bash
./ical-sync -input "https://calendar.example.com/cal.ics" -start -30 -end today
```

### Filter out meetings in the next quarter:
```bash
./ical-sync -input "https://calendar.example.com/cal.ics" -filter "meeting" -start "2024-07-01" -end "2024-09-30"
```

### Create privacy-focused calendar with generic titles:
```bash
./ical-sync -input "https://calendar.example.com/cal.ics" -summary "Busy" -output "private.ics"
```

### Combine filtering with summary replacement:
```bash
./ical-sync -input "https://calendar.example.com/cal.ics" -filter "*personal*" -summary "Meeting" -start "2024-06-01"
```

## Google Calendar Integration

### Setup

1. **Enable Google Calendar API**:
   - Go to [Google Cloud Console](https://console.cloud.google.com/)
   - Create a new project or select an existing one
   - Enable the Google Calendar API
   - Create OAuth 2.0 credentials (Desktop application)
   - Download the credentials and save as `credentials.json`

2. **First-time authentication**:
   ```bash
   ./ical-sync -input "https://calendar.example.com/cal.ics" \
     -gcal-id "your-calendar-id@group.calendar.google.com" \
     -gcal-creds "credentials.json" \
     -gcal-token "token.json"
   ```
   - On first run, you'll be prompted to authorize the application
   - A browser window will open for authentication
   - After authorization, the token will be saved to `token.json`

3. **Find your Calendar ID**:
   - Open Google Calendar
   - Click settings (gear icon) → Settings
   - Select the calendar from the left sidebar
   - Scroll down to "Integrate calendar"
   - Copy the "Calendar ID"

### Examples

#### Write filtered events to Google Calendar:
```bash
./ical-sync -input "https://calendar.example.com/cal.ics" \
  -filter "meeting" \
  -gcal-id "my-calendar@group.calendar.google.com" \
  -gcal-creds "credentials.json" \
  -gcal-token "token.json"
```

#### Write to both file and Google Calendar:
```bash
./ical-sync -input "https://calendar.example.com/cal.ics" \
  -output "filtered.ics" \
  -gcal-id "my-calendar@group.calendar.google.com" \
  -gcal-creds "credentials.json" \
  -gcal-token "token.json"
```

#### Using config file for Google Calendar:
```json
{
  "input_url": "https://calendar.example.com/cal.ics",
  "google_calendar_id": "my-calendar@group.calendar.google.com",
  "google_credentials": "credentials.json",
  "google_token": "token.json",
  "filter_patterns": ["meeting", "*personal*"],
  "replacement_summary": "Busy"
}
```