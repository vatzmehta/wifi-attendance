package main

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/getlantern/systray"
	"github.com/vatzmehta/wifi-attendance/internal/attendance"
	"github.com/vatzmehta/wifi-attendance/internal/config"
	"github.com/vatzmehta/wifi-attendance/internal/daysoff"
	"github.com/vatzmehta/wifi-attendance/internal/history"
	"github.com/vatzmehta/wifi-attendance/internal/loginitem"
	"github.com/vatzmehta/wifi-attendance/internal/menucal"
	"github.com/vatzmehta/wifi-attendance/internal/notification"
	"github.com/vatzmehta/wifi-attendance/internal/policy"
	"github.com/vatzmehta/wifi-attendance/internal/wifi"
)

// iconBytes is a fallback 1×1 transparent PNG if the assets embed fails.
// Real icon loaded from assets/icon.png at build time via iconData below.
var iconBytes []byte

func main() {
	iconBytes = loadIcon()
	systray.Run(onReady, onExit)
}

func onReady() {
	systray.SetIcon(iconBytes)
	systray.SetTooltip("WiFi Attendance Tracker")

	ist, err := time.LoadLocation("Asia/Kolkata")
	if err != nil {
		ist = time.UTC
	}

	// Load or prompt for config
	cfg, err := config.Load()
	if err != nil {
		if !errors.Is(err, config.ErrNotConfigured) {
			fmt.Fprintf(os.Stderr, "config load error: %v\n", err)
		}
		ssid, promptErr := config.PromptSSID()
		if promptErr != nil {
			systray.SetTitle("⚠ Setup")
			addQuit()
			return
		}
		cfg = &config.Config{OfficeSSID: ssid}
		if gw, gwErr := wifi.DefaultGateway(); gwErr == nil {
			cfg.OfficeGateway = gw
		}
		if saveErr := config.Save(cfg); saveErr != nil {
			fmt.Fprintf(os.Stderr, "config save error: %v\n", saveErr)
		}
	}

	store, err := attendance.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "attendance load error: %v\n", err)
		store = &attendance.Store{}
	}

	off, err := daysoff.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "daysoff load error: %v\n", err)
		off = &daysoff.Store{}
	}

	throttle := &notification.Throttle{}

	// --- Build menu items ---
	mToday := systray.AddMenuItem("Today: checking…", "")
	mToday.Disable()

	systray.AddSeparator()

	mMonth := systray.AddMenuItem("Month: …", "")
	mMonth.Disable()
	mNeeded := systray.AddMenuItem("Need … more days for 60%", "")
	mNeeded.Disable()
	mWeek := systray.AddMenuItem("This week: …", "")
	mWeek.Disable()

	systray.AddSeparator()

	mWarn := systray.AddMenuItem("", "")
	mWarn.Hide()

	systray.AddSeparator()

	ssidLabel := fmt.Sprintf("Office WiFi: %q", cfg.OfficeSSID)
	if cfg.OfficeGateway != "" {
		ssidLabel = fmt.Sprintf("Office WiFi: %q (gw %s)", cfg.OfficeSSID, cfg.OfficeGateway)
	}
	mSSIDLabel := systray.AddMenuItem(ssidLabel, "")
	mSSIDLabel.Disable()
	mStatus := systray.AddMenuItem("Status: checking…", "")
	mStatus.Disable()
	mLastChecked := systray.AddMenuItem("Last checked: —", "")
	mLastChecked.Disable()

	systray.AddSeparator()

	mCheckNow := systray.AddMenuItem("Check Now", "Run a WiFi check immediately")
	mMarkDate := systray.AddMenuItem("Mark Attendance for Date…", "Manually mark a date as attended")

	// systray cannot add menu items later, so a fixed pool of month rows is shown
	// and hidden as needed. Each month's submenu holds one placeholder item that
	// menucal replaces with a drawn calendar.
	mHistory := systray.AddMenuItem("History", "Calendar of this and previous months")
	historyRows := make([]*systray.MenuItem, maxHistoryMonths)
	for i := range historyRows {
		historyRows[i] = mHistory.AddSubMenuItem("", "")
		historyRows[i].Hide()
		historyRows[i].AddSubMenuItem(calendarPlaceholder(i), "")
	}

	mDaysOff := systray.AddMenuItem("Holidays & Leaves", "Mark holidays and leaves; they are excluded from working days")
	mTodayHoliday := mDaysOff.AddSubMenuItem("Mark Today as Holiday", "")
	mTodayLeave := mDaysOff.AddSubMenuItem("Mark Today as Leave", "")
	mDateHoliday := mDaysOff.AddSubMenuItem("Mark Holiday for Date…", "Dates from the start of this month to 3 months ahead")
	mDateLeave := mDaysOff.AddSubMenuItem("Mark Leave for Date…", "Dates from the start of this month to 3 months ahead")
	mDaysOffHeader := mDaysOff.AddSubMenuItem("Marked dates (click to remove):", "")
	mDaysOffHeader.Disable()
	daysOffRows := make([]*systray.MenuItem, maxDaysOffRows)
	for i := range daysOffRows {
		daysOffRows[i] = mDaysOff.AddSubMenuItem("", "")
		daysOffRows[i].Hide()
	}
	mDaysOffMore := mDaysOff.AddSubMenuItem("", "")
	mDaysOffMore.Disable()
	mDaysOffMore.Hide()

	// rowClicked carries the index of a clicked list row; one forwarder per row
	// because select cannot range over a slice of channels.
	rowClicked := make(chan int)
	for i, row := range daysOffRows {
		go func(i int, clicks <-chan struct{}) {
			for range clicks {
				rowClicked <- i
			}
		}(i, row.ClickedCh)
	}
	// listed mirrors what daysOffRows currently show. Only the menu goroutine touches it.
	var listed []daysoff.Entry

	mChangeSSID := systray.AddMenuItem("Change Office WiFi", "Update the office WiFi name")
	mCaptureGateway := systray.AddMenuItem("Capture Office Gateway", "Save current router IP as the office gateway")

	loginLabel := "Launch at Login"
	if loginitem.IsEnabled() {
		loginLabel = "Launch at Login ✓"
	}
	mLoginItem := systray.AddMenuItem(loginLabel, "Toggle auto-start on macOS login")

	systray.AddSeparator()
	addQuit()

	updateSSIDLabel := func() {
		label := fmt.Sprintf("Office WiFi: %q", cfg.OfficeSSID)
		if cfg.OfficeGateway != "" {
			label = fmt.Sprintf("Office WiFi: %q (gw %s)", cfg.OfficeSSID, cfg.OfficeGateway)
		}
		mSSIDLabel.SetTitle(label)
	}

	refreshDaysOffList := func(nowIST time.Time) {
		from, to := daysOffWindow(nowIST)
		listed = off.Between(from, to)
		for i, row := range daysOffRows {
			if i < len(listed) {
				row.SetTitle(fmt.Sprintf("%s · %s", offKindLabel(listed[i].Kind), prettyDate(listed[i].Date)))
				row.Show()
			} else {
				row.Hide()
			}
		}
		if extra := len(listed) - maxDaysOffRows; extra > 0 {
			mDaysOffMore.SetTitle(fmt.Sprintf("…and %d more", extra))
			mDaysOffMore.Show()
		} else {
			mDaysOffMore.Hide()
		}
	}

	refreshHistory := func(now time.Time) {
		months := history.Build(store.Days, off.OffDays(), now, ist, maxHistoryMonths)
		for i, row := range historyRows {
			if i >= len(months) {
				row.Hide()
				continue
			}
			m := months[i]
			row.SetTitle(strings.TrimSpace(fmt.Sprintf("%s · %d of %d required %s", m.Title, m.Attended, m.Required, m.Mark)))
			row.Show()
			menucal.Set(calendarPlaceholder(i), calendarCells(m))
		}
	}

	updateMenu := func() {
		now := time.Now()
		nowIST := now.In(ist)

		atOffice, _ := wifi.IsAtOffice(cfg.OfficeSSID, cfg.OfficeGateway)
		if atOffice {
			changed := store.MarkToday(ist)
			if changed {
				if saveErr := store.Save(); saveErr != nil {
					fmt.Fprintf(os.Stderr, "attendance save error: %v\n", saveErr)
				}
			}
		}

		year, month, _ := nowIST.Date()
		attended := store.DaysThisMonth(year, month, ist)
		weekAttended := store.DaysThisWeek(ist)
		presentToday := store.IsPresentToday(ist)
		// A day actually spent in office stays a working day even if it was later
		// marked as a holiday or leave, so attended can never exceed required.
		offDays := off.OffDays()
		for _, d := range store.Days {
			delete(offDays, d)
		}
		stats := policy.Calculate(attended, weekAttended, presentToday, now, ist, offDays)

		// Menu bar title
		systray.SetTitle(stats.MenuLabel)

		// Today
		switch {
		case presentToday:
			mToday.SetTitle("Today: Present ✓")
		case stats.TodayOff == policy.Holiday:
			mToday.SetTitle("Today: Holiday")
		case stats.TodayOff == policy.Leave:
			mToday.SetTitle("Today: On leave")
		default:
			mToday.SetTitle("Today: Not yet marked")
		}

		// Month stats
		mMonth.SetTitle(fmt.Sprintf("Month: %d of %d working days attended",
			stats.Attended, stats.WorkingDaysSoFar))
		if stats.StillNeeded == 0 {
			mNeeded.SetTitle(fmt.Sprintf("Target met ✓ (%d required)", stats.Required))
		} else {
			mNeeded.SetTitle(fmt.Sprintf("Need %d more days to reach 60%% (%d required)",
				stats.StillNeeded, stats.Required))
		}
		if stats.WeekRequired == 0 {
			mWeek.SetTitle("This week: no working days")
		} else {
			mWeek.SetTitle(fmt.Sprintf("This week: %d of %d days", stats.WeekAttended, stats.WeekRequired))
		}
		refreshDaysOffList(nowIST)
		refreshHistory(now)

		// Warning
		if stats.ShouldWarn {
			mWarn.SetTitle(fmt.Sprintf("⚠ At risk: need %d days in %d remaining",
				stats.StillNeeded, stats.WorkingDaysRemaining))
			mWarn.Show()
		} else {
			mWarn.Hide()
		}

		// WiFi / gateway status
		if atOffice {
			mStatus.SetTitle("Status: At office ✓")
		} else {
			mStatus.SetTitle("Status: Not at office")
		}
		mLastChecked.SetTitle("Last checked: " + nowIST.Format("3:04 PM IST"))

		// Notification (at most once per day)
		todayStr := nowIST.Format("2006-01-02")
		if stats.ShouldWarn && throttle.ShouldNotify(todayStr) {
			_ = notification.SendWarning(stats.StillNeeded, stats.WorkingDaysRemaining)
			throttle.MarkNotified(todayStr)
		}
	}

	// markDayOff records a holiday or leave and refreshes the menu.
	markDayOff := func(kind policy.OffKind, date string) {
		if off.Mark(kind, date) {
			if saveErr := off.Save(); saveErr != nil {
				fmt.Fprintf(os.Stderr, "daysoff save error: %v\n", saveErr)
			}
		}
		updateMenu()
	}

	// promptDayOff asks for a date and marks it if it falls inside the allowed window.
	promptDayOff := func(kind policy.OffKind) {
		nowIST := time.Now().In(ist)
		label := offKindLabel(kind)
		dateStr, err := config.PromptDateWith(
			fmt.Sprintf("Enter date to mark as %s (DD/MM/YYYY):", strings.ToLower(label)),
			"Mark "+label, "Mark", nowIST.Format("02/01/2006"))
		if err != nil {
			return
		}
		from, to := daysOffWindow(nowIST)
		if dateStr < from || dateStr > to {
			config.ShowAlert(fmt.Sprintf("Holidays and leaves can only be marked from %s (start of this month) to %s (3 months from today).",
				prettyDate(from), prettyDate(to)))
			return
		}
		markDayOff(kind, dateStr)
	}

	// Initial check
	updateMenu()

	// Ticker goroutine
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				updateMenu()
			case <-mCheckNow.ClickedCh:
				updateMenu()
			case <-mMarkDate.ClickedCh:
				defaultDate := time.Now().In(ist).Format("02/01/2006")
				dateStr, err := config.PromptDate(defaultDate)
				if err != nil {
					continue
				}
				store.MarkDate(dateStr)
				if saveErr := store.Save(); saveErr != nil {
					fmt.Fprintf(os.Stderr, "attendance save error: %v\n", saveErr)
				}
				updateMenu()
			case <-mTodayHoliday.ClickedCh:
				markDayOff(policy.Holiday, time.Now().In(ist).Format("2006-01-02"))
			case <-mTodayLeave.ClickedCh:
				markDayOff(policy.Leave, time.Now().In(ist).Format("2006-01-02"))
			case <-mDateHoliday.ClickedCh:
				promptDayOff(policy.Holiday)
			case <-mDateLeave.ClickedCh:
				promptDayOff(policy.Leave)
			case i := <-rowClicked:
				if i >= len(listed) {
					continue
				}
				entry := listed[i]
				if !config.PromptConfirm(fmt.Sprintf("Remove %s on %s?", strings.ToLower(offKindLabel(entry.Kind)), prettyDate(entry.Date)), "Remove") {
					continue
				}
				if off.Unmark(entry.Date) {
					if saveErr := off.Save(); saveErr != nil {
						fmt.Fprintf(os.Stderr, "daysoff save error: %v\n", saveErr)
					}
				}
				updateMenu()
			case <-mLoginItem.ClickedCh:
				if loginitem.IsEnabled() {
					_ = loginitem.Disable()
					mLoginItem.SetTitle("Launch at Login")
				} else {
					_ = loginitem.Enable()
					mLoginItem.SetTitle("Launch at Login ✓")
				}
			case <-mChangeSSID.ClickedCh:
				ssid, err := config.PromptSSID()
				if err != nil {
					continue
				}
				cfg.OfficeSSID = ssid
				if gw, gwErr := wifi.DefaultGateway(); gwErr == nil {
					cfg.OfficeGateway = gw
				}
				_ = config.Save(cfg)
				updateSSIDLabel()
				updateMenu()
			case <-mCaptureGateway.ClickedCh:
				gw, err := wifi.DefaultGateway()
				if err != nil {
					continue
				}
				cfg.OfficeGateway = gw
				_ = config.Save(cfg)
				updateSSIDLabel()
				updateMenu()
			}
		}
	}()
}

