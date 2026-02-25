# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build & Install

```bash
# Build
go build ./cmd/ical-sync

# Build and install to user bin
go build -o ~/.local/bin/ical-sync ./cmd/ical-sync

# After changes to systemd units, copy and reload
cp systemd/ical-sync.service systemd/ical-sync.timer ~/.config/systemd/user/
systemctl --user daemon-reload
```

No tests exist in this project.

## Architecture

Go application that fetches iCal events from a URL, filters/transforms them, then writes to an iCal file and/or Google Calendar. Runs on a systemd user timer (hourly).

**Pipeline (in `cmd/ical-sync/main.go`):**
1. Load config (file + CLI flag overrides)
2. Fetch events from iCal URL
3. Normalize Windows timezone IDs → IANA
4. Expand recurring events (RRULE) into individual instances within date window
5. Filter by: title patterns (text/wildcard/regex), date range, acceptance status
6. Clean properties (strip description, location, X-properties; apply replacement summary)
7. Apply cache buster suffix to UIDs (if configured)
8. Write to iCal file and/or Google Calendar

**Key design details:**
- Recurring event expansion generates unique UIDs: `{originalUID}_{occurrenceDatetime}`
- Google Calendar deduplication relies on `iCalUID` — the `cache_buster` config field appends `_{value}` to all UIDs, forcing re-creation of previously deleted events
- `eventExists` checks include `ShowDeleted(true)` so deleted gcal events aren't recreated (unless cache buster changes)
- The `only_accepted` filter checks `X-MICROSOFT-CDO-BUSYSTATUS` first (Outlook feeds), falls back to standard ATTENDEE PARTSTAT
- End dates in relative config (`"+7"`) are calculated relative to the start date, not today

## Runtime config

Config lives at `~/.config/ical-sync/config.json`. Google OAuth credentials and token are also stored there. The systemd service sends a desktop notification (`notify-send`) on failure.
