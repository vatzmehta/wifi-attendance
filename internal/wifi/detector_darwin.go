//go:build darwin

package wifi

/*
#cgo LDFLAGS: -framework CoreWLAN
#include <stdlib.h>
char *currentSSID();
*/
import "C"
import (
	"errors"
	"unsafe"
)

func currentSSIDNative() (string, error) {
	cSSID := C.currentSSID()
	if cSSID == nil {
		return "", errors.New("wifi: no native SSID")
	}
	defer C.free(unsafe.Pointer(cSSID))
	return C.GoString(cSSID), nil
}
