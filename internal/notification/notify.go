package notification

import (
	"fmt"
	"os/exec"
	"sync"
)

// SendWarning sends a macOS notification warning about attendance risk.
func SendWarning(stillNeeded, remaining int) error {
	msg := fmt.Sprintf("Need %d more office days in %d remaining working days", stillNeeded, remaining)
	script := fmt.Sprintf(
		`display notification %q with title "WiFi Attendance ⚠" subtitle "At risk of missing 60%% target"`,
		msg,
	)
	return exec.Command("osascript", "-e", script).Run()
}

// Throttle prevents duplicate notifications within the same IST calendar day.
type Throttle struct {
	notifiedDate string
	mu           sync.Mutex
}

// ShouldNotify returns true if a notification has not yet been sent today.
func (t *Throttle) ShouldNotify(todayIST string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.notifiedDate != todayIST
}

// MarkNotified records that a notification was sent today.
func (t *Throttle) MarkNotified(todayIST string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.notifiedDate = todayIST
}
