package policy

import (
	"fmt"
	"math"
	"time"
)

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
	ShouldWarn           bool   // need >80% of remaining to hit target
	MenuLabel            string // e.g. "6/10 ✓"
}

// Calculate derives all attendance stats from attended counts and the current time.
// Pure function — no I/O, fully testable.
func Calculate(attended, attendedThisWeek int, presentToday bool, now time.Time, loc *time.Location) Stats {
	nowIST := now.In(loc)
	year, month, todayDay := nowIST.Date()

	soFar := countWorkingDays(year, month, 1, todayDay, loc)
	lastDay := daysInMonth(year, month)
	remaining := countWorkingDays(year, month, todayDay+1, lastDay, loc)
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

	return Stats{
		WorkingDaysSoFar:     soFar,
		WorkingDaysRemaining: remaining,
		TotalWorkingDays:     total,
		Required:             required,
		Attended:             attended,
		StillNeeded:          stillNeeded,
		PresentToday:         presentToday,
		WeekAttended:         attendedThisWeek,
		ShouldWarn:           shouldWarn,
		MenuLabel:            label,
	}
}

// countWorkingDays counts Mon–Fri days between fromDay and toDay (inclusive) in the given month.
func countWorkingDays(year int, month time.Month, fromDay, toDay int, loc *time.Location) int {
	count := 0
	for d := fromDay; d <= toDay; d++ {
		t := time.Date(year, month, d, 12, 0, 0, 0, loc)
		wd := t.Weekday()
		if wd != time.Saturday && wd != time.Sunday {
			count++
		}
	}
	return count
}

func daysInMonth(year int, month time.Month) int {
	return time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC).Day()
}
