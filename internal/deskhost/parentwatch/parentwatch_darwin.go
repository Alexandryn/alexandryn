//go:build darwin

package parentwatch

import (
	"fmt"
	"os"
	"syscall"
)

func watch(parentPID int) error {
	kq, err := syscall.Kqueue()
	if err != nil {
		return fmt.Errorf("kqueue: %w", err)
	}

	// EVFILT_PROC monitors a process event. NOTE_EXIT fires when the watched PID exits.
	events := []syscall.Kevent_t{
		{
			Ident:  uint64(parentPID),
			Filter: syscall.EVFILT_PROC,
			Flags:  syscall.EV_ADD | syscall.EV_ENABLE | syscall.EV_ONESHOT,
			Fflags: syscall.NOTE_EXIT,
			Data:   0,
			Udata:  nil,
		},
	}

	// Pass nil as eventlist during registration so Kevent does not block waiting for events.
	if _, err := syscall.Kevent(kq, events, nil, nil); err != nil {
		_ = syscall.Close(kq)
		return fmt.Errorf("kevent register: %w", err)
	}

	// Guard against TOCTOU race: check if parent died before or during kevent registration.
	if os.Getppid() != parentPID {
		_ = syscall.Close(kq)
		_ = syscall.Kill(os.Getpid(), syscall.SIGKILL)
		os.Exit(1)
	}

	// Background watcher goroutine: when the parent exits, kevent unblocks.
	go func() {
		defer func() { _ = syscall.Close(kq) }()
		outEvents := make([]syscall.Kevent_t, 1)
		for {
			n, err := syscall.Kevent(kq, nil, outEvents, nil)
			if err != nil {
				// If kevent errors out (e.g. invalid descriptor or process killed), exit.
				os.Exit(1)
				return
			}
			if n > 0 && (outEvents[0].Fflags&syscall.NOTE_EXIT) != 0 {
				_ = syscall.Kill(os.Getpid(), syscall.SIGKILL)
				os.Exit(0)
				return
			}
		}
	}()

	return nil
}
