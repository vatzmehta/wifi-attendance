package main

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/getlantern/systray"
	"github.com/vatzmehta/wifi-attendance/internal/attendance"
	"github.com/vatzmehta/wifi-attendance/internal/config"
	"github.com/vatzmehta/wifi-attendance/internal/loginitem"
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
		store, _ = attendance.Load()
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
		stats := policy.Calculate(attended, weekAttended, presentToday, now, ist)

		// Menu bar title
		systray.SetTitle(stats.MenuLabel)

		// Today
		if presentToday {
			mToday.SetTitle("Today: Present ✓")
		} else {
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
		mWeek.SetTitle(fmt.Sprintf("This week: %d of 3 days", stats.WeekAttended))

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
