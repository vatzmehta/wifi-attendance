package wifi

import (
	"fmt"
	"os/exec"
	"strings"
)

// CurrentSSID returns the SSID of the currently connected WiFi network.
// Tries en0, en1, en2 in order. Returns "" if not connected.
func CurrentSSID() (string, error) {
	for _, iface := range []string{"en0", "en1", "en2"} {
		ssid, err := queryInterface(iface)
		if err == nil && ssid != "" {
			return ssid, nil
		}
	}
	return "", nil
}

// IsConnectedTo returns true if the current WiFi SSID matches the given ssid (case-insensitive).
func IsConnectedTo(ssid string) (bool, error) {
	current, err := CurrentSSID()
	if err != nil {
		return false, err
	}
	return strings.EqualFold(strings.TrimSpace(current), strings.TrimSpace(ssid)), nil
}

func queryInterface(iface string) (string, error) {
	out, err := exec.Command("networksetup", "-getairportnetwork", iface).Output()
	if err != nil {
		return "", fmt.Errorf("wifi: networksetup %s: %w", iface, err)
	}
	line := strings.TrimSpace(string(out))
	// "Current Wi-Fi Network: OfficeSSID"
	// "You are not associated with an AirPort network."
	const prefix = "Current Wi-Fi Network: "
	if !strings.HasPrefix(line, prefix) {
		return "", nil
	}
	return strings.TrimPrefix(line, prefix), nil
}
