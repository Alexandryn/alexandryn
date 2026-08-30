//go:build linux

package parentwatch

import (
	"fmt"
	"os"
	"syscall"
)

const prSetPdeathsig = 1 // PR_SET_PDEATHSIG from linux/prctl.h

func watch(parentPID int) error {
	// PR_SET_PDEATHSIG instructs the Linux kernel to send SIGKILL to this process
	// whenever its parent dies (ADR 0005).
	_, _, errno := syscall.Syscall(syscall.SYS_PRCTL, uintptr(prSetPdeathsig), uintptr(syscall.SIGKILL), 0)
	if errno != 0 {
		return fmt.Errorf("setting PR_SET_PDEATHSIG: %w", errno)
	}

	// Guard against TOCTOU race: if the parent died before PR_SET_PDEATHSIG was registered,
	// getppid() will no longer match parentPID (it will be 1 or another init subreaper).
	if os.Getppid() != parentPID {
		// Parent is already dead — self-terminate immediately.
		_ = syscall.Kill(os.Getpid(), syscall.SIGKILL)
		os.Exit(1)
	}

	return nil
}