func onExit() {}

func addQuit() {
	mQuit := systray.AddMenuItem("Quit", "Quit WiFi Attendance")
	go func() {
		<-mQuit.ClickedCh
		systray.Quit()
	}()
}

// maxHistoryMonths caps the History submenu at the current month plus the eleven before it.
const maxHistoryMonths = 12

// calendarPlaceholder is the title of the menu item that carries month i's calendar.
func calendarPlaceholder(i int) string {
	return fmt.Sprintf("Calendar %d", i+1)
}

// calendarCells encodes a month's grid for menucal.Set, one character per cell.
func calendarCells(m history.Month) string {
	codes := map[history.State]byte{
		history.Present: 'p',
		history.Absent:  'a',
		history.Holiday: 'h',
		history.Leave:   'l',
		history.Weekend: 'w',
		history.None:    'n',
	}
	var b strings.Builder
	for _, week := range m.Weeks {
		for _, d := range week {
			if d.Num == 0 {
				b.WriteByte('.')
			} else {
				b.WriteByte(codes[d.State])
			}
		}
	}
	return b.String()
}

// maxDaysOffRows caps how many marked dates the submenu lists; systray cannot
// delete menu items, so a fixed pool of rows is shown and hidden as needed.
const maxDaysOffRows = 24

// daysOffWindow returns the inclusive ISO date range in which a holiday or leave may
// be marked: the first of the current month through three months from today.
func daysOffWindow(now time.Time) (from, to string) {
	year, month, _ := now.Date()
	from = time.Date(year, month, 1, 0, 0, 0, 0, now.Location()).Format("2006-01-02")
	to = now.AddDate(0, 3, 0).Format("2006-01-02")
	return from, to
}

