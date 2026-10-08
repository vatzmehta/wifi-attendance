# WiFi Attendance

A macOS menu bar app that automatically tracks office attendance by detecting your office WiFi network.

![Screenshot 2026-07-06 at 11.01.01 AM.png](Screenshot%202026-07-06%20at%2011.01.01%E2%80%AFAM.png)

## How it works

- Checks every 5 minutes if your office WiFi is connected
- If connected, marks the current day as attended (IST timezone)
- Displays `attended/required` in the menu bar (e.g. `6/14 ✓`)
- Warns (`⚠`) when you need to attend more than 80% of remaining working days to hit the monthly target
- All calculations are month-to-date, weekdays only (Mon–Fri), in IST
- Holidays and leaves you mark are excluded from working days
- **History** lists the last 12 months with their totals; hover a month to see its day-by-day calendar

## Policy

- **Monthly target**: 60% of working days in the month
- **Weekly minimum**: 3 days per week
- **Warning threshold**: If `days_still_needed / days_remaining > 80%`, a macOS notification fires (once per day)
- **Working day**: Mon–Fri, excluding any date marked as a holiday or leave. The weekly minimum drops by one for each weekday holiday or leave that week.

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
History ▸
  October 2026 · 3 of 13 required ▸
  September 2026 · 13 of 12 required ✓ ▸
    (calendar of the month: present, absent, holiday and leave days)
Holidays & Leaves ▸
  Mark Today as Holiday
  Mark Today as Leave
  Mark Holiday for Date…
  Mark Leave for Date…
  Marked dates (click to remove):
  Holiday · Fri 02 Oct 2026
  Leave · Fri 18 Sep 2026
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
| `~/Library/Application Support/wifi-attendance/daysoff.json` | Holiday and leave dates (ISO, IST) |
| `~/Library/LaunchAgents/com.vatzmehta.wifi-attendance.plist` | Login item (if enabled) |

## Makefile targets

| Target | Action |
|---|---|
| `make app` | Build `WiFiAttendance.app` |
| `make install` | Build + copy to `/Applications` + restart |
| `make run` | Build + `open` the app |
| `make clean` | Remove binary and `.app` |
