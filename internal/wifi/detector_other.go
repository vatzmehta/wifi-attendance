//go:build !darwin

package wifi

import "errors"

func currentSSIDNative() (string, error) {
	return "", errors.New("wifi: native SSID not available on this OS")
}
