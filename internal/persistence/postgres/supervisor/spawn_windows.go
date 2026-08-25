//go:build windows

package supervisor

import (
	"fmt"
	"os/exec"
	"syscall"
	"unsafe"
)

// Raw kernel32.dll syscalls rather than golang.org/x/sys/windows
// (T25-D1, constitution §9) — one direct syscall surface, no dependency
// to justify, matching this project's existing precedent
// (.claude/decisions/0005-process-model-prototype/go-child-pdeathsig/main.go's
// own "one direct syscall, no dependency to justify" stance for Linux).
var (
	modKernel32                  = syscall.NewLazyDLL("kernel32.dll")
	procCreateJobObjectW         = modKernel32.NewProc("CreateJobObjectW")
	procSetInformationJobObject  = modKernel32.NewProc("SetInformationJobObject")
	procAssignProcessToJobObject = modKernel32.NewProc("AssignProcessToJobObject")
)

// JobObjectLimitKillOnJobClose is Win32's JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
// (winnt.h) — when set on a job's basic limit information, the kernel
// kills every process still assigned to the job as soon as the job's
// last open handle closes, including as a side effect of this process
// dying (Windows closes all of a process's open handles on exit,
// unconditionally, the same guarantee Pdeathsig gives on Linux).
const JobObjectLimitKillOnJobClose = 0x00002000

const jobObjectExtendedLimitInformationClass = 9 // JobObjectExtendedLimitInformation, JOBOBJECTINFOCLASS enum

type jobObjectBasicLimitInformation struct {
	PerProcessUserTimeLimit int64
	PerJobUserTimeLimit     int64
	LimitFlags              uint32
	MinimumWorkingSetSize   uintptr
	MaximumWorkingSetSize   uintptr
	ActiveProcessLimit      uint32
	Affinity                uintptr
	PriorityClass           uint32
	SchedulingClass         uint32
}

type ioCounters struct {
	ReadOperationCount  uint64
	WriteOperationCount uint64
	OtherOperationCount uint64
	ReadTransferCount   uint64
	WriteTransferCount  uint64
	OtherTransferCount  uint64
}

type jobObjectExtendedLimitInformation struct {
	BasicLimitInformation jobObjectBasicLimitInformation
	IoInfo                ioCounters
	ProcessMemoryLimit    uintptr
	JobMemoryLimit        uintptr
	PeakProcessMemoryUsed uintptr
	PeakJobMemoryUsed     uintptr
}

// SpawnWithOrphanPrevention creates a Windows Job Object with
// JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE, *then* starts cmd and assigns it
// to that job (architecture-persistence.md FR-9) — the parent-side,
// spawn-time mechanism architecture-desktop-host.md FR-9 specifies for
// Electron→Go-server, applied one level down. This requires no changes
// to PostgreSQL's own source, the same "third-party binary, no
// cooperation needed" property SpawnWithOrphanPrevention's Linux
// counterpart has.
//
// Residual race, not fully closed (found by a post-commit security
// review of this task and left honestly documented rather than silently
// narrowed and forgotten): between cmd.Start() returning and
// AssignProcessToJobObject completing, the process is running but not
// yet protected — if this process dies in that exact window, PostgreSQL
// would orphan anyway, precisely what FR-9 exists to prevent. Doing job
// creation and limit configuration *before* Start() (this function's
// actual shape) shrinks that window to one syscall instead of three, but
// doesn't eliminate it. A fully race-free fix needs CREATE_SUSPENDED —
// start the process suspended, assign it to the job while no code in it
// can run yet, then resume its main thread — but Go's os/exec closes the
// new process's thread handle immediately after CreateProcess returns
// and never exposes it to the caller (syscall/exec_windows.go), so a
// suspended process started through the public os/exec API can never be
// resumed. Implementing that properly means bypassing exec.Cmd entirely
// and hand-rolling syscall.CreateProcess directly — reimplementing
// argument quoting, environment-block construction, and STARTUPINFO
// stdio wiring that os/exec normally handles internally. Not attempted
// here: that much new, low-level Windows-specific code, written with no
// ability to execute or test any of it in this Linux authoring
// environment, is a worse risk trade than this smaller, well-understood
// residual window — revisit once real Windows CI feedback exists to
// develop and verify it against (T25-D1).
func SpawnWithOrphanPrevention(cmd *exec.Cmd) error {
	jobHandle, _, callErr := procCreateJobObjectW.Call(0, 0)
	if jobHandle == 0 {
		return fmt.Errorf("creating job object: %w", callErr)
	}
	// jobHandle is deliberately never closed here: this job object must
	// outlive this function call, for as long as cmd/server itself runs.
	// Windows closes every open handle when a process exits, unconditionally
	// — that implicit close-on-exit is the mechanism JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	// relies on to kill the spawned process when cmd/server dies for any
	// reason, so closing this handle early would defeat the whole point.

	info := jobObjectExtendedLimitInformation{
		BasicLimitInformation: jobObjectBasicLimitInformation{
			LimitFlags: JobObjectLimitKillOnJobClose,
		},
	}
	ret, _, callErr := procSetInformationJobObject.Call(
		jobHandle,
		uintptr(jobObjectExtendedLimitInformationClass),
		uintptr(unsafe.Pointer(&info)),
		unsafe.Sizeof(info),
	)
	if ret == 0 {
		return fmt.Errorf("setting job object limits: %w", callErr)
	}

	if err := cmd.Start(); err != nil {
		return err
	}

	// Process.WithHandle gives a valid handle to the just-spawned process
	// without a separate OpenProcess call that could race the process
	// already exiting — the same handle os/exec itself used at creation.
	// Any failure from here on means orphan-prevention could not be
	// guaranteed for a process that is now genuinely running — kill it
	// before returning rather than leaving an unprotected, untracked
	// PostgreSQL process behind: the caller's own retry logic (T25-D3)
	// assumes a failed spawn leaves nothing running for it to trip over.
	var assignErr error
	if err := cmd.Process.WithHandle(func(processHandle uintptr) {
		ret, _, callErr := procAssignProcessToJobObject.Call(jobHandle, processHandle)
		if ret == 0 {
			assignErr = fmt.Errorf("assigning process to job object: %w", callErr)
		}
	}); err != nil {
		_ = cmd.Process.Kill()
		return fmt.Errorf("obtaining a handle to the spawned process: %w", err)
	}
	if assignErr != nil {
		_ = cmd.Process.Kill()
		return assignErr
	}
	return nil
}
