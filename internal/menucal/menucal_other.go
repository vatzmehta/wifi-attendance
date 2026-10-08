//go:build !darwin

// Package menucal draws a month calendar inside a menu item.
package menucal

// Set is a no-op off macOS; the placeholder menu item keeps its title.
func Set(placeholder, cells string) {}
