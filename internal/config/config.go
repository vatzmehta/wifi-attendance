package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Config holds app configuration persisted to disk.
type Config struct {
	OfficeSSID string `json:"office_ssid"`
}

// ErrNotConfigured is returned when no config file exists yet.
var ErrNotConfigured = errors.New("wifi-attendance: no office SSID configured")

// Load reads config from ~/Library/Application Support/wifi-attendance/config.json.
func Load() (*Config, error) {
	dir, err := appSupportDir()
	if err != nil {
		return nil, err
	}
	path := filepath.Join(dir, "config.json")
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrNotConfigured
	}
	if err != nil {
		return nil, fmt.Errorf("config: read: %w", err)
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("config: parse: %w", err)
	}
	if cfg.OfficeSSID == "" {
		return nil, ErrNotConfigured
	}
	return &cfg, nil
}

// Save writes config atomically to disk.
func Save(c *Config) error {
	dir, err := appSupportDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("config: mkdir: %w", err)
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("config: marshal: %w", err)
	}
	path := filepath.Join(dir, "config.json")
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("config: write tmp: %w", err)
	}
	return os.Rename(tmp, path)
}

// PromptSSID opens an osascript dialog asking the user to enter their office WiFi SSID.
func PromptSSID() (string, error) {
	script := `display dialog "Enter your office WiFi network name (SSID):" ` +
		`default answer "" with title "WiFi Attendance Setup" ` +
		`buttons {"Cancel", "OK"} default button "OK"`
	out, err := exec.Command("osascript", "-e", script).Output()
	if err != nil {
		return "", fmt.Errorf("config: prompt cancelled or failed: %w", err)
	}
	// Output: "button returned:OK, text returned:MySSID\n"
	result := strings.TrimSpace(string(out))
	_, after, found := strings.Cut(result, "text returned:")
	if !found {
		return "", fmt.Errorf("config: unexpected osascript output: %s", result)
	}
	ssid := strings.TrimSpace(after)
	if ssid == "" {
		return "", fmt.Errorf("config: empty SSID entered")
	}
	return ssid, nil
}

// PromptDate opens an osascript dialog asking for a date in DD/MM/YYYY format.
// Returns the date as a YYYY-MM-DD string on success.
func PromptDate(defaultDate string) (string, error) {
	script := fmt.Sprintf(
		`display dialog "Enter date to mark as attended (DD/MM/YYYY):" `+
			`default answer %q with title "Mark Attendance" `+
			`buttons {"Cancel", "Mark"} default button "Mark"`,
		defaultDate,
	)
	out, err := exec.Command("osascript", "-e", script).Output()
	if err != nil {
		return "", fmt.Errorf("config: date prompt cancelled: %w", err)
	}
	_, after, found := strings.Cut(strings.TrimSpace(string(out)), "text returned:")
	if !found {
		return "", fmt.Errorf("config: unexpected osascript output: %s", string(out))
	}
	input := strings.TrimSpace(after)
	t, err := time.Parse("02/01/2006", input)
	if err != nil {
		return "", fmt.Errorf("config: invalid date %q — use DD/MM/YYYY", input)
	}
	return t.Format("2006-01-02"), nil
}

func appSupportDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("config: home dir: %w", err)
	}
	return filepath.Join(home, "Library", "Application Support", "wifi-attendance"), nil
}
