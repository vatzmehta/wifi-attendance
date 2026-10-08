// Package history lays out past months of attendance as calendar grids.
package history

import (
	"time"

	"github.com/vatzmehta/wifi-attendance/internal/policy"
)

// State says how a calendar cell is drawn.
type State string

const (
	None    State = "" // padding, a day still to come, or a day before tracking began
	Present State = "present"
	Absent  State = "absent"
	Holiday State = "holiday"
	Leave   State = "leave"
	Weekend State = "weekend"
)

// Day is one cell of a month grid. Num is 0 for padding cells.
type Day struct {
	Num   int
	State State
}

// Month is one calendar month of attendance.
type Month struct {
	Title    string // e.g. "September 2026"
	Attended int
	Required int
	Mark     string   // "✓" target met, "⚠" at risk or missed, "" still on track
	Weeks    [][7]Day // Monday first
}

const isoDate = "2006-01-02"

// Build returns up to max months, newest first, from the current month back to the
// first attended day. days are ISO dates; off is the set of holidays and leaves.
// Pure function — no I/O, fully testable.
func Build(days []string, off policy.OffDays, now time.Time, loc *time.Location, max int) []Month {
	nowIST := now.In(loc)
	today := nowIST.Format(isoDate)

	attended := make(map[string]bool, len(days))
	first := today
	for _, d := range days {
		if _, err := time.Parse(isoDate, d); err != nil {
			continue // skip a malformed date instead of panicking on first[:7]
		}
		attended[d] = true
		if d < first {
			first = d
		}
	}
	// A day actually spent in office stays a working day even if it was also
	// marked as a holiday or leave — the same rule the menu bar uses.
	working := make(policy.OffDays, len(off))
	for d, kind := range off {
		if !attended[d] {
			working[d] = kind
		}
	}

	var months []Month
	year, month, _ := nowIST.Date()
	cur := time.Date(year, month, 1, 12, 0, 0, 0, loc)
	for len(months) < max && cur.Format(isoDate)[:7] >= first[:7] {
		months = append(months, buildMonth(cur, nowIST, first, today, attended, working, loc))
		cur = cur.AddDate(0, -1, 0)
	}
	return months
}

func buildMonth(start, nowIST time.Time, first, today string, attended map[string]bool, off policy.OffDays, loc *time.Location) Month {
	year, month, _ := start.Date()
	lastDay := start.AddDate(0, 1, -1).Day()

	// Past months are measured as of their last day; the current month as of now.
	asOf := time.Date(year, month, lastDay, 12, 0, 0, 0, loc)
	if asOf.After(nowIST) {
		asOf = nowIST
	}

	m := Month{Title: start.Format("January 2006")}
	// Pad so the 1st lands under its weekday in a Monday-first grid.
	cells := make([]Day, (int(start.Weekday())+6)%7)
	for d := 1; d <= lastDay; d++ {
		t := time.Date(year, month, d, 12, 0, 0, 0, loc)
		date := t.Format(isoDate)
		if attended[date] {
			m.Attended++
		}
		cells = append(cells, Day{Num: d, State: dayState(t, date, first, today, attended, off)})
	}
	for len(cells)%7 != 0 {
		cells = append(cells, Day{})
	}
	for i := 0; i < len(cells); i += 7 {
		m.Weeks = append(m.Weeks, [7]Day(cells[i:i+7]))
	}

	stats := policy.Calculate(m.Attended, 0, false, asOf, loc, off)
	m.Required = stats.Required
	switch {
	case stats.StillNeeded == 0:
		m.Mark = "✓"
	case stats.ShouldWarn:
		m.Mark = "⚠"
	}
	return m
}

func dayState(t time.Time, date, first, today string, attended map[string]bool, off policy.OffDays) State {
	wd := t.Weekday()
	switch {
	case attended[date]:
		return Present
	case wd == time.Saturday || wd == time.Sunday:
		return Weekend
	case off[date] == policy.Holiday:
		return Holiday
	case off[date] == policy.Leave:
		return Leave
	case date >= today || date < first:
		return None
	default:
		return Absent
	}
}