// offKindLabel returns the menu label for a kind of day off.
func offKindLabel(kind policy.OffKind) string {
	if kind == policy.Holiday {
		return "Holiday"
	}
	return "Leave"
}

// prettyDate formats an ISO date for display, e.g. "Fri 02 Oct 2026".
func prettyDate(iso string) string {
	t, err := time.Parse("2006-01-02", iso)
	if err != nil {
		return iso
	}
	return t.Format("Mon 02 Jan 2006")
}

func loadIcon() []byte {
	data, err := os.ReadFile("assets/icon.png")
	if err != nil {
		// minimal 1×1 white PNG fallback
		return []byte{
			0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a,
			0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52,
			0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
			0x08, 0x02, 0x00, 0x00, 0x00, 0x90, 0x77, 0x53,
			0xde, 0x00, 0x00, 0x00, 0x0c, 0x49, 0x44, 0x41,
			0x54, 0x08, 0xd7, 0x63, 0xf8, 0xcf, 0xc0, 0x00,
			0x00, 0x00, 0x02, 0x00, 0x01, 0xe2, 0x21, 0xbc,
			0x33, 0x00, 0x00, 0x00, 0x00, 0x49, 0x45, 0x4e,
			0x44, 0xae, 0x42, 0x60, 0x82,
		}
	}
	return data
}
