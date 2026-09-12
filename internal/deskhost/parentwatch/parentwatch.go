// Package parentwatch provides orphan prevention for the Go server when hosted
// by the Electron desktop application.
package parentwatch

import (
	"fmt"
)

// Watch enables parent process monitoring. If the parent process identified by
// parentPID terminates for any reason, the current process self-terminates.
//
// On Linux, this uses PR_SET_PDEATHSIG.
// On macOS, this uses kqueue EVFILT_PROC / NOTE_EXIT.
// On other platforms (Windows, etc.), this is a no-op (Windows uses parent-side Job Objects).
func Watch(parentPID int) error {
	if parentPID <= 0 {
		return fmt.Errorf("invalid parent PID %d: must be positive", parentPID)
	}
	return watch(parentPID)
}
