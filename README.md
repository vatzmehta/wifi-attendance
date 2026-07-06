# WiFi Attendance

A macOS menu bar app that automatically tracks office attendance by detecting your office WiFi network.

## How it works

- Checks every 5 minutes if your office WiFi is connected
- If connected, marks the current day as attended (IST timezone)
- Displays `attended/required` in the menu bar (e.g. `6/14 ✓`)
- Warns (`⚠`) when you need to attend more than 80% of remaining working days to hit the monthly target
- All calculations are month-to-date, weekdays only (Mon–Fri), in IST

## Policy

- **Monthly target**: 60% of working days in the month
- **Weekly minimum**: 3 days per week
- **Warning threshold**: If `days_still_needed / days_remaining > 80%`, a macOS notification fires (once per day)

## Menu bar

```
6/14 ✓
─────────────────────────
Today: Present ✓
─────────────────────────
Month: 6 of 10 working days attended
Need 8 more days to reach 60% (14 required)
This week: 2 of 3 days
─────────────────────────
Office WiFi: "..."
Status: Connected ✓
Last checked: 2:35 PM IST
─────────────────────────
Check Now
Mark Attendance for Date…
Change Office WiFi
Launch at Login
─────────────────────────
Quit
```

## Requirements

- macOS 12+
- Go 1.21+

## Build & install

```bash
git clone https://github.com/vatzmehta/wifi-attendance
cd wifi-attendance
make install     # builds WiFiAttendance.app and copies to /Applications
```

Then launch:

```bash
open /Applications/WiFiAttendance.app
```

On first launch, a dialog asks for your office WiFi network name (SSID). This is stored locally in `~/Library/Application Support/wifi-attendance/config.json` and never leaves your machine.

## Data stored locally

| File | Contents |
|---|---|
| `~/Library/Application Support/wifi-attendance/config.json` | Office WiFi SSID |
| `~/Library/Application Support/wifi-attendance/attendance.json` | Attended dates (ISO, IST) |
| `~/Library/LaunchAgents/com.vatzmehta.wifi-attendance.plist` | Login item (if enabled) |

## Makefile targets

| Target | Action |
|---|---|
| `make app` | Build `WiFiAttendance.app` |
| `make install` | Build + copy to `/Applications` + restart |
| `make run` | Build + `open` the app |
| `make clean` | Remove binary and `.app` |
