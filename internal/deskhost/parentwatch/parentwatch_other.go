//go:build !linux && !darwin

package parentwatch

func watch(parentPID int) error {
	// No-op: Windows orphan prevention is handled via Job Objects on the Electron parent side
	// (architecture-desktop-host.md FR-9). Other unsupported platforms do not monitor parent.
	return nil
}
