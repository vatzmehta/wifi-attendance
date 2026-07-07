package wifi

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// CurrentSSID returns the SSID of the currently connected WiFi network.
// Returns "" if not connected.
func CurrentSSID() (string, error) {
	// Native CoreWLAN path is the most reliable on macOS when entitlements allow it.
	if ssid, err := currentSSIDNative(); err == nil && ssid != "" {
		return ssid, nil
	}

	// system_profiler works on modern macOS when it is not privacy-redacted.
	ssid, err := ssidFromSystemProfiler()
	if err == nil && ssid != "" {
		return ssid, nil
	}

	// Fallback to networksetup for older macOS versions.
	ifaces, err := wifiInterfaces()
	if err != nil {
		ifaces = []string{"en0", "en1", "en2"}
	}
	for _, iface := range ifaces {
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

// IsAtOffice returns true if the current network is the configured office network.
// It prefers SSID matching, then falls back to the office gateway IP if the SSID is
// hidden by macOS privacy/entitlement rules.
func IsAtOffice(officeSSID, officeGateway string) (bool, error) {
	connected, err := IsConnectedTo(officeSSID)
	if err == nil && connected {
		return true, nil
	}

	if officeGateway == "" {
		return false, err
	}

	gw, err := DefaultGateway()
	if err != nil {
		return false, err
	}
	if gw != officeGateway {
		return false, nil
	}

	// Make sure the default route is through a Wi-Fi interface, not Ethernet/VPN.
	defIface, err := DefaultInterface()
	if err != nil {
		return false, err
	}
	wifiIfaces, err := wifiInterfaces()
	if err != nil {
		return false, err
	}
	for _, iface := range wifiIfaces {
		if iface == defIface {
			return true, nil
		}
	}
	return false, nil
}

// DefaultGateway returns the IPv4 address of the current default gateway.
func DefaultGateway() (string, error) {
	out, err := exec.Command("route", "-n", "get", "default").Output()
	if err != nil {
		return "", fmt.Errorf("wifi: route get default: %w", err)
	}
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[0] == "gateway:" {
			return fields[1], nil
		}
	}
	return "", fmt.Errorf("wifi: default gateway not found")
}

// DefaultInterface returns the network interface used by the current default route.
func DefaultInterface() (string, error) {
	out, err := exec.Command("route", "-n", "get", "default").Output()
	if err != nil {
		return "", fmt.Errorf("wifi: route get default: %w", err)
	}
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[0] == "interface:" {
			return fields[1], nil
		}
	}
	return "", fmt.Errorf("wifi: default interface not found")
}

// wifiInterfaces returns the Wi-Fi device names (e.g. en0) from hardware port info.
func wifiInterfaces() ([]string, error) {
	out, err := exec.Command("networksetup", "-listallhardwareports").Output()
	if err != nil {
		return nil, fmt.Errorf("wifi: list hardware ports: %w", err)
	}
	var ifaces []string
	var inWiFi bool
	for _, line := range strings.Split(string(out), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "Hardware Port:") {
			inWiFi = strings.Contains(strings.ToLower(trimmed), "wi-fi")
			continue
		}
		if inWiFi && strings.HasPrefix(trimmed, "Device:") {
			device := strings.TrimSpace(strings.TrimPrefix(trimmed, "Device:"))
			if device != "" {
				ifaces = append(ifaces, device)
			}
			inWiFi = false
		}
	}
	return ifaces, nil
}

func queryInterface(iface string) (string, error) {
	out, err := exec.Command("networksetup", "-getairportnetwork", iface).Output()
	if err != nil {
		return "", fmt.Errorf("wifi: networksetup %s: %w", iface, err)
	}
	line := strings.TrimSpace(string(out))
	const prefix = "Current Wi-Fi Network: "
	if !strings.HasPrefix(line, prefix) {
		return "", nil
	}
	return strings.TrimPrefix(line, prefix), nil
}

// ssidFromSystemProfiler extracts the SSID of the connected Wi-Fi network using
// system_profiler. This is a fallback on modern macOS where networksetup is unreliable,
// but system_profiler may privacy-redact the SSID to "<redacted>".
func ssidFromSystemProfiler() (string, error) {
	out, err := exec.Command("system_profiler", "SPAirPortDataType", "-json").Output()
	if err != nil {
		return "", fmt.Errorf("wifi: system_profiler: %w", err)
	}
	var parsed map[string]interface{}
	if err := json.Unmarshal(out, &parsed); err != nil {
		return "", fmt.Errorf("wifi: parse system_profiler: %w", err)
	}
	root, ok := parsed["SPAirPortDataType"].([]interface{})
	if !ok || len(root) == 0 {
		return "", fmt.Errorf("wifi: no SPAirPortDataType root")
	}
	first, ok := root[0].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("wifi: malformed SPAirPortDataType root")
	}
	interfaces, ok := first["spairport_airport_interfaces"].([]interface{})
	if !ok {
		return "", fmt.Errorf("wifi: no airport interfaces")
	}
	for _, iface := range interfaces {
		im, ok := iface.(map[string]interface{})
		if !ok {
			continue
		}
		status, _ := im["spairport_status_information"].(string)
		if status != "spairport_status_connected" {
			continue
		}
		info, ok := im["spairport_current_network_information"].(map[string]interface{})
		if !ok {
			continue
		}
		name, _ := info["_name"].(string)
		if name != "" && name != "<redacted>" {
			return name, nil
		}
	}
	return "", fmt.Errorf("wifi: no connected network")
}
