// Prototype only — fix candidate for FR-10 (Go server must exit if the
// Electron/Node parent that spawned it dies, including via SIGKILL, which
// bypasses every graceful-shutdown path the parent might otherwise run).
//
// Linux-specific: prctl(PR_SET_PDEATHSIG) asks the kernel to signal this
// process when its parent dies, for ANY reason including SIGKILL of the
// parent. Uses only the standard library (syscall.Syscall + SYS_PRCTL),
// deliberately avoiding golang.org/x/sys/unix per constitution §9 — one
// direct syscall, no dependency to justify.
//
// macOS has no prctl equivalent; the analogous fix there is watching the
// parent PID via kqueue's EVFILT_PROC/NOTE_EXIT. Windows has no signal-based
// equivalent either; the analogous fix is the parent (Electron/Node)
// assigning the child to a Job Object with
// JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE, which kills the child automatically
// when the job handle closes — including on parent crash. Neither was
// implemented or tested here; only the Linux mechanism was empirically
// verified in this prototype. See the prototype README for what that means
// for FR-10.
package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

const (
	prSetPdeathsig = 1 // PR_SET_PDEATHSIG, from linux/prctl.h
)

func setParentDeathSignal(sig syscall.Signal) error {
	_, _, errno := syscall.Syscall(syscall.SYS_PRCTL, uintptr(prSetPdeathsig), uintptr(sig), 0)
	if errno != 0 {
		return errno
	}
	return nil
}

func main() {
	if err := setParentDeathSignal(syscall.SIGTERM); err != nil {
		log.Fatalf("prctl(PR_SET_PDEATHSIG): %v", err)
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		log.Fatalf("listen: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	fmt.Printf("PORT=%d\n", port)
	os.Stdout.Sync()

	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})
	srv := &http.Server{Handler: mux}

	go func() {
		if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
			log.Printf("serve error: %v", err)
		}
	}()

	log.Printf("pdeathsig child pid=%d ppid=%d listening on %d", os.Getpid(), os.Getppid(), port)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	sig := <-sigCh
	log.Printf("received %v (parent-death or normal shutdown), shutting down", sig)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	srv.Shutdown(ctx)
	log.Printf("shutdown complete")
}
