//go:build windows

package supervisor_test

import (
	"os/exec"
	"testing"

	"github.com/Alexandryn/alexandryn/internal/persistence/postgres/supervisor"
)

// T25-D1: these assertions run only on real Windows (GOOS=windows) and
// don't need a live spawned process — the struct layout and constant
// value this package's raw kernel32.dll calls depend on, checked
// directly against the documented Win32 shape. This file cannot be
// executed in this Linux authoring environment; only cross-compiled and
// type-checked here (`GOOS=windows go vet ./...`) — first real execution
// happens on Windows CI, not in this session.

// JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE per the Win32 JOBOBJECT_BASIC_LIMIT_INFORMATION
// docs — the flag architecture-persistence.md FR-9 requires be set so the
// spawned process dies when this process's job handle closes.
const wantJobObjectLimitKillOnJobClose = 0x00002000

func TestJobObjectLimitKillOnJobClose_MatchesTheDocumentedWin32Value(t *testing.T) {
	if got := supervisor.JobObjectLimitKillOnJobClose; got != wantJobObjectLimitKillOnJobClose {
		t.Fatalf("JobObjectLimitKillOnJobClose = %#x, want %#x (the documented Win32 constant)", got, wantJobObjectLimitKillOnJobClose)
	}
}

// SpawnWithOrphanPrevention must actually start cmd (even though the Job
// Object assignment that follows is unverified here) — a real process is
// spawned exactly like SpawnWithOrphanPrevention's Linux counterpart
// does, proven by asserting a non-nil PID once Start succeeds, before
// this test's own cleanup kills it. If job-object setup fails on real
// Windows, that failure is what CI's first run of this suite exists to
// catch — not asserted correct here.
func TestSpawnWithOrphanPrevention_StartsTheProcess(t *testing.T) {
	cmd := exec.Command("cmd.exe", "/C", "timeout /T 5")
	t.Cleanup(func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
	})

	if err := supervisor.SpawnWithOrphanPrevention(cmd); err != nil {
		t.Fatalf("SpawnWithOrphanPrevention: %v", err)
	}
	if cmd.Process == nil || cmd.Process.Pid == 0 {
		t.Fatal("SpawnWithOrphanPrevention did not start a real process")
	}
}
