package policy_test

import (
	"testing"
	"time"

	"github.com/vatzmehta/wifi-attendance/internal/policy"
)

// September 2026: 22 weekdays; the 1st is a Tuesday.

// off is a small helper to build an OffDays map for a given kind and dates.
func off(kind policy.OffKind, dates ...string) policy.OffDays {
	m := make(policy.OffDays, len(dates))
	for _, d := range dates {
		m[d] = kind
	}
	return m
}

// restOfMonth is every remaining weekday from the 16th through the 30th.
var restOfMonth = []string{
	"2026-09-16", "2026-09-17", "2026-09-18",
	"2026-09-21", "2026-09-22", "2026-09-23", "2026-09-24", "2026-09-25",
	"2026-09-28", "2026-09-29", "2026-09-30",
}

func TestCalculate(t *testing.T) {
	loc, _ := time.LoadLocation("Asia/Kolkata")

	tests := []struct {
		name             string
		day              int
		attended         int
		attendedThisWeek int
		presentToday     bool
		off              policy.OffDays
		want             policy.Stats
	}{
		{
			name:             "no days off",
			day:              15,
			attended:         8,
			attendedThisWeek: 2,
			presentToday:     true,
			off:              nil,
			want: policy.Stats{
				WorkingDaysSoFar:     11,
				WorkingDaysRemaining: 11,
				TotalWorkingDays:     22,
				Required:             14,
				Attended:             8,
				StillNeeded:          6,
				PresentToday:         true,
				WeekAttended:         2,
				WeekRequired:         3,
				TodayOff:             "",
				ShouldWarn:           false,
				MenuLabel:            "8/14 ✓",
			},
		},
		// A marked date is off regardless of attendance. main.go deletes attended
		// dates from the set before calling Calculate, so a day actually attended
		// stays a working day.
		{
			name:             "holiday on the 14th reduces the denominator",
			day:              15,
			attended:         8,
			attendedThisWeek: 1,
			presentToday:     true,
			off:              off(policy.Holiday, "2026-09-14"),
			want: policy.Stats{
				WorkingDaysSoFar:     10,
				WorkingDaysRemaining: 11,
				TotalWorkingDays:     21,
				Required:             13,
				Attended:             8,
				StillNeeded:          5,
				PresentToday:         true,
				WeekAttended:         1,
				WeekRequired:         2,
				TodayOff:             "",
				ShouldWarn:           false,
				MenuLabel:            "8/13 ✓",
			},
		},
		{
			name:             "today is a holiday",
			day:              14,
			attended:         7,
			attendedThisWeek: 0,
			presentToday:     false,
			off:              off(policy.Holiday, "2026-09-14"),
			want: policy.Stats{
				WorkingDaysSoFar:     9,
				WorkingDaysRemaining: 12,
				TotalWorkingDays:     21,
				Required:             13,
				Attended:             7,
				StillNeeded:          6,
				PresentToday:         false,
				WeekAttended:         0,
				WeekRequired:         2,
				TodayOff:             policy.Holiday,
				ShouldWarn:           false,
				MenuLabel:            "7/13 ✓",
			},
		},
		{
			name:             "all remaining days off, target missed",
			day:              15,
			attended:         5,
			attendedThisWeek: 2,
			presentToday:     true,
			off:              off(policy.Leave, restOfMonth...),
			want: policy.Stats{
				WorkingDaysSoFar:     11,
				WorkingDaysRemaining: 0,
				TotalWorkingDays:     11,
				Required:             7,
				Attended:             5,
				StillNeeded:          2,
				PresentToday:         true,
				WeekAttended:         2,
				WeekRequired:         0,
				TodayOff:             "",
				ShouldWarn:           true,
				MenuLabel:            "5/7 ⚠",
			},
		},
		{
			name:             "all remaining days off, target met",
			day:              15,
			attended:         7,
			attendedThisWeek: 2,
			presentToday:     true,
			off:              off(policy.Leave, restOfMonth...),
			want: policy.Stats{
				WorkingDaysSoFar:     11,
				WorkingDaysRemaining: 0,
				TotalWorkingDays:     11,
				Required:             7,
				Attended:             7,
				StillNeeded:          0,
				PresentToday:         true,
				WeekAttended:         2,
				WeekRequired:         0,
				TodayOff:             "",
				ShouldWarn:           false,
				MenuLabel:            "7/7 ✓",
			},
		},
		{
			name:             "full week off gives WeekRequired 0",
			day:              16,
			attended:         6,
			attendedThisWeek: 0,
			presentToday:     false,
			off: off(policy.Leave,
				"2026-09-14", "2026-09-15", "2026-09-16", "2026-09-17", "2026-09-18",
			),
			want: policy.Stats{
				WorkingDaysSoFar:     9,
				WorkingDaysRemaining: 8,
				TotalWorkingDays:     17,
				Required:             11,
				Attended:             6,
				StillNeeded:          5,
				PresentToday:         false,
				WeekAttended:         0,
				WeekRequired:         0,
				TodayOff:             policy.Leave,
				ShouldWarn:           false,
				MenuLabel:            "6/11 ✓",
			},
		},
		{
			name:             "weekend mark is ignored",
			day:              19,
			attended:         6,
			attendedThisWeek: 3,
			presentToday:     false,
			off:              off(policy.Holiday, "2026-09-19"),
			want: policy.Stats{
				WorkingDaysSoFar:     14,
				WorkingDaysRemaining: 8,
				TotalWorkingDays:     22,
				Required:             14,
				Attended:             6,
				StillNeeded:          8,
				PresentToday:         false,
				WeekAttended:         3,
				WeekRequired:         3,
				TodayOff:             "",
				ShouldWarn:           true,
				MenuLabel:            "6/14 ⚠",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			now := time.Date(2026, time.September, tc.day, 10, 0, 0, 0, loc)
			got := policy.Calculate(tc.attended, tc.attendedThisWeek, tc.presentToday, now, loc, tc.off)
			if got != tc.want {
				t.Errorf("Calculate() = %+v\nwant %+v", got, tc.want)
			}
		})
	}
}
