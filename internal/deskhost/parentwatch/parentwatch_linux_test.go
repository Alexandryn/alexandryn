//go:build linux

package parentwatch_test

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

	"github.com/Alexandryn/alexandryn/internal/deskhost/parentwatch"
)

var helperMode = flag.String("parentwatch-helper", "", "internal: re-exec helper mode")

func TestMain(m *testing.M) {
	flag.Parse()
	switch *helperMode {
	case "parent_spawner":
		runHelperParentSpawner()
		return
	case "child":
		runHelperChild()
		return
	default:
		os.Exit(m.Run())
	}
}

func runHelperChild() {
	ppid := os.Getppid()
	if err := parentwatch.Watch(ppid); err != nil {
		fmt.Fprintf(os.Stderr, "child: watch error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(os.Getpid())
	if err := os.Stdout.Sync(); err != nil && !isIgnorableSyncErr(err) {
		fmt.Fprintf(os.Stderr, "child: stdout sync error: %v\n", err)
	}
	// Block indefinitely until kernel kills us via PR_SET_PDEATHSIG
	select {}
}

func isIgnorableSyncErr(err error) bool {
	if err == nil {
		return true
	}
	return err == syscall.EINVAL || err == syscall.ENOTSUP || err == syscall.EBADF || err == syscall.ENODEV
}

func runHelperParentSpawner() {
	childCmd := exec.Command(os.Args[0], "-parentwatch-helper=child")
	stdout, err := childCmd.StdoutPipe()
	if err != nil {
		fmt.Fprintf(os.Stderr, "parent_spawner: stdout pipe: %v\n", err)
		os.Exit(1)
	}
	if err := childCmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "parent_spawner: start child: %v\n", err)
		os.Exit(1)
	}
	defer func() {
		_ = childCmd.Process.Kill()
	}()

	reader := bufio.NewReader(stdout)
	line, err := reader.ReadString('\n')
	if err != nil {
		fmt.Fprintf(os.Stderr, "parent_spawner: read child PID: %v\n", err)
		os.Exit(1)
	}

	// Forward child PID to test runner
	fmt.Print(line)
	if err := os.Stdout.Sync(); err != nil && !isIgnorableSyncErr(err) {
		fmt.Fprintf(os.Stderr, "parent_spawner: stdout sync error: %v\n", err)
	}

	// Block until parent_spawner itself is killed
	select {}
}

// architecture-desktop-host.md FR-8 / desktop-host-process-model.md FR-6 / ADR 0005:
// Killing the parent process (Electron desktop host) kills the Go server child
// process on Linux via PR_SET_PDEATHSIG.
func TestParentwatch_Linux_ChildDiesWhenParentKilled(t *testing.T) {
	parentCmd := exec.Command(os.Args[0], "-parentwatch-helper=parent_spawner")
	stdout, err := parentCmd.StdoutPipe()
	if err != nil {
		t.Fatalf("StdoutPipe: %v", err)
	}
	if err := parentCmd.Start(); err != nil {
		t.Fatalf("starting parent spawner: %v", err)
	}
	t.Cleanup(func() {
		if parentCmd.Process != nil {
			_ = parentCmd.Process.Kill()
			_ = parentCmd.Wait()
		}
	})

	line, err := bufio.NewReader(stdout).ReadString('\n')
	if err != nil {
		t.Fatalf("reading child PID from parent spawner: %v", err)
	}
	childPID, err := strconv.Atoi(strings.TrimSpace(line))
	if err != nil {
		t.Fatalf("parsing child PID %q: %v", line, err)
	}

	// Verify child process is alive right after spawn
	if err := syscall.Kill(childPID, 0); err != nil {
		t.Fatalf("child (PID %d) not alive after spawn: %v", childPID, err)
	}

	// Abruptly kill the parent spawner process with SIGKILL (bypasses any graceful cleanup)
	if err := parentCmd.Process.Kill(); err != nil {
		t.Fatalf("killing parent spawner: %v", err)
	}

	// Poll child PID liveness: kernel should deliver SIGKILL to child via PR_SET_PDEATHSIG
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if killErr := syscall.Kill(childPID, 0); killErr != nil {
			return // Child process is dead — PR_SET_PDEATHSIG successfully fired!
		}
		time.Sleep(20 * time.Millisecond)
	}

	// Cleanup if still running
	_ = syscall.Kill(childPID, syscall.SIGKILL)
	t.Fatalf("child (PID %d) remained alive after parent was killed — PR_SET_PDEATHSIG did not fire", childPID)
}
