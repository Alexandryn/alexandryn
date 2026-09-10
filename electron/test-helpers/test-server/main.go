// Test helper: a minimal Go HTTP server that mimics the Go server binary's
// startup contract (architecture-desktop-host.md API and contracts):
//   1. Binds a port (OS-assigned unless --port is given)
//   2. Prints "PORT=<n>" on stdout
//   3. Serves GET /healthz → 200 {"status":"ok"}
//
// Flags used by integration tests:
//   --exit-before-ready      exit(1) before announcing the port (E10/E13 crash tests)
//   --exit-after-ready       announce port + serve /healthz once, then exit(0) (E13)
//   --slow-shutdown N        ignore SIGTERM for N seconds before exiting (E11 SIGKILL test)
//   --port N                 bind to port N instead of OS-assigned
//   --requests-to-serve N    exit after serving N requests (default: never)

package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

// isIgnorableSyncErr checks if a sync error is due to stdout being connected to a pipe or pseudo-device
// where fsync is not supported by the OS kernel (returns EINVAL or ENOTSUP).
func isIgnorableSyncErr(err error) bool {
	if err == nil {
		return true
	}
	return errors.Is(err, syscall.EINVAL) ||
		errors.Is(err, syscall.ENOTSUP) ||
		errors.Is(err, syscall.EBADF) ||
		errors.Is(err, syscall.ENODEV)
}

func main() {
	exitBeforeReady := flag.Bool("exit-before-ready", false, "exit(1) before announcing the port")
	exitAfterReady := flag.Bool("exit-after-ready", false, "exit after the first /healthz request")
	slowShutdownSec := flag.Int("slow-shutdown", 0, "ignore SIGTERM for N seconds")
	port := flag.Int("port", 0, "bind to this port (0 = OS-assigned)")
	requestsToServe := flag.Int("requests-to-serve", 0, "exit after N requests (0 = never)")
	_ = flag.String("config", "", "path to config file (accepted and ignored — matches the real server's flag)")
	flag.Parse()

	if *exitBeforeReady {
		fmt.Fprintln(os.Stderr, "test-server: exiting before ready (--exit-before-ready)")
		os.Exit(1)
	}

	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", *port))
	if err != nil {
		fmt.Fprintf(os.Stderr, "test-server: listen: %v\n", err)
		os.Exit(1)
	}
	actualPort := ln.Addr().(*net.TCPAddr).Port

	served := 0
	doneCh := make(chan struct{})

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if _, writeErr := w.Write([]byte(`{"status":"ok"}`)); writeErr != nil {
			fmt.Fprintf(os.Stderr, "test-server: write error: %v\n", writeErr)
			return
		}

		served++
		if *requestsToServe > 0 && served >= *requestsToServe {
			go func() { close(doneCh) }()
		}
		if *exitAfterReady {
			go func() { close(doneCh) }()
		}
	})

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if strings.HasPrefix(r.URL.Path, "/book/") {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`<!DOCTYPE html>
<html lang="en">
<head><meta charset="utf-8"><title>Alexandryn</title></head>
<body>
  <div id="main" tabindex="-1">
    <h1>Book</h1>
    <p>Invisible Cities</p>
  </div>
  <script>
    document.getElementById('main')?.focus();
  </script>
</body>
</html>`))
			return
		}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`<!DOCTYPE html>
<html lang="en">
<head><meta charset="utf-8"><title>Alexandryn</title></head>
<body>
  <a href="#main" id="skip-link">Skip to content</a>
  <nav aria-label="Primary"><a href="/library">Library</a></nav>
  <div id="main" tabindex="-1">
    <h1>Library</h1>
    <a href="/book/ol-1" id="book-link">Invisible Cities</a>
  </div>
  <script>
    document.getElementById('skip-link')?.addEventListener('click', (e) => {
      e.preventDefault();
      const main = document.getElementById('main');
      if (main) {
        main.focus();
      }
    });
  </script>
</body>
</html>`))
	})

	srv := &http.Server{

		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	// Announce the port AFTER the listener is bound — matches the real
	// server's contract (architecture-desktop-host.md API and contracts).
	if _, printErr := fmt.Printf("PORT=%d\n", actualPort); printErr != nil {
		fmt.Fprintf(os.Stderr, "test-server: stdout print error: %v\n", printErr)
		os.Exit(1)
	}

	if syncErr := os.Stdout.Sync(); syncErr != nil && !isIgnorableSyncErr(syncErr) {
		fmt.Fprintf(os.Stderr, "test-server: stdout sync error: %v\n", syncErr)
	}

	// Handle shutdown.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)

	go func() {
		if serveErr := srv.Serve(ln); serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			fmt.Fprintf(os.Stderr, "test-server: serve error: %v\n", serveErr)
		}
	}()

	select {
	case <-doneCh:
		// Requested self-exit (--exit-after-ready or --requests-to-serve).
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()
		if shutErr := srv.Shutdown(ctx); shutErr != nil {
			fmt.Fprintf(os.Stderr, "test-server: shutdown error: %v\n", shutErr)
		}
		os.Exit(0)
	case sig := <-sigCh:
		if *slowShutdownSec > 0 {
			fmt.Fprintf(os.Stderr, "test-server: got %v, sleeping %ds before exit (--slow-shutdown)\n", sig, *slowShutdownSec)
			time.Sleep(time.Duration(*slowShutdownSec) * time.Second)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()
		if shutErr := srv.Shutdown(ctx); shutErr != nil {
			fmt.Fprintf(os.Stderr, "test-server: shutdown error: %v\n", shutErr)
		}
		os.Exit(0)
	}
}
