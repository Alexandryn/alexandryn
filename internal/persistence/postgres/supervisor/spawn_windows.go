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

// SpawnWithOrphanPrevention starts cmd, then creates a Windows Job
// Object with JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE and assigns the spawned
// process to it (architecture-persistence.md FR-9) — the parent-side,
// spawn-time mechanism architecture-desktop-host.md FR-9 specifies for
// Electron→Go-server, applied one level down. This requires no changes
// to PostgreSQL's own source, the same "third-party binary, no
// cooperation needed" property SpawnWithOrphanPrevention's Linux
// counterpart has. Unverified in this Linux authoring environment
// (T25-D1) — first real verification happens on Windows CI, not here.
func SpawnWithOrphanPrevention(cmd *exec.Cmd) error {
	if err := cmd.Start(); err != nil {
		return err
	}

	// From here on, cmd.Process is a real, running process. Any failure
	// below means orphan-prevention could not be guaranteed for it — kill
	// it before returning rather than leaving an unprotected, untracked
	// PostgreSQL process running: the caller's own retry logic (T25-D3)
	// assumes a failed spawn leaves nothing behind for it to trip over.
	if err := spawnWithOrphanPrevention(cmd); err != nil {
		_ = cmd.Process.Kill()
		return err
	}
	return nil
}

func spawnWithOrphanPrevention(cmd *exec.Cmd) error {
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

	// Process.WithHandle gives a valid handle to the just-spawned process
	// without a separate OpenProcess call that could race the process
	// already exiting — the same handle os/exec itself used at creation.
	var assignErr error
	if err := cmd.Process.WithHandle(func(processHandle uintptr) {
		ret, _, callErr := procAssignProcessToJobObject.Call(jobHandle, processHandle)
		if ret == 0 {
			assignErr = fmt.Errorf("assigning process to job object: %w", callErr)
		}
	}); err != nil {
		return fmt.Errorf("obtaining a handle to the spawned process: %w", err)
	}
	return assignErr
}
