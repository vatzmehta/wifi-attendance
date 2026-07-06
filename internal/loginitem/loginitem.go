package loginitem

import (
	"fmt"
	"os"
	"path/filepath"
)

const plistLabel = "com.vatzmehta.wifi-attendance"

var plistContent = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.vatzmehta.wifi-attendance</string>
    <key>ProgramArguments</key>
    <array>
        <string>/Applications/WiFiAttendance.app/Contents/MacOS/wifi-attendance</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <false/>
</dict>
</plist>
`

func plistPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "Library", "LaunchAgents", plistLabel+".plist"), nil
}

// IsEnabled returns true if the LaunchAgent plist is installed.
func IsEnabled() bool {
	p, err := plistPath()
	if err != nil {
		return false
	}
	_, err = os.Stat(p)
	return err == nil
}

// Enable writes the LaunchAgent plist so the app starts on login.
func Enable() error {
	p, err := plistPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return fmt.Errorf("loginitem: mkdir: %w", err)
	}
	return os.WriteFile(p, []byte(plistContent), 0o644)
}

// Disable removes the LaunchAgent plist.
func Disable() error {
	p, err := plistPath()
	if err != nil {
		return err
	}
	if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("loginitem: remove: %w", err)
	}
	return nil
}
