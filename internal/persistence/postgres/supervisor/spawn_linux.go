//go:build linux

package supervisor

import (
	"os/exec"
	"syscall"
)

// SpawnWithOrphanPrevention starts cmd with SysProcAttr.Pdeathsig set to
// SIGKILL (architecture-persistence.md FR-8): if this process dies for
// any reason — including SIGKILL, which bypasses every graceful-shutdown
// path — the kernel signals cmd to die too, with no cooperation needed
// from cmd's own source. This requires no modification of PostgreSQL's
// own binary, unlike ADR 0005's Linux fix for the Go server itself (which
// added a prctl call inside code Alexandryn controls) — Postgres is
// third-party, its source isn't Alexandryn's to change.
func SpawnWithOrphanPrevention(cmd *exec.Cmd) error {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Pdeathsig = syscall.SIGKILL
	return cmd.Start()
}
