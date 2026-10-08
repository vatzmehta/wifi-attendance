//go:build darwin

// Package menucal draws a month calendar inside a menu item.
package menucal

/*
#cgo CFLAGS: -fobjc-arc
#cgo LDFLAGS: -framework Cocoa
#include <stdlib.h>
void setMenuCalendar(const char *placeholder, const char *cells);
*/
import "C"
import "unsafe"

// Set replaces the menu item titled placeholder with a calendar drawn from cells:
// one character per grid cell, Monday first — '.' for padding outside the month,
// 'p' present, 'a' absent, 'h' holiday, 'l' leave, 'w' weekend, 'n' nothing to show.
func Set(placeholder, cells string) {
	cPlaceholder := C.CString(placeholder)
	defer C.free(unsafe.Pointer(cPlaceholder))
	cCells := C.CString(cells)
	defer C.free(unsafe.Pointer(cCells))
	C.setMenuCalendar(cPlaceholder, cCells)
}
