package policy

import (
	"fmt"
	"math"
	"time"
)

// OffKind says why a date is not a working day.
type OffKind string

const (
	Holiday OffKind = "holiday"
	Leave   OffKind = "leave"
)

// OffDays maps an ISO date (YYYY-MM-DD) to why it is not a working day. nil means none.
type OffDays map[string]OffKind

// Stats holds all computed attendance metrics for the current month.
type Stats struct {
	WorkingDaysSoFar     int
	WorkingDaysRemaining int
	TotalWorkingDays     int
	Required             int // ceil(Total * 0.60)
	Attended             int
	StillNeeded          int // max(0, Required - Attended)
	PresentToday         bool
	WeekAttended         int
	WeekRequired         int     // max(0, 3 - weekday off-days in the current Mon–Fri week)
	TodayOff             OffKind // "" when today is a working day or a weekend
	ShouldWarn           bool    // need >80% of remaining to hit target
	MenuLabel            string  // e.g. "6/10 ✓"
}

// Calculate derives all attendance stats from attended counts, the current time, and days off.
// Pure function — no I/O, fully testable.
func Calculate(attended, attendedThisWeek int, presentToday bool, now time.Time, loc *time.Location, off OffDays) Stats {
	nowIST := now.In(loc)
	year, month, todayDay := nowIST.Date()

	soFar := countWorkingDays(year, month, 1, todayDay, loc, off)
	lastDay := daysInMonth(year, month)
	remaining := countWorkingDays(year, month, todayDay+1, lastDay, loc, off)
	total := soFar + remaining
	required := int(math.Ceil(float64(total) * 0.60))
	stillNeeded := max(0, required-attended)

	var shouldWarn bool
	if remaining > 0 {
		shouldWarn = float64(stillNeeded)/float64(remaining) > 0.80
	} else {
		shouldWarn = stillNeeded > 0
	}

	indicator := "✓"
	if shouldWarn {
		indicator = "⚠"
	}
	label := fmt.Sprintf("%d/%d %s", attended, required, indicator)

	var todayOff OffKind
	wd := nowIST.Weekday()
	if wd != time.Saturday && wd != time.Sunday {
		todayOff = off[nowIST.Format("2006-01-02")]
	}

	return Stats{
		WorkingDaysSoFar:     soFar,
		WorkingDaysRemaining: remaining,
		TotalWorkingDays:     total,
		Required:             required,
		Attended:             attended,
		StillNeeded:          stillNeeded,
		PresentToday:         presentToday,
		WeekAttended:         attendedThisWeek,
		WeekRequired:         max(0, 3-offDaysThisWeek(nowIST, off)),
		TodayOff:             todayOff,
		ShouldWarn:           shouldWarn,
		MenuLabel:            label,
	}
}

// countWorkingDays counts Mon–Fri days between fromDay and toDay (inclusive) in the given month
// that are not marked as a day off.
func countWorkingDays(year int, month time.Month, fromDay, toDay int, loc *time.Location, off OffDays) int {
	count := 0
	for d := fromDay; d <= toDay; d++ {
		t := time.Date(year, month, d, 12, 0, 0, 0, loc)
		wd := t.Weekday()
		if wd != time.Saturday && wd != time.Sunday && off[t.Format("2006-01-02")] == "" {
			count++
		}
	}
	return count
}

// offDaysThisWeek counts off-days on Mon–Fri of the week containing now.
func offDaysThisWeek(now time.Time, off OffDays) int {
	weekday := int(now.Weekday())
	if weekday == 0 {
		weekday = 7 // Sunday → 7
	}
	monday := now.AddDate(0, 0, -(weekday - 1))
	count := 0
	for i := 0; i < 5; i++ {
		if off[monday.AddDate(0, 0, i).Format("2006-01-02")] != "" {
			count++
		}
	}
	return count
}

func daysInMonth(year int, month time.Month) int {
	return time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC).Day()
}
