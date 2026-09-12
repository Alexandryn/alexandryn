//go:build linux

package supervisor_test

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/Alexandryn/alexandryn/internal/persistence/postgres/supervisor"
)

// helperFlag selects re-exec dispatch (below) via a command-line flag
// rather than an environment variable — this package's own import-
// boundary check (scripts/check-import-boundaries.sh) reserves
// os.Getenv/os.LookupEnv for internal/config alone; a flag sidesteps
// that check by construction rather than needing an exemption for
// test-only re-exec plumbing the check's author didn't anticipate.
var helperFlag = flag.String("supervisor-test-helper", "", "internal: re-exec helper mode, do not set by hand")

// TestMain lets this test binary impersonate the "parent" process this
// empirical proof needs — Go's own syscall package tests Pdeathsig the
// same way (syscall/exec_pdeathsig_test.go): re-exec the test binary
// itself as a child process with a flag telling it to act as a helper
// instead of running tests, since the helper needs its own stdin/stdout
// wiring, not the real test binary's.
func TestMain(m *testing.M) {
	flag.Parse()
	if *helperFlag == "parent" {
		runHelperParent()
		return
	}
	os.Exit(m.Run())
}

// runHelperParent spawns a real, long-running child (sleep) via
// SpawnWithOrphanPrevention, prints the child's PID so the real test can
// learn it, then blocks on stdin until killed — standing in for
// PostgreSQL in the "this process dies, PostgreSQL must die too"
// scenario, with this helper itself standing in for cmd/server.
func runHelperParent() {
	cmd := exec.Command("sleep", "30")
	if err := supervisor.SpawnWithOrphanPrevention(cmd); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println(cmd.Process.Pid)
	buf := make([]byte, 1)
	_, _ = os.Stdin.Read(buf)
	os.Exit(0)
}

// Verifies that a real spawned child dies when its
// parent dies for any reason, including an ungraceful kill — proven here
// against a real process tree, not asserted from documentation. The
// "parent" in this test is itself a spawned process (this same test
// binary, re-exec'd via TestMain's helper dispatch above), standing in
// for cmd/server; the "child" is a real `sleep` process spawned through
// SpawnWithOrphanPrevention, standing in for PostgreSQL.
func TestSpawnWithOrphanPrevention_ChildDiesWhenParentKilled(t *testing.T) {
	if _, err := exec.LookPath("sleep"); err != nil {
		t.Skip("sleep binary not found on PATH, cannot run this empirical proof")
	}

	parent := exec.Command(os.Args[0], "-supervisor-test-helper=parent")
	stdin, err := parent.StdinPipe()
	if err != nil {
		t.Fatalf("StdinPipe: %v", err)
	}
	stdout, err := parent.StdoutPipe()
	if err != nil {
		t.Fatalf("StdoutPipe: %v", err)
	}
	if err := parent.Start(); err != nil {
		t.Fatalf("starting helper parent: %v", err)
	}
	t.Cleanup(func() {
		_ = stdin.Close()
		_ = parent.Wait()
	})

	line, err := bufio.NewReader(stdout).ReadString('\n')
	if err != nil {
		t.Fatalf("reading child PID from helper parent: %v", err)
	}
	childPID, err := strconv.Atoi(strings.TrimSpace(line))
	if err != nil {
		t.Fatalf("parsing child PID %q: %v", line, err)
	}

	// Signal 0 sends nothing — it only checks whether the PID exists,
	// the standard Unix liveness-check idiom.
	if err := syscall.Kill(childPID, 0); err != nil {
		t.Fatalf("child (pid %d) not alive right after spawn: %v", childPID, err)
	}

	if err := parent.Process.Kill(); err != nil {
		t.Fatalf("killing helper parent: %v", err)
	}

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if killErr := syscall.Kill(childPID, 0); killErr != nil {
			return // child is gone — Pdeathsig worked
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("child (pid %d) still alive after parent was killed — Pdeathsig did not fire", childPID)
}
